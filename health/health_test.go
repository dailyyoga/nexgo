package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func okChecker(name string) Checker {
	return NewChecker(name, 0, func(context.Context) error { return nil })
}

func failChecker(name string, err error) Checker {
	return NewChecker(name, 0, func(context.Context) error { return err })
}

func resultByName(results []Result, name string) (Result, bool) {
	for _, r := range results {
		if r.Name == name {
			return r, true
		}
	}
	return Result{}, false
}

func TestRunMixed(t *testing.T) {
	boom := errors.New("connection refused")
	ok, results := Run(context.Background(), okChecker("redis"), failChecker("mysql", boom))

	if ok {
		t.Errorf("ok = true, want false (one checker failed)")
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	r, found := resultByName(results, "redis")
	if !found || !r.OK || r.Error != "" {
		t.Errorf("redis result = %+v, want OK with no error", r)
	}
	r, found = resultByName(results, "mysql")
	if !found || r.OK || r.Error != boom.Error() {
		t.Errorf("mysql result = %+v, want failed with %q", r, boom.Error())
	}
}

func TestRunAllHealthy(t *testing.T) {
	ok, results := Run(context.Background(), okChecker("a"), okChecker("b"))
	if !ok {
		t.Errorf("ok = false, want true")
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
}

func TestRunPreservesOrder(t *testing.T) {
	_, results := Run(context.Background(), okChecker("first"), okChecker("second"), okChecker("third"))
	want := []string{"first", "second", "third"}
	for i, w := range want {
		if results[i].Name != w {
			t.Errorf("results[%d].Name = %q, want %q", i, results[i].Name, w)
		}
	}
}

func TestRunRecoversPanic(t *testing.T) {
	panicker := NewChecker("boom", 0, func(context.Context) error { panic("kaboom") })
	ok, results := Run(context.Background(), okChecker("ok"), panicker)
	if ok {
		t.Errorf("ok = true, want false (a checker panicked)")
	}
	r, _ := resultByName(results, "boom")
	if r.OK || r.Error == "" {
		t.Errorf("panicking checker result = %+v, want failed with error", r)
	}
}

func TestCheckerTimeout(t *testing.T) {
	// A checker whose work outlives its own timeout must report a deadline error.
	slow := NewChecker("slow", 10*time.Millisecond, func(ctx context.Context) error {
		select {
		case <-time.After(time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	ok, results := Run(context.Background(), slow)
	if ok {
		t.Errorf("ok = true, want false (checker timed out)")
	}
	if r := results[0]; r.OK || r.Error == "" {
		t.Errorf("slow result = %+v, want failed", r)
	}
}

func TestHandlerHealthy(t *testing.T) {
	r := gin.New()
	r.GET("/health", Handler(time.Second, okChecker("redis"), okChecker("mysql")))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body struct {
		Status       string            `json:"status"`
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("status = %q, want ok", body.Status)
	}
	if body.Dependencies["redis"] != "ok" || body.Dependencies["mysql"] != "ok" {
		t.Errorf("dependencies = %v, want all ok", body.Dependencies)
	}
}

func TestHandlerUnhealthy(t *testing.T) {
	r := gin.New()
	r.GET("/health", Handler(time.Second, okChecker("redis"), failChecker("mysql", errors.New("down"))))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
	var body struct {
		Status       string            `json:"status"`
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Status != "error" {
		t.Errorf("status = %q, want error", body.Status)
	}
	if body.Dependencies["redis"] != "ok" {
		t.Errorf("redis = %q, want ok", body.Dependencies["redis"])
	}
	if got := body.Dependencies["mysql"]; got != "error: down" {
		t.Errorf("mysql = %q, want %q", got, "error: down")
	}
}
