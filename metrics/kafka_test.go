package metrics

import (
	"errors"
	"testing"
	"time"
)

func TestKafkaMetricsRecordsSeries(t *testing.T) {
	reg := NewRegistry(Options{})
	km := NewKafkaMetrics(reg)

	// Consume: one success, one failure.
	km.OnConsume("cg", "t1", 0, nil, 5*time.Millisecond)
	km.OnConsume("cg", "t1", 0, errors.New("handler boom"), time.Millisecond)
	// Lag for a partition.
	km.OnLag("cg", "t1", 3, 42)
	// Produce: one success (with latency), one failure.
	km.OnDelivery("t1", nil, 10*time.Millisecond)
	km.OnDelivery("t1", errors.New("delivery boom"), 0)

	// Consumed / consume errors.
	if m := findMetric(t, reg, "atlas_kafka_messages_consumed_total", map[string]string{"group": "cg", "topic": "t1"}); m == nil || m.GetCounter().GetValue() != 1 {
		t.Errorf("messages_consumed_total = %v, want 1", m)
	}
	if m := findMetric(t, reg, "atlas_kafka_consume_errors_total", map[string]string{"group": "cg", "topic": "t1"}); m == nil || m.GetCounter().GetValue() != 1 {
		t.Errorf("consume_errors_total = %v, want 1", m)
	}

	// Lag gauge carries the partition label and the set value.
	if m := findMetric(t, reg, "atlas_kafka_consumer_lag", map[string]string{"group": "cg", "topic": "t1", "partition": "3"}); m == nil || m.GetGauge().GetValue() != 42 {
		t.Errorf("consumer_lag{partition=3} = %v, want 42", m)
	}

	// Produced / produce errors.
	if m := findMetric(t, reg, "atlas_kafka_messages_produced_total", map[string]string{"topic": "t1"}); m == nil || m.GetCounter().GetValue() != 1 {
		t.Errorf("messages_produced_total = %v, want 1", m)
	}
	if m := findMetric(t, reg, "atlas_kafka_produce_errors_total", map[string]string{"topic": "t1"}); m == nil || m.GetCounter().GetValue() != 1 {
		t.Errorf("produce_errors_total = %v, want 1", m)
	}

	// Produce duration observed exactly once (only the successful delivery).
	if m := findMetric(t, reg, "atlas_kafka_produce_duration_seconds", map[string]string{"topic": "t1"}); m == nil || m.GetHistogram().GetSampleCount() != 1 {
		t.Errorf("produce_duration sample count = %v, want 1", m)
	}
}

// TestKafkaMetricsNoDurationWithoutLatency verifies the degrade-to-count path:
// a successful delivery with zero latency (Opaque unavailable) is counted but
// records no duration sample.
func TestKafkaMetricsNoDurationWithoutLatency(t *testing.T) {
	reg := NewRegistry(Options{})
	km := NewKafkaMetrics(reg)

	km.OnDelivery("t2", nil, 0)

	if m := findMetric(t, reg, "atlas_kafka_messages_produced_total", map[string]string{"topic": "t2"}); m == nil || m.GetCounter().GetValue() != 1 {
		t.Errorf("messages_produced_total = %v, want 1", m)
	}
	// No duration family for t2 (never observed).
	if m := findMetric(t, reg, "atlas_kafka_produce_duration_seconds", map[string]string{"topic": "t2"}); m != nil {
		t.Errorf("expected no produce_duration sample for t2, got %v", m)
	}
}
