package metrics

import (
	"testing"

	"github.com/dailyyoga/nexgo/dlq"
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

func TestRegisterDLQ(t *testing.T) {
	reg := NewRegistry(Options{})
	s := &fakeStats{dropped: 7, produceErrors: 3, bufLen: 5, bufCap: 100}
	RegisterDLQ(reg, s)

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

// TestRegisterDLQNilStats verifies the skip path: a nil Stats (e.g. a Recorder
// that does not implement Stats) registers nothing and does not panic.
func TestRegisterDLQNilStats(t *testing.T) {
	reg := NewRegistry(Options{})
	RegisterDLQ(reg, nil)

	if m := findMetric(t, reg, "atlas_dlq_dropped_total", map[string]string{}); m != nil {
		t.Errorf("expected no dlq metrics registered for nil Stats, got %v", m)
	}
}
