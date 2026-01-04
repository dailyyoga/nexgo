package cron

import (
	"context"

	"github.com/dailyyoga/go-kit/logger"
)

// Task is the interface for a cron task
// Each task must have a unique name and implement the Run method
type Task interface {
	// Name returns the unique identifier for this task
	Name() string
	// Run executes the task with the given context
	// The context may contain SharedData for inter-task communication
	Run(ctx context.Context) error
}

// Chain represents a chain of tasks that execute sequentially
type Chain struct {
	// Name is the name of the chain
	Name string
	// Spec is the cron spec for the chain
	Spec string
	// Tasks are the tasks in the chain
	Tasks []Task
}

// Cron is the interface for managing cron jobs
// It supports chain-based task execution with middleware support
type Cron interface {
	// Start begins the cron scheduler
	Start()
	// Close stops the cron scheduler and waits for running jobs to complete
	Close()
	// AddTasks adds a chain of tasks to be executed according to the cron spec
	// The spec follows the standard cron format with support for seconds
	// Tasks are executed sequentially, and if any task fails, the chain is aborted
	AddTasks(name string, spec string, tasks ...Task) error
	// AddChain is alias for AddTasks
	AddChain(chain Chain) error
}

// NewCron creates a new cron manager with the given logger and middlewares
// Middlewares are applied to all tasks in the order they are provided
// Built-in middlewares: recoveryMiddleware, loggingMiddleware
func NewCron(log logger.Logger, mws ...Middleware) Cron {
	defaultMws := []Middleware{
		recoveryMiddleware(log),
		loggingMiddleware(log),
	}
	return newCronManager(log, append(defaultMws, mws...)...)
}
