package dlq

import "github.com/dailyyoga/nexgo/kafka"

const (
	defaultBufferSize      = 10000
	defaultMaxMessageBytes = 1 << 20 // 1MB, matches kafka's typical message.max.bytes
)

// Config configures a kafka-backed Recorder. It carries only runtime knobs plus
// a destination topic — no business semantics. The topic value is decided by the
// business layer and passed in here; nexgo never interprets it.
type Config struct {
	// Producer is the dedicated kafka producer config for the DLQ topic. The
	// recorder creates and owns this producer, so the DLQ is fully isolated from
	// the service's main producers.
	Producer *kafka.ProducerConfig `mapstructure:"producer"`

	// Topic is the kafka topic failed records are produced to.
	Topic string `mapstructure:"topic"`

	// BufferSize is the capacity of the internal async channel. When full,
	// Record drops the payload and increments the dropped counter. 0 => default.
	BufferSize int `mapstructure:"buffer_size"`

	// MaxMessageBytes is a hard ceiling on the marshaled payload size. Payloads
	// larger than this are dropped (and counted) so a single oversized record can
	// never exceed the kafka message size limit. This is a last-resort safety net
	// on top of whatever semantic truncation the Payload already applies.
	// 0 => default.
	MaxMessageBytes int `mapstructure:"max_message_bytes"`
}

// withDefaults returns a copy of c with zero-valued knobs filled in.
func (c *Config) withDefaults() *Config {
	cp := *c
	if cp.BufferSize <= 0 {
		cp.BufferSize = defaultBufferSize
	}
	if cp.MaxMessageBytes <= 0 {
		cp.MaxMessageBytes = defaultMaxMessageBytes
	}
	return &cp
}

// Validate checks the required fields.
func (c *Config) Validate() error {
	if c == nil {
		return ErrInvalidConfig("config is required")
	}
	if c.Topic == "" {
		return ErrInvalidConfig("topic is required")
	}
	if c.Producer == nil {
		return ErrInvalidConfig("producer config is required")
	}
	return c.Producer.Validate()
}
