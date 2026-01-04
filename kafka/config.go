package kafka

import (
	"fmt"
	"strings"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

// ConsumerConfig is the configuration for kafka consumer
type ConsumerConfig struct {
	// kafka connection config
	Brokers []string `mapstructure:"brokers"`
	GroupID string   `mapstructure:"group_id"`
	Topics  []string `mapstructure:"topics"`

	// Max retries for kafka consumer
	// default: 3
	MaxRetries int `mapstructure:"max_retries"`

	// Instance number for parallel processing
	// default: 1
	InstanceNum int `mapstructure:"instance_num"`

	// Auto offset reset policy: "earliest" or "latest"
	// - earliest: start from the beginning if no offset is committed
	// - latest: start from the end if no offset is committed
	// default: "latest"
	AutoOffsetReset string `mapstructure:"auto_offset_reset"`

	// Enable auto commit of offsets
	// default: false
	EnableAutoCommit bool `mapstructure:"enable_auto_commit"`

	// Auto commit interval (only used when EnableAutoCommit is true)
	// default: 5s
	AutoCommitInterval time.Duration `mapstructure:"auto_commit_interval"`

	// Session timeout
	// default: 30s
	SessionTimeout time.Duration `mapstructure:"session_timeout"`

	// Max poll interval - maximum time between two polls
	MaxPollInterval time.Duration `mapstructure:"max_poll_interval"`

	// Security protocol: "PLAINTEXT", "SASL_PLAINTEXT", "SASL_SSL"
	// only support PLAINTEXT for now
	// default: "PLAINTEXT"
	SecurityProtocol string `mapstructure:"security_protocol"`

	// Debug Model - enable consumer debug logs
	Debug bool `mapstructure:"debug"`
}

func DefaultConsumerConfig() *ConsumerConfig {
	return &ConsumerConfig{
		MaxRetries:         3,
		InstanceNum:        1,
		AutoOffsetReset:    "latest",
		EnableAutoCommit:   false,
		AutoCommitInterval: 5 * time.Second,
		SessionTimeout:     30 * time.Second,
		MaxPollInterval:    120 * time.Second,
		SecurityProtocol:   "PLAINTEXT",
		Debug:              false,
	}
}

func (c *ConsumerConfig) Validate() error {
	if len(c.Brokers) == 0 {
		return ErrInvalidConfig("brokers are required")
	}
	if c.GroupID == "" {
		return ErrInvalidConfig("group_id is required")
	}
	if len(c.Topics) == 0 {
		return ErrInvalidConfig("topics are required")
	}

	if c.AutoOffsetReset != "earliest" && c.AutoOffsetReset != "latest" {
		return ErrInvalidConfig(
			fmt.Sprintf("invalid auto_offset_reset: %s, must be either 'earliest' or 'latest'", c.AutoOffsetReset),
		)
	}

	if c.EnableAutoCommit && c.AutoCommitInterval <= 0 {
		return ErrInvalidConfig("auto_commit_interval must be greater than 0 when enable_auto_commit is true")
	}

	if c.SessionTimeout <= 0 {
		return ErrInvalidConfig("session_timeout must be greater than 0")
	}

	if c.MaxPollInterval <= 0 {
		return ErrInvalidConfig("max_poll_interval must be greater than 0")
	}

	return nil
}

func (c *ConsumerConfig) BuildConfigMap() *kafka.ConfigMap {
	configMap := &kafka.ConfigMap{
		"bootstrap.servers":    strings.Join(c.Brokers, ","),
		"group.id":             c.GroupID,
		"auto.offset.reset":    strings.ToLower(c.AutoOffsetReset), // latest, earliest
		"enable.auto.commit":   c.EnableAutoCommit,
		"session.timeout.ms":   int(c.SessionTimeout.Milliseconds()),
		"max.poll.interval.ms": int(c.MaxPollInterval.Milliseconds()),
		"security.protocol":    c.SecurityProtocol,
	}

	if c.EnableAutoCommit {
		_ = configMap.SetKey("auto.commit.interval.ms", int(c.AutoCommitInterval.Milliseconds()))
	}

	if c.Debug {
		_ = configMap.SetKey("debug", "consumer,cgrp,topic,fetch")
	}

	return configMap
}

// ProducerConfig is the configuration for kafka producer
type ProducerConfig struct {
	// kafka cluster brokers
	Brokers []string `mapstructure:"brokers"`

	// Optional: kafka client id
	// Used to identify this producer in Kafka Broker logs and metrics.
	// Suggest using meaningful names for monitoring and troubleshooting.
	ClientID string `mapstructure:"client_id"`

	// Acks message confirmation mechanism.
	// Determines the number of confirmations the Leader Broker must receive before considering the message committed.
	// - all or -1: highest reliability.
	// Leader must wait for all replicas (In-Sync Replicas, ISR) to receive the message before returning confirmation.
	// 	Highest latency, but lowest data loss risk.
	// - 1: default setting.
	// Leader returns confirmation immediately after receiving the message, without waiting for replicas.
	// 	Medium reliability, good performance.
	// - 0: lowest reliability. Producer sends without waiting for any confirmation.
	// 	Highest performance, but highest data loss risk.
	// default: "all"
	Acks string `mapstructure:"acks"`

	// Compression codec message compression algorithm. Used to compress messages sent to Kafka.
	// Can significantly reduce network bandwidth usage, but will increase client CPU load.
	// - none: do not use compression.
	// - gzip, snappy, lz4, zstd: commonly used compression algorithms.
	// 	Snappy or lz4 usually have a good balance between performance and compression rate.
	// default: "none"
	Compression string `mapstructure:"compression"`

	// LingerMs batch sending wait time (milliseconds).
	// The producer will wait for this time to collect more messages and pack them into a request to send,
	// reducing network requests and improving throughput.
	// Usually used with batch.size.
	// Suggest setting a small positive value (e.g., 5 to 100 milliseconds)
	// default: 0 (no wait, send immediately)
	LingerMs int `mapstructure:"linger_ms"`

	// Batch size maximum bytes to send. When the accumulated message size reaches this size, send immediately.
	// The batch size and linger.ms together determine the batch sending strategy.
	// default value is usually small; suggest to increase based on message size and expected throughput.
	// default: 100KB
	BatchSize int `mapstructure:"batch_size"`

	// Security protocol: "PLAINTEXT", "SASL_PLAINTEXT", "SASL_SSL"
	// only support PLAINTEXT for now
	// default: "PLAINTEXT"
	SecurityProtocol string `mapstructure:"security_protocol"`

	// Max retries for kafka producer
	// default: 3
	MaxRetries int `mapstructure:"max_retries"`
}

func DefaultProducerConfig() *ProducerConfig {
	return &ProducerConfig{
		Acks:             "all",
		Compression:      "none",
		LingerMs:         0,
		BatchSize:        100 * 1024, // 100KB
		SecurityProtocol: "PLAINTEXT",
		MaxRetries:       3,
	}
}

func (p *ProducerConfig) Validate() error {
	if len(p.Brokers) == 0 {
		return ErrInvalidConfig("brokers are required")
	}
	return nil
}

func (p *ProducerConfig) BuildConfigMap() *kafka.ConfigMap {
	configMap := &kafka.ConfigMap{
		"bootstrap.servers": strings.Join(p.Brokers, ","),
		"compression.type":  strings.ToLower(p.Compression),
		"acks":              strings.ToLower(p.Acks),
		"linger.ms":         p.LingerMs,
		"batch.size":        p.BatchSize,
		"retries":           p.MaxRetries,
		"security.protocol": p.SecurityProtocol,
	}

	if p.ClientID != "" {
		_ = configMap.SetKey("client.id", p.ClientID)
	}

	return configMap
}
