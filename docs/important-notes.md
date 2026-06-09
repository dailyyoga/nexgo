# Important Notes

Read this before shipping a change that touches a package's lifecycle or runtime behavior — `Start()`/`Close()`/`Stop()` ordering, flush/drain-on-shutdown, reference-type data races, schema-cache refresh, and other per-package gotchas. One bullet per package.

- **Logger Package**: `logger.New(cfg)` creates a logger for DI and automatically sets the global logger. Use `logger.Info()`, `logger.Error()` etc. for convenient global access. Both DI and global functions use the same configuration.
- **Global Logger**: Concurrency-safe, lazy-initialized with default config if `New()` is never called. `CallerSkip` is handled automatically for correct caller information.
- **Logger Interface**: All packages use `logger.Logger` interface - compatible with `*zap.Logger`
- **Database Package**: Must call `Close()` on shutdown to release connections. Use `Ping(ctx)` for health checks. Connection pool settings should be tuned based on expected load.
- **Database Logging**: Set `LogLevel: "warn"` with `SlowThreshold` to monitor slow queries. Use `LogLevel: "info"` for full SQL tracing in development.
- **GORM Usage**: Always use `WithContext(ctx)` for query operations to support cancellation and timeout. Check `result.Error` after all GORM operations.
- **ClickHouse Writer**: Must call `Start()` before `Write()`, call `Close()` to flush pending data
- **Kafka Consumer**: Context cancellation is the primary shutdown mechanism
- **Kafka Producer**: Must call `Close()` to flush pending messages (10s timeout). Delivery reports are logged asynchronously
- **Cron Tasks**: Return errors from `Run()`, never panic (recovery middleware will catch)
- **Schema Changes**: ClickHouse schema cache auto-refreshes by default (every 5 minutes). For immediate effect after DDL changes, call `writer.RefreshTableSchema(ctx, tableName)` or set `SchemaRefreshInterval: 0` to disable auto-refresh
- **Type Conversion**: Int64 timestamps are assumed to be milliseconds, converted to `time.Time` automatically
- **Enum Handling**: Empty strings in enum columns replaced with first enum value
- **Routine Package**: Use `Runner` interface when you need to track goroutines and wait for completion; use standalone functions for fire-and-forget scenarios
- **Cache Package**: Must call `Start()` before `Get()`, `Start()` blocks until initial sync succeeds. Call `Stop()` for graceful shutdown. Sync errors are retried automatically with exponential backoff. **CRITICAL**: `Get()` returns a reference for reference types (slice, map, pointer) - treat returned data as read-only to avoid data races. Create a deep copy if modification is needed. Value types (int, string, struct without pointers) are automatically safe as Go copies them
- **Redis Package**: Call `Close()` on shutdown. Use `cache.Nil` to check for non-existent keys. Use `Unwrap()` for Pipeline/Transaction operations. The client is fully thread-safe
- **DLQ Package**: `Record()` never blocks and never returns an error — the DLQ degrades (drop + count, or fall back to log) instead of disturbing the main flow. Must call `Close()` on shutdown to drain the buffer before releasing the producer; `Close()` is idempotent. Monitor `Dropped()` for alerting. Keep business semantics (wire schema, raw-data truncation) in the `Payload` implementation; `dlq` stays semantics-free. Use `NewNoopRecorder()` when the DLQ is disabled so call sites need no nil check
