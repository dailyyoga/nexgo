# routine

Safe goroutine execution with automatic panic recovery to prevent application crashes.

## Features

- **Panic Recovery**: Automatic recovery from panics with stack trace logging
- **Runner Interface**: Track and wait for multiple goroutines
- **Standalone Functions**: Simple API for fire-and-forget scenarios
- **Named Goroutines**: Better debugging and logging with goroutine names
- **Context Support**: Full context propagation for cancellation and values
- **Unified Logging**: Integration with go-kit's Logger interface

## Installation

```bash
go get github.com/dailyyoga/nexgo/routine
```

## Why Use This Package?

In Go, if a goroutine panics and the panic is not recovered, the entire application crashes:

```go
// Dangerous - will crash the entire application if panic occurs
go func() {
    panic("oops") // Application crashes!
}()
```

This package wraps goroutine execution with automatic panic recovery:

```go
// Safe - panic is recovered and logged
routine.Go(log, func() {
    panic("oops") // Panic is caught, logged, application continues
})
```

## Quick Start

### Using Runner (Track and Wait)

Use `Runner` when you need to track goroutines and wait for them to complete:

```go
package main

import (
    "context"
    "fmt"

    "github.com/dailyyoga/nexgo/logger"
    "github.com/dailyyoga/nexgo/routine"
)

func main() {
    log, _ := logger.New(nil)
    defer log.Sync()

    runner := routine.New(log)

    // Start multiple goroutines
    for i := 0; i < 10; i++ {
        i := i // capture loop variable
        runner.GoNamed(fmt.Sprintf("worker-%d", i), func() {
            // Do work - panics will be recovered
            fmt.Printf("Worker %d running\n", i)
        })
    }

    // Wait for all goroutines to complete
    runner.Wait()
    fmt.Println("All workers completed")
}
```

### Standalone Functions (Fire-and-Forget)

Use standalone functions for simple one-off goroutines:

```go
package main

import (
    "github.com/dailyyoga/nexgo/logger"
    "github.com/dailyyoga/nexgo/routine"
)

func main() {
    log, _ := logger.New(nil)
    defer log.Sync()

    // Simple fire-and-forget goroutine
    routine.Go(log, func() {
        // Do background work
    })

    // Named for better logging
    routine.GoNamed(log, "cleanup-task", func() {
        // Do cleanup
    })

    // With context
    ctx := context.Background()
    routine.GoWithContext(ctx, log, func(ctx context.Context) {
        // Work with context
    })
}
```

## API Reference

### Runner Interface

```go
type Runner interface {
    // Go executes a function in a new goroutine with panic recovery
    Go(fn func())

    // GoWithContext executes a function with context in a new goroutine
    GoWithContext(ctx context.Context, fn func(ctx context.Context))

    // GoNamed executes a named function in a new goroutine
    // The name is used for logging purposes
    GoNamed(name string, fn func())

    // GoNamedWithContext executes a named function with context
    GoNamedWithContext(ctx context.Context, name string, fn func(ctx context.Context))

    // Wait waits for all goroutines started by this runner to complete
    Wait()
}
```

### Factory Function

```go
// New creates a new Runner with the given logger
func New(log logger.Logger) Runner
```

### Standalone Functions

```go
// Go executes a function in a new goroutine with panic recovery
func Go(log logger.Logger, fn func())

// GoWithContext executes a function with context in a new goroutine
func GoWithContext(ctx context.Context, log logger.Logger, fn func(ctx context.Context))

// GoNamed executes a named function in a new goroutine
func GoNamed(log logger.Logger, name string, fn func())

// GoNamedWithContext executes a named function with context in a new goroutine
func GoNamedWithContext(ctx context.Context, log logger.Logger, name string, fn func(ctx context.Context))
```

## Error Handling

When a panic occurs, it is recovered and logged with stack trace:

```go
// Predefined errors
var ErrPanicRecovered = fmt.Errorf("routine: panic recovered")

// Error constructor
func ErrPanic(recovered any) error
```

