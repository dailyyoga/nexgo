// Package metrics provides a self-contained Prometheus registry plus the glue
// needed to expose it over HTTP.
//
// It deliberately owns an independent *prometheus.Registry rather than the
// global default one: this avoids the "everything ends up in promauto's global
// registry" trap and lets every service decide exactly which collectors it
// exposes. Built-in Go runtime / process collectors are opt-in via Options.
//
// Prometheus adapters live close to this package but are split by dependency
// weight. The core metrics package only depends on prometheus + gin, so any
// service can expose HTTP RED metrics without dragging in heavyweight
// transitive deps. Adapters that DO need such deps live in their own
// subpackages — kafkametrics (which pulls the cgo confluent-kafka-go in via
// nexgo/kafka) and dlqmetrics — so importing metrics alone never forces kafka
// into a build. Source packages (kafka, dlq, ...) stay prometheus-free and only
// expose hooks or accessors; the dependency direction is always adapter ->
// source, never the reverse.
package metrics

import (
	"errors"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Options controls which built-in collectors a Registry registers at
// construction time. The zero value disables both; use DefaultOptions to get the
// recommended setup (both enabled).
type Options struct {
	// EnableGoCollector registers the Go runtime collector (go_* metrics).
	// DefaultOptions enables it.
	EnableGoCollector bool
	// EnableProcessCollector registers the process collector (process_*
	// metrics). DefaultOptions enables it.
	EnableProcessCollector bool
}

// DefaultOptions returns the recommended Options with both the Go runtime and
// process collectors enabled.
func DefaultOptions() Options {
	return Options{
		EnableGoCollector:      true,
		EnableProcessCollector: true,
	}
}

// Registry wraps an independent *prometheus.Registry together with a cached
// OpenMetrics HTTP handler.
type Registry struct {
	registry *prometheus.Registry
	handler  http.Handler
}

// NewRegistry creates a Registry backed by a fresh *prometheus.Registry. The
// built-in Go runtime / process collectors are registered according to opts.
func NewRegistry(opts Options) *Registry {
	registry := prometheus.NewRegistry()

	if opts.EnableGoCollector {
		registry.MustRegister(collectors.NewGoCollector())
	}
	if opts.EnableProcessCollector {
		registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	}

	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})

	return &Registry{
		registry: registry,
		handler:  handler,
	}
}

// MustRegister registers the given collectors, panicking on error. It mirrors
// prometheus.Registry.MustRegister.
func (r *Registry) MustRegister(cs ...prometheus.Collector) {
	r.registry.MustRegister(cs...)
}

// Register registers a single collector, returning an error (e.g.
// prometheus.AlreadyRegisteredError) instead of panicking.
func (r *Registry) Register(c prometheus.Collector) error {
	return r.registry.Register(c)
}

// RegisterOrExisting registers c against reg, returning c on success. If an
// equivalent collector is already registered (AlreadyRegisteredError), the
// existing one is returned so repeated registration on the same registry shares
// a single collector. Any other registration error panics, mirroring
// MustRegister.
//
// It is exported so adapter subpackages (kafkametrics, dlqmetrics, ...) can
// register their metric vectors with the same idempotent semantics the RED
// middleware uses internally.
func RegisterOrExisting[C prometheus.Collector](reg *Registry, c C) C {
	if err := reg.Register(c); err != nil {
		var are prometheus.AlreadyRegisteredError
		if errors.As(err, &are) {
			if existing, ok := are.ExistingCollector.(C); ok {
				return existing
			}
		}
		panic(err)
	}
	return c
}

// Handler returns the cached OpenMetrics HTTP handler for the registry.
func (r *Registry) Handler() http.Handler {
	return r.handler
}

// Prometheus returns the underlying *prometheus.Registry so adapters in this
// package can register their metric vectors directly.
func (r *Registry) Prometheus() *prometheus.Registry {
	return r.registry
}
