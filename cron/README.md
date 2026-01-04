# Cron Package

A high-performance cron job manager with chain-based task execution, middleware support, and thread-safe inter-task communication.

## Features

- **Chain-Based Execution**: Execute multiple tasks sequentially with automatic abort on failure
- **Middleware Support**: Built-in recovery and logging middlewares, extensible for custom behaviors
- **Thread-Safe Data Sharing**: Share data between tasks in a chain using `SharedData`
- **Panic Recovery**: Automatic panic recovery to prevent process crashes
- **Structured Logging**: Integration with `zap` for structured logging
- **Standard Cron Format**: Supports 6-field cron expressions (with seconds)

## Installation

```bash
go get github.com/robfig/cron/v3
go get go.uber.org/zap
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "your-project/pkg/cron"
    "go.uber.org/zap"
)

// Define a task by implementing the Task interface
type MyTask struct{}

func (t *MyTask) Name() string {
    return "my-task"
}

func (t *MyTask) Run(ctx context.Context) error {
    // Access shared data from context
    shared := cron.GetSharedData(ctx)

    // Perform task logic
    fmt.Println("Task running...")

    // Store data for next task
    shared.Set("result", "success")

    return nil
}

func main() {
    // Create logger
    logger, _ := zap.NewProduction()
    defer logger.Sync()

    // Create cron manager with middlewares
    manager := cron.NewCron(
        logger,
        cron.RecoverMiddleware(logger),  // Panic recovery
        cron.LoggingMiddleware(logger),  // Task logging
    )

    // Add a chain of tasks
    err := manager.AddTasks(
        "my-chain",
        "0 0 * * * *",  // Run every hour at minute 0
        &MyTask{},
        // Add more tasks...
    )
    if err != nil {
        log.Fatal(err)
    }

    // Start the scheduler
    manager.Start()

    // Wait for signal or run indefinitely
    select {}

    // Graceful shutdown
    manager.Close()
}
```

## Architecture

### File Structure

```
pkg/cron/
├── cron.go         # Interface definitions and public API
├── manager.go      # Cron manager and chain job implementation
├── middleware.go   # Built-in middlewares (Recovery, Logging)
├── shared_data.go  # Thread-safe data sharing between tasks
└── errors.go       # Error definitions
```

### Core Components

#### 1. Task Interface

```go
type Task interface {
    Name() string
    Run(ctx context.Context) error
}
```

Every cron task must implement this interface:
- `Name()`: Returns a unique identifier for the task
- `Run(ctx)`: Executes the task logic with context support

#### 2. Cron Manager

```go
type Cron interface {
    Start()
    Close()
    AddTasks(name string, spec string, tasks ...Task) error
}
```

The cron manager orchestrates job scheduling:
- `Start()`: Begins the cron scheduler
- `Close()`: Gracefully stops the scheduler
- `AddTasks()`: Registers a chain of tasks with a cron schedule

#### 3. Middlewares

Middlewares wrap tasks with additional behavior:

```go
type Middleware func(Task) Task
```

**Built-in Middlewares**:
- `RecoverMiddleware(logger)`: Catches panics and converts them to errors
- `LoggingMiddleware(logger)`: Logs task start, completion, and errors

**Custom Middleware Example**:

```go
func MetricsMiddleware(metrics *Metrics) cron.Middleware {
    return func(next cron.Task) cron.Task {
        return &wrappedTask{
            name: next.Name(),
            exec: func(ctx context.Context) error {
                metrics.Increment("task.started")
                err := next.Run(ctx)
                if err != nil {
                    metrics.Increment("task.failed")
                } else {
                    metrics.Increment("task.completed")
                }
                return err
            },
        }
    }
}
```

#### 4. SharedData

Thread-safe data sharing between tasks in a chain:

```go
type SharedData struct {
    // internal sync.Map
}

// Methods
func GetSharedData(ctx context.Context) *SharedData
func (s *SharedData) Set(key string, value any)
func (s *SharedData) Get(key string) (any, bool)
func (s *SharedData) Delete(key string)
func (s *SharedData) Range(f func(key string, value any) bool)
```

**Example Usage**:

```go
type Task1 struct{}

func (t *Task1) Name() string { return "task1" }

func (t *Task1) Run(ctx context.Context) error {
    shared := cron.GetSharedData(ctx)

    // Store data for next task
    shared.Set("user_id", 12345)
    shared.Set("timestamp", time.Now())

    return nil
}

type Task2 struct{}

func (t *Task2) Name() string { return "task2" }

func (t *Task2) Run(ctx context.Context) error {
    shared := cron.GetSharedData(ctx)

    // Retrieve data from previous task
    userID, ok := shared.Get("user_id")
    if !ok {
        return errors.New("user_id not found")
    }

    fmt.Printf("Processing user: %v\n", userID)
    return nil
}
```

