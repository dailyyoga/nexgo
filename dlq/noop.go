package dlq

import "context"

// noopRecorder is a Recorder that does nothing. Used when DLQ is disabled, so the
// call sites never need a nil check.
type noopRecorder struct{}

// NewNoopRecorder returns a Recorder that drops everything silently.
func NewNoopRecorder() Recorder { return noopRecorder{} }

func (noopRecorder) Record(context.Context, Payload) {}
func (noopRecorder) Dropped() uint64                 { return 0 }
func (noopRecorder) Close() error                    { return nil }
