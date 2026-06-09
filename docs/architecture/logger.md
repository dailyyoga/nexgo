# logger (Logging Interface)

Read this when working on the `logger` package itself, or when wiring a logger into another package. Referenced from the Reading Index in `CLAUDE.md`.

**Core Pattern**: Unified logging interface based on zap with configurable output, encoding, and levels. Supports both dependency injection and global package-level functions.

**Key Components**:
- `Logger` interface - Standard logging interface with `Debug()`, `Info()`, `Warn()`, `Error()`, `Sync()`
- `Config` struct - Configuration for log level, encoding, and output paths
- `New(cfg)` factory - Creates a new logger instance and automatically sets it as the global logger
- Package-level functions - `Debug()`, `Info()`, `Warn()`, `Error()`, `Sync()` for convenient global access
- `SetGlobalLogger(l)` - Manually set the global logger
- `GetGlobalLogger()` - Retrieve the current global logger

**Architecture Details**:
- Built on `uber-go/zap` for high-performance structured logging
- Supports two encoding formats: JSON (default) and Console
- Configurable log levels: debug, info, warn, error, dpanic, panic, fatal
- Configurable output paths for normal logs and error logs
- Automatic default value merging for partial configurations
- Validation ensures only valid log levels and encodings are accepted
- **Global Logger**: `New()` automatically sets a global logger with correct `CallerSkip` for package-level functions
- **Concurrency Safe**: Global logger access is protected by `sync.RWMutex`
- **Lazy Initialization**: If `New()` is never called, package-level functions use a default logger

**Important Files**:
- `logger.go` - Logger interface and factory function
- `global.go` - Global logger instance and package-level functions
- `config.go` - Configuration with validation and defaults
- `errors.go` - Error constructors for invalid configuration

**Configuration**:
```go
cfg := &logger.Config{
    Level:            "info",            // Log level
    Encoding:         "json",            // "json" or "console"
    OutputPaths:      []string{"stdout"}, // Output destinations
    ErrorOutputPaths: []string{"stderr"}, // Error output destinations
}
log, err := logger.New(cfg)
```

**Usage** (Two Ways):
```go
import "github.com/dailyyoga/nexgo/logger"

func main() {
    // Create logger - automatically sets global logger
    log, err := logger.New(&logger.Config{Level: "debug"})
    if err != nil {
        panic(err)
    }
    defer logger.Sync()

    // Way 1: Dependency Injection (recommended for libraries/components)
    svc := NewService(log)
    log.Info("using DI logger")

    // Way 2: Global functions (convenient for application code)
    logger.Info("using global logger")
    logger.Error("error occurred", zap.Error(err))
}
```

**Usage Across Packages**:
All other packages (db, ch, kafka, cron, routine, cache, dlq) accept `logger.Logger` interface in their constructors:
- `db.NewMySQL(logger, config)`
- `ch.NewClient(config, logger)`
- `kafka.NewConsumer(logger, config)`
- `kafka.NewProducer(logger, config)`
- `cron.NewCron(logger, middlewares...)`
- `routine.New(logger)`
- `cache.NewSyncableCache(logger, config, syncFunc)`
- `cache.NewRedis(logger, config)`
- `dlq.NewKafkaRecorder(logger, config)`
