# kafka (Consumer / Producer)

Read this when working on the `kafka` consumer or producer — retry loops, parallel instances, offset commits, delivery reports, or graceful shutdown. Referenced from the Reading Index in `CLAUDE.md`.

**Core Pattern**: Wrapper around confluent-kafka-go with automatic retry, parallel processing, and robust error handling.

#### Consumer

**Key Components**:
- `Consumer` interface - `Start(ctx, handler)` and `Close()`
- `ConsumerMsgHandler` - User-defined function `func(ctx, *Message) error`
- `defaultConsumer` - Manages multiple consumer instances for parallel processing

**Architecture Details**:
- Creates `InstanceNum` parallel consumer instances (goroutines)
- Each instance runs independent consumption loop with retry logic
- Manual offset commit after successful processing (unless `EnableAutoCommit=true`)
- Retry mechanism: up to `MaxRetries` attempts per message with exponential backoff
- Context cancellation propagates to all instances for graceful shutdown

**Data Flow** (Consumer):
1. `Start()` spawns `InstanceNum` goroutines
2. Each goroutine polls Kafka with `consumer.ReadMessage()`
3. On message: call handler with retry logic
4. On success: commit offset (if manual commit)
5. On context cancel: close all instances

#### Producer

**Key Components**:
- `Producer` interface - `Produce(ctx, msg)` and `Close()`
- `defaultProducer` - Manages producer instance with async delivery reports
- `Message` - Unified message type for both consumer and producer

**Architecture Details**:
- Single producer instance with async message delivery
- Background goroutine handles delivery reports from Events() channel
- Configurable batch sending via `LingerMs` and `BatchSize`
- Configurable compression (none, gzip, snappy, lz4, zstd)
- Configurable acks mechanism (0, 1, all) for reliability vs performance tradeoff
- Graceful shutdown: `Close()` flushes pending messages (10s timeout) before closing
- Auto-reconnection: producer creation retries on failure (3 attempts)

**Data Flow** (Producer):
1. User calls `Produce(ctx, msg)` → message queued by librdkafka
2. Background goroutine monitors `Events()` channel for delivery reports
3. On delivery: log success (Debug) or failure (Error)
4. On `ErrAllBrokersDown`: auto-close producer
5. On `Close()`: flush pending messages and wait for goroutine completion

**Important Files**:
- `kafka.go` - Core interfaces and types (shared by consumer and producer)
- `consumer.go` - Consumer factory and interface
- `consume.go` - Consumer implementation with retry loop
- `producer.go` - Producer factory and implementation
- `config.go` - Configuration for both consumer and producer with defaults
- `validate.go` - Kafka cluster validation (shared by consumer and producer)
- `errors.go` - Structured error types
