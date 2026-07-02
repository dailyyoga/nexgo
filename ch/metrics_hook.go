package ch

import "time"

// WriterMetricsHook receives batch-write observability events from the writer's
// flush loop. It is deliberately prometheus-free: the ch package only emits raw
// events, while the prometheus adapter lives in nexgo/metrics (dependency
// direction metrics -> ch, never the reverse), mirroring the kafka hooks.
//
// Implementations MUST be safe for concurrent use and MUST NOT block — the
// callback runs inline on the writer's flush loop, so a slow hook stalls
// flushing. A nil hook disables collection entirely (the legacy behavior).
type WriterMetricsHook interface {
	// OnFlush is called once per table per flush, after that table's batch
	// insert (and any retries) has settled. rows is the number of rows in the
	// table's batch; duration covers the whole insert+retry attempt; err is the
	// final outcome — nil when the batch was persisted, non-nil when it exhausted
	// retries and was handed to OnPermanentFailure. Because flush retries a table
	// as a whole, the batch is all-or-nothing: on err != nil every one of the
	// rows failed.
	OnFlush(table TableName, rows int, duration time.Duration, err error)
}

// WriterStats is an optional accessor a Writer may implement to expose its
// current ingress-buffer occupancy for scrape-time gauges. The default writer
// implements it; callers type-assert (a Writer that does not implement it simply
// exposes no buffer gauge), mirroring the dlq.Stats accessor pattern.
type WriterStats interface {
	// BufferLen returns the number of rows currently queued in the writer's
	// ingress channel awaiting the flush loop. It is a single figure across all
	// tables — the channel is not partitioned by table.
	BufferLen() int
}
