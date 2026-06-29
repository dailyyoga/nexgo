package metrics

import (
	"strconv"
	"time"

	"github.com/dailyyoga/nexgo/kafka"
	"github.com/prometheus/client_golang/prometheus"
)

// KafkaMetrics is the Prometheus adapter for the kafka package's metrics hooks.
// A single instance implements both kafka.ConsumerMetricsHook and
// kafka.ProducerMetricsHook, so the same value can be wired into both a
// ConsumerConfig and a ProducerConfig.
type KafkaMetrics struct {
	consumerLag      *prometheus.GaugeVec
	messagesConsumed *prometheus.CounterVec
	consumeErrors    *prometheus.CounterVec
	messagesProduced *prometheus.CounterVec
	produceErrors    *prometheus.CounterVec
	produceDuration  *prometheus.HistogramVec
}

// Compile-time proof that KafkaMetrics satisfies both hook interfaces.
var (
	_ kafka.ConsumerMetricsHook = (*KafkaMetrics)(nil)
	_ kafka.ProducerMetricsHook = (*KafkaMetrics)(nil)
)

// NewKafkaMetrics builds the kafka metric vectors and registers them against
// reg. Registration is idempotent: calling it twice on the same Registry reuses
// the existing collectors instead of panicking.
func NewKafkaMetrics(reg *Registry) *KafkaMetrics {
	return &KafkaMetrics{
		consumerLag: registerOrExisting(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "atlas_kafka_consumer_lag",
			Help: "Consumer lag (messages behind the log end) per partition.",
		}, []string{"group", "topic", "partition"})),
		messagesConsumed: registerOrExisting(reg, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "atlas_kafka_messages_consumed_total",
			Help: "Total number of successfully consumed messages.",
		}, []string{"group", "topic"})),
		consumeErrors: registerOrExisting(reg, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "atlas_kafka_consume_errors_total",
			Help: "Total number of message handling failures.",
		}, []string{"group", "topic"})),
		messagesProduced: registerOrExisting(reg, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "atlas_kafka_messages_produced_total",
			Help: "Total number of successfully delivered messages.",
		}, []string{"topic"})),
		produceErrors: registerOrExisting(reg, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "atlas_kafka_produce_errors_total",
			Help: "Total number of message delivery failures.",
		}, []string{"topic"})),
		produceDuration: registerOrExisting(reg, prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "atlas_kafka_produce_duration_seconds",
			Help:    "Message delivery latency in seconds (produce to delivery report).",
			Buckets: prometheus.DefBuckets,
		}, []string{"topic"})),
	}
}

// OnConsume records one consume outcome: a success increments the consumed
// counter, a failure increments the error counter. The latency argument is part
// of the hook contract but unused here (no consume-duration metric is exposed).
func (k *KafkaMetrics) OnConsume(group, topic string, _ int32, err error, _ time.Duration) {
	if err != nil {
		k.consumeErrors.WithLabelValues(group, topic).Inc()
		return
	}
	k.messagesConsumed.WithLabelValues(group, topic).Inc()
}

// OnLag sets the lag gauge for one partition. The partition number is bounded
// (one series per real partition) so it is safe as a label.
func (k *KafkaMetrics) OnLag(group, topic string, partition int32, lag int64) {
	k.consumerLag.WithLabelValues(group, topic, strconv.Itoa(int(partition))).Set(float64(lag))
}

// OnDelivery records one delivery outcome: a failure increments the error
// counter, a success increments the produced counter and (when a latency is
// available) observes the delivery duration.
func (k *KafkaMetrics) OnDelivery(topic string, err error, latency time.Duration) {
	if err != nil {
		k.produceErrors.WithLabelValues(topic).Inc()
		return
	}
	k.messagesProduced.WithLabelValues(topic).Inc()
	if latency > 0 {
		k.produceDuration.WithLabelValues(topic).Observe(latency.Seconds())
	}
}
