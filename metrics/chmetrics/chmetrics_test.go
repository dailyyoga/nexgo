package chmetrics

import (
	"errors"
	"testing"
	"time"

	"github.com/dailyyoga/nexgo/ch"
	"github.com/dailyyoga/nexgo/metrics"
	dto "github.com/prometheus/client_model/go"
)

// TestOnFlush verifies that a successful flush advances rows_written and the
// last-flush timestamp while a failed flush advances write_errors; both always
// observe batch size and duration.
func TestOnFlush(t *testing.T) {
	reg := metrics.NewRegistry(metrics.Options{})
	m := New(reg)

	// two successful "events" flushes (10 + 5 rows) and one failed one (3 rows)
	m.OnFlush("events", 10, 20*time.Millisecond, nil)
	m.OnFlush("events", 5, 5*time.Millisecond, nil)
	m.OnFlush("events", 3, time.Millisecond, errors.New("boom"))
	// a separate table stays isolated by the {table} label
	m.OnFlush("user_devices", 7, 2*time.Millisecond, nil)

	events := map[string]string{"table": "events"}
	userDevices := map[string]string{"table": "user_devices"}

	if got := counter(t, reg, "atlas_ingest_rows_written_total", events); got != 15 {
		t.Errorf("rows_written{events} = %v, want 15", got)
	}
	if got := counter(t, reg, "atlas_ingest_write_errors_total", events); got != 3 {
		t.Errorf("write_errors{events} = %v, want 3", got)
	}
	if got := counter(t, reg, "atlas_ingest_rows_written_total", userDevices); got != 7 {
		t.Errorf("rows_written{user_devices} = %v, want 7", got)
	}
	// user_devices never failed, so its write_errors series must be absent.
	if m := findMetric(t, reg, "atlas_ingest_write_errors_total", userDevices); m != nil {
		t.Errorf("write_errors{user_devices} should be absent, got %v", m)
	}

	// batch_size histogram: 3 events flushes observed (10,5,3) => count 3, sum 18.
	if h := findMetric(t, reg, "atlas_ingest_batch_size", events).GetHistogram(); h.GetSampleCount() != 3 || h.GetSampleSum() != 18 {
		t.Errorf("batch_size{events} count/sum = %d/%v, want 3/18", h.GetSampleCount(), h.GetSampleSum())
	}
	// flush_duration histogram: 3 events flushes observed.
	if h := findMetric(t, reg, "atlas_ingest_batch_flush_duration_seconds", events).GetHistogram(); h.GetSampleCount() != 3 {
		t.Errorf("flush_duration{events} count = %d, want 3", h.GetSampleCount())
	}
	// last_flush_timestamp advances on success (non-zero after two successful flushes).
	if got := gauge(t, reg, "atlas_ingest_last_flush_timestamp", events); got <= 0 {
		t.Errorf("last_flush_timestamp{events} = %v, want > 0", got)
	}
}

// fakeStats is a ch.WriterStats whose buffer length can be mutated to verify the
// scrape-time gauge reads live.
type fakeStats struct{ n int }

func (f *fakeStats) BufferLen() int { return f.n }

var _ ch.WriterStats = (*fakeStats)(nil)

// TestRegisterBuffer verifies the buffer gauge reads BufferLen live at scrape
// time and that a nil accessor registers nothing.
func TestRegisterBuffer(t *testing.T) {
	reg := metrics.NewRegistry(metrics.Options{})
	s := &fakeStats{n: 12}
	RegisterBuffer(reg, s)

	none := map[string]string{}
	if got := gauge(t, reg, "atlas_ingest_buffer_rows", none); got != 12 {
		t.Errorf("buffer_rows = %v, want 12", got)
	}
	s.n = 99 // read live at scrape time
	if got := gauge(t, reg, "atlas_ingest_buffer_rows", none); got != 99 {
		t.Errorf("buffer_rows (live) = %v, want 99", got)
	}

	regNil := metrics.NewRegistry(metrics.Options{})
	RegisterBuffer(regNil, nil)
	if m := findMetric(t, regNil, "atlas_ingest_buffer_rows", none); m != nil {
		t.Errorf("nil stats should register no buffer gauge, got %v", m)
	}
}

func counter(t *testing.T, reg *metrics.Registry, name string, want map[string]string) float64 {
	t.Helper()
	m := findMetric(t, reg, name, want)
	if m == nil {
		t.Fatalf("metric %s%v not found", name, want)
	}
	return m.GetCounter().GetValue()
}

func gauge(t *testing.T, reg *metrics.Registry, name string, want map[string]string) float64 {
	t.Helper()
	m := findMetric(t, reg, name, want)
	if m == nil {
		t.Fatalf("metric %s%v not found", name, want)
	}
	return m.GetGauge().GetValue()
}

// findMetric returns the first sample of family `name` whose labels are a
// superset of `want`, or nil if none matches.
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
