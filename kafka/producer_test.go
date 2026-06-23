package kafka

import (
	"errors"
	"testing"
	"time"

	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

func TestFromKafkaMessage(t *testing.T) {
	topic := "atlas-events"
	ts := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	ev := &ckafka.Message{
		Value:     []byte(`{"foo":"bar"}`),
		Key:       []byte("u_10086"),
		Timestamp: ts,
		TopicPartition: ckafka.TopicPartition{
			Topic:     &topic,
			Partition: 3,
			Offset:    42,
			Error:     errors.New("delivery failed"),
		},
		Headers: []ckafka.Header{
			{Key: "x-atlas-source", Value: []byte("atlas-gateway")},
		},
	}

	msg := fromKafkaMessage(ev)

	if string(msg.Value) != `{"foo":"bar"}` {
		t.Errorf("Value = %q, want %q", msg.Value, `{"foo":"bar"}`)
	}
	if string(msg.Key) != "u_10086" {
		t.Errorf("Key = %q, want %q", msg.Key, "u_10086")
	}
	if !msg.Timestamp.Equal(ts) {
		t.Errorf("Timestamp = %v, want %v", msg.Timestamp, ts)
	}
	if msg.TopicPartition.Topic == nil || *msg.TopicPartition.Topic != topic {
		t.Errorf("Topic = %v, want %q", msg.TopicPartition.Topic, topic)
	}
	// the converted topic must be a copy, not the librdkafka-owned pointer
	if msg.TopicPartition.Topic == ev.TopicPartition.Topic {
		t.Error("Topic pointer was not copied; it aliases the confluent message")
	}
	if msg.TopicPartition.Partition != 3 {
		t.Errorf("Partition = %d, want 3", msg.TopicPartition.Partition)
	}
	if msg.TopicPartition.Offset != 42 {
		t.Errorf("Offset = %d, want 42", msg.TopicPartition.Offset)
	}
	if len(msg.Headers) != 1 || msg.Headers[0].Key != "x-atlas-source" ||
		string(msg.Headers[0].Value) != "atlas-gateway" {
		t.Errorf("Headers = %+v, want one x-atlas-source header", msg.Headers)
	}
}

func TestFromKafkaMessage_NilTopicNoHeaders(t *testing.T) {
	ev := &ckafka.Message{
		Value:          []byte("v"),
		TopicPartition: ckafka.TopicPartition{Topic: nil, Partition: 0, Offset: 0},
	}

	msg := fromKafkaMessage(ev)

	if msg.TopicPartition.Topic != nil {
		t.Errorf("Topic = %v, want nil", msg.TopicPartition.Topic)
	}
	if msg.Headers != nil {
		t.Errorf("Headers = %v, want nil", msg.Headers)
	}
}

// TestNotifyDeliveryFailure_NilCallback verifies the no-op path: with no callback
// configured, notifyDeliveryFailure must not panic (it is hit on every failed
// delivery in the legacy log-only mode).
func TestNotifyDeliveryFailure_NilCallback(t *testing.T) {
	kp := &defaultProducer{onDeliveryFailure: nil}
	topic := "t"
	kp.notifyDeliveryFailure(&ckafka.Message{
		TopicPartition: ckafka.TopicPartition{Topic: &topic, Error: errors.New("boom")},
	})
}
