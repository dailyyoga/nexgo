// Package metrics provides a self-contained Prometheus registry plus the glue
// needed to expose it over HTTP.
//
// It deliberately owns an independent *prometheus.Registry rather than the
// global default one: this avoids the "everything ends up in promauto's global
// registry" trap and lets every service decide exactly which collectors it
// exposes. Built-in Go runtime / process collectors are opt-in via Options.
//
// This package is the single home for all Prometheus adapters in nexgo. Other
// packages (kafka, dlq, ...) stay prometheus-free and only expose hooks or
// accessors; the adapters that turn those into metrics live here, so the
// dependency direction is always metrics -> kafka/dlq, never the reverse.
package metrics

import (
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

// Handler returns the cached OpenMetrics HTTP handler for the registry.
func (r *Registry) Handler() http.Handler {
	return r.handler
}

// Prometheus returns the underlying *prometheus.Registry so adapters in this
// package can register their metric vectors directly.
func (r *Registry) Prometheus() *prometheus.Registry {
	return r.registry
}
