# Cross-Cutting Patterns

Read this when the conventions in `CLAUDE.md` are unclear or you are about to apply one — defining/structuring errors, using the shared logger interface, writing a `Config`/`Validate`, or following the interface-driven design. This is the full rationale behind the one-line rules in `CLAUDE.md`.

## Logger Interface

The logger package provides a unified `Logger` interface used across all packages:

```go
// From github.com/dailyyoga/nexgo/logger
type Logger interface {
    Debug(msg string, fields ...zap.Field)
    Info(msg string, fields ...zap.Field)
    Warn(msg string, fields ...zap.Field)
    Error(msg string, fields ...zap.Field)
    Sync() error
}
```

**Key Points**:
- All packages (ch, kafka, cron, routine, cache) import and use `logger.Logger` interface
- Compatible with `*zap.Logger` - you can pass a zap logger directly
- Factory function `logger.New(cfg)` creates configured logger instances and sets the global logger
- Supports structured logging with zap fields
- Package-level functions (`logger.Info()`, etc.) provide convenient global access

**Usage Pattern**:
```go
import "github.com/dailyyoga/nexgo/logger"

// Create logger - also sets global logger automatically
log, err := logger.New(nil) // Uses default config
if err != nil {
    panic(err)
}
defer logger.Sync()

// Use with other packages (DI)
client, err := ch.NewClient(chConfig, log)
consumer, err := kafka.NewConsumer(log, kafkaConfig)
cronMgr := cron.NewCron(log)
runner := routine.New(log)
cache, err := cache.NewSyncableCache(log, cacheConfig, syncFunc)

// Or use global functions directly
logger.Info("server started", zap.String("addr", ":8080"))
logger.Error("request failed", zap.Error(err))
```

## Error Handling

All packages follow a unified error handling pattern, defining package-level errors in `errors.go`.

**Error Types**:

1. **Predefined Error Variables** - Package-level sentinel errors created using `fmt.Errorf`, suitable for errors that don't require additional context:
   ```go
   var ErrBufferFull = fmt.Errorf("ch: buffer is full")
   ```

2. **Error Constructor Functions** - Return new errors wrapping underlying errors, using `%w` format verb to support error chains:
   ```go
   func ErrConnection(err error) error {
       return fmt.Errorf("ch: connection failed: %w", err)
   }
   ```

**Error Definitions by Package**:

- **logger** package:
  - Error constructors: `ErrInvalidLevel(level, err)`, `ErrInvalidEncoding(encoding)`, `ErrBuildLogger(err)`

- **db** package:
  - Predefined errors: `ErrConnectionNotEstablished`
  - Error constructors: `ErrInvalidConfig(msg)`, `ErrConnection(err)`

- **ch** package:
  - Predefined errors: `ErrBufferFull`, `ErrWriterClosed`, `ErrConnectionClosed`, `ErrInvalidTable`
  - Error constructors: `ErrInvalidConfig(msg)`, `ErrConnection(err)`, `ErrInsert(tableName, err)`

- **kafka** package:
  - Predefined errors: `ErrNoConsumerInstances`
  - Error constructors: `ErrInvalidConfig(msg)`, `ErrConnection(err)`, `ErrSubscribe(topics, err)`, `ErrConsume(err)`, `ErrCommit(err)`

- **cron** package:
  - Predefined errors: `ErrNoTasks`, `ErrInvalidSpec`, `ErrCronClosed`

- **routine** package:
  - Predefined errors: `ErrPanicRecovered`
  - Error constructors: `ErrPanic(recovered)`

- **cache** package:
  - Predefined errors: `ErrCacheClosed`, `ErrInvalidConfig`
  - Error constructors: `ErrSync(err)`, `ErrInvalidName(name)`, `ErrInvalidSyncInterval(interval)`, `ErrInvalidSyncTimeout(timeout)`, `ErrInvalidMaxRetries(retries)`, `ErrInvalidRedisConfig(msg)`, `ErrRedisConnection(err)`, `ErrRedisOperation(op, err)`

- **dlq** package:
  - Error constructors: `ErrInvalidConfig(msg)`

**Error Checking**:
- Use `errors.Is()` to check predefined errors:
  ```go
  if errors.Is(err, ch.ErrBufferFull) {
      // handle buffer full
  }
  ```
- Use `errors.As()` to extract wrapped underlying errors (if needed)
- All error constructor functions use `%w` to wrap underlying errors, preserving the complete error chain for tracing

## Configuration Pattern
- Each package has `Config` struct with `Validate()` method
- Default configurations provided via `DefaultConfig()` functions
- `Validate()` performs validation only (does not modify config)
- `New()` functions merge default values before validation
- Required fields validated on construction

## Interface-Driven Design
- All major components exposed as interfaces for mocking in tests
- Implementations are private (`defaultClient`, `defaultConsumer`, `cronManager`)
- Factory functions return interfaces (`NewClient()`, `NewConsumer()`, `NewProducer()`, `NewCron()`)
- Logger interface from logger package used consistently across all packages
