package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	dto "github.com/prometheus/client_model/go"
)

// findMetric returns the first sample of family `name` whose labels are a
// superset of `want`, or nil if none matches.
func findMetric(t *testing.T, reg *Registry, name string, want map[string]string) *dto.Metric {
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

func gaugeValue(t *testing.T, reg *Registry, name string) float64 {
	t.Helper()
	mfs, err := reg.Prometheus().Gather()
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() == name {
			return mf.Metric[0].GetGauge().GetValue()
		}
	}
	t.Fatalf("gauge %s not found", name)
	return 0
}

func TestREDUsesTemplateRoute(t *testing.T) {
	reg := NewRegistry(Options{})
	r := gin.New()
	r.Use(RED(reg))
	r.GET("/x/:id", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x/123", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Series carries the template route, not the concrete path.
	if m := findMetric(t, reg, "atlas_http_requests_total", map[string]string{
		"method": "GET", "route": "/x/:id", "status": "200",
	}); m == nil || m.GetCounter().GetValue() != 1 {
		t.Errorf("expected requests_total{route=/x/:id}==1, got %v", m)
	}
	// The concrete path must NOT appear as a label (high-cardinality guard).
	if m := findMetric(t, reg, "atlas_http_requests_total", map[string]string{"route": "/x/123"}); m != nil {
		t.Errorf("did not expect a series with route=/x/123, got %v", m)
	}
	// Duration is recorded under the template route too.
	if m := findMetric(t, reg, "atlas_http_request_duration_seconds", map[string]string{
		"method": "GET", "route": "/x/:id",
	}); m == nil || m.GetHistogram().GetSampleCount() != 1 {
		t.Errorf("expected duration sample for route=/x/:id, got %v", m)
	}
}

func TestREDSkipsHealth(t *testing.T) {
	reg := NewRegistry(Options{})
	r := gin.New()
	r.Use(RED(reg)) // default skip: /health,/ping,/metrics
	r.GET("/health", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if m := findMetric(t, reg, "atlas_http_requests_total", map[string]string{"route": "/health"}); m != nil {
		t.Errorf("/health should not produce a series, got %v", m)
	}
}

func TestREDCountsServerErrors(t *testing.T) {
	reg := NewRegistry(Options{})
	r := gin.New()
	r.Use(RED(reg))
	r.GET("/boom", func(c *gin.Context) { c.String(http.StatusInternalServerError, "boom") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	if m := findMetric(t, reg, "atlas_http_requests_total", map[string]string{
		"method": "GET", "route": "/boom", "status": "500",
	}); m == nil || m.GetCounter().GetValue() != 1 {
		t.Errorf("expected requests_total{status=500}==1, got %v", m)
	}
}

func TestREDUnmatchedRoute(t *testing.T) {
	reg := NewRegistry(Options{})
	r := gin.New()
	r.Use(RED(reg))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/does/not/exist", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	if m := findMetric(t, reg, "atlas_http_requests_total", map[string]string{
		"route": unmatchedRoute, "status": "404",
	}); m == nil || m.GetCounter().GetValue() != 1 {
		t.Errorf("expected requests_total{route=<unmatched>,status=404}==1, got %v", m)
	}
}

func TestREDInFlight(t *testing.T) {
	reg := NewRegistry(Options{})
	r := gin.New()
	r.Use(RED(reg))

	entered := make(chan struct{})
	release := make(chan struct{})
	r.GET("/slow", func(c *gin.Context) {
		close(entered)
		<-release
		c.String(http.StatusOK, "ok")
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/slow", nil))
	}()

	<-entered
	if v := gaugeValue(t, reg, "atlas_http_requests_in_flight"); v != 1 {
		t.Errorf("in_flight during request = %v, want 1", v)
	}

	close(release)
	<-done
	if v := gaugeValue(t, reg, "atlas_http_requests_in_flight"); v != 0 {
		t.Errorf("in_flight after request = %v, want 0", v)
	}
}

func TestREDIdempotentRegistration(t *testing.T) {
	reg := NewRegistry(Options{})
	// Calling RED twice on the same registry must not panic on duplicate
	// registration; both middlewares share the same collectors.
	mw1 := RED(reg)
	mw2 := RED(reg)
	if mw1 == nil || mw2 == nil {
		t.Fatal("RED returned nil middleware")
	}

	r := gin.New()
	r.Use(mw2)
	r.GET("/ok", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", nil))

	if m := findMetric(t, reg, "atlas_http_requests_total", map[string]string{"route": "/ok"}); m == nil {
		t.Error("expected series after second RED middleware recorded a request")
	}
}

func TestREDCustomSkipReplacesDefaults(t *testing.T) {
	reg := NewRegistry(Options{})
	r := gin.New()
	r.Use(RED(reg, "/secret")) // custom skip replaces defaults entirely
	r.GET("/secret", func(c *gin.Context) { c.String(http.StatusOK, "s") })
	r.GET("/health", func(c *gin.Context) { c.String(http.StatusOK, "h") })

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/secret", nil))
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))

	// /secret is skipped...
	if m := findMetric(t, reg, "atlas_http_requests_total", map[string]string{"route": "/secret"}); m != nil {
		t.Errorf("/secret should be skipped, got %v", m)
	}
	// ...but /health is now counted because defaults were replaced.
	if m := findMetric(t, reg, "atlas_http_requests_total", map[string]string{"route": "/health"}); m == nil {
		t.Error("/health should be counted when custom skip replaces defaults")
	}
}
