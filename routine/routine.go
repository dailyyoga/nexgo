// Package routine provides safe goroutine execution with panic recovery.
//
// It prevents direct use of `go func()` from crashing the entire application
// when a panic occurs, by wrapping goroutine execution with recovery logic.
package routine

import (
	"context"
	"runtime/debug"
	"sync"

	"github.com/dailyyoga/nexgo/logger"
	"go.uber.org/zap"
)

// Runner provides safe goroutine execution with panic recovery
type Runner interface {
	// Go executes a function in a new goroutine with panic recovery
	Go(fn func())

	// GoWithContext executes a function in a new goroutine with context and panic recovery
	GoWithContext(ctx context.Context, fn func(ctx context.Context))

	// GoNamed executes a named function in a new goroutine with panic recovery
	// The name is used for logging purposes
	GoNamed(name string, fn func())

	// GoNamedWithContext executes a named function with context in a new goroutine
	GoNamedWithContext(ctx context.Context, name string, fn func(ctx context.Context))

	// Wait waits for all goroutines started by this runner to complete
	Wait()
}

// defaultRunner implements Runner interface
type defaultRunner struct {
	log logger.Logger
	wg  sync.WaitGroup
}

// New creates a new Runner with the given logger
func New(log logger.Logger) Runner {
	return &defaultRunner{
		log: log,
	}
}

// Go executes a function in a new goroutine with panic recovery
func (r *defaultRunner) Go(fn func()) {
	r.GoNamed("", fn)
}

// GoWithContext executes a function in a new goroutine with context and panic recovery
func (r *defaultRunner) GoWithContext(ctx context.Context, fn func(ctx context.Context)) {
	r.GoNamedWithContext(ctx, "", fn)
}

// GoNamed executes a named function in a new goroutine with panic recovery
func (r *defaultRunner) GoNamed(name string, fn func()) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		defer r.recover(name)
		fn()
	}()
}

// GoNamedWithContext executes a named function with context in a new goroutine
func (r *defaultRunner) GoNamedWithContext(ctx context.Context, name string, fn func(ctx context.Context)) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		defer r.recover(name)
		fn(ctx)
	}()
}

// Wait waits for all goroutines started by this runner to complete
func (r *defaultRunner) Wait() {
	r.wg.Wait()
}

// recover handles panic recovery and logging
func (r *defaultRunner) recover(name string) {
	if rec := recover(); rec != nil {
		fields := []zap.Field{
			zap.Any("panic", rec),
			zap.String("stack", string(debug.Stack())),
		}
		if name != "" {
			fields = append([]zap.Field{zap.String("routine", name)}, fields...)
		}
		r.log.Error("goroutine panicked", fields...)
	}
}

// Go is a convenience function that executes a function in a new goroutine with panic recovery
// It uses the provided logger for error logging
func Go(log logger.Logger, fn func()) {
	go func() {
		defer recoverWithLog(log, "")
		fn()
	}()
}

// GoWithContext is a convenience function that executes a function with context
// in a new goroutine with panic recovery
func GoWithContext(ctx context.Context, log logger.Logger, fn func(ctx context.Context)) {
	go func() {
		defer recoverWithLog(log, "")
		fn(ctx)
	}()
}

// GoNamed is a convenience function that executes a named function
// in a new goroutine with panic recovery
func GoNamed(log logger.Logger, name string, fn func()) {
	go func() {
		defer recoverWithLog(log, name)
		fn()
	}()
}

// GoNamedWithContext is a convenience function that executes a named function
// with context in a new goroutine with panic recovery
func GoNamedWithContext(ctx context.Context, log logger.Logger, name string, fn func(ctx context.Context)) {
	go func() {
		defer recoverWithLog(log, name)
		fn(ctx)
	}()
}

// recoverWithLog handles panic recovery and logging for standalone functions
func recoverWithLog(log logger.Logger, name string) {
	if rec := recover(); rec != nil {
		fields := []zap.Field{
			zap.Any("panic", rec),
			zap.String("stack", string(debug.Stack())),
		}
		if name != "" {
			fields = append([]zap.Field{zap.String("routine", name)}, fields...)
		}
		log.Error("goroutine panicked", fields...)
	}
}
