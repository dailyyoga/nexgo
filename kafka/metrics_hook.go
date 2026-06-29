package kafka

import "time"

// ConsumerMetricsHook receives consume-side observability events. It is
// deliberately prometheus-free: the kafka package only emits raw events, while
// the prometheus adapter lives in nexgo/metrics (dependency direction
// metrics -> kafka, never the reverse).
//
// Implementations MUST be safe for concurrent use and MUST NOT block — the
// callbacks run inline on the consume loop, so a slow hook stalls consumption.
// A nil hook disables collection entirely (the legacy behavior).
type ConsumerMetricsHook interface {
	// OnConsume is called once per message after handling completes. err is the
	// final outcome (nil on success, non-nil when handling or commit failed);
	// latency covers handling, retries and commit.
	OnConsume(group, topic string, partition int32, err error, latency time.Duration)
	// OnLag reports the consumer lag for a single partition, parsed from
	// librdkafka statistics events. It only fires when StatisticsIntervalMs is
	// set and a hook is configured.
	OnLag(group, topic string, partition int32, lag int64)
}

// ProducerMetricsHook receives produce-side observability events. Like
// ConsumerMetricsHook it is prometheus-free and the adapter lives in
// nexgo/metrics.
//
// It is independent from and orthogonal to ProducerConfig.OnDeliveryFailure:
// that callback routes failed messages to a dead-letter sink (business
// concern), whereas this hook only measures delivery outcomes.
//
// Implementations MUST be concurrent-safe and non-blocking — the callback runs
// inline on the delivery-report loop. A nil hook disables collection.
type ProducerMetricsHook interface {
	// OnDelivery is called for every delivery report. err is nil on success and
	// the delivery error on failure; latency is the produce-to-report duration
	// (zero when it cannot be determined).
	OnDelivery(topic string, err error, latency time.Duration)
}
