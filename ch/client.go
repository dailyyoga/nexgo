package ch

import (
	"context"
	"sync"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/dailyyoga/nexgo/logger"
	"go.uber.org/zap"
)

// defaultClient is the default implementation of the Client interface
type defaultClient struct {
	config *Config
	logger logger.Logger

	// clickhouse connection (shared by Writer and Query)
	conn driver.Conn

	// writer instance (lazy initialization)
	writer     Writer
	writerOnce sync.Once

	// control
	closed bool
	mu     sync.RWMutex
}

// NewClient creates a new ClickHouse client with unified connection management
func NewClient(config *Config, log logger.Logger) (Client, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	ctx := context.Background()

	// connect to clickhouse server
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: config.Hosts,
		Auth: clickhouse.Auth{
			Database: config.Database,
			Username: config.Username,
			Password: config.Password,
		},
		DialTimeout: config.DialTimeout,
		Debug:       config.Debug,
		Settings:    config.Settings,
	})
	if err != nil {
		return nil, ErrConnection(err)
	}

	// test connection
	if err := conn.Ping(ctx); err != nil {
		conn.Close()
		return nil, ErrConnection(err)
	}

	client := &defaultClient{
		config: config,
		logger: log,
		conn:   conn,
	}

	log.Info("clickhouse client initialized",
		zap.Strings("hosts", config.Hosts),
		zap.String("database", config.Database),
	)

	return client, nil
}

// Writer returns the Writer interface for batch writing
// The writer is lazily initialized and cached
// Note: Caller must call writer.Start() to begin processing
// Returns ErrWriterDisabled if WriterConfig is not set
func (c *defaultClient) Writer() (Writer, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, ErrWriterClosed
	}
	c.mu.RUnlock()

	// check if writer is enabled
	if c.config.WriterConfig == nil {
		return nil, ErrWriterDisabled
	}

	c.writerOnce.Do(func() {
		c.writer = newWriterWithConn(c.conn, c.config, c.logger)
	})

	return c.writer, nil
}

// Query executes a ClickHouse query and returns driver.Rows
func (c *defaultClient) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, ErrConnectionClosed
	}

	rows, err := c.conn.Query(ctx, query, args...)
	if err != nil {
		c.logger.Error("query failed",
			zap.String("query", query),
			zap.Error(err),
		)
		return nil, err
	}

	return rows, nil
}

// QueryRow executes a query that is expected to return at most one row
func (c *defaultClient) QueryRow(ctx context.Context, query string, args ...any) driver.Row {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		c.logger.Error("connection is closed", zap.String("query", query))
		return nil
	}

	return c.conn.QueryRow(ctx, query, args...)
}

// Close closes the client and all associated resources
func (c *defaultClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.logger.Info("clickhouse client shutting down")

	// close writer if it was initialized
	if c.writer != nil {
		if err := c.writer.Close(); err != nil {
			c.logger.Error("failed to close writer", zap.Error(err))
		}
	}

	// close connection
	if err := c.conn.Close(); err != nil {
		c.logger.Error("failed to close clickhouse connection", zap.Error(err))
		c.closed = true
		return err
	}

	c.closed = true
	c.logger.Info("clickhouse client shutdown complete")
	return nil
}
