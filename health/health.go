// Package health provides a tiny, dependency-light building block for liveness/
// readiness endpoints: a Checker interface, a concurrent aggregator (Run) and a
// gin handler that returns 200 when every dependency is healthy and 503 when any
// is not.
//
// It deliberately knows nothing about concrete clients. Instead of importing
// redis/mysql/clickhouse/kafka (which would drag all of them — and gin — into
// every client package), callers wrap each dependency's ping with NewChecker,
// baking in a per-dependency timeout:
//
//	checkers := []health.Checker{
//		health.NewChecker("redis", time.Second, func(ctx context.Context) error { return redis.Ping(ctx).Err() }),
//		health.NewChecker("mysql", time.Second, func(ctx context.Context) error { return dbClient.Ping(ctx) }),
//		health.NewChecker("clickhouse", 2*time.Second, func(ctx context.Context) error { return chClient.Ping(ctx) }),
//		health.NewChecker("kafka", 2*time.Second, kafkaPing),
//	}
//	r.GET("/health", health.Handler(5*time.Second, checkers...))
//
// Only stateful infrastructure dependencies should be checked — never peer
// microservices, to avoid cascading unhealthiness.
package health

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Checker reports the health of a single dependency.
type Checker interface {
	// Name identifies the dependency in the aggregated result (e.g. "redis").
	Name() string
	// Check returns nil when the dependency is healthy, or an error describing
	// why it is not. It should honor ctx cancellation/deadline.
	Check(ctx context.Context) error
}

// Result is the outcome of one Checker.
type Result struct {
	Name  string `json:"name"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// funcChecker adapts a plain function into a Checker, bounding each check with
// its own timeout so one slow dependency cannot stall the aggregate.
type funcChecker struct {
	name    string
	timeout time.Duration
	check   func(ctx context.Context) error
}

func (c funcChecker) Name() string { return c.name }

func (c funcChecker) Check(ctx context.Context) error {
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	return c.check(ctx)
}

// NewChecker builds a Checker from a name, a per-check timeout and a check
// function. A timeout <= 0 disables the per-check deadline (the parent context
// still applies).
func NewChecker(name string, timeout time.Duration, check func(ctx context.Context) error) Checker {
	return funcChecker{name: name, timeout: timeout, check: check}
}

// Run executes every checker concurrently and aggregates the outcomes. It
// returns ok=true only when all checks pass; results preserve the input order.
// A panicking checker is converted into a failed result rather than crashing the
// caller.
func Run(ctx context.Context, checkers ...Checker) (ok bool, results []Result) {
	results = make([]Result, len(checkers))

	var wg sync.WaitGroup
	for i, c := range checkers {
		wg.Add(1)
		go func(i int, c Checker) {
			defer wg.Done()
			err := runCheck(ctx, c)
			r := Result{Name: c.Name(), OK: err == nil}
			if err != nil {
				r.Error = err.Error()
			}
			results[i] = r // each goroutine owns a distinct index: no race
		}(i, c)
	}
	wg.Wait()

	ok = true
	for _, r := range results {
		if !r.OK {
			ok = false
			break
		}
	}
	return ok, results
}

// runCheck invokes c.Check, recovering any panic into an error so a misbehaving
// checker degrades to "unhealthy" instead of taking down the health endpoint.
func runCheck(ctx context.Context, c Checker) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("health: checker %q panicked: %v", c.Name(), r)
		}
	}()
	return c.Check(ctx)
}

// Handler returns a gin handler that runs the checkers and responds 200 when all
// pass or 503 when any fails. timeout (when > 0) bounds the whole health check;
// individual checkers may impose tighter deadlines of their own.
//
// The body follows the gateway convention: a top-level status plus a
// per-dependency map ("ok" on success, "error: <reason>" on failure).
func Handler(timeout time.Duration, checkers ...Checker) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}

		ok, results := Run(ctx, checkers...)

		deps := make(map[string]string, len(results))
		for _, r := range results {
			if r.OK {
				deps[r.Name] = "ok"
			} else {
				deps[r.Name] = "error: " + r.Error
			}
		}

		status := "ok"
		code := http.StatusOK
		if !ok {
			status = "error"
			code = http.StatusServiceUnavailable
		}

		c.JSON(code, gin.H{
			"status":       status,
			"dependencies": deps,
		})
	}
}
