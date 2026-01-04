package kafka

import (
	"context"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

// Message is the message of a kafka message
type Message struct {
	Value          []byte
	Key            []byte
	Timestamp      time.Time
	TopicPartition TopicPartition
	Headers        []Header
}

// GetHeader gets the header value by key
func (m *Message) GetHeader(k string) []byte {
	for _, header := range m.Headers {
		if header.Key == k {
			return header.Value
		}
	}
	return nil
}

// PartitionAny is the any partition of a kafka message
const PartitionAny = kafka.PartitionAny

// TopicPartition is the topic and partition of a kafka message
type TopicPartition struct {
	Topic     *string
	Partition int32
	Offset    Offset
}

// Offset is the offset of a kafka message
type Offset int64

// Header is the header of a kafka message
type Header struct {
	Key   string
	Value []byte
}

// ConsumerMsgHandler is the function type for handling a single message from kafka
type ConsumerMsgHandler func(ctx context.Context, msg *Message) error

// Consumer is the interface for kafka consumer
type Consumer interface {
	Start(ctx context.Context, handler ConsumerMsgHandler) error
	Close() error
}

// Producer is the interface for kafka producer
type Producer interface {
	Produce(ctx context.Context, msg *Message) error
	Close() error
}
