# logger

A unified logging interface based on zap with configurable levels, encoding, and output paths.

## Features

- **Unified Interface**: Standard Logger interface compatible with `*zap.Logger`
- **High Performance**: Built on `uber-go/zap` for high-performance structured logging
- **Flexible Encoding**: Support for JSON (production) and Console (development) formats
- **Configurable Levels**: Support for debug, info, warn, error, dpanic, panic, fatal levels
- **Multiple Outputs**: Configurable output paths for normal logs and error logs
- **Auto Defaults**: Automatic default value merging for partial configurations
- **Cross-Package Usage**: Used consistently across all go-kit packages (ch, kafka, cron, routine)

## Installation

```bash
go get github.com/dailyyoga/nexgo/logger
```

## Quick Start

### Default Configuration

```go
package main

import (
    "github.com/dailyyoga/nexgo/logger"
    "go.uber.org/zap"
)

func main() {
    // Use default configuration (info level, JSON encoding, stdout)
    log, err := logger.New(nil)
    if err != nil {
        panic(err)
    }
    defer log.Sync()

    log.Info("application started")
    log.Debug("this won't show with info level")
    log.Warn("warning message", zap.String("key", "value"))
    log.Error("error occurred", zap.Error(err))
}
```

### Custom Configuration

```go
package main

import (
    "github.com/dailyyoga/nexgo/logger"
    "go.uber.org/zap"
)

func main() {
    cfg := &logger.Config{
        Level:            "debug",
        Encoding:         "console",
        OutputPaths:      []string{"stdout", "/var/log/app.log"},
        ErrorOutputPaths: []string{"stderr", "/var/log/app-error.log"},
    }

    log, err := logger.New(cfg)
    if err != nil {
        panic(err)
    }
    defer log.Sync()

    log.Debug("debug message now visible")
    log.Info("info message",
        zap.String("user", "john"),
        zap.Int("age", 30),
    )
}
```

### Partial Configuration

You can provide partial configuration; missing fields will use defaults:

```go
// Only override level, use defaults for everything else
cfg := &logger.Config{
    Level: "debug",
}
log, _ := logger.New(cfg)
```

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Level` | `string` | `"info"` | Log level: debug, info, warn, error, dpanic, panic, fatal |
| `Encoding` | `string` | `"json"` | Output format: json or console |
| `OutputPaths` | `[]string` | `["stdout"]` | Output destinations for normal logs |
| `ErrorOutputPaths` | `[]string` | `["stderr"]` | Output destinations for error logs |

### Log Levels

| Level | Description |
|-------|-------------|
| `debug` | Detailed information for debugging |
| `info` | General operational information |
| `warn` | Warning conditions that should be addressed |
| `error` | Error conditions that don't stop execution |
| `dpanic` | Critical errors that panic in development |
| `panic` | Critical errors that always panic |
| `fatal` | Fatal errors that call `os.Exit(1)` |

### Encoding Formats

**JSON** (default, recommended for production):
```json
{"level":"INFO","timestamp":"2024-01-15T10:30:00.000Z","caller":"main.go:15","msg":"application started","user":"john"}
```

**Console** (recommended for development):
```
2024-01-15T10:30:00.000+0800	INFO	main.go:15	application started	{"user": "john"}
```

## API Reference

### Logger Interface

```go
type Logger interface {
    Debug(msg string, fields ...zap.Field)
    Info(msg string, fields ...zap.Field)
    Warn(msg string, fields ...zap.Field)
    Error(msg string, fields ...zap.Field)
    Fatal(msg string, fields ...zap.Field)
    Sync() error
}
```

### Factory Function

```go
// New creates a new logger with the given configuration
// If cfg is nil, default configuration is used
func New(cfg *Config) (Logger, error)
```

### Configuration

```go
// Config is the configuration for the logger
type Config struct {
    Level            string   // Log level
    Encoding         string   // Output format
    OutputPaths      []string // Output destinations
    ErrorOutputPaths []string // Error output destinations
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config

// Validate validates the configuration
func (c *Config) Validate() error
```

## Error Handling

```go
// Error constructors
ErrInvalidLevel(level string, err error) error   // Invalid log level
ErrInvalidEncoding(encoding string) error        // Invalid encoding format
ErrBuildLogger(err error) error                  // Logger build failure
```

**Example**:
```go
cfg := &logger.Config{
    Level: "invalid",
}
log, err := logger.New(cfg)
if err != nil {
    // err: logger: invalid level "invalid": must be one of: debug, info, warn, error, dpanic, panic, fatal
    fmt.Println(err)
}
```

## Integration with Other Packages

The Logger interface is used across all go-kit packages:

```go
import (
    "github.com/dailyyoga/nexgo/logger"
    "github.com/dailyyoga/nexgo/ch"
    "github.com/dailyyoga/nexgo/kafka"
    "github.com/dailyyoga/nexgo/cron"
    "github.com/dailyyoga/nexgo/routine"
)

func main() {
    log, _ := logger.New(nil)
    defer log.Sync()

    // Use with ClickHouse client
    chClient, _ := ch.NewClient(chConfig, log)

    // Use with Kafka consumer/producer
    consumer, _ := kafka.NewConsumer(log, kafkaConfig)
    producer, _ := kafka.NewProducer(log, kafkaConfig)

    // Use with Cron manager
    cronMgr := cron.NewCron(log)

    // Use with Routine runner
    runner := routine.New(log)
}
```

## Using with zap.Logger Directly

Since the Logger interface is compatible with `*zap.Logger`, you can also use zap directly:

```go
import "go.uber.org/zap"

// Create a zap logger directly
zapLogger, _ := zap.NewProduction()

// Use it with go-kit packages (they accept logger.Logger interface)
chClient, _ := ch.NewClient(config, zapLogger)
```

## Structured Logging

Use zap fields for structured logging:

```go
import "go.uber.org/zap"

log.Info("user action",
    zap.String("user_id", "12345"),
    zap.String("action", "login"),
    zap.Int("attempt", 1),
    zap.Duration("latency", time.Millisecond*150),
    zap.Time("timestamp", time.Now()),
    zap.Error(err),
)
```

Common zap field types:
- `zap.String(key, value)` - String values
- `zap.Int(key, value)` - Integer values
- `zap.Int64(key, value)` - Int64 values
- `zap.Float64(key, value)` - Float values
- `zap.Bool(key, value)` - Boolean values
- `zap.Duration(key, value)` - Duration values
- `zap.Time(key, value)` - Time values
- `zap.Error(err)` - Error values (key is "error")
- `zap.Any(key, value)` - Any type (uses reflection)

## Best Practices

1. **Always call `Sync()`**: Use `defer log.Sync()` to ensure all logs are flushed
2. **Use JSON in production**: JSON encoding is easier to parse by log aggregators
3. **Use Console in development**: Console encoding is more human-readable
4. **Choose appropriate levels**: Don't log everything as Error; use appropriate levels
5. **Use structured fields**: Prefer `zap.String()` over string formatting for better performance
6. **Don't log sensitive data**: Avoid logging passwords, tokens, or PII

## Dependencies

- [uber-go/zap](https://github.com/uber-go/zap) - High-performance structured logging

## License

This project is licensed under the MIT License - see the [LICENSE](../LICENSE) file for details.
