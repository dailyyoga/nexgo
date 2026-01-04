package kafka

import (
	"fmt"
	"strings"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/dailyyoga/nexgo/logger"
	"go.uber.org/zap"
)

// validateKafkaCluster validates the kafka cluster connection
func validateKafkaCluster(log logger.Logger, brokers []string) error {
	configMap := &kafka.ConfigMap{
		"bootstrap.servers":  strings.Join(brokers, ","),
		"request.timeout.ms": 10000, // 10s
	}

	maxRetries := 3
	retryDelay := 2 * time.Second

	var adminClient *kafka.AdminClient
	var err error

	for i := 0; i < maxRetries; i++ {
		adminClient, err = kafka.NewAdminClient(configMap)
		if err == nil {
			break
		}

		if i < maxRetries-1 {
			log.Warn("failed to create kafka admin client, retrying...",
				zap.Error(err),
				zap.Int("attempt", i+1),
				zap.Int("max_retries", maxRetries),
			)
			time.Sleep(retryDelay)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to create kafka admin client after %d retries: %w", maxRetries, err)
	}

	defer adminClient.Close()

	// try to get cluster metadata to verify connection
	_, err = adminClient.GetMetadata(nil, false, int(10*time.Second/time.Microsecond))
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka brokers: %w", err)
	}

	log.Info("Kafka brokers connection validated", zap.Strings("brokers", brokers))
	return nil
}
