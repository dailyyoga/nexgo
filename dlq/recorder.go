package dlq

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/dailyyoga/nexgo/kafka"
	"github.com/dailyyoga/nexgo/logger"
	"go.uber.org/zap"
)

// kafkaRecorder is the async, best-effort Recorder backed by a kafka producer.
type kafkaRecorder struct {
	logger   logger.Logger
	producer kafka.Producer
	topic    string
	maxBytes int

	ch   chan Payload
	done chan struct{}
	wg   sync.WaitGroup

	dropped atomic.Uint64
	closed  atomic.Bool
}

// NewKafkaRecorder creates a Recorder that delivers payloads to cfg.Topic via a
// dedicated kafka producer it creates and owns. All the tricky runtime behavior
// (async buffer, drop-on-full, degrade-to-log, byte cap, drain on Close) lives
// here and is written exactly once.
func NewKafkaRecorder(log logger.Logger, cfg *Config) (Recorder, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	cfg = cfg.withDefaults()

	producer, err := kafka.NewProducer(log, cfg.Producer)
	if err != nil {
		return nil, err
	}

	r := newKafkaRecorder(log, producer, cfg.Topic, cfg.BufferSize, cfg.MaxMessageBytes)

	log.Info("dlq kafka recorder started",
		zap.String("topic", cfg.Topic),
		zap.Int("buffer_size", cfg.BufferSize),
		zap.Int("max_message_bytes", cfg.MaxMessageBytes),
	)
	return r, nil
}

// newKafkaRecorder wires the runtime core around an already-built producer. Split
// out from NewKafkaRecorder so tests can inject a fake producer without a broker.
func newKafkaRecorder(
	log logger.Logger, producer kafka.Producer, topic string, bufferSize, maxBytes int,
) *kafkaRecorder {
	r := &kafkaRecorder{
		logger:   log,
		producer: producer,
		topic:    topic,
		maxBytes: maxBytes,
		ch:       make(chan Payload, bufferSize),
		done:     make(chan struct{}),
	}
	r.wg.Add(1)
	go r.loop()
	return r
}

// Record never blocks: it does a non-blocking send and drops on a full buffer.
// It never closes ch, so it can never panic on a concurrent Close.
func (r *kafkaRecorder) Record(_ context.Context, p Payload) {
	if p == nil {
		return
	}
	select {
	case <-r.done:
		return // already closing/closed; silently drop
	default:
	}

	select {
	case r.ch <- p:
	case <-r.done:
	default:
		// buffer full: the DLQ itself is allowed to shed load — the main flow is not.
		n := r.dropped.Add(1)
		// log sparsely (powers of ~10) to avoid a log storm during a failure flood
		if n == 1 || n%1000 == 0 {
			r.logger.Warn("dlq buffer full, dropping failed record",
				append(p.LogFields(), zap.Uint64("dropped_total", n))...)
		}
	}
}

func (r *kafkaRecorder) Dropped() uint64 { return r.dropped.Load() }

// loop drains ch until done is closed, then drains whatever is still buffered.
func (r *kafkaRecorder) loop() {
	defer r.wg.Done()
	for {
		select {
		case p := <-r.ch:
			r.produce(p)
		case <-r.done:
			for {
				select {
				case p := <-r.ch:
					r.produce(p)
				default:
					return
				}
			}
		}
	}
}

// produce marshals and fire-and-forgets one payload, degrading to a log line on
// any failure (marshal error, over-cap, or produce error).
func (r *kafkaRecorder) produce(p Payload) {
	data, err := p.Marshal()
	if err != nil {
		r.logger.Error("dlq marshal failed, dropping failed record",
			append(p.LogFields(), zap.Error(err))...)
		return
	}

	if r.maxBytes > 0 && len(data) > r.maxBytes {
		n := r.dropped.Add(1)
		r.logger.Error("dlq payload exceeds max message bytes, dropping failed record",
			append(p.LogFields(),
				zap.Int("size", len(data)),
				zap.Int("max_message_bytes", r.maxBytes),
				zap.Uint64("dropped_total", n),
			)...)
		return
	}

	key := []byte(p.Key())
	msg := &kafka.Message{
		Value: data,
		Key:   key,
		TopicPartition: kafka.TopicPartition{
			Topic:     &r.topic,
			Partition: kafka.PartitionAny,
		},
	}
	if err := r.producer.Produce(context.Background(), msg); err != nil {
		// last line of defense: never lose the record entirely, fall back to log
		r.logger.Error("dlq produce failed, falling back to log",
			append(p.LogFields(), zap.Error(err))...)
	}
}

// Close stops the loop, drains the buffer and closes the producer. Idempotent.
func (r *kafkaRecorder) Close() error {
	if !r.closed.CompareAndSwap(false, true) {
		return nil
	}
	close(r.done)
	r.wg.Wait()

	if d := r.dropped.Load(); d > 0 {
		r.logger.Warn("dlq recorder closed with dropped records", zap.Uint64("dropped_total", d))
	}
	return r.producer.Close()
}
