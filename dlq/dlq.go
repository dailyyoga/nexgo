// Package dlq provides a generic, best-effort dead-letter recorder.
//
// It owns ONLY the runtime behavior that is hard to get right and easy to get
// wrong: a buffered channel, a background producer goroutine, drop-on-full with
// a counter, degrade-to-log on produce failure, a byte-size safety cap and a
// clean Close lifecycle.
//
// It deliberately knows NOTHING about any business semantics — it never sees an
// app_id, an error_type enum or a specific topic. The business layer implements
// Payload (which owns the JSON schema, raw-data truncation, etc.) and passes the
// topic in via Config. This keeps the reusable concurrency core in one place
// while every Atlas-specific contract stays in each service's internal/dlq.
package dlq

import (
	"context"

	"go.uber.org/zap"
)

// Payload is implemented by the business layer. nexgo/dlq only needs to turn it
// into bytes and deliver those bytes asynchronously and best-effort.
type Payload interface {
	// Marshal returns the wire bytes of the record. The business layer owns the
	// JSON schema as well as any semantic truncation (e.g. raw-data size limits).
	Marshal() ([]byte, error)
	// Key is the kafka partition key (e.g. the service name).
	Key() string
	// LogFields are emitted to the fallback log when a record cannot be produced,
	// so the last line of defense still carries enough context to investigate.
	LogFields() []zap.Field
}

// Recorder records failed payloads to a dead-letter sink.
//
// Record never blocks the caller and never returns an error: the DLQ itself is
// allowed to degrade (drop + count, or fall back to logging), but it must never
// disturb the main data flow.
type Recorder interface {
	// Record asynchronously enqueues p for delivery. It is safe for concurrent
	// use and returns immediately. If the internal buffer is full the payload is
	// dropped and the dropped counter is incremented.
	Record(ctx context.Context, p Payload)
	// Dropped returns the total number of payloads dropped so far (buffer full or
	// over the byte cap). Useful for /metrics and alerting.
	Dropped() uint64
	// Close stops accepting new payloads, drains what is already buffered and
	// releases the underlying producer. It is idempotent.
	Close() error
}
