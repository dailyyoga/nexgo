package cron

import (
	"context"
	"fmt"

	"github.com/dailyyoga/nexgo/logger"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// chainJob represents a chain of tasks that execute sequentially
type chainJob struct {
	name   string
	tasks  []Task
	logger logger.Logger
}

// Run executes all tasks in the chain sequentially
// If any task fails, the chain is aborted and subsequent tasks are not executed
func (j *chainJob) Run() {
	// Create shared data for inter-task communication
	shared := &SharedData{}
	ctx := context.WithValue(context.Background(), sharedDataKey, shared)

	j.logger.Info("chain job started", zap.String("chain_name", j.name))

	for _, task := range j.tasks {
		if err := task.Run(ctx); err != nil {
			j.logger.Error("chain job aborted due to task failure",
				zap.String("chain_name", j.name),
				zap.String("task_name", task.Name()),
				zap.Error(err),
			)
			return
		}
	}

	j.logger.Info("chain job completed", zap.String("chain_name", j.name))
}

// cronManager is the default implementation of the Cron interface
type cronManager struct {
	cron        *cron.Cron
	middlewares []Middleware
	logger      logger.Logger
}

// newCronManager creates a new cron manager instance
func newCronManager(log logger.Logger, mws ...Middleware) *cronManager {
	return &cronManager{
		cron:        cron.New(cron.WithSeconds()),
		middlewares: mws,
		logger:      log,
	}
}

// Start begins the cron scheduler
func (m *cronManager) Start() {
	m.cron.Start()
}

// Close stops the cron scheduler and waits for running jobs to complete
func (m *cronManager) Close() {
	ctx := m.cron.Stop()
	<-ctx.Done()
}

// AddTasks adds a chain of tasks to be executed according to the cron spec
// The spec follows the standard cron format with support for seconds (6 fields)
// Example: "0 0 * * * *" (every hour at minute 0, second 0)
func (m *cronManager) AddTasks(name, spec string, tasks ...Task) error {
	if len(tasks) == 0 {
		return ErrNoTasks
	}

	// apply middlewares to all tasks
	wrappedTasks := make([]Task, len(tasks))
	for i, task := range tasks {
		// wrap task with chain name prefix for logging
		wrapTask := &wrappedTask{
			name: fmt.Sprintf("%s:%s", name, task.Name()),
			exec: func(ctx context.Context) error {
				return task.Run(ctx)
			},
		}
		// apply middlewares
		wrappedTasks[i] = applyMiddlewares(wrapTask, m.middlewares...)
	}

	job := &chainJob{
		name:   name,
		tasks:  wrappedTasks,
		logger: m.logger,
	}

	if _, err := m.cron.AddJob(spec, job); err != nil {
		return fmt.Errorf("failed to add chain job %s with spec %s: %w", name, spec, err)
	}

	m.logger.Info("chain added",
		zap.String("chain_name", name),
		zap.String("spec", spec),
		zap.Int("task_count", len(tasks)),
	)

	return nil
}

// AddChain is alias for AddTasks
func (m *cronManager) AddChain(chain Chain) error {
	return m.AddTasks(chain.Name, chain.Spec, chain.Tasks...)
}
