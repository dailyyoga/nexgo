package kafka

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/dailyyoga/nexgo/logger"
)

type defaultConsumer struct {
	consumerInstances []*consumeInstance

	closed atomic.Bool
}

// NewConsumer creates a new kafka consumer
func NewConsumer(log logger.Logger, config *ConsumerConfig) (Consumer, error) {
	if config == nil {
		config = DefaultConsumerConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	if err := validateKafkaCluster(log, config.Brokers); err != nil {
		return nil, err
	}

	// create consumer instances
	consumerInstances := make([]*consumeInstance, config.InstanceNum)
	for i := 0; i < config.InstanceNum; i++ {
		instanceName := fmt.Sprintf("%s-instance-%d", config.GroupID, i+1)
		consumeInstance, err := newConsumeInstance(instanceName, config, log)
		if err != nil {
			return nil, err
		}
		consumerInstances[i] = consumeInstance
	}

	consumer := &defaultConsumer{
		consumerInstances: consumerInstances,
	}
	return consumer, nil
}

// Start starts the kafka consumer
func (c *defaultConsumer) Start(ctx context.Context, handler ConsumerMsgHandler) error {
	if len(c.consumerInstances) == 0 {
		return ErrNoConsumerInstances
	}

	for _, instance := range c.consumerInstances {
		if err := instance.Start(ctx, handler); err != nil {
			return err
		}
	}
	return nil
}

// Close closes the kafka consumer
func (c *defaultConsumer) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}

	if len(c.consumerInstances) == 0 {
		return ErrNoConsumerInstances
	}

	for _, instance := range c.consumerInstances {
		if err := instance.Close(); err != nil {
			return err
		}
	}
	return nil
}
