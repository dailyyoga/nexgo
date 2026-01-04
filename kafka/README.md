# Kafka

A lightweight Kafka consumer and producer wrapper for Go, built on top of [confluent-kafka-go](https://github.com/confluentinc/confluent-kafka-go).

## Features

### Consumer
- Simple and intuitive API
- Multiple consumer instances for parallel processing
- Automatic retry mechanism with configurable attempts
- Flexible offset management (manual/auto commit)
- Comprehensive error handling
- Structured logging support with zap
- Type-safe message handling

### Producer
- Async message delivery with delivery reports
- Configurable batch sending (linger time and batch size)
- Configurable compression (none, gzip, snappy, lz4, zstd)
- Configurable acks mechanism (0, 1, all) for reliability vs performance
- Automatic retry on producer creation failure
- Graceful shutdown with pending message flush
- Type-safe message handling

## Installation

```bash
go get github.com/confluentinc/confluent-kafka-go/v2/kafka
```

## Quick Start

### Consumer - Basic Usage

```go
package main

import (
    "context"
    "log"

    "your-module/kafka"
    "go.uber.org/zap"
)

func main() {
    // Create logger
    logger, _ := zap.NewProduction()

    // Create consumer config
    config := &kafka.ConsumerConfig{
        Brokers: []string{"localhost:9092"},
        GroupID: "my-consumer-group",
        Topics:  []string{"my-topic"},
    }

    // Create consumer
    consumer, err := kafka.NewConsumer(logger, config)
    if err != nil {
        log.Fatal(err)
    }
    defer consumer.Close()

    // Define message handler
    handler := func(ctx context.Context, msg *kafka.Message) error {
        logger.Info("received message",
            zap.String("topic", *msg.TopicPartition.Topic),
            zap.ByteString("value", msg.Value),
        )
        return nil
    }

    // Start consuming
    ctx := context.Background()
    if err := consumer.Start(ctx, handler); err != nil {
        log.Fatal(err)
    }

    // Wait for termination signal
    select {}
}
```

### Producer - Basic Usage

```go
package main

import (
    "context"
    "log"

    "your-module/kafka"
    "go.uber.org/zap"
)

func main() {
    // Create logger
    logger, _ := zap.NewProduction()

    // Create producer config
    config := &kafka.ProducerConfig{
        Brokers: []string{"localhost:9092"},
    }

    // Create producer
    producer, err := kafka.NewProducer(logger, config)
    if err != nil {
        log.Fatal(err)
    }
    defer producer.Close()

    // Send a message
    topic := "my-topic"
    msg := &kafka.Message{
        TopicPartition: kafka.TopicPartition{
            Topic: &topic,
        },
        Value: []byte("Hello, Kafka!"),
        Key:   []byte("key1"),
    }

    ctx := context.Background()
    if err := producer.Produce(ctx, msg); err != nil {
        log.Fatal(err)
    }

    logger.Info("message sent successfully")
}
```

## Configuration

### ConsumerConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Brokers` | `[]string` | - | Kafka broker addresses (required) |
| `GroupID` | `string` | - | Consumer group ID (required) |
| `Topics` | `[]string` | - | Topics to subscribe to (required) |
| `MaxRetries` | `int` | `3` | Maximum retry attempts for message processing |
| `InstanceNum` | `int` | `1` | Number of consumer instances for parallel processing |
| `AutoOffsetReset` | `string` | `"latest"` | Offset reset policy: `"earliest"` or `"latest"` |
| `EnableAutoCommit` | `bool` | `false` | Enable automatic offset commit |
| `AutoCommitInterval` | `time.Duration` | `5s` | Auto commit interval (when auto commit is enabled) |
| `SessionTimeout` | `time.Duration` | `30s` | Session timeout duration |
| `MaxPollInterval` | `time.Duration` | `120s` | Maximum time between polls |
| `SecurityProtocol` | `string` | `"PLAINTEXT"` | Security protocol (currently only PLAINTEXT supported) |
| `Debug` | `bool` | `false` | Enable debug logging |

### ProducerConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Brokers` | `[]string` | - | Kafka broker addresses (required) |
| `ClientID` | `string` | - | Client ID for identification in broker logs and metrics (optional) |
| `Acks` | `string` | `"all"` | Message acknowledgment: `"all"`/`"-1"` (highest reliability), `"1"` (medium), `"0"` (lowest) |
| `Compression` | `string` | `"none"` | Compression algorithm: `"none"`, `"gzip"`, `"snappy"`, `"lz4"`, `"zstd"` |
| `LingerMs` | `int` | `0` | Batch sending wait time in milliseconds (0 = send immediately) |
| `BatchSize` | `int` | `102400` | Maximum batch size in bytes (100KB) |
| `SecurityProtocol` | `string` | `"PLAINTEXT"` | Security protocol (currently only PLAINTEXT supported) |
| `MaxRetries` | `int` | `3` | Maximum retry attempts for producer creation |

### Example with Custom Configuration

#### Consumer

```go
config := &kafka.ConsumerConfig{
    Brokers:            []string{"broker1:9092", "broker2:9092"},
    GroupID:            "my-service",
    Topics:             []string{"orders", "payments"},
    MaxRetries:         5,
    InstanceNum:        3,
    AutoOffsetReset:    "earliest",
    EnableAutoCommit:   false,
    SessionTimeout:     30 * time.Second,
    MaxPollInterval:    120 * time.Second,
}

consumer, err := kafka.NewConsumer(logger, config)
```

#### Producer

```go
config := &kafka.ProducerConfig{
    Brokers:     []string{"broker1:9092", "broker2:9092"},
    ClientID:    "my-service",
    Acks:        "all",  // Highest reliability
    Compression: "snappy",  // Good balance of speed and compression
    LingerMs:    10,  // Wait 10ms to batch messages
    BatchSize:   200 * 1024,  // 200KB batch size
}

producer, err := kafka.NewProducer(logger, config)
```

## Advanced Usage

### Consumer

#### Manual Offset Commit

By default, the consumer uses manual offset commit. After successfully processing a message, the offset is committed automatically.

```go
config := &kafka.ConsumerConfig{
    Brokers:          []string{"localhost:9092"},
    GroupID:          "manual-commit-group",
    Topics:           []string{"my-topic"},
    EnableAutoCommit: false, // Manual commit (default)
}
```

#### Auto Offset Commit

Enable automatic offset commit with a custom interval:

```go
config := &kafka.ConsumerConfig{
    Brokers:            []string{"localhost:9092"},
    GroupID:            "auto-commit-group",
    Topics:             []string{"my-topic"},
    EnableAutoCommit:   true,
    AutoCommitInterval: 10 * time.Second,
}
```

#### Parallel Processing with Multiple Instances

Scale consumption by running multiple consumer instances in parallel:

```go
config := &kafka.ConsumerConfig{
    Brokers:     []string{"localhost:9092"},
    GroupID:     "parallel-group",
    Topics:      []string{"high-throughput-topic"},
    InstanceNum: 5, // Run 5 consumer instances in parallel
}
```

#### Error Handling with Retries

The consumer automatically retries failed messages up to `MaxRetries` times:

```go
handler := func(ctx context.Context, msg *kafka.Message) error {
    // This will be retried up to MaxRetries times on failure
    if err := processMessage(msg); err != nil {
        return err // Will trigger retry
    }
    return nil // Success, commit offset
}
```

#### Reading from the Beginning

To consume all messages from the beginning of a topic:

```go
config := &kafka.ConsumerConfig{
    Brokers:         []string{"localhost:9092"},
    GroupID:         "new-group",
    Topics:          []string{"my-topic"},
    AutoOffsetReset: "earliest", // Start from beginning
}
```

#### Accessing Message Metadata

```go
handler := func(ctx context.Context, msg *kafka.Message) error {
    logger.Info("message details",
        zap.String("topic", *msg.TopicPartition.Topic),
        zap.Int32("partition", msg.TopicPartition.Partition),
        zap.Int64("offset", int64(msg.TopicPartition.Offset)),
        zap.Time("timestamp", msg.Timestamp),
        zap.ByteString("key", msg.Key),
        zap.ByteString("value", msg.Value),
    )

    // Access headers
    for _, header := range msg.Headers {
        logger.Debug("header",
            zap.String("key", header.Key),
            zap.ByteString("value", header.Value),
        )
    }

    return nil
}
```

### Producer

#### Sending Messages with Headers

Include headers in your messages for metadata or tracing:

```go
topic := "my-topic"
msg := &kafka.Message{
    TopicPartition: kafka.TopicPartition{
        Topic: &topic,
    },
    Value: []byte("message payload"),
    Key:   []byte("message-key"),
    Headers: []kafka.Header{
        {Key: "trace-id", Value: []byte("abc123")},
        {Key: "source", Value: []byte("service-a")},
    },
}

if err := producer.Produce(ctx, msg); err != nil {
    log.Fatal(err)
}
```

#### Sending to Specific Partition

Send messages to a specific partition instead of using the default partitioner:

```go
topic := "my-topic"
msg := &kafka.Message{
    TopicPartition: kafka.TopicPartition{
        Topic:     &topic,
        Partition: 2, // Send to partition 2
    },
    Value: []byte("message payload"),
}

if err := producer.Produce(ctx, msg); err != nil {
    log.Fatal(err)
}
```

#### High-Throughput Configuration

Optimize for high throughput by enabling batching and compression:

```go
config := &kafka.ProducerConfig{
    Brokers:     []string{"localhost:9092"},
    Compression: "snappy",     // Enable compression
    LingerMs:    50,           // Wait 50ms to batch messages
    BatchSize:   500 * 1024,   // 500KB batch size
    Acks:        "1",          // Lower reliability for better performance
}

producer, err := kafka.NewProducer(logger, config)
```

#### Reliable Delivery Configuration

Optimize for reliability with highest acks and no compression:

```go
config := &kafka.ProducerConfig{
    Brokers:     []string{"localhost:9092"},
    Acks:        "all",        // Wait for all replicas
    Compression: "none",       // No compression for lowest latency
    LingerMs:    0,            // Send immediately
    MaxRetries:  10,           // More retries on producer creation
}

producer, err := kafka.NewProducer(logger, config)
```

## Message Structure

### Message

```go
type Message struct {
    Value          []byte           // Message payload
    Key            []byte           // Message key
    Timestamp      time.Time        // Message timestamp
    TopicPartition TopicPartition   // Topic and partition info
    Headers        []Header         // Message headers
}
```

### TopicPartition

```go
type TopicPartition struct {
    Topic     *string   // Topic name
    Partition int32     // Partition number
    Offset    Offset    // Message offset
}
```

### Header

```go
type Header struct {
    Key   string   // Header key
    Value []byte   // Header value
}
```

## Error Handling

The library provides structured errors for different failure scenarios:

- `ErrNoConsumerInstances`: No consumer instances available
- `ErrInvalidConfig`: Invalid configuration
- `ErrConnection`: Kafka connection failed
- `ErrSubscribe`: Topic subscription failed
- `ErrConsume`: Message consumption failed
- `ErrCommit`: Offset commit failed

Example:

```go
consumer, err := kafka.NewConsumer(logger, config)
if err != nil {
    switch {
    case errors.Is(err, kafka.ErrNoConsumerInstances):
        log.Fatal("no consumer instances")
    default:
        log.Fatalf("failed to create consumer: %v", err)
    }
}
```

## Graceful Shutdown

### Consumer

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

consumer, err := kafka.NewConsumer(logger, config)
if err != nil {
    log.Fatal(err)
}
defer consumer.Close()

if err := consumer.Start(ctx, handler); err != nil {
    log.Fatal(err)
}

// Wait for interrupt signal
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
<-sigChan

// Cancel context to stop consumers
cancel()

// Close consumer
if err := consumer.Close(); err != nil {
    log.Printf("error closing consumer: %v", err)
}
```

### Producer

```go
producer, err := kafka.NewProducer(logger, config)
if err != nil {
    log.Fatal(err)
}

// Wait for interrupt signal
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
<-sigChan

// Close producer (will flush pending messages with 10s timeout)
if err := producer.Close(); err != nil {
    log.Printf("error closing producer: %v", err)
}
```

## Logger Interface

The library expects a logger that implements the following interface:

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

This is compatible with `*zap.Logger` from [uber-go/zap](https://github.com/uber-go/zap).

## Best Practices

### Consumer

1. **Use Manual Commit for Critical Data**: For important messages, disable auto-commit to ensure messages are only committed after successful processing.

2. **Configure Appropriate Retry Counts**: Set `MaxRetries` based on your use case. Too many retries can cause lag, too few might lose messages.

3. **Monitor Consumer Lag**: Keep track of consumer lag to ensure your consumer keeps up with producers.

4. **Use Multiple Instances for High Throughput**: Scale horizontally by increasing `InstanceNum` for high-volume topics.

5. **Set Appropriate Timeouts**: Configure `SessionTimeout` and `MaxPollInterval` based on your message processing time.

6. **Handle Context Cancellation**: Always respect context cancellation in your message handler for graceful shutdowns.

### Producer

1. **Always Close Producers**: Always call `Close()` on producers to flush pending messages. Use `defer producer.Close()` immediately after creation.

2. **Balance Reliability and Performance**: Choose appropriate `Acks` setting:
   - Use `"all"` for critical data requiring highest reliability
   - Use `"1"` for balanced reliability and performance
   - Use `"0"` only for metrics or non-critical data where performance is paramount

3. **Enable Compression for Large Messages**: Use `snappy` or `lz4` compression for large messages to reduce network bandwidth.

4. **Optimize Batching for Throughput**: For high-throughput scenarios, increase `BatchSize` and `LingerMs` to batch more messages together.

5. **Monitor Delivery Reports**: Check application logs for delivery failures. The producer logs all delivery results asynchronously.

6. **Use Message Keys for Ordering**: If message ordering is important, always set the message key. Messages with the same key go to the same partition.

7. **Set Client ID for Monitoring**: Set a meaningful `ClientID` to help identify producers in broker logs and monitoring tools.

## Dependencies

- [confluent-kafka-go](https://github.com/confluentinc/confluent-kafka-go) v2
- [zap](https://github.com/uber-go/zap) - Structured logging
- [pkg/errors](https://github.com/pkg/errors) - Error handling

## License

See the main project license.