**Log output example** (when panic occurs):
```json
{
  "level": "ERROR",
  "msg": "goroutine panicked",
  "routine": "worker-1",
  "panic": "something went wrong",
  "stack": "goroutine 18 [running]:\nruntime/debug.Stack()..."
}
```

## Usage Patterns

### Pattern 1: Worker Pool

```go
func processItems(log logger.Logger, items []Item) {
    runner := routine.New(log)

    for _, item := range items {
        item := item // capture
        runner.GoNamed(fmt.Sprintf("process-%s", item.ID), func() {
            processItem(item)
        })
    }

    runner.Wait() // Wait for all items to be processed
}
```

### Pattern 2: Background Tasks with Context

```go
func startBackgroundTasks(ctx context.Context, log logger.Logger) {
    runner := routine.New(log)

    runner.GoNamedWithContext(ctx, "metrics-reporter", func(ctx context.Context) {
        ticker := time.NewTicker(time.Minute)
        defer ticker.Stop()

        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                reportMetrics()
            }
        }
    })

    runner.GoNamedWithContext(ctx, "health-checker", func(ctx context.Context) {
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()

        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                checkHealth()
            }
        }
    })

    // Don't wait here - let tasks run in background
}
```

### Pattern 3: Fire-and-Forget Notifications

```go
func handleRequest(log logger.Logger, req *Request) {
    // Process request synchronously
    result := processRequest(req)

    // Send notification asynchronously (don't wait)
    routine.GoNamed(log, "send-notification", func() {
        sendNotification(req.UserID, result)
    })

    // Return immediately
    return result
}
```

### Pattern 4: Graceful Shutdown

```go
func main() {
    log, _ := logger.New(nil)
    runner := routine.New(log)

    // Start background workers
    for i := 0; i < 5; i++ {
        runner.GoNamed(fmt.Sprintf("worker-%d", i), func() {
            for {
                // Do work
                time.Sleep(time.Second)
            }
        })
    }

    // Wait for shutdown signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan

    // Note: For proper graceful shutdown, workers should check a done channel
    // runner.Wait() would block indefinitely for infinite loops
    log.Info("shutting down")
}
```

## Comparison: Runner vs Standalone Functions

| Feature | Runner | Standalone |
|---------|--------|------------|
| Track goroutines | Yes | No |
| Wait for completion | Yes (`Wait()`) | No |
| Use case | Coordinated tasks | Fire-and-forget |
| Overhead | Slightly higher (WaitGroup) | Minimal |

**When to use Runner**:
- Need to wait for all goroutines to complete
- Processing batches of items in parallel
- Coordinated concurrent operations

**When to use Standalone**:
- Simple background tasks
- Fire-and-forget operations
- Notifications, logging, metrics

## Best Practices

1. **Always use named goroutines in production**: Makes debugging much easier
   ```go
   // Good
   runner.GoNamed("process-order-123", func() { ... })

   // Less helpful
   runner.Go(func() { ... })
   ```

2. **Use Runner when you need to wait**: Don't use standalone functions if you need synchronization
   ```go
   // Good
   runner := routine.New(log)
   runner.Go(processItem)
   runner.Wait() // Properly waits

   // Bad - no way to wait
   routine.Go(log, processItem)
   // How do we know when it's done?
   ```

3. **Respect context cancellation**: Check context in long-running goroutines
   ```go
   runner.GoNamedWithContext(ctx, "worker", func(ctx context.Context) {
       for {
           select {
           case <-ctx.Done():
               return // Properly exit on cancellation
           default:
               doWork()
           }
       }
   })
   ```

4. **Don't rely on panic recovery for error handling**: Use proper error handling; panic recovery is a safety net
   ```go
   // Good - proper error handling
   runner.GoNamed("worker", func() {
       if err := doWork(); err != nil {
           log.Error("work failed", zap.Error(err))
       }
   })

   // Bad - using panic for control flow
   runner.GoNamed("worker", func() {
       if err := doWork(); err != nil {
           panic(err) // Don't do this
       }
   })
   ```

## Dependencies

- [github.com/dailyyoga/nexgo/logger](../logger) - Unified logging interface

## License

This project is licensed under the MIT License - see the [LICENSE](../LICENSE) file for details.
