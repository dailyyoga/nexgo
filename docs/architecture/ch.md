# ch (ClickHouse Client)

Read this when working on the `ch` client/writer — async batch writes, type conversion, schema discovery/caching, or the flush strategy. Referenced from the Reading Index in `CLAUDE.md`.

**Core Pattern**: Unified client with shared connection for both query and async batch write operations.

**Key Components**:
- `Client` interface - Unified entry point providing `Writer()`, `Query()`, `QueryRow()`, and `Close()`
- `Writer` interface - Async batch writer with `Start()`, `Write()`, `Close()`, `RefreshTableSchema()`
- `Table` interface - Data models must implement `TableName()` and `ToValueMap()`
- `Converter` system - Automatic type conversion between Go types and ClickHouse types

**Architecture Details**:
- `defaultClient` manages a single `driver.Conn` shared by queries and writes (thread-safe)
- `Writer` uses unbounded channels (`chanx`) for non-blocking writes
- Schema discovery and caching: `DESCRIBE TABLE` results are cached in `tableSchemaCache`
- Schema refresh: Background goroutine periodically refreshes all cached table schemas (configurable via `SchemaRefreshInterval`)
- Manual schema refresh: Use `RefreshTableSchema(ctx, table)` to immediately update specific table schema
- Dual flush triggers: time-based (`FlushInterval`) and size-based (`FlushSize`)
- Type converters (`StringConverter`, `IntConverter`, etc.) handle automatic type coercion
- Non-insertable columns (MATERIALIZED, ALIAS, EPHEMERAL) are automatically filtered

**Important Files**:
- `ch.go` - Core interfaces
- `client.go` - Client implementation with connection management
- `writer.go` - Async batch writer with buffering and flushing logic
- `converter.go` - Type conversion system with `TableColumn` metadata parsing
- `config.go` - Configuration with validation

**Data Flow** (Write):
1. User calls `Write(rows)` → rows sent to unbounded channel
2. Background goroutine accumulates rows by table in local buffer
3. Flush triggered by size/time/shutdown
4. Schema fetched from cache or `DESCRIBE TABLE`
5. Batch insert via `conn.PrepareBatch()`

**Configuration**:
```go
cfg := &ch.Config{
    DSN:   "clickhouse://localhost:9000/default",
    Debug: false,
    Settings: clickhouse.Settings{
        "max_execution_time": 60,  // ClickHouse query settings
    },
    WriterConfig: &ch.WriterConfig{
        FlushInterval:         10 * time.Second, // Time-based flush check interval
        FlushSize:             5000,              // Size-based flush trigger (immediate flush)
        MinFlushSize:          500,               // Minimum batch size for time-triggered flush
        MaxWaitTime:           60 * time.Second,  // Maximum wait time to ensure data freshness
        SchemaRefreshInterval: 5 * time.Minute,   // Auto-refresh interval (0 = disabled)
    },
}
client, err := ch.NewClient(cfg, logger)
```

**Flush Strategy**:
The Writer uses a smart flush strategy to balance data freshness and ClickHouse best practices:
1. **Size trigger**: Flush immediately when `totalRows >= FlushSize` (default: 5000)
2. **Time trigger with MinFlushSize**: Flush on interval only when `totalRows >= MinFlushSize` (default: 500)
3. **MaxWaitTime guarantee**: Force flush when data has been waiting longer than `MaxWaitTime` (default: 60s)
4. **Shutdown flush**: Flush all remaining data regardless of size

This prevents frequent small batch writes during low traffic periods while ensuring data doesn't wait too long.
