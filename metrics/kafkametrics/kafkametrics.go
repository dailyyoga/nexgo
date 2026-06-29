// Package kafkametrics is the Prometheus adapter for the nexgo/kafka package's
// metrics hooks. It lives in its own subpackage (not in metrics) on purpose:
// importing it transitively pulls in nexgo/kafka and its cgo
// confluent-kafka-go dependency, so a service that only wants HTTP RED metrics
// can import metrics without dragging kafka into the build.
package kafkametrics

import (
	"strconv"
	"time"

	"github.com/dailyyoga/nexgo/kafka"
	"github.com/dailyyoga/nexgo/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics is the Prometheus adapter for the kafka package's metrics hooks. A
// single instance implements both kafka.ConsumerMetricsHook and
// kafka.ProducerMetricsHook, so the same value can be wired into both a
// ConsumerConfig and a ProducerConfig.
type Metrics struct {
	consumerLag      *prometheus.GaugeVec
	messagesConsumed *prometheus.CounterVec
	consumeErrors    *prometheus.CounterVec
	messagesProduced *prometheus.CounterVec
	produceErrors    *prometheus.CounterVec
	produceDuration  *prometheus.HistogramVec
}

// Compile-time proof that Metrics satisfies both hook interfaces.
var (
	_ kafka.ConsumerMetricsHook = (*Metrics)(nil)
	_ kafka.ProducerMetricsHook = (*Metrics)(nil)
)

// New builds the kafka metric vectors and registers them against reg.
// Registration is idempotent: calling it twice on the same Registry reuses the
// existing collectors instead of panicking.
func New(reg *metrics.Registry) *Metrics {
	return &Metrics{
		consumerLag: metrics.RegisterOrExisting(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "atlas_kafka_consumer_lag",
			Help: "Consumer lag (messages behind the log end) per partition.",
		}, []string{"group", "topic", "partition"})),
		messagesConsumed: metrics.RegisterOrExisting(reg, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "atlas_kafka_messages_consumed_total",
			Help: "Total number of successfully consumed messages.",
		}, []string{"group", "topic"})),
		consumeErrors: metrics.RegisterOrExisting(reg, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "atlas_kafka_consume_errors_total",
			Help: "Total number of message handling failures.",
		}, []string{"group", "topic"})),
		messagesProduced: metrics.RegisterOrExisting(reg, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "atlas_kafka_messages_produced_total",
			Help: "Total number of successfully delivered messages.",
		}, []string{"topic"})),
		produceErrors: metrics.RegisterOrExisting(reg, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "atlas_kafka_produce_errors_total",
			Help: "Total number of message delivery failures.",
		}, []string{"topic"})),
		produceDuration: metrics.RegisterOrExisting(reg, prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "atlas_kafka_produce_duration_seconds",
			Help:    "Message delivery latency in seconds (produce to delivery report).",
			Buckets: prometheus.DefBuckets,
		}, []string{"topic"})),
	}
}

// OnConsume records one consume outcome: a success increments the consumed
// counter, a failure increments the error counter. The latency argument is part
// of the hook contract but unused here (no consume-duration metric is exposed).
func (k *Metrics) OnConsume(group, topic string, _ int32, err error, _ time.Duration) {
	if err != nil {
		k.consumeErrors.WithLabelValues(group, topic).Inc()
		return
	}
	k.messagesConsumed.WithLabelValues(group, topic).Inc()
}

// OnLag sets the lag gauge for one partition. The partition number is bounded
// (one series per real partition) so it is safe as a label.
func (k *Metrics) OnLag(group, topic string, partition int32, lag int64) {
	k.consumerLag.WithLabelValues(group, topic, strconv.Itoa(int(partition))).Set(float64(lag))
}

// OnDelivery records one delivery outcome: a failure increments the error
// counter, a success increments the produced counter and (when a latency is
// available) observes the delivery duration.
func (k *Metrics) OnDelivery(topic string, err error, latency time.Duration) {
	if err != nil {
		k.produceErrors.WithLabelValues(topic).Inc()
		return
	}
	k.messagesProduced.WithLabelValues(topic).Inc()
	if latency > 0 {
		k.produceDuration.WithLabelValues(topic).Observe(latency.Seconds())
	}
}
