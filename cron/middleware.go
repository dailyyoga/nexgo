package cron

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/dailyyoga/nexgo/logger"
	"go.uber.org/zap"
)

// Middleware is a function that wraps a Task with additional behavior
// Middlewares can be used for logging, recovery, metrics, etc.
type Middleware func(Task) Task

// applyMiddlewares applies multiple middlewares to a task
// Middlewares are applied from last to first, ensuring execution order is intuitive
// Example: applyMiddlewares(task, mw1, mw2, mw3) results in: mw1(mw2(mw3(task)))
func applyMiddlewares(t Task, mws ...Middleware) Task {
	for i := len(mws) - 1; i >= 0; i-- {
		t = mws[i](t)
	}
	return t
}

// recoveryMiddleware wraps a task with panic recovery
// If a task panics, the panic is recovered and logged, preventing the entire process from crashing
// The panic is converted to an error and returned to the caller
func recoveryMiddleware(log logger.Logger) Middleware {
	return func(next Task) Task {
		return &wrappedTask{
			name: next.Name(),
			exec: func(ctx context.Context) (err error) {
				defer func() {
					if r := recover(); r != nil {
						log.Error("task panicked",
							zap.String("task", next.Name()),
							zap.Any("panic", r),
							zap.String("stack", string(debug.Stack())),
						)
						err = fmt.Errorf("panic recovered: %v", r)
					}
				}()
				return next.Run(ctx)
			},
		}
	}
}

// loggingMiddleware wraps a task with logging for start, finish, and errors
// It logs the task name, execution duration, and any errors that occur
func loggingMiddleware(log logger.Logger) Middleware {
	return func(next Task) Task {
		return &wrappedTask{
			name: next.Name(),
			exec: func(ctx context.Context) error {
				start := time.Now()
				log.Info("task started", zap.String("task", next.Name()))

				err := next.Run(ctx)

				duration := time.Since(start)
				if err != nil {
					log.Error("task failed",
						zap.String("task", next.Name()),
						zap.Duration("duration", duration),
						zap.Error(err),
					)
				} else {
					log.Info("task completed",
						zap.String("task", next.Name()),
						zap.Duration("duration", duration),
					)
				}
				return err
			},
		}
	}
}

// wrappedTask is an internal helper struct used to wrap tasks with middleware
type wrappedTask struct {
	name string
	exec func(ctx context.Context) error
}

// Name returns the name of the wrapped task
func (w *wrappedTask) Name() string {
	return w.name
}

// Run executes the wrapped task function
func (w *wrappedTask) Run(ctx context.Context) error {
	return w.exec(ctx)
}
