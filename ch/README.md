# ch

A unified Go client library for ClickHouse with support for both query operations and asynchronous batch writes.

## Features

- **Unified Interface**: Single client for both query and batch write operations with a shared connection
- **Asynchronous Batch Writes**: Non-blocking writes with automatic batching and flushing
- **Automatic Type Conversion**: Seamless conversion between Go types and ClickHouse types
- **Schema Discovery**: Automatic table schema detection and caching
- **Dual Flush Triggers**: Time-based and size-based batch flushing for optimal performance
- **Graceful Shutdown**: Ensures all pending data is written before closing
- **Smart Column Handling**: Automatically filters non-insertable columns (MATERIALIZED, ALIAS, EPHEMERAL)

## Installation

```bash
go get github.com/dailyyoga/go-kit/ch
```

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "log"

    "github.com/dailyyoga/go-kit/ch"
    "go.uber.org/zap"
)

// Define your data model
type UserEvent struct {
    UserID    int64
    EventName string
    Timestamp int64
}

func (e *UserEvent) TableName() ch.TableName {
    return "user_events"
}

func (e *UserEvent) ToValueMap() map[string]any {
    return map[string]any{
        "user_id":    e.UserID,
        "event_name": e.EventName,
        "timestamp":  e.Timestamp,
    }
}

func main() {
    logger, _ := zap.NewProduction()
    defer logger.Sync()

    // Create client
    config := &ch.Config{
        Hosts:    []string{"localhost:9000"},
        Database: "analytics",
        Username: "default",
        Password: "password",
        WriterConfig: &ch.WriterConfig{
            FlushInterval: 3 * time.Second,
            FlushSize:     5000,
        },
    }

    client, err := ch.NewClient(config, logger)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // For batch writes
    writer, err := client.Writer()
    if err != nil {
        log.Fatal(err)
    }

    // Must call Start() before writing
    if err := writer.Start(); err != nil {
        log.Fatal(err)
    }

    // Write data asynchronously
    events := []ch.Table{
        &UserEvent{UserID: 1001, EventName: "login", Timestamp: time.Now().UnixMilli()},
        &UserEvent{UserID: 1002, EventName: "click", Timestamp: time.Now().UnixMilli()},
    }

    if err := writer.Write(context.Background(), events); err != nil {
        log.Printf("write failed: %v", err)
    }

    // For queries
    rows, err := client.Query(context.Background(), "SELECT * FROM user_events LIMIT 10")
    if err != nil {
        log.Fatal(err)
    }
    defer rows.Close()

    for rows.Next() {
        var event UserEvent
        if err := rows.Scan(&event.UserID, &event.EventName, &event.Timestamp); err != nil {
            log.Printf("scan failed: %v", err)
        }
        log.Printf("Event: %+v", event)
    }
}
```

### Query Single Row

```go
row := client.QueryRow(context.Background(), "SELECT count(*) FROM user_events")
var count uint64
if err := row.Scan(&count); err != nil {
    log.Printf("scan failed: %v", err)
}
log.Printf("Total events: %d", count)
```

## Configuration

### Connection Configuration

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `Hosts` | `[]string` | Yes | - | ClickHouse server addresses (e.g., `["localhost:9000"]`) |
| `Database` | `string` | No | `"default"` | Database name |
| `Username` | `string` | Yes | - | Authentication username |
| `Password` | `string` | Yes | - | Authentication password |
| `DialTimeout` | `time.Duration` | No | `10s` | Connection timeout |
| `Debug` | `bool` | No | `false` | Enable debug mode |
| `Settings` | `clickhouse.Settings` | No | - | ClickHouse query settings ([reference](https://clickhouse.com/docs/en/operations/settings/settings)) |

### Writer Configuration

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `FlushInterval` | `time.Duration` | No | `3s` | Time interval to trigger batch flush |
| `FlushSize` | `int` | No | `5000` | Number of rows to trigger batch flush |

### Configuration Example

```go
import "github.com/ClickHouse/clickhouse-go/v2"

