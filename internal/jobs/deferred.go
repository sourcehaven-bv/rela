package jobs

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Commit deferral: a job enqueued inside an open store transaction must not
// become runnable until that transaction commits.
//
// Without it, a worker picks the job up on a DIFFERENT connection and reads a
// snapshot that cannot yet see the enqueueing transaction's writes. The
// handler acts on the pre-write world — a mail about an entity that "does not
// exist", a notification carrying a stale property. It is a race, so it passes
// tests and fails under load. That is why it lives in the seam rather than in
// caller discipline.
//
// This is NOT transactional enqueue. Transactional enqueue is about a job that
// should never have existed because the transaction rolled back; this is about
// a job that exists correctly but ran too early. Deferral happens to cover much
// of the rollback case too — [Collector.Discard] drops everything a failed
// transaction queued.
//
// The mechanism mirrors pgstore's txPending (internal/store/pgstore/tx.go):
// side effects are collected during the transaction and run only after commit
// succeeds. It lives here rather than in the store because the store must not
// learn about jobs — arch-lint forbids the import, and the concept does not
// belong there either.
//
// STATUS: this seam is BUILT AND PINNED by jobstest, but not yet wired to a
// transaction. No production code calls [WithDeferral] today, because no write
// path enqueues a job from inside store.Store.Tx yet — the scheduler, the only
// current producer, enqueues from its own goroutine. The first caller that
// enqueues inside a transaction MUST wire it at that transaction's boundary
// rather than reaching for Enqueue directly; until then the guarantee below is
// available, not active.

// collectorKey is the context key for the in-flight [Collector].
//
// An unexported struct type, so no other package can collide with it or reach
// the collector without going through [WithDeferral].
type collectorKey struct{}

// Collector accumulates enqueues made during a transaction and releases them
// once it commits.
//
// Safe for concurrent use: a transaction's fn may fan out to goroutines that
// each enqueue.
type Collector struct {
	mu      sync.Mutex
	pending []Job
	done    bool
}

// WithDeferral returns a context carrying a fresh [Collector], and the
// collector itself.
//
// A transaction seam calls this at the top of a transaction, then calls
// exactly one of [Collector.Flush] (on commit) or [Collector.Discard] (on
// rollback or error). Enqueues made with the returned context are held until
// then. See the STATUS note above: no seam calls it yet.
//
// A nested transaction that joins an outer one must NOT call this again —
// reusing the outer context keeps every enqueue on the outer collector, so
// they all flush once, at the outer commit.
func WithDeferral(ctx context.Context) (context.Context, *Collector) {
	c := &Collector{}
	return context.WithValue(ctx, collectorKey{}, c), c
}

// withoutCollector returns a context with no in-flight collector.
//
// Storing an untyped nil makes collectorFrom's type assertion fail, which is
// what detaches the returned context from any enclosing transaction. Spelled
// out as its own function because the mechanism is not obvious at the call
// site — the alternative reads like a mistake.
func withoutCollector(ctx context.Context) context.Context {
	return context.WithValue(ctx, collectorKey{}, nil)
}

// collectorFrom returns the in-flight collector on ctx, if any.
func collectorFrom(ctx context.Context) (*Collector, bool) {
	c, ok := ctx.Value(collectorKey{}).(*Collector)
	return c, ok
}

// add queues a job for release at commit. It reports false once the collector
// has been flushed or discarded, so a late enqueue (from a goroutine that
// outlived the transaction) is refused rather than silently lost.
func (c *Collector) add(job Job) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.done {
		return false
	}
	c.pending = append(c.pending, job)
	return true
}

// Flush enqueues everything collected, in the order it was submitted, and
// closes the collector.
//
// Call it AFTER the transaction has committed — the whole point is that a
// handler must not observe pre-commit state. Errors from individual enqueues
// are joined and returned; the transaction itself has already committed by
// then, so a failure here means the write landed and its follow-up work did
// not. Callers should log it loudly rather than treat it as a transaction
// failure.
//
// Flushing twice is a no-op.
func (c *Collector) Flush(ctx context.Context, q Queue) error {
	c.mu.Lock()
	if c.done {
		c.mu.Unlock()
		return nil
	}
	c.done = true
	pending := c.pending
	c.pending = nil
	c.mu.Unlock()

	// Enqueue on a context WITHOUT the collector, or each job would be
	// collected again by the collector we just closed and dropped.
	base := withoutCollector(ctx)

	var errs []error
	for _, job := range pending {
		if err := q.Enqueue(base, job); err != nil {
			// Name the kind. The transaction has already committed, so this
			// job is lost — an operator reading the log needs to know WHICH
			// follow-up work vanished, and a bare joined error does not say.
			errs = append(errs, fmt.Errorf("jobs: flush %q: %w", job.Kind, err))
		}
	}
	return errors.Join(errs...)
}

// Discard drops everything collected and closes the collector. Call it when
// the transaction did not commit.
//
// Discarding twice is a no-op, and Discard after Flush does nothing — which
// makes `defer Discard` safe alongside an explicit Flush on the success path.
func (c *Collector) Discard() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.done = true
	c.pending = nil
}

// Len reports how many jobs are currently held. For tests and diagnostics.
func (c *Collector) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pending)
}

// deferEnqueue routes a job to the in-flight collector if there is one.
//
// It reports handled=true when the job was collected, meaning the backend must
// NOT enqueue it now. Every [Queue] implementation calls this first; that is
// what makes the invariant a property of the seam rather than of one backend.
func deferEnqueue(ctx context.Context, job Job) (handled bool) {
	c, ok := collectorFrom(ctx)
	if !ok || c == nil {
		return false
	}
	return c.add(job)
}
