package routine

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dailyyoga/nexgo/logger"
)

func newTestLogger(t *testing.T) logger.Logger {
	log, err := logger.New(&logger.Config{
		Level:    "debug",
		Encoding: "console",
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	return log
}

func TestRunner_Go(t *testing.T) {
	log := newTestLogger(t)
	runner := New(log)

	var executed atomic.Bool
	runner.Go(func() {
		executed.Store(true)
	})

	runner.Wait()

	if !executed.Load() {
		t.Error("expected function to be executed")
	}
}

func TestRunner_Go_WithPanic(t *testing.T) {
	log := newTestLogger(t)
	runner := New(log)

	var beforePanic, afterPanic atomic.Bool
	runner.Go(func() {
		beforePanic.Store(true)
		panic("test panic")
	})

	// Start another goroutine to verify runner still works after panic
	runner.Go(func() {
		afterPanic.Store(true)
	})

	runner.Wait()

	if !beforePanic.Load() {
		t.Error("expected code before panic to execute")
	}
	if !afterPanic.Load() {
		t.Error("expected goroutine after panic to execute")
	}
}

func TestRunner_GoWithContext(t *testing.T) {
	log := newTestLogger(t)
	runner := New(log)

	ctx := context.WithValue(context.Background(), "key", "value")
	var receivedValue string
	var mu sync.Mutex

	runner.GoWithContext(ctx, func(ctx context.Context) {
		mu.Lock()
		defer mu.Unlock()
		receivedValue = ctx.Value("key").(string)
	})

	runner.Wait()

	mu.Lock()
	defer mu.Unlock()
	if receivedValue != "value" {
		t.Errorf("expected context value 'value', got '%s'", receivedValue)
	}
}

func TestRunner_GoNamed(t *testing.T) {
	log := newTestLogger(t)
	runner := New(log)

	var executed atomic.Bool
	runner.GoNamed("test-routine", func() {
		executed.Store(true)
	})

	runner.Wait()

	if !executed.Load() {
		t.Error("expected named function to be executed")
	}
}

func TestRunner_GoNamed_WithPanic(t *testing.T) {
	log := newTestLogger(t)
	runner := New(log)

	runner.GoNamed("panic-routine", func() {
		panic("named panic")
	})

	// Should not panic, runner should recover
	runner.Wait()
}

func TestRunner_GoNamedWithContext(t *testing.T) {
	log := newTestLogger(t)
	runner := New(log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var executed atomic.Bool
	runner.GoNamedWithContext(ctx, "context-routine", func(ctx context.Context) {
		executed.Store(true)
	})

	runner.Wait()

	if !executed.Load() {
		t.Error("expected named function with context to be executed")
	}
}

func TestRunner_Wait_MultipleGoroutines(t *testing.T) {
	log := newTestLogger(t)
	runner := New(log)

	var counter atomic.Int32
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		runner.Go(func() {
			time.Sleep(time.Millisecond)
			counter.Add(1)
		})
	}

	runner.Wait()

	if counter.Load() != int32(numGoroutines) {
		t.Errorf("expected %d executions, got %d", numGoroutines, counter.Load())
	}
}

func TestGo_Standalone(t *testing.T) {
	log := newTestLogger(t)

	var executed atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)

	Go(log, func() {
		defer wg.Done()
		executed.Store(true)
	})

	wg.Wait()

	if !executed.Load() {
		t.Error("expected standalone Go function to execute")
	}
}

func TestGo_Standalone_WithPanic(t *testing.T) {
	log := newTestLogger(t)

	var wg sync.WaitGroup
	wg.Add(1)

	Go(log, func() {
		defer wg.Done()
		panic("standalone panic")
	})

	// Should not panic
	wg.Wait()
}

func TestGoWithContext_Standalone(t *testing.T) {
	log := newTestLogger(t)

	ctx := context.WithValue(context.Background(), "key", "standalone")
	var receivedValue string
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(1)

	GoWithContext(ctx, log, func(ctx context.Context) {
		defer wg.Done()
		mu.Lock()
		defer mu.Unlock()
		receivedValue = ctx.Value("key").(string)
	})

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if receivedValue != "standalone" {
		t.Errorf("expected 'standalone', got '%s'", receivedValue)
	}
}

func TestGoNamed_Standalone(t *testing.T) {
	log := newTestLogger(t)

	var executed atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)

	GoNamed(log, "standalone-named", func() {
		defer wg.Done()
		executed.Store(true)
	})

	wg.Wait()

	if !executed.Load() {
		t.Error("expected standalone named function to execute")
	}
}

func TestGoNamedWithContext_Standalone(t *testing.T) {
	log := newTestLogger(t)

	ctx := context.Background()
	var executed atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)

	GoNamedWithContext(ctx, log, "standalone-named-ctx", func(ctx context.Context) {
		defer wg.Done()
		executed.Store(true)
	})

	wg.Wait()

	if !executed.Load() {
		t.Error("expected standalone named function with context to execute")
	}
}

func TestErrPanic(t *testing.T) {
	err := ErrPanic("test error")
	expected := "routine: panic recovered: test error"
	if err.Error() != expected {
		t.Errorf("expected '%s', got '%s'", expected, err.Error())
	}
}
