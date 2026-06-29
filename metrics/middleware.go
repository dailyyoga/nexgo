package metrics

import (
	"errors"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

// unmatchedRoute is the route label used when gin matched no template route
// (e.g. a 404). Using a constant keeps the {route} label cardinality bounded:
// the raw request path is NEVER used as a label.
const unmatchedRoute = "<unmatched>"

// defaultSkipPaths are excluded from RED accounting unless the caller overrides
// the skip list. These are health/liveness probes and the scrape endpoint
// itself — counting them would only add noise.
var defaultSkipPaths = []string{"/health", "/ping", "/metrics"}

// durationBuckets matches the Prometheus default histogram buckets, tuned for
// sub-second to ~10s HTTP latencies.
var durationBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

// responseSizeBuckets covers HTTP response bodies from ~100B to ~1GB on a
// power-of-ten scale.
var responseSizeBuckets = prometheus.ExponentialBuckets(100, 10, 8)

// httpMetrics holds the four RED collectors. They are registered against a
// Registry once (idempotently) when the middleware is constructed, never per
// request.
type httpMetrics struct {
	requests     *prometheus.CounterVec
	duration     *prometheus.HistogramVec
	inFlight     prometheus.Gauge
	responseSize *prometheus.HistogramVec
}

// newHTTPMetrics builds the RED collectors and registers them against reg.
// Registration is idempotent: calling RED more than once on the same Registry
// reuses the already-registered collectors instead of panicking, and distinct
// registries each get their own set.
func newHTTPMetrics(reg *Registry) *httpMetrics {
	return &httpMetrics{
		requests: registerOrExisting(reg, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "atlas_http_requests_total",
			Help: "Total number of HTTP requests.",
		}, []string{"method", "route", "status"})),
		duration: registerOrExisting(reg, prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "atlas_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: durationBuckets,
		}, []string{"method", "route"})),
		inFlight: registerOrExisting(reg, prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "atlas_http_requests_in_flight",
			Help: "Number of HTTP requests currently being served.",
		})),
		responseSize: registerOrExisting(reg, prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "atlas_http_response_size_bytes",
			Help:    "HTTP response size in bytes.",
			Buckets: responseSizeBuckets,
		}, []string{"route"})),
	}
}

// registerOrExisting registers c against reg, returning c on success. If an
// equivalent collector is already registered (AlreadyRegisteredError), the
// existing one is returned so repeated RED calls on the same registry share a
// single collector. Any other registration error panics, mirroring
// MustRegister.
func registerOrExisting[C prometheus.Collector](reg *Registry, c C) C {
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

// RED returns a gin middleware that records the four §6.1 HTTP metrics against
// reg: request count, latency, in-flight count and response size. The metrics
// are registered (idempotently) when RED is called, not per request.
//
// Requests whose path is in the skip list are not accounted. When skip is empty
// the defaults (/health, /ping, /metrics) are used; passing any value replaces
// the defaults entirely.
//
// The {route} label is always derived from c.FullPath() (the template path,
// e.g. "/x/:id"), falling back to "<unmatched>" for unrouted requests. The raw
// URL path is never used as a label to keep cardinality bounded.
func RED(reg *Registry, skip ...string) gin.HandlerFunc {
	m := newHTTPMetrics(reg)

	if len(skip) == 0 {
		skip = defaultSkipPaths
	}
	skipSet := make(map[string]struct{}, len(skip))
	for _, p := range skip {
		skipSet[p] = struct{}{}
	}

	return func(c *gin.Context) {
		if _, skipped := skipSet[c.Request.URL.Path]; skipped {
			c.Next()
			return
		}

		start := time.Now()
		m.inFlight.Inc()
		defer m.inFlight.Dec()

		c.Next()

		route := c.FullPath()
		if route == "" {
			route = unmatchedRoute
		}
		method := c.Request.Method
		status := strconv.Itoa(c.Writer.Status())

		m.requests.WithLabelValues(method, route, status).Inc()
		m.duration.WithLabelValues(method, route).Observe(time.Since(start).Seconds())

		size := max(c.Writer.Size(), 0) // gin returns -1 when nothing was written
		m.responseSize.WithLabelValues(route).Observe(float64(size))
	}
}
