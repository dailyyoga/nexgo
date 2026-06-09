# dlq (Dead-Letter Recorder)

Read this when working on the `dlq` package — implementing `Payload`, the drop-on-full / degrade-to-log behavior, the byte-size cap, or the recorder lifecycle. Referenced from the Reading Index in `CLAUDE.md`.

**Core Pattern**: Generic, best-effort dead-letter recorder that owns only the tricky runtime behavior (async buffer, drop-on-full, degrade-to-log, byte cap, clean shutdown) while staying completely free of business semantics.

**Key Components**:
- `Recorder` interface - Provides `Record(ctx, payload)`, `Dropped()`, and `Close()`
- `Payload` interface - Implemented by the business layer with `Marshal()`, `Key()`, and `LogFields()`
- `Config` struct - Runtime knobs (buffer size, byte cap) plus a destination `Topic` and a dedicated `kafka.ProducerConfig`
- `kafkaRecorder` - Async kafka-backed implementation
- `noopRecorder` - No-op implementation used when the DLQ is disabled (so call sites never need a nil check)

**Architecture Details**:
- Separation of concerns: `dlq` knows nothing about `app_id`, error-type enums, or specific topics — the business layer owns the wire schema and semantic truncation via `Payload`, and passes the topic in through `Config`
- `Record` never blocks and never returns an error: it does a non-blocking channel send and the DLQ is allowed to degrade so it can never disturb the main data flow
- Drop-on-full: when the internal buffer is full, the payload is dropped, the `dropped` counter (`atomic.Uint64`) is incremented, and a sparse `Warn` is logged (n==1 or every 1000th) to avoid a log storm
- Degrade-to-log: marshal errors, over-cap payloads, and produce failures all fall back to a structured `Error` log carrying the payload's `LogFields`, so a record is never lost silently
- Byte-size safety cap (`MaxMessageBytes`): a hard ceiling on the marshaled payload, a last-resort net on top of whatever truncation the `Payload` already applies, ensuring a single oversized record never exceeds the kafka message limit
- Dedicated producer: the recorder creates and owns its own kafka producer, fully isolating the DLQ from the service's main producers
- Clean lifecycle: a single background goroutine drains the channel; `Close()` signals via a `done` channel, drains whatever is still buffered, waits on a `sync.WaitGroup`, then closes the producer — and is idempotent (`atomic.Bool` guard). `Record` after `Close` is silently dropped and never panics (the channel is never closed by `Record`)
- Testability: `newKafkaRecorder` wires the runtime core around an already-built producer so tests can inject a fake producer without a broker

**Important Files**:
- `dlq.go` - Core `Recorder` and `Payload` interfaces with package documentation
- `recorder.go` - `kafkaRecorder` async implementation and `NewKafkaRecorder` factory
- `noop.go` - `noopRecorder` and `NewNoopRecorder`
- `config.go` - Configuration with `withDefaults()` and `Validate()`
- `errors.go` - `ErrInvalidConfig` constructor

**Data Flow** (Record):
1. User calls `Record(ctx, payload)` → non-blocking send to the internal channel
2. If buffer full → drop + increment `dropped` counter (sparse warn log)
3. Background `loop()` pulls payloads and calls `produce()`
4. `produce()` marshals; on marshal error / over byte cap → degrade to log (over-cap also counts)
5. Fire-and-forget produce to kafka; on produce error → degrade to log with `LogFields`
6. `Close()` → stop accepting, drain buffer, close producer (idempotent)

**Configuration**:
```go
cfg := &dlq.Config{
    Topic: "atlas-dead-letter",          // Destination topic (required)
    Producer: &kafka.ProducerConfig{     // Dedicated, isolated producer (required)
        Brokers:  []string{"localhost:9092"},
        ClientID: "atlas-dlq",
    },
    BufferSize:      10000,   // Async channel capacity (0 => default 10000)
    MaxMessageBytes: 1 << 20, // Hard payload ceiling (0 => default 1MB)
}
recorder, err := dlq.NewKafkaRecorder(log, cfg)
```
