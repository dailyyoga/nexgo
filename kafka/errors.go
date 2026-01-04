package kafka

import "fmt"

var (
	// ErrNoConsumerInstances no consumer instances
	ErrNoConsumerInstances = fmt.Errorf("kafka: no consumer instances")
)

// ErrInvalidConfig Kafka configuration error
func ErrInvalidConfig(msg string) error {
	return fmt.Errorf("kafka: invalid config: %s", msg)
}

// ErrConnection Kafka connection error
func ErrConnection(err error) error {
	return fmt.Errorf("kafka: connection failed: %w", err)
}

// ErrSubscribe subscribe error
func ErrSubscribe(topics []string, err error) error {
	return fmt.Errorf("kafka: subscribe to topics %v failed: %w", topics, err)
}

// ErrConsume consume message error
func ErrConsume(err error) error {
	return fmt.Errorf("kafka: consume message failed: %w", err)
}

// ErrCommit commit message error
func ErrCommit(err error) error {
	return fmt.Errorf("kafka: commit offsets failed: %w", err)
}
