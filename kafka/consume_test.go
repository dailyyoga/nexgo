package kafka

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"go.uber.org/zap"
)

type consumeCall struct {
	group     string
	topic     string
	partition int32
	err       error
	latency   time.Duration
}

type lagCall struct {
	group     string
	topic     string
	partition int32
	lag       int64
}

// fakeConsumerHook records hook invocations for assertions.
type fakeConsumerHook struct {
	mu       sync.Mutex
	consumes []consumeCall
	lags     []lagCall
}

func (f *fakeConsumerHook) OnConsume(group, topic string, partition int32, err error, latency time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.consumes = append(f.consumes, consumeCall{group, topic, partition, err, latency})
}

func (f *fakeConsumerHook) OnLag(group, topic string, partition int32, lag int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lags = append(f.lags, lagCall{group, topic, partition, lag})
}

// TestHandlerMessageInvokesHook injects a fake hook and processes one success
// and one failure, asserting the reported group/topic/partition, error and a
// non-negative latency. EnableAutoCommit avoids touching the (nil) underlying
// consumer, so no real broker is required.
func TestHandlerMessageInvokesHook(t *testing.T) {
	topic := "atlas-events"
	hook := &fakeConsumerHook{}
	c := &consumeInstance{
		name:   "test",
		logger: zap.NewNop(),
		hook:   hook,
		config: &ConsumerConfig{
			GroupID:          "cg-test",
			MaxRetries:       1,
			EnableAutoCommit: true,
		},
	}
	msg := &ckafka.Message{
		Value:          []byte("v"),
		TopicPartition: ckafka.TopicPartition{Topic: &topic, Partition: 2, Offset: 10},
	}

	// Success.
	okHandler := func(context.Context, *Message) error { return nil }
	if err := c.handlerMessage(context.Background(), msg, okHandler); err != nil {
		t.Fatalf("success handlerMessage returned err: %v", err)
	}

	// Failure: handler always errors, exhausting the single retry.
	wantErr := errors.New("handler boom")
	failHandler := func(context.Context, *Message) error { return wantErr }
	if err := c.handlerMessage(context.Background(), msg, failHandler); !errors.Is(err, wantErr) {
		t.Fatalf("failure handlerMessage err = %v, want %v", err, wantErr)
	}

	hook.mu.Lock()
	defer hook.mu.Unlock()
	if len(hook.consumes) != 2 {
		t.Fatalf("expected 2 OnConsume calls, got %d: %+v", len(hook.consumes), hook.consumes)
	}

	// Success call: nil error, correct labels, non-negative latency.
	got := hook.consumes[0]
	if got.group != "cg-test" || got.topic != topic || got.partition != 2 {
		t.Errorf("success labels = %+v, want group=cg-test topic=%s partition=2", got, topic)
	}
	if got.err != nil {
		t.Errorf("success err = %v, want nil", got.err)
	}
	if got.latency < 0 {
		t.Errorf("success latency = %v, want >= 0", got.latency)
	}

	// Failure call: carries the handler error.
	got = hook.consumes[1]
	if !errors.Is(got.err, wantErr) {
		t.Errorf("failure err = %v, want %v", got.err, wantErr)
	}
	if got.latency < 0 {
		t.Errorf("failure latency = %v, want >= 0", got.latency)
	}
}

// TestHandlerMessageNilHook is the regression guard: with no hook configured,
// handlerMessage must behave exactly as before (no panic, error passthrough).
func TestHandlerMessageNilHook(t *testing.T) {
	topic := "t"
	c := &consumeInstance{
		name:   "test",
		logger: zap.NewNop(),
		config: &ConsumerConfig{GroupID: "g", MaxRetries: 2, EnableAutoCommit: true},
	}
	msg := &ckafka.Message{
		Value:          []byte("v"),
		TopicPartition: ckafka.TopicPartition{Topic: &topic},
	}

	if err := c.handlerMessage(context.Background(), msg, func(context.Context, *Message) error { return nil }); err != nil {
		t.Fatalf("nil-hook success returned err: %v", err)
	}
	wantErr := errors.New("boom")
	if err := c.handlerMessage(context.Background(), msg, func(context.Context, *Message) error { return wantErr }); !errors.Is(err, wantErr) {
		t.Fatalf("nil-hook failure err = %v, want %v", err, wantErr)
	}
}
