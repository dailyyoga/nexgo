package metrics

import (
	"errors"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// gatheredFamilyNames returns the names of all metric families currently
// exposed by the registry.
func gatheredFamilyNames(t *testing.T, reg *Registry) []string {
	t.Helper()
	mfs, err := reg.Prometheus().Gather()
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}
	names := make([]string, 0, len(mfs))
	for _, mf := range mfs {
		names = append(names, mf.GetName())
	}
	return names
}

func hasPrefix(names []string, prefix string) bool {
	for _, n := range names {
		if strings.HasPrefix(n, prefix) {
			return true
		}
	}
	return false
}

// isAlreadyRegistered reports whether err signals that an equivalent collector
// is already present in the registry. We use this to assert registration in a
// platform-independent way: the process collector does not emit process_*
// samples on non-Linux platforms (e.g. macOS dev machines), so Gather() alone
// cannot prove it was registered.
func isAlreadyRegistered(err error) bool {
	var are prometheus.AlreadyRegisteredError
	return errors.As(err, &are)
}

func TestNewRegistryDefaultCollectors(t *testing.T) {
	reg := NewRegistry(DefaultOptions())

	// go_* metrics are emitted on every platform, so Gather proves them.
	if names := gatheredFamilyNames(t, reg); !hasPrefix(names, "go_") {
		t.Errorf("expected go_* metric families, got %v", names)
	}

	// Re-registering an equivalent collector must fail with
	// AlreadyRegisteredError, proving both collectors are in the registry.
	if err := reg.Register(collectors.NewGoCollector()); !isAlreadyRegistered(err) {
		t.Errorf("go collector not registered: got err %v", err)
	}
	if err := reg.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})); !isAlreadyRegistered(err) {
		t.Errorf("process collector not registered: got err %v", err)
	}
}

func TestNewRegistryCollectorsDisabled(t *testing.T) {
	reg := NewRegistry(Options{})

	if names := gatheredFamilyNames(t, reg); hasPrefix(names, "go_") || hasPrefix(names, "process_") {
		t.Errorf("expected no go_*/process_* families when disabled, got %v", names)
	}

	// With collectors disabled, registering them fresh must succeed (i.e. they
	// were not already present).
	if err := reg.Register(collectors.NewGoCollector()); err != nil {
		t.Errorf("go collector unexpectedly already registered: %v", err)
	}
	if err := reg.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})); err != nil {
		t.Errorf("process collector unexpectedly already registered: %v", err)
	}
}

func TestRegistryRegisterAndGather(t *testing.T) {
	reg := NewRegistry(Options{})

	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "atlas_test_total",
		Help: "test counter",
	})
	if err := reg.Register(counter); err != nil {
		t.Fatalf("Register() error: %v", err)
	}
	counter.Inc()

	if names := gatheredFamilyNames(t, reg); !hasPrefix(names, "atlas_test_total") {
		t.Errorf("expected atlas_test_total family, got %v", names)
	}

	// Registering the same collector again must be rejected.
	if err := reg.Register(counter); !isAlreadyRegistered(err) {
		t.Errorf("expected AlreadyRegisteredError on duplicate Register, got %v", err)
	}
}

func TestRegistryMustRegister(t *testing.T) {
	reg := NewRegistry(Options{})
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "atlas_must_register_total",
		Help: "test counter",
	})

	reg.MustRegister(counter)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected MustRegister to panic on duplicate registration")
		}
	}()
	reg.MustRegister(counter)
}
