# CLAUDE.md

Entry point for Claude Code / any AI assistant working in this repository. This file stays small on purpose: it gives the project shape, the conventions you must follow on every change, and a **reading index** that tells you **when** to open each detailed doc — so the deep docs are loaded only when the task actually needs them, not all at once.

## Overview

This is a Go toolkit library (`github.com/dailyyoga/nexgo`) providing eight core packages for common infrastructure patterns:

- **logger** - Unified logging interface based on zap with configurable levels, encoding, and output
- **db** - MySQL database client wrapper with connection pool management, structured logging, and slow query detection
- **ch** - ClickHouse client with unified query/write operations and async batch writing
- **kafka** - Kafka consumer/producer wrappers with retry, parallel processing, and error handling
- **cron** - Cron job manager with chain-based task execution and middleware support
- **routine** - Safe goroutine execution with panic recovery to prevent application crashes
- **cache** - Syncable cache with periodic data synchronization, automatic retry logic, and Redis client wrapper
- **dlq** - Generic best-effort dead-letter recorder with async buffering, drop-on-full counting, and a Kafka backend

All packages use interface-driven design for testability. The logger package provides a unified `Logger` interface used across all other packages.

## Reading Index — open these docs when…

Each row maps a **situation** to the one doc that covers it. Open the doc only when the situation applies; do not preload them.

| Situation | Read |
|---|---|
| Working on the **logger** package, or wiring a logger into another package | [`docs/architecture/logger.md`](./docs/architecture/logger.md) |
| Working on the **db** MySQL/GORM client — pool tuning, slow-query logging, DSN, health checks | [`docs/architecture/db.md`](./docs/architecture/db.md) |
| Working on the **ch** ClickHouse client/writer — async batch writes, type conversion, schema cache, flush strategy | [`docs/architecture/ch.md`](./docs/architecture/ch.md) |
| Working on the **kafka** consumer or producer — retries, parallel instances, offsets, delivery reports | [`docs/architecture/kafka.md`](./docs/architecture/kafka.md) |
| Working on the **cron** manager — task chains, middleware, shared data | [`docs/architecture/cron.md`](./docs/architecture/cron.md) |
| Working on the **routine** package — panic recovery, goroutine tracking | [`docs/architecture/routine.md`](./docs/architecture/routine.md) |
| Working on the **cache** SyncableCache or the Redis client wrapper | [`docs/architecture/cache.md`](./docs/architecture/cache.md) |
| Working on the **dlq** dead-letter recorder — Payload, drop/degrade behavior, byte cap | [`docs/architecture/dlq.md`](./docs/architecture/dlq.md) |
| Adding/structuring errors, using the shared logger interface, writing a `Config`/`Validate`, or applying interface-driven design | [`docs/cross-cutting-patterns.md`](./docs/cross-cutting-patterns.md) |
| Before shipping a change that touches lifecycle/shutdown (`Start`/`Close`/`Stop` ordering, flush-on-close, reference-type races, schema refresh) | [`docs/important-notes.md`](./docs/important-notes.md) |

## Development Commands

### Running Tests
```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./logger
go test ./db
go test ./ch
go test ./kafka
go test ./cron
go test ./routine
go test ./cache
go test ./dlq

# Verbose / with coverage
go test -v ./...
go test -cover ./...
```

### Building & Quality
```bash
go build ./...        # Verify all packages compile
go mod tidy           # Sync dependencies
go mod verify

go fmt ./...          # Format
go vet ./...          # Vet
staticcheck ./...     # Static analysis (if available)
```

## Conventions (follow on every change)

One line each. Full rationale and per-package detail live in [`docs/cross-cutting-patterns.md`](./docs/cross-cutting-patterns.md); lifecycle gotchas in [`docs/important-notes.md`](./docs/important-notes.md).

### Errors
- Define every package's errors in its `errors.go`. Use a sentinel `var ErrX = fmt.Errorf("pkg: ...")` when no context is needed, and a constructor `func ErrX(err error) error` that wraps with `%w` when there is. Prefix each message with the package name.

### Config
- Each package owns a `Config` with a `Validate()` that only validates (never mutates). `New()` merges defaults **then** validates. Required fields are checked at construction.

### Interfaces & Logger
- Expose components as interfaces and keep implementations private (`defaultClient`, `defaultConsumer`, …); factories return the interface, not the struct. Every constructor takes the `logger.Logger` interface (compatible with `*zap.Logger`).

### Lifecycle
- Anything with a background goroutine must be released on shutdown — `db.Close()`, `ch` writer `Start()`→`Close()`, `kafka` producer/consumer `Close()`, `cache` `Start()`→`Stop()`, `dlq` `Close()`. `Close()`/`Stop()` are idempotent and flush/drain before releasing resources.
- `cache.Get()` returns a **reference** for slice/map/pointer types — treat it as read-only or copy before mutating.

### Testing
- Run `go test ./...` before committing. Each package is independently testable through its interfaces (inject fakes via the unexported constructor where one exists, e.g. `dlq.newKafkaRecorder`).
