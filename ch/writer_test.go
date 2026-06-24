package ch

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.uber.org/zap"
)

// failingConn is a driver.Conn stub whose Query always fails. The writer's
// batchInsert path first DESCRIBEs the table via Query, so a failing Query
// forces a permanent, non-retryable error (a plain error is not classified as
// retryable, so flush breaks on the first attempt without sleeping). All other
// Conn methods are inherited from the embedded nil interface and must not be
// called by these tests.
type failingConn struct {
	driver.Conn
	err error
}

func (c *failingConn) Query(_ context.Context, _ string, _ ...any) (driver.Rows, error) {
	return nil, c.err
}

// fakeRow is a minimal Table implementation for driving flush().
type fakeRow struct {
	table  TableName
	values map[string]any
}

func (r fakeRow) TableName() TableName       { return r.table }
func (r fakeRow) ToValueMap() map[string]any { return r.values }

// newTestWriter builds a writer backed by a Query-failing conn so that flush()
// always hits the permanent-failure branch.
func newTestWriter(t *testing.T, queryErr error) *defaultWriter {
	t.Helper()
	cfg := &Config{
		Hosts:        []string{"localhost:9000"},
		Username:     "u",
		Password:     "p",
		WriterConfig: DefaultWriterConfig(),
	}
	w, ok := newWriterWithConn(&failingConn{err: queryErr}, cfg, zap.NewNop()).(*defaultWriter)
	if !ok {
		t.Fatalf("newWriterWithConn did not return *defaultWriter")
	}
	return w
}

// TestFlush_OnPermanentFailureInvoked asserts that when a batch exhausts retries
// the callback fires exactly once with the correct table, the full row slice,
// and an error that wraps the underlying cause.
func TestFlush_OnPermanentFailureInvoked(t *testing.T) {
	sentinel := errors.New("describe table failed")
	w := newTestWriter(t, sentinel)

	var (
		mu        sync.Mutex
		callCount int
		gotTable  TableName
		gotRows   []Table
		gotErr    error
	)
	w.config.WriterConfig.OnPermanentFailure = func(table TableName, rows []Table, err error) {
		mu.Lock()
		defer mu.Unlock()
		callCount++
		gotTable = table
		gotRows = rows
		gotErr = err
	}

	rows := []Table{
		fakeRow{table: "events", values: map[string]any{"id": 1}},
		fakeRow{table: "events", values: map[string]any{"id": 2}},
	}
	w.flush(map[TableName][]Table{"events": rows})

	mu.Lock()
	defer mu.Unlock()
	if callCount != 1 {
		t.Fatalf("callback call count = %d, want 1", callCount)
	}
	if gotTable != "events" {
		t.Fatalf("callback table = %q, want %q", gotTable, "events")
	}
	if len(gotRows) != 2 {
		t.Fatalf("callback rows len = %d, want 2", len(gotRows))
	}
	if !errors.Is(gotErr, sentinel) {
		t.Fatalf("callback err = %v, want it to wrap %v", gotErr, sentinel)
	}
}

// TestFlush_NilCallbackPreservesOldBehaviour is the regression guard: with no
// callback set, flush must drop the batch and return without panicking, exactly
// as before this feature existed.
func TestFlush_NilCallbackPreservesOldBehaviour(t *testing.T) {
	w := newTestWriter(t, errors.New("describe table failed"))
	// OnPermanentFailure intentionally left nil.
	w.flush(map[TableName][]Table{"events": {fakeRow{table: "events"}}})
}

// TestFlush_CallbackPanicRecovered asserts a panicking callback cannot escape
// and crash the flush loop.
func TestFlush_CallbackPanicRecovered(t *testing.T) {
	w := newTestWriter(t, errors.New("describe table failed"))
	w.config.WriterConfig.OnPermanentFailure = func(TableName, []Table, error) {
		panic("callback boom")
	}
	// Must not propagate the panic.
	w.flush(map[TableName][]Table{"events": {fakeRow{table: "events"}}})
}
