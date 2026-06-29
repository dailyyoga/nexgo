package kafka

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/dailyyoga/nexgo/logger"
	"go.uber.org/zap"
)

// consumeInstance represents a single kafka consumer instance
type consumeInstance struct {
	logger logger.Logger

	config *ConsumerConfig
	name   string
	c      *kafka.Consumer

	// hook, when non-nil, receives consume/lag metrics events. Copied from
	// config.MetricsHook at construction.
	hook ConsumerMetricsHook

	closed atomic.Bool
}

func newConsumeInstance(name string, config *ConsumerConfig, log logger.Logger) (*consumeInstance, error) {
	// build kafka consumer configuration
	configMap := config.BuildConfigMap()

	// create kafka consumer
	consumer, err := kafka.NewConsumer(configMap)
	if err != nil {
		return nil, ErrConnection(err)
	}

	// subscribe to topics
	if err := consumer.SubscribeTopics(config.Topics, nil); err != nil {
		consumer.Close()
		return nil, ErrSubscribe(config.Topics, err)
	}

	c := &consumeInstance{
		config: config,
		name:   name,
		c:      consumer,
		logger: log,
		hook:   config.MetricsHook,
	}

	return c, nil
}

// Start starts the kafka consumer consume loop
func (c *consumeInstance) Start(ctx context.Context, handler ConsumerMsgHandler) error {
	go func() {
		if err := c.consumeLoop(ctx, handler); err != nil {
			c.logger.Error("kafka consumer loop exited with error",
				zap.String("instance_name", c.name),
				zap.Error(err))
		}
	}()
	c.logger.Info("kafka consumer instance started", zap.String("instance_name", c.name))
	return nil
}

// Close closes the kafka consumer instance
func (c *consumeInstance) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}

	c.c.Close()
	c.logger.Info("kafka consumer instance closed", zap.String("instance_name", c.name))
	return nil
}

// consumeLoop is the main loop for consuming messages from kafka
func (c *consumeInstance) consumeLoop(ctx context.Context, handler ConsumerMsgHandler) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		default:
			ev := c.c.Poll(-1)
			if ev == nil {
				continue
			}

			switch e := ev.(type) {
			case *kafka.Message:
				// process message
				if err := c.handlerMessage(ctx, e, handler); err != nil {
					c.logger.Error("kafka consumer handle message failed",
						zap.String("topic", *e.TopicPartition.Topic),
						zap.Int32("partition", e.TopicPartition.Partition),
						zap.Int64("offset", int64(e.TopicPartition.Offset)),
						zap.Error(err),
					)

					// Continue processing even if one message fails
					// You can customize this behavior based on your needs
				}
			case kafka.Error:
				// kafka error
				c.logger.Error("kafka consumer error", zap.Int("code", int(e.Code())), zap.String("error", e.String()))

				if e.Code() == kafka.ErrAllBrokersDown {
					c.logger.Error("all kafka brokers are down", zap.Error(e))
					return ErrConsume(e)
				}
			case kafka.OffsetsCommitted:
				if e.Error != nil {
					c.logger.Error("failed to commit offsets", zap.Error(e.Error))
				}
			case *kafka.Stats:
				// Only parse the (non-trivial) statistics JSON when a hook is
				// configured to consume the lag; otherwise this is a no-op and
				// behavior matches the legacy code path.
				if c.hook != nil {
					for _, lag := range parseStatsLag(e.String()) {
						c.hook.OnLag(c.config.GroupID, lag.Topic, lag.Partition, lag.Lag)
					}
				}
			default:
				c.logger.Debug("received unknown event", zap.String("type", fmt.Sprintf("%T", e)))
			}
		}
	}
}

// topicName safely dereferences the (librdkafka-owned) topic pointer, returning
// "" when it is nil so metrics hooks never panic on an unexpected message.
func topicName(msg *kafka.Message) string {
	if msg.TopicPartition.Topic != nil {
		return *msg.TopicPartition.Topic
	}
	return ""
}

// just a wrapper for kafka.Message to Message
func toMessage(msg *kafka.Message) *Message {
	message := &Message{
		Value:     msg.Value,
		Key:       msg.Key,
		Timestamp: msg.Timestamp,
		TopicPartition: TopicPartition{
			Topic:     msg.TopicPartition.Topic,
			Partition: msg.TopicPartition.Partition,
			Offset:    Offset(msg.TopicPartition.Offset),
		},
		Headers: make([]Header, len(msg.Headers)),
	}

	for i, header := range msg.Headers {
		message.Headers[i] = Header{
			Key:   header.Key,
			Value: header.Value,
		}
	}

	return message
}

// handlerMessage is the function for handling a single message from kafka
func (c *consumeInstance) handlerMessage(ctx context.Context, msg *kafka.Message, handler ConsumerMsgHandler) (err error) {
	startTime := time.Now()

	// Report the consume outcome (final error + total latency) exactly once,
	// covering the success, handler-failure and commit-failure paths via the
	// named return. Registered only when a hook is configured so the no-hook
	// path is unchanged.
	if c.hook != nil {
		defer func() {
			c.hook.OnConsume(
				c.config.GroupID,
				topicName(msg),
				msg.TopicPartition.Partition,
				err,
				time.Since(startTime),
			)
		}()
	}

	for i := 1; i <= c.config.MaxRetries; i++ {
		if err = handler(ctx, toMessage(msg)); err == nil {
			break
		}
	}

	if err != nil {
		return err
	}

	// manual commit if auto commit is disabled
	if !c.config.EnableAutoCommit {
		if _, commitErr := c.c.CommitMessage(msg); commitErr != nil {
			err = ErrCommit(commitErr)
			return err
		}
	}

	c.logger.Debug("kafka consumer instance processed message successfully",
		zap.String("topic", *msg.TopicPartition.Topic),
		zap.Int32("partition", msg.TopicPartition.Partition),
		zap.Int64("offset", int64(msg.TopicPartition.Offset)),
		zap.Duration("duration", time.Since(startTime)),
	)
	return nil
}
