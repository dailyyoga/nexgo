package dlqmetrics

import (
	"testing"

	"github.com/dailyyoga/nexgo/dlq"
	"github.com/dailyyoga/nexgo/metrics"
	dto "github.com/prometheus/client_model/go"
)

// fakeStats is a dlq.Stats whose values can be mutated to verify scrape-time
// reads.
type fakeStats struct {
	dropped       uint64
	produceErrors uint64
	bufLen        int
	bufCap        int
}

func (f *fakeStats) Dropped() uint64       { return f.dropped }
func (f *fakeStats) ProduceErrors() uint64 { return f.produceErrors }
func (f *fakeStats) BufferLen() int        { return f.bufLen }
func (f *fakeStats) BufferCap() int        { return f.bufCap }

var _ dlq.Stats = (*fakeStats)(nil)

func TestRegister(t *testing.T) {
	reg := metrics.NewRegistry(metrics.Options{})
	s := &fakeStats{dropped: 7, produceErrors: 3, bufLen: 5, bufCap: 100}
	Register(reg, s)

	none := map[string]string{}
	check := func(name string, want float64, get func() float64) {
		t.Helper()
		if got := get(); got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}

	check("atlas_dlq_dropped_total", 7, func() float64 {
		return findMetric(t, reg, "atlas_dlq_dropped_total", none).GetCounter().GetValue()
	})
	check("atlas_dlq_produce_errors_total", 3, func() float64 {
		return findMetric(t, reg, "atlas_dlq_produce_errors_total", none).GetCounter().GetValue()
	})
	check("atlas_dlq_buffer_used", 5, func() float64 {
		return findMetric(t, reg, "atlas_dlq_buffer_used", none).GetGauge().GetValue()
	})
	check("atlas_dlq_buffer_capacity", 100, func() float64 {
		return findMetric(t, reg, "atlas_dlq_buffer_capacity", none).GetGauge().GetValue()
	})

	// Values are read live at scrape time, so mutating the source is reflected.
	s.dropped = 9
	s.bufLen = 42
	check("atlas_dlq_dropped_total (live)", 9, func() float64 {
		return findMetric(t, reg, "atlas_dlq_dropped_total", none).GetCounter().GetValue()
	})
	check("atlas_dlq_buffer_used (live)", 42, func() float64 {
		return findMetric(t, reg, "atlas_dlq_buffer_used", none).GetGauge().GetValue()
	})
}

// TestRegisterNilStats verifies the skip path: a nil Stats (e.g. a Recorder that
// does not implement Stats) registers nothing and does not panic.
func TestRegisterNilStats(t *testing.T) {
	reg := metrics.NewRegistry(metrics.Options{})
	Register(reg, nil)

	if m := findMetric(t, reg, "atlas_dlq_dropped_total", map[string]string{}); m != nil {
		t.Errorf("expected no dlq metrics registered for nil Stats, got %v", m)
	}
}

// findMetric returns the first sample of family `name` whose labels are a
// superset of `want`, or nil if none matches. It reads through the exported
// Registry.Prometheus() gatherer so the helper needs no access to metrics
// package internals.
func findMetric(t *testing.T, reg *metrics.Registry, name string, want map[string]string) *dto.Metric {
	t.Helper()
	mfs, err := reg.Prometheus().Gather()
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.Metric {
			if labelsContain(m, want) {
				return m
			}
		}
	}
	return nil
}

func labelsContain(m *dto.Metric, want map[string]string) bool {
	got := make(map[string]string, len(m.Label))
	for _, lp := range m.Label {
		got[lp.GetName()] = lp.GetValue()
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}