## Cron Expression Format

The package uses 6-field cron expressions (with seconds support):

```
┌───────────── second (0 - 59)
│ ┌───────────── minute (0 - 59)
│ │ ┌───────────── hour (0 - 23)
│ │ │ ┌───────────── day of month (1 - 31)
│ │ │ │ ┌───────────── month (1 - 12)
│ │ │ │ │ ┌───────────── day of week (0 - 6) (Sunday to Saturday)
│ │ │ │ │ │
│ │ │ │ │ │
* * * * * *
```

**Examples**:
- `0 0 * * * *` - Every hour at minute 0, second 0
- `0 30 * * * *` - Every hour at minute 30
- `0 0 0 * * *` - Every day at midnight
- `0 0 2 * * *` - Every day at 2:00 AM
- `0 0 0 * * 0` - Every Sunday at midnight
- `*/10 * * * * *` - Every 10 seconds

## Advanced Usage

### Chain Execution Flow

1. Manager creates a `chainJob` with all tasks
2. Tasks are wrapped with middlewares (Recovery → Logging → Custom)
3. On schedule trigger:
   - Create `SharedData` in context
   - Execute tasks sequentially
   - If any task fails, abort chain
   - Log chain completion or failure

### Error Handling

The package defines standard errors:

```go
var (
    ErrNoTasks    = errors.New("cron: no tasks provided")
    ErrInvalidSpec = errors.New("cron: invalid cron spec")
    ErrCronClosed = errors.New("cron: cron manager is closed")
)
```

### Graceful Shutdown

```go
// Create signal channel
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

// Start cron
manager.Start()

// Wait for signal
<-sigChan

// Graceful shutdown (waits for running jobs to complete)
manager.Close()
```

## Integration Example

Here's a complete example integrating with the DTS Sensors project:

```go
package app

import (
    "context"
    "time"

    "dts-sensors/internal/service"
    "dts-sensors/pkg/cron"
    "go.uber.org/zap"
)

type SyncEventsTask struct {
    syncService *service.SyncService
}

func (t *SyncEventsTask) Name() string {
    return "sync-events"
}

func (t *SyncEventsTask) Run(ctx context.Context) error {
    shared := cron.GetSharedData(ctx)

    // Sync events for current hour
    now := time.Now()
    date := now.Format("2006-01-02")
    hour := now.Hour()

    result, err := t.syncService.SyncEvents(ctx, date, hour)
    if err != nil {
        return err
    }

    // Share results with next task
    shared.Set("sync_result", result)
    return nil
}

type NotifyTask struct {
    notifier *service.Notifier
}

func (t *NotifyTask) Name() string {
    return "notify"
}

func (t *NotifyTask) Run(ctx context.Context) error {
    shared := cron.GetSharedData(ctx)

    // Get result from previous task
    result, ok := shared.Get("sync_result")
    if !ok {
        return errors.New("sync_result not found")
    }

    // Send notification
    return t.notifier.Send("Sync completed", result)
}

func SetupCronJobs(logger *zap.Logger, syncService *service.SyncService) cron.Cron {
    manager := cron.NewCron(
        logger,
        cron.RecoverMiddleware(logger),
        cron.LoggingMiddleware(logger),
    )

    // Add hourly sync job
    manager.AddTasks(
        "hourly-sync",
        "0 0 * * * *",  // Every hour
        &SyncEventsTask{syncService: syncService},
        &NotifyTask{notifier: notifier},
    )

    return manager
}
```

## Best Practices

1. **Task Naming**: Use descriptive, unique names for tasks
2. **Error Handling**: Always return errors from tasks; don't panic
3. **Context Usage**: Respect context cancellation in long-running tasks
4. **SharedData Keys**: Use constant keys to avoid typos
5. **Middleware Order**: Apply middlewares in logical order (Recovery → Logging → Custom)
6. **Testing**: Mock the Task interface for unit testing

## Comparison with Standard robfig/cron

| Feature | Standard cron | This Package |
|---------|--------------|--------------|
| Task chaining | ✗ | ✓ |
| Middleware support | ✗ | ✓ |
| Inter-task data sharing | ✗ | ✓ |
| Panic recovery | ✗ | ✓ (built-in) |
| Structured logging | ✗ | ✓ (zap) |
| Task interface | ✗ | ✓ |

## License

This package follows the same license as the parent project.
