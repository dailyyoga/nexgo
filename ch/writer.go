package ch

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/dailyyoga/nexgo/logger"
	"github.com/smallnest/chanx"
	"go.uber.org/zap"
)

type defaultWriter struct {
	config *Config
	logger logger.Logger

	// clickhouse connection
	conn driver.Conn

	tableColumns map[TableName][]TableColumn
	columnsMu    sync.RWMutex

	// channel-based batch insert
	dataChan    *chanx.UnboundedChan[Table]
	flushTicker *time.Ticker

	// schema refresh
	schemaRefreshTicker *time.Ticker

	// control
	done   chan struct{}
	wg     sync.WaitGroup
	closed atomic.Bool
}

// newWriterWithConn creates a writer with an existing connection (used by Client)
func newWriterWithConn(conn driver.Conn, config *Config, log logger.Logger) Writer {
	ctx := context.Background()

	// set default writer config if not set
	if config.WriterConfig == nil {
		config.WriterConfig = DefaultWriterConfig()
	}

	// initialize data channel
	dataChan := chanx.NewUnboundedChan[Table](ctx, config.WriterConfig.FlushSize)

	writer := &defaultWriter{
		config:       config,
		logger:       log,
		conn:         conn,
		tableColumns: make(map[TableName][]TableColumn),
		dataChan:     dataChan,
		flushTicker:  time.NewTicker(config.WriterConfig.FlushInterval),
		done:         make(chan struct{}),
	}

	// initialize schema refresh ticker (if enabled)
	if config.WriterConfig.SchemaRefreshInterval > 0 {
		writer.schemaRefreshTicker = time.NewTicker(config.WriterConfig.SchemaRefreshInterval)
	}

	log.Info("clickhouse writer initialized",
		zap.Duration("flush_interval", config.WriterConfig.FlushInterval),
		zap.Int("flush_size", config.WriterConfig.FlushSize),
		zap.Int("min_flush_size", config.WriterConfig.MinFlushSize),
		zap.Duration("max_wait_time", config.WriterConfig.MaxWaitTime),
		zap.Duration("schema_refresh_interval", config.WriterConfig.SchemaRefreshInterval),
	)

	return writer
}

func (w *defaultWriter) Start() error {
	w.wg.Add(1)
	go w.processLoop()

	// start schema refresh goroutine (if enabled)
	if w.schemaRefreshTicker != nil {
		w.wg.Add(1)
		go w.schemaRefreshLoop()
	}

	w.logger.Info("clickhouse writer started")
	return nil
}

func (w *defaultWriter) Write(ctx context.Context, rows []Table) error {
	if len(rows) == 0 {
		return nil
	}

	if w.closed.Load() {
		return ErrWriterClosed
	}

	for _, row := range rows {
		select {
		case w.dataChan.In <- row:
			// success
			continue
		case <-ctx.Done():
			return ctx.Err()
		default:
			w.logger.Error("channel is full, data may be lost",
				zap.Int("channel_size", w.dataChan.Len()),
				zap.Int("rows", len(rows)),
			)
			return ErrBufferFull
		}
	}
	return nil
}

func (w *defaultWriter) Close() error {
	if !w.closed.CompareAndSwap(false, true) {
		return nil
	}

	w.logger.Info("clickhouse writer shutting down")

	w.flushTicker.Stop()
	if w.schemaRefreshTicker != nil {
		w.schemaRefreshTicker.Stop()
	}
	close(w.done)
	close(w.dataChan.In)
	w.wg.Wait()

	w.logger.Info("clickhouse writer shutdown complete")
	return nil
}

func (w *defaultWriter) processLoop() {
	defer w.wg.Done()

	// local buffer
	buffer := make(map[TableName][]Table)
	totalRows := 0
	var firstDataTime time.Time // track when first data arrived in current batch

	for {
		select {
		case row, ok := <-w.dataChan.Out:
			if !ok {
				// channel closed unexpectedly
				w.logger.Warn("data channel closed unexpectedly")
				return
			}
			if row == nil {
				// skip nil values
				continue
			}
			// record first data time for MaxWaitTime calculation
			if totalRows == 0 {
				firstDataTime = time.Now()
			}
			// store data to buffer and capacity trigger flush
			tableName := row.TableName()
			buffer[tableName] = append(buffer[tableName], row)
			totalRows++

			if totalRows >= w.config.WriterConfig.FlushSize {
				w.flush(buffer)
				buffer = make(map[TableName][]Table)
				totalRows = 0
				firstDataTime = time.Time{}
			}

		case <-w.flushTicker.C:
			// timer flush with MinFlushSize and MaxWaitTime strategy
			if totalRows > 0 {
				shouldFlush := w.shouldFlush(totalRows, firstDataTime)
				if shouldFlush {
					w.flush(buffer)
					buffer = make(map[TableName][]Table)
					totalRows = 0
					firstDataTime = time.Time{}
				} else {
					w.logger.Debug("skipping flush, waiting for more data",
						zap.Int("current_rows", totalRows),
						zap.Int("min_flush_size", w.config.WriterConfig.MinFlushSize),
						zap.Duration("waited", time.Since(firstDataTime)),
						zap.Duration("max_wait_time", w.config.WriterConfig.MaxWaitTime),
					)
				}
			}

		case <-w.done:
			// shutdown flush
			w.logger.Info("process loop stopping, draining remaining data",
				zap.Int("buffered_rows", totalRows),
				zap.Int("pending_requests", w.dataChan.Len()),
			)

			w.drainChannel(buffer, &totalRows)
			if totalRows > 0 {
				w.flush(buffer)
			}

			w.logger.Info("process loop stopped")
			return
		}
	}
}

