package metrics

import (
	"github.com/dailyyoga/nexgo/dlq"
	"github.com/prometheus/client_golang/prometheus"
)

// RegisterDLQ registers scrape-time collectors that expose a dlq recorder's
// infra counters on reg. The values are read live at scrape time via
// CounterFunc/GaugeFunc, so no background goroutine or push path is needed —
// this mirrors the "read on scrape" collector pattern used elsewhere.
//
// It registers:
//   - atlas_dlq_dropped_total        (CounterFunc -> Dropped)
//   - atlas_dlq_produce_errors_total (CounterFunc -> ProduceErrors)
//   - atlas_dlq_buffer_used          (GaugeFunc   -> BufferLen)
//   - atlas_dlq_buffer_capacity      (GaugeFunc   -> BufferCap)
//
// No "service" label is added: that is injected per pod by ARMS external_labels.
// The labelled atlas_dlq_records_total{stage,error_type} is emitted by each
// service's own internal/dlq layer, not here.
//
// s is the optional dlq.Stats accessor. A Recorder that does not implement Stats
// (e.g. the no-op recorder or a test fake) yields a nil interface, in which case
// registration is skipped. Registration is idempotent: an AlreadyRegisteredError
// is ignored so calling RegisterDLQ twice on the same registry is safe.
func RegisterDLQ(reg *Registry, s dlq.Stats) {
	if s == nil {
		return
	}

	registerOrExisting(reg, prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "atlas_dlq_dropped_total",
		Help: "Total number of dead-letter records dropped (buffer full or over byte cap).",
	}, func() float64 { return float64(s.Dropped()) }))

	registerOrExisting(reg, prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "atlas_dlq_produce_errors_total",
		Help: "Total number of dead-letter records that failed to be produced (marshal or producer error).",
	}, func() float64 { return float64(s.ProduceErrors()) }))

	registerOrExisting(reg, prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "atlas_dlq_buffer_used",
		Help: "Number of dead-letter records currently buffered awaiting delivery.",
	}, func() float64 { return float64(s.BufferLen()) }))

	registerOrExisting(reg, prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "atlas_dlq_buffer_capacity",
		Help: "Capacity of the dead-letter buffer.",
	}, func() float64 { return float64(s.BufferCap()) }))
}
