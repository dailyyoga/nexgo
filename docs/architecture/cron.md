# cron (Cron Job Manager)

Read this when working on the `cron` manager — task chains, middleware order, or inter-task shared data. Referenced from the Reading Index in `CLAUDE.md`.

**Core Pattern**: Chain-based task execution with middleware pipeline and inter-task data sharing.

**Key Components**:
- `Cron` interface - `Start()`, `Close()`, `AddTasks(name, spec, ...tasks)`
- `Task` interface - `Name()` and `Run(ctx)`
- `Middleware` - `func(Task) Task` for wrapping tasks (Recovery, Logging)
- `SharedData` - Thread-safe `sync.Map` wrapper in context for inter-task communication

**Architecture Details**:
- Built on `robfig/cron/v3` with 6-field cron expressions (with seconds)
- `cronManager` wraps tasks in `chainJob` which executes tasks sequentially
- Middleware chain: Recovery → Logging → Custom (applied in order)
- Each chain execution gets fresh `SharedData` in context
- Chain aborts on first task error
- Graceful shutdown: `Close()` waits for running jobs via `cron.Stop().Wait()`

**Important Files**:
- `cron.go` - Core interfaces and `NewCron()` factory
- `manager.go` - `cronManager` and `chainJob` implementation
- `middleware.go` - Built-in middlewares (recovery, logging)
- `shared_data.go` - Thread-safe data sharing via `sync.Map`
- `errors.go` - Standard errors

**Data Flow** (Task Chain):
1. `AddTasks()` creates `chainJob` wrapping all tasks
2. On schedule trigger: create `SharedData` in context
3. For each task: apply middleware chain, execute `Run(ctx)`
4. If error: abort chain and log
5. If success: continue to next task
6. After all tasks: log completion
