package kafka

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/dailyyoga/go-kit/logger"
	"go.uber.org/zap"
)

type defaultProducer struct {
	logger logger.Logger

	p *kafka.Producer

	wg   sync.WaitGroup
	done chan struct{}
}

// NewProducer creates a new kafka producer
func NewProducer(log logger.Logger, config *ProducerConfig) (Producer, error) {
	if config == nil {
		config = DefaultProducerConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	if err := validateKafkaCluster(log, config.Brokers); err != nil {
		return nil, err
	}

	configMap := config.BuildConfigMap()

	var producer *kafka.Producer
	var err error

	maxRetries := 3
	retryDelay := 3 * time.Second
	for i := 0; i < maxRetries; i++ {
		producer, err = kafka.NewProducer(configMap)
		if err == nil {
			break
		}

		if i < maxRetries-1 {
			log.Warn("failed to create kafka producer, retrying...",
				zap.Error(err),
				zap.Int("attempt", i+1),
				zap.Int("max_retries", maxRetries),
			)
			time.Sleep(retryDelay)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create kafka producer after %d retries: %w", maxRetries, err)
	}

	kp := &defaultProducer{
		p:      producer,
		logger: log,
		done:   make(chan struct{}),
	}

	kp.wg.Add(1)
	go kp.handleDeliveryReports()

	log.Info("kafka producer initialized and validated", zap.Strings("brokers", config.Brokers))
	return kp, nil
}

// handleDeliveryReports handles the delivery reports from the kafka producer
func (kp *defaultProducer) handleDeliveryReports() {
	defer kp.wg.Done()

	for {
		select {
		case <-kp.done:
			return
		case e := <-kp.p.Events():
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					kp.logger.Error("failed to deliver message",
						zap.Error(ev.TopicPartition.Error),
						zap.String("topic", *ev.TopicPartition.Topic),
					)
				} else {
					kp.logger.Debug("message delivered",
						zap.String("topic", *ev.TopicPartition.Topic),
						zap.Int32("partition", ev.TopicPartition.Partition),
						zap.Int64("offset", int64(ev.TopicPartition.Offset)),
					)
				}
			case kafka.Error:
				kp.logger.Error("kafka producer error",
					zap.Int("code", int(ev.Code())),
					zap.String("error", ev.String()),
				)
				if ev.Code() == kafka.ErrAllBrokersDown {
					kp.logger.Error("all kafka brokers are down", zap.Error(ev))
					close(kp.done)
					return
				}
			default:
				kp.logger.Debug("received unknown event", zap.String("type", fmt.Sprintf("%T", ev)))
			}
		}
	}
}

// Produce produces a message to the kafka topic
func (kp *defaultProducer) Produce(ctx context.Context, msg *Message) error {
	if msg.TopicPartition.Topic == nil {
		return ErrInvalidConfig("topic is required")
	}
	if msg.Value == nil {
		return ErrInvalidConfig("value is required")
	}

	message := &kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     msg.TopicPartition.Topic,
			Partition: kafka.PartitionAny,
		},
		Value: msg.Value,
	}

	if msg.TopicPartition.Partition != PartitionAny {
		message.TopicPartition.Partition = msg.TopicPartition.Partition
	}
	if msg.Key != nil {
		message.Key = msg.Key
	}
	for _, header := range msg.Headers {
		message.Headers = append(message.Headers, kafka.Header{Key: header.Key, Value: header.Value})
	}

	return kp.p.Produce(message, nil)
}

// Close closes the kafka producer
func (kp *defaultProducer) Close() error {
	close(kp.done)
	kp.wg.Wait()

	remaining := kp.p.Flush(10000) // 10 seconds
	if remaining > 0 {
		kp.logger.Warn("producer flushed messages before shutdown", zap.Int("remaining", remaining))
	}

	kp.p.Close()
	return nil
}