config := &ch.Config{
    Hosts:       []string{"localhost:9000", "localhost:9001"},
    Database:    "analytics",
    Username:    "default",
    Password:    "secret",
    DialTimeout: 15 * time.Second,
    Debug:       false,
    Settings: clickhouse.Settings{
        "max_execution_time": 60,
    },
    WriterConfig: &ch.WriterConfig{
        FlushInterval: 5 * time.Second,
        FlushSize:     10000,
    },
}
```

## API Reference

### Client Interface

```go
type Client interface {
    // Writer returns the Writer interface for batch writing
    Writer() (Writer, error)

    // Query executes a ClickHouse query and returns driver.Rows
    Query(ctx context.Context, query string, args ...any) (driver.Rows, error)

    // QueryRow executes a query that is expected to return at most one row
    QueryRow(ctx context.Context, query string, args ...any) driver.Row

    // Close closes the client and all associated resources
    Close() error
}
```

### Writer Interface

```go
type Writer interface {
    // Start begins processing writes (must be called before Write)
    Start() error

    // Close stops the writer and flushes remaining data
    Close() error

    // Write asynchronously writes rows to the buffer
    Write(ctx context.Context, rows []Table) error
}
```

### Table Interface

Your data models must implement this interface:

```go
type Table interface {
    // TableName returns the ClickHouse table name
    TableName() TableName

    // ToValueMap returns a map of column names to values
    ToValueMap() map[string]any
}
```

## Type Conversion

The library automatically converts between Go types and ClickHouse types:

| ClickHouse Type | Go Type | Notes |
|-----------------|---------|-------|
| String, FixedString | `string` | - |
| Int8, Int16, Int32, Int64, UInt* | `int64` | Automatic conversion from various int types |
| Float32, Float64 | `float64` | - |
| Decimal | `decimal.Decimal` | Uses `shopspring/decimal` |
| Bool | `bool` | - |
| DateTime | `time.Time` | Int64 timestamps assumed to be milliseconds |
| Enum8, Enum16 | `string` | Empty strings replaced with first enum value |
| Nullable(*) | `nil` or underlying type | - |

## Error Handling

```go
import "github.com/dailyyoga/go-kit/ch"

// Common errors
var (
    ErrBufferFull       // Buffer is full, retry later
    ErrWriterClosed     // Writer has been closed
    ErrConnectionClosed // Connection has been closed
    ErrInvalidTable     // Invalid table name
)

// Error constructors
ErrInvalidConfig(msg string) error        // Configuration validation error
ErrConnection(err error) error            // Connection error
ErrInsert(tableName TableName, err error) error  // Insert operation error
```

## Best Practices

1. **Always call `Writer.Start()`** before writing data
2. **Graceful Shutdown**: Call `client.Close()` to ensure all pending data is flushed
3. **Schema Caching**: The library caches table schemas. Restart the client if schema changes
4. **Error Handling**: Handle `ErrBufferFull` by implementing retry logic or backpressure
5. **Connection Sharing**: Query and write operations share the same connection (thread-safe)
6. **Nullable Columns**: Use `nil` for nullable column values in `ToValueMap()`
7. **Enum Types**: Provide valid enum values or empty string (will use first enum value)

## Architecture

### Key Components

- **Client**: Manages the ClickHouse connection and provides unified access to queries and writes
- **Writer**: Handles asynchronous batch writing with automatic flushing
- **Converter**: Automatically converts Go types to ClickHouse types
- **Schema Cache**: Caches table column metadata to avoid repeated DESCRIBE queries

### Write Flow

1. User calls `Write()` → data sent to unbounded channel
2. Background process loop accumulates rows in local buffer by table
3. Flush triggered by capacity threshold OR timer OR shutdown signal
4. On flush: batch insert using ClickHouse `PrepareBatch` API
5. On shutdown: drain channel and flush remaining data

## Dependencies

- [clickhouse-go/v2](https://github.com/ClickHouse/clickhouse-go) - Official ClickHouse Go driver
- [chanx](https://github.com/smallnest/chanx) - Unbounded channels for buffering
- [decimal](https://github.com/shopspring/decimal) - Decimal number handling
- [zap](https://github.com/uber-go/zap) - Structured logging

## License

This project is licensed under the MIT License - see the [LICENSE](../LICENSE) file for details.
