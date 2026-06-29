package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRegisterRouteServesMetrics(t *testing.T) {
	r := gin.New()
	reg := NewRegistry(DefaultOptions())
	RegisterRoute(r, reg, "")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, DefaultPath, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("expected Prometheus text Content-Type, got %q", ct)
	}

	// The exposition must be well-formed: every metric carries its HELP/TYPE
	// comments followed by samples. promhttp is the canonical producer, so a
	// structural check here is the in-test stand-in for `promtool check
	// metrics` (run separately in CI).
	body := w.Body.String()
	for _, want := range []string{
		"# HELP go_goroutines",
		"# TYPE go_goroutines gauge",
		"\ngo_goroutines ",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q in metrics output, body=%q", want, body)
		}
	}
}

func TestRegisterRouteCustomPath(t *testing.T) {
	r := gin.New()
	reg := NewRegistry(Options{})
	RegisterRoute(r, reg, "/custom/metrics")

	// Custom path serves metrics.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/custom/metrics", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on custom path, got %d", w.Code)
	}

	// Default path is not mounted when a custom one is given.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, DefaultPath, nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 on default path, got %d", w.Code)
	}
}
