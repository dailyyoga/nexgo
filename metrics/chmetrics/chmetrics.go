// Package chmetrics is the Prometheus adapter for the nexgo/ch writer's metrics
// hook. It lives in its own subpackage (not in metrics) on purpose: importing it
// transitively pulls in nexgo/ch and the ClickHouse driver, so a service that
// only wants HTTP RED metrics can import metrics without dragging ch into the
// build. The dependency direction is always adapter -> source (chmetrics -> ch),
// never the reverse.
//
// It emits the shared atlas_ingest_* write metrics (§6.2) — event-ingest and
// user-ingest have byte-for-byte identical write surfaces, so a single adapter
// keeps them consistent and duplication-free. Services with a different metric
// namespace (e.g. dlq-ingest's atlas_dlq_ingest_*) implement ch.WriterMetricsHook
// themselves. The `service` / `env` labels are injected per pod by the
// cloud-monitoring external_labels, so they are never added here.
package chmetrics

import (
	"time"

	"github.com/dailyyoga/nexgo/ch"
	"github.com/dailyyoga/nexgo/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

// batchSizeBuckets covers a single flush's row count from a handful up to the
// writer's default FlushSize (5000) and a little beyond, so both trickle flushes
// (MinFlushSize) and full-capacity flushes land in a meaningful bucket.
var batchSizeBuckets = []float64{1, 10, 50, 100, 500, 1000, 2500, 5000, 10000}

// Metrics is the Prometheus adapter for the ch writer's flush hook. It emits the
// atlas_ingest_* family, labeled by ClickHouse table.
type Metrics struct {
	rowsWritten   *prometheus.CounterVec
	writeErrors   *prometheus.CounterVec
	flushDuration *prometheus.HistogramVec
	batchSize     *prometheus.HistogramVec
	lastFlush     *prometheus.GaugeVec
}

// Compile-time proof that Metrics satisfies the writer hook interface.
var _ ch.WriterMetricsHook = (*Metrics)(nil)

// New builds the atlas_ingest_* metric vectors and registers them against reg.
// Registration is idempotent: calling it twice on the same Registry reuses the
// existing collectors instead of panicking.
func New(reg *metrics.Registry) *Metrics {
	return &Metrics{
		rowsWritten: metrics.RegisterOrExisting(reg, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "atlas_ingest_rows_written_total",
			Help: "Total number of rows successfully written to ClickHouse, by table.",
		}, []string{"table"})),
		writeErrors: metrics.RegisterOrExisting(reg, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "atlas_ingest_write_errors_total",
			Help: "Total number of rows whose ClickHouse batch permanently failed (exhausted retries), by table.",
		}, []string{"table"})),
		flushDuration: metrics.RegisterOrExisting(reg, prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "atlas_ingest_batch_flush_duration_seconds",
			Help:    "ClickHouse batch flush (insert + retries) duration in seconds, by table.",
			Buckets: prometheus.DefBuckets,
		}, []string{"table"})),
		batchSize: metrics.RegisterOrExisting(reg, prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "atlas_ingest_batch_size",
			Help:    "Number of rows per ClickHouse batch flush, by table.",
			Buckets: batchSizeBuckets,
		}, []string{"table"})),
		lastFlush: metrics.RegisterOrExisting(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "atlas_ingest_last_flush_timestamp",
			Help: "Unix timestamp of the last successful ClickHouse batch flush, by table.",
		}, []string{"table"})),
	}
}

// OnFlush implements ch.WriterMetricsHook. Every flush observes its batch size
// and duration; a successful flush advances rows_written and the last-flush
// timestamp, a failed one advances write_errors (the batch is all-or-nothing —
// see WriterMetricsHook).
func (m *Metrics) OnFlush(table ch.TableName, rows int, duration time.Duration, err error) {
	t := string(table)
	m.batchSize.WithLabelValues(t).Observe(float64(rows))
	m.flushDuration.WithLabelValues(t).Observe(duration.Seconds())
	if err != nil {
		m.writeErrors.WithLabelValues(t).Add(float64(rows))
		return
	}
	m.rowsWritten.WithLabelValues(t).Add(float64(rows))
	m.lastFlush.WithLabelValues(t).Set(float64(time.Now().Unix()))
}

// RegisterBuffer registers a scrape-time gauge (atlas_ingest_buffer_rows) that
// exposes the writer's ingress-buffer occupancy. The buffer is a single figure
// across all tables (the writer's ingress channel is not partitioned by table),
// so this gauge is intentionally unlabeled — a documented deviation from the
// per-table {table} label sketched in the plan. A writer that does not implement
// ch.WriterStats yields a nil accessor, in which case registration is skipped.
// Registration is idempotent.
func RegisterBuffer(reg *metrics.Registry, s ch.WriterStats) {
	if s == nil {
		return
	}
	metrics.RegisterOrExisting(reg, prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "atlas_ingest_buffer_rows",
		Help: "Rows currently queued in the writer's ingress buffer awaiting flush (across all tables).",
	}, func() float64 { return float64(s.BufferLen()) }))
}
