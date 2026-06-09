# dlq

A generic, best-effort dead-letter recorder. It owns only the runtime behavior
that is hard to get right and easy to get wrong — a buffered channel, a
background producer goroutine, drop-on-full with a counter, degrade-to-log on
produce failure, a byte-size safety cap, and a clean `Close` lifecycle.

It deliberately knows **nothing** about business semantics — it never sees an
`app_id`, an error-type enum, or a specific topic. The business layer implements
`Payload` (which owns the JSON schema, raw-data truncation, etc.) and passes the
destination topic in via `Config`. This keeps the reusable concurrency core in
one place while every service-specific contract stays in its own code.

## Features

- **Never blocks the caller**: `Record` does a non-blocking send and returns immediately
- **Best-effort by design**: the DLQ is allowed to degrade so it can never disturb the main data flow
- **Drop-on-full with counter**: when the buffer is full, the payload is dropped and the dropped counter is incremented (`Dropped()` for `/metrics` and alerting)
- **Degrade-to-log**: marshal errors, over-cap payloads, and produce failures fall back to a structured log line so the record is never lost silently
- **Byte-size safety cap**: a hard ceiling on the marshaled payload guards against a single oversized record exceeding the kafka message limit
- **Clean lifecycle**: `Close` stops accepting, drains what is buffered, releases the producer, and is idempotent
- **Pluggable sink**: ships with a kafka-backed recorder plus a no-op recorder for when the DLQ is disabled

## Installation

```bash
go get github.com/dailyyoga/nexgo/dlq
```

## Why Use This Package?

A dead-letter path looks trivial but is full of concurrency traps: blocking the
hot path on a slow broker, panicking on a send-after-close, losing records when
the buffer fills, or letting one oversized record blow past the broker's message
limit. This package writes that runtime core exactly once so every call site
gets the safe behavior for free, and the only nil check it needs is replaced by a
`NoopRecorder`:

```go
// Disabled DLQ — call sites never need a nil check
recorder := dlq.NewNoopRecorder()

// Enabled DLQ — same interface, kafka-backed
recorder, err := dlq.NewKafkaRecorder(log, cfg)
```

## Quick Start

### 1. Implement `Payload` in the business layer

```go
import (
    "encoding/json"

    "go.uber.org/zap"
)

// failedEvent is owned by the business layer: it decides the wire schema,
// the partition key, and what context to carry into the fallback log.
type failedEvent struct {
    Service   string `json:"service"`
    ErrorType string `json:"error_type"`
    RawData   string `json:"raw_data"`
}

func (e failedEvent) Marshal() ([]byte, error) { return json.Marshal(e) }
func (e failedEvent) Key() string              { return e.Service }
func (e failedEvent) LogFields() []zap.Field {
    return []zap.Field{
        zap.String("service", e.Service),
        zap.String("error_type", e.ErrorType),
    }
}
```

### 2. Create the recorder and record failures

```go
package main

import (
    "context"

    "github.com/dailyyoga/nexgo/dlq"
    "github.com/dailyyoga/nexgo/kafka"
    "github.com/dailyyoga/nexgo/logger"
)

func main() {
    log, _ := logger.New(nil)
    defer log.Sync()

    cfg := &dlq.Config{
        Topic: "atlas-dead-letter",
        Producer: &kafka.ProducerConfig{
            Brokers:  []string{"localhost:9092"},
            ClientID: "atlas-dlq",
        },
        BufferSize:      10000,   // optional, 0 => default (10000)
        MaxMessageBytes: 1 << 20, // optional, 0 => default (1MB)
    }

    recorder, err := dlq.NewKafkaRecorder(log, cfg)
    if err != nil {
        log.Fatal("create dlq recorder failed")
    }
    defer recorder.Close() // drains the buffer and closes the producer

    // In the main data flow: when a record can't be processed, hand it to the DLQ.
    // Record never blocks and never returns an error.
    recorder.Record(context.Background(), failedEvent{
        Service:   "ingest",
        ErrorType: "parse_error",
        RawData:   "{...}",
    })
}
```

## API Reference

### Payload Interface

Implemented by the business layer — `dlq` only turns it into bytes and delivers
them asynchronously.

```go
type Payload interface {
    // Marshal returns the wire bytes of the record. The business layer owns the
    // JSON schema as well as any semantic truncation (e.g. raw-data size limits).
    Marshal() ([]byte, error)
    // Key is the kafka partition key (e.g. the service name).
    Key() string
    // LogFields are emitted to the fallback log when a record cannot be produced.
    LogFields() []zap.Field
}
```

