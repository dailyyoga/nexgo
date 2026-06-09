# routine (Safe Goroutine Execution)

Read this when working on the `routine` package — panic recovery, goroutine tracking via `Runner`, or the standalone fire-and-forget helpers. Referenced from the Reading Index in `CLAUDE.md`.

**Core Pattern**: Safe goroutine execution with automatic panic recovery to prevent application crashes.

**Key Components**:
- `Runner` interface - Provides `Go()`, `GoWithContext()`, `GoNamed()`, `GoNamedWithContext()`, `Wait()`
- Standalone functions - `Go()`, `GoWithContext()`, `GoNamed()`, `GoNamedWithContext()` for one-off usage
- `defaultRunner` - Implementation with `sync.WaitGroup` for coordinated shutdown

**Architecture Details**:
- Wraps all goroutine executions with `defer recover()` to catch panics
- Logs panics with stack traces using the `logger.Logger` interface
- `Runner` interface tracks goroutines via `sync.WaitGroup` for graceful shutdown
- Standalone functions provide simpler API when tracking is not needed
- Named variants allow identifying goroutines in logs for debugging

**Important Files**:
- `routine.go` - Core interfaces, Runner implementation, and standalone functions
- `errors.go` - Error types for panic recovery

**Usage** (Runner):
```go
import "github.com/dailyyoga/nexgo/routine"

runner := routine.New(log)

// Simple goroutine
runner.Go(func() {
    // work that might panic
})

// Named goroutine for better logging
runner.GoNamed("process-data", func() {
    // work
})

// With context
runner.GoWithContext(ctx, func(ctx context.Context) {
    // work with context
})

// Wait for all goroutines to complete
runner.Wait()
```

**Usage** (Standalone):
```go
// Simple one-off goroutine
routine.Go(log, func() {
    // work
})

// Named one-off goroutine
routine.GoNamed(log, "background-task", func() {
    // work
})
```