// shouldFlush determines whether to flush based on MinFlushSize and MaxWaitTime strategy
func (w *defaultWriter) shouldFlush(totalRows int, firstDataTime time.Time) bool {
	minFlushSize := w.config.WriterConfig.MinFlushSize
	maxWaitTime := w.config.WriterConfig.MaxWaitTime

	// if MinFlushSize is 0 or not set, always flush (backward compatible)
	if minFlushSize == 0 {
		return true
	}

	// flush if we have enough data
	if totalRows >= minFlushSize {
		return true
	}

	// flush if MaxWaitTime is set and exceeded (ensure data freshness)
	if maxWaitTime > 0 && time.Since(firstDataTime) >= maxWaitTime {
		w.logger.Debug("max wait time exceeded, forcing flush",
			zap.Int("current_rows", totalRows),
			zap.Duration("waited", time.Since(firstDataTime)),
		)
		return true
	}

	return false
}

// drainChannel process all remaining requests in channel
func (w *defaultWriter) drainChannel(buffer map[TableName][]Table, totalRows *int) {
	for {
		select {
		case row, ok := <-w.dataChan.Out:
			if !ok {
				// channel closed
				return
			}
			if row == nil {
				// skip nil values
				continue
			}
			tableName := row.TableName()
			buffer[tableName] = append(buffer[tableName], row)
			*totalRows++
		default:
			return
		}
	}
}

// flush process all data in buffer, send to clickhouse
func (w *defaultWriter) flush(buffer map[TableName][]Table) {
	successRows := 0
	failedRows := 0
	totalRows := 0

	// flush each table data
	for table, rows := range buffer {
		totalRows += len(rows)

		if err := w.batchInsert(context.Background(), table, rows); err != nil {
			w.logger.Error("failed to batch insert", zap.Error(err))
			failedRows += len(rows)
			// @TODO: retry logic or fallback strategy, etc.
		} else {
			successRows += len(rows)
		}
	}

	w.logger.Info("flush completed",
		zap.Int("total_rows", totalRows),
		zap.Int("success_rows", successRows),
		zap.Int("failed_rows", failedRows),
	)
}

// batchInsert batch insert data to clickhouse
func (w *defaultWriter) batchInsert(ctx context.Context, table TableName, rows []Table) error {
	if len(rows) == 0 {
		return nil
	}

	// get table columns
	columns, err := w.getTableColumns(ctx, table)
	if err != nil {
		return ErrInsert(table, err)
	}

	// prepare batch insert
	query := fmt.Sprintf("INSERT INTO `%s`", table)
	batch, err := w.conn.PrepareBatch(ctx, query)
	if err != nil {
		return ErrInsert(table, err)
	}

	// append data to batch
	for _, row := range rows {
		valueMap := row.ToValueMap()

		values := make([]any, len(columns))
		for i := range columns {
			values[i] = w.getColumnValue(valueMap, &columns[i])
		}
		if err := batch.Append(values...); err != nil {
			return ErrInsert(table, err)
		}
	}

	// send batch
	if err := batch.Send(); err != nil {
		return ErrInsert(table, err)
	}

	return nil
}