### Recorder Interface

```go
type Recorder interface {
    // Record asynchronously enqueues p for delivery. Safe for concurrent use,
    // returns immediately, and drops (counting) when the buffer is full.
    Record(ctx context.Context, p Payload)
    // Dropped returns the total number of payloads dropped so far
    // (buffer full or over the byte cap). Useful for /metrics and alerting.
    Dropped() uint64
    // Close stops accepting new payloads, drains the buffer, and releases the
    // producer. It is idempotent.
    Close() error
}
```

### Factory Functions

```go
// NewKafkaRecorder creates a Recorder that delivers payloads to cfg.Topic via a
// dedicated kafka producer it creates and owns (fully isolated from the
// service's main producers).
func NewKafkaRecorder(log logger.Logger, cfg *Config) (Recorder, error)

// NewNoopRecorder returns a Recorder that drops everything silently, so call
// sites never need a nil check when the DLQ is disabled.
func NewNoopRecorder() Recorder
```

## Configuration

```go
type Config struct {
    // Producer is the dedicated kafka producer config for the DLQ topic. The
    // recorder creates and owns this producer.
    Producer *kafka.ProducerConfig `mapstructure:"producer"`

    // Topic is the kafka topic failed records are produced to. (required)
    Topic string `mapstructure:"topic"`

    // BufferSize is the capacity of the internal async channel. When full,
    // Record drops the payload and increments the dropped counter.
    // 0 => default (10000).
    BufferSize int `mapstructure:"buffer_size"`

    // MaxMessageBytes is a hard ceiling on the marshaled payload size. Larger
    // payloads are dropped (and counted) so a single oversized record can never
    // exceed the kafka message size limit. A last-resort safety net on top of
    // whatever semantic truncation the Payload already applies.
    // 0 => default (1MB).
    MaxMessageBytes int `mapstructure:"max_message_bytes"`
}
```

| Field             | Required | Default | Description                                          |
|-------------------|----------|---------|------------------------------------------------------|
| `Topic`           | yes      | -       | Destination kafka topic for failed records           |
| `Producer`        | yes      | -       | Dedicated kafka producer config (isolated from main) |
| `BufferSize`      | no       | 10000   | Async channel capacity; drop-on-full beyond it       |
| `MaxMessageBytes` | no       | 1MB     | Hard ceiling on the marshaled payload size           |

## Error Handling

```go
// Error constructor — invalid configuration
func ErrInvalidConfig(msg string) error
```

`NewKafkaRecorder` validates the config and returns `ErrInvalidConfig` (or a
kafka producer error) on failure. After construction, the recorder **never
returns an error to the caller**: every failure degrades to a structured log
line instead.

**Degradation behavior:**

| Situation                        | Behavior                                                       |
|----------------------------------|---------------------------------------------------------------|
| Buffer full                      | Drop payload, increment `Dropped()`, sparse `Warn` log        |
| Payload over `MaxMessageBytes`   | Drop payload, increment `Dropped()`, `Error` log              |
| `Marshal()` returns an error     | Drop payload, `Error` log (not counted)                       |
| Producer `Produce` fails         | `Error` log with the payload's `LogFields` as the last resort |
| `Record` after `Close`           | Silently dropped (never panics)                               |

## Best Practices

1. **Use a dedicated producer**: give the DLQ its own `ProducerConfig` so a
   failure flood never contends with the service's main producers.
2. **Monitor `Dropped()`**: export it to `/metrics` and alert on a non-zero (or
   rising) value — it is the signal that the DLQ itself is shedding load.
3. **Truncate raw data in `Payload.Marshal`**: keep `MaxMessageBytes` as a
   last-resort safety net, not the primary size control.
4. **Always `Close()` on shutdown**: it drains whatever is still buffered before
   releasing the producer, so in-flight records are not lost.
5. **Wire `NewNoopRecorder` when disabled**: keep the `Recorder` interface at the
   call site so toggling the DLQ never adds nil checks.

## Dependencies

- [github.com/dailyyoga/nexgo/logger](../logger) - Unified logging interface
- [github.com/dailyyoga/nexgo/kafka](../kafka) - Kafka producer (kafka-backed recorder)

## License

This project is licensed under the MIT License - see the [LICENSE](../LICENSE) file for details.
