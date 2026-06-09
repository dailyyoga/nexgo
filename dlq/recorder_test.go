package dlq

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/dailyyoga/nexgo/kafka"
	"go.uber.org/zap"
)

// --- test doubles ---

type noopLogger struct{}

func (noopLogger) Debug(string, ...zap.Field) {}
func (noopLogger) Info(string, ...zap.Field)  {}
func (noopLogger) Warn(string, ...zap.Field)  {}
func (noopLogger) Error(string, ...zap.Field) {}
func (noopLogger) Sync() error                { return nil }

// recordingProducer collects produced message values.
type recordingProducer struct {
	mu   sync.Mutex
	msgs [][]byte
}

func (p *recordingProducer) Produce(_ context.Context, msg *kafka.Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.msgs = append(p.msgs, msg.Value)
	return nil
}
func (p *recordingProducer) Close() error { return nil }
func (p *recordingProducer) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.msgs)
}

// blockingProducer blocks inside Produce until release is closed, signalling the
// first entry on entered (non-blocking, so it never stalls the loop).
type blockingProducer struct {
	entered chan struct{}
	release chan struct{}
	mu      sync.Mutex
	msgs    [][]byte
}

func (p *blockingProducer) Produce(_ context.Context, msg *kafka.Message) error {
	select {
	case p.entered <- struct{}{}:
	default:
	}
	<-p.release
	p.mu.Lock()
	p.msgs = append(p.msgs, msg.Value)
	p.mu.Unlock()
	return nil
}
func (p *blockingProducer) Close() error { return nil }

type testPayload struct {
	data       []byte
	marshalErr error
}

func (p testPayload) Marshal() ([]byte, error) { return p.data, p.marshalErr }
func (p testPayload) Key() string              { return "svc" }
func (p testPayload) LogFields() []zap.Field   { return nil }

// --- tests ---

func TestRecorder_DeliversAll(t *testing.T) {
	fp := &recordingProducer{}
	r := newKafkaRecorder(noopLogger{}, fp, "t", 100, 1<<20)

	const n = 50
	for i := 0; i < n; i++ {
		r.Record(context.Background(), testPayload{data: []byte(fmt.Sprintf("m%d", i))})
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if got := fp.count(); got != n {
		t.Fatalf("delivered %d, want %d", got, n)
	}
	if d := r.Dropped(); d != 0 {
		t.Fatalf("dropped %d, want 0", d)
	}
}

func TestRecorder_DropsWhenBufferFull(t *testing.T) {
	fp := &blockingProducer{entered: make(chan struct{}, 1), release: make(chan struct{})}
	r := newKafkaRecorder(noopLogger{}, fp, "t", 2, 1<<20)

	// first record gets pulled by the loop and blocks inside Produce
	r.Record(context.Background(), testPayload{data: []byte("a")})
	<-fp.entered // loop is now parked inside produce("a"); buffer (cap 2) is empty

	r.Record(context.Background(), testPayload{data: []byte("b")}) // buffered
	r.Record(context.Background(), testPayload{data: []byte("c")}) // buffered (full)
	r.Record(context.Background(), testPayload{data: []byte("d")}) // dropped
	r.Record(context.Background(), testPayload{data: []byte("e")}) // dropped

	if d := r.Dropped(); d != 2 {
		t.Fatalf("dropped %d, want 2", d)
	}

	close(fp.release) // let everything drain
	if err := r.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestRecorder_DropsOverByteCap(t *testing.T) {
	fp := &recordingProducer{}
	r := newKafkaRecorder(noopLogger{}, fp, "t", 100, 4) // 4-byte cap

	r.Record(context.Background(), testPayload{data: []byte("12345")}) // 5 bytes > 4
	r.Record(context.Background(), testPayload{data: []byte("ok")})    // fits
	if err := r.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if got := fp.count(); got != 1 {
		t.Fatalf("delivered %d, want 1", got)
	}
	if d := r.Dropped(); d != 1 {
		t.Fatalf("dropped %d, want 1", d)
	}
}

func TestRecorder_MarshalErrorIsSwallowed(t *testing.T) {
	fp := &recordingProducer{}
	r := newKafkaRecorder(noopLogger{}, fp, "t", 100, 1<<20)

	r.Record(context.Background(), testPayload{marshalErr: errors.New("boom")})
	r.Record(context.Background(), testPayload{data: []byte("ok")})
	if err := r.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if got := fp.count(); got != 1 {
		t.Fatalf("delivered %d, want 1 (marshal error must not be produced)", got)
	}
}

func TestRecorder_CloseIsIdempotent(t *testing.T) {
	fp := &recordingProducer{}
	r := newKafkaRecorder(noopLogger{}, fp, "t", 10, 1<<20)
	if err := r.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
	// recording after close must not panic
	r.Record(context.Background(), testPayload{data: []byte("late")})
}

func TestNoopRecorder(t *testing.T) {
	r := NewNoopRecorder()
	r.Record(context.Background(), testPayload{data: []byte("x")})
	if d := r.Dropped(); d != 0 {
		t.Fatalf("dropped %d, want 0", d)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}