// fetchTableColumns fetch table columns from clickhouse (not using cache)
func (w *defaultWriter) fetchTableColumns(ctx context.Context, table TableName) ([]TableColumn, error) {
	query := fmt.Sprintf("DESCRIBE TABLE `%s`", table)
	rows, err := w.conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to describe table %s: %w", table, err)
	}
	defer rows.Close()

	var columns []TableColumn
	for rows.Next() {
		var name, typeStr, defaultType, defaultExpr, comment, codecExpr, ttlExpr string
		if err := rows.Scan(
			&name,
			&typeStr,
			&defaultType,
			&defaultExpr,
			&comment,
			&codecExpr,
			&ttlExpr,
		); err != nil {
			return nil, err
		}

		// skip non-insertable columns (MATERIALIZED, ALIAS, EPHEMERAL)
		if defaultType == "MATERIALIZED" || defaultType == "ALIAS" || defaultType == "EPHEMERAL" {
			w.logger.Debug("skipping non-insertable column",
				zap.String("table", string(table)),
				zap.String("column", name),
				zap.String("default_type", defaultType),
			)
			continue
		}

		col := parseColumnType(name, typeStr, defaultExpr)
		columns = append(columns, col)
	}

	return columns, nil
}

// getTableColumns get table columns from cache or clickhouse
func (w *defaultWriter) getTableColumns(ctx context.Context, table TableName) ([]TableColumn, error) {
	// try to get from cache
	w.columnsMu.RLock()
	if columns, exists := w.tableColumns[table]; exists {
		w.columnsMu.RUnlock()
		return columns, nil
	}
	w.columnsMu.RUnlock()

	// cache not found, get from clickhouse
	w.columnsMu.Lock()
	defer w.columnsMu.Unlock()

	// double check
	if columns, exists := w.tableColumns[table]; exists {
		return columns, nil
	}

	// fetch from clickhouse
	columns, err := w.fetchTableColumns(ctx, table)
	if err != nil {
		return nil, err
	}

	w.tableColumns[table] = columns
	w.logger.Debug("table schema cached",
		zap.String("table", string(table)),
		zap.Int("columns", len(columns)),
	)
	return columns, nil
}

// getColumnValue get column value from value map
func (w *defaultWriter) getColumnValue(valueMap map[string]any, col *TableColumn) any {
	val, exists := valueMap[col.Name]
	if !exists || val == nil {
		// using default value
		if col.DefaultValue != nil {
			// Check if default value is a function that needs to be evaluated
			if fn, ok := col.DefaultValue.(*DefaultFunc); ok {
				return fn.Evaluate()
			}
			return col.DefaultValue
		}
		return getZeroValue(col)
	}

	// convert value
	return w.convertValue(val, col)
}

// convertValue convert value to the correct type
func (w *defaultWriter) convertValue(val any, col *TableColumn) any {
	converter := getConverter(col)
	converted, err := converter.Convert(val, w.logger)
	if err != nil {
		w.logger.Error("failed to convert value",
			zap.String("column", col.Name),
			zap.String("type", col.OriginalType),
			zap.Error(err),
		)
		return getZeroValue(col)
	}
	return converted
}

// schemaRefreshLoop periodically refresh all cached table schemas
func (w *defaultWriter) schemaRefreshLoop() {
	defer w.wg.Done()

	for {
		select {
		case <-w.schemaRefreshTicker.C:
			w.refreshAllTableSchemas()
		case <-w.done:
			w.logger.Info("schema refresh loop stopped")
			return
		}
	}
}

// refreshAllTableSchemas refresh all cached table schemas
func (w *defaultWriter) refreshAllTableSchemas() {
	w.columnsMu.RLock()
	tables := make([]TableName, 0, len(w.tableColumns))
	for table := range w.tableColumns {
		tables = append(tables, table)
	}
	w.columnsMu.RUnlock()

	if len(tables) == 0 {
		return
	}

	w.logger.Debug("refreshing table schemas",
		zap.Int("table_count", len(tables)),
	)

	successCount := 0
	for _, table := range tables {
		if err := w.RefreshTableSchema(context.Background(), table); err != nil {
			w.logger.Error("failed to refresh table schema",
				zap.String("table", string(table)),
				zap.Error(err),
			)
		} else {
			successCount++
		}
	}

	w.logger.Info("table schemas refreshed",
		zap.Int("total", len(tables)),
		zap.Int("success", successCount),
		zap.Int("failed", len(tables)-successCount),
	)
}

// RefreshTableSchema refresh specified table schema (force from clickhouse)
func (w *defaultWriter) RefreshTableSchema(ctx context.Context, table TableName) error {
	// fetch from clickhouse
	columns, err := w.fetchTableColumns(ctx, table)
	if err != nil {
		return err
	}

	// update cache
	w.columnsMu.Lock()
	oldColumns := w.tableColumns[table]
	w.tableColumns[table] = columns
	w.columnsMu.Unlock()

	w.logger.Debug("table schema refreshed",
		zap.String("table", string(table)),
		zap.Int("old_columns", len(oldColumns)),
		zap.Int("new_columns", len(columns)),
	)

	return nil
}
