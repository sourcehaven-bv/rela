package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

// defaultConcurrency is the worker count for a memory queue.
//
// Small on purpose: this tier runs on a user's machine alongside the desktop
// app, and the work is I/O-bound (mail, HTTP), so a handful of workers keeps
// throughput up without making the queue a noticeable load of its own.
const defaultConcurrency = 4

// NewMemoryQueue returns an in-process, EPHEMERAL queue: jobs live in memory
// and vanish when the process exits. This is the queue for the fs and desktop
// tiers; see the package doc for why ephemeral is the right semantic there.
//
// The queue is rela's own code rather than a library backend. It is a slice,
// a condition variable and a fixed set of workers, so owning it costs less
// than tracking the races and leaks of a third-party one (TKT-TMXHBZW).
//
// ctx is unused; it keeps the constructor's shape the same as
// [NewPostgresQueue].
//
// Nil: rejected — a nil logger is a wiring bug.
func NewMemoryQueue(_ context.Context, logger *slog.Logger) (Queue, error) {
	if logger == nil {
		return nil, errors.New("jobs: logger must not be nil")
	}
	q := &memQueue{
		logger:      logger,
		concurrency: defaultConcurrency,
		handlers:    make(map[Kind]Handler),
		keys:        make(map[string]struct{}),
		timers:      make(map[*time.Timer]struct{}),
	}
	q.cond = sync.NewCond(&q.mu)
	return q, nil
}

// memQueue is the in-process [Queue].
//
// Every field below mu is guarded by it. A job is in exactly one place while
// it is pending: in ready, waiting for a worker; on a worker, running; or
// behind a timer in timers, waiting for its next attempt. Its idempotency key
// stays in keys through all three and is removed only when the job is
// finished for good (see [memQueue.finish]).
type memQueue struct {
	logger      *slog.Logger
	concurrency int

	mu       sync.Mutex
	cond     *sync.Cond // signaled when ready grows or the queue closes
	handlers map[Kind]Handler
	started  bool
	closed   bool
	ready    []memJob
	keys     map[string]struct{}
	timers   map[*time.Timer]struct{}

	// cancel ends the context Start hands its workers, the parent of every
	// handler context. It is how running handlers learn of Close.
	cancel context.CancelFunc

	workers sync.WaitGroup
	// drained is closed once every worker has returned. Set by the first
	// Close; later calls wait on it too.
	drained chan struct{}
}

// memJob is a pending job and the bookkeeping that travels with it.
type memJob struct {
	// job carries the attempt about to run and the EFFECTIVE deadline,
	// which for RetryPersistent includes the outer time window.
	job Job
	// key is the job's idempotency key, or empty when it has none.
	key string
}

// Register implements [Queue].
func (q *memQueue) Register(kind Kind, h Handler) error {
	if kind == "" {
		return ErrNoKind
	}
	if h == nil {
		return fmt.Errorf("jobs: nil handler for kind %q", kind)
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return ErrClosed
	}
	if _, exists := q.handlers[kind]; exists {
		return fmt.Errorf("%w: %q", ErrDuplicateKind, kind)
	}
	q.handlers[kind] = h
	return nil
}

// Enqueue implements [Queue].
func (q *memQueue) Enqueue(ctx context.Context, job Job) error {
	if err := job.validate(); err != nil {
		return err
	}

	q.mu.Lock()
	closed := q.closed
	_, known := q.handlers[job.Kind]
	q.mu.Unlock()
	if closed {
		return ErrClosed
	}
	// Reject an unroutable job at the door, as every backend does.
	if !known {
		return fmt.Errorf("%w: %q", ErrUnknownKind, job.Kind)
	}

	now := time.Now()
	if job.expired(now) {
		q.logger.Debug("job dropped, deadline already passed",
			"kind", job.Kind, "deadline", job.Deadline)
		return nil
	}

	// Commit deferral comes after validation, so a malformed job fails at
	// the call site, and before the started check, because the real enqueue
	// happens at commit.
	if deferEnqueue(ctx, job) {
		return nil
	}

	mj := memJob{job: Job{
		Kind: job.Kind,
		// Copied, so a caller that reuses its map after Enqueue cannot
		// change a job that is already queued.
		Payload:  clonePayload(job.Payload),
		Retry:    job.Retry,
		Deadline: job.Retry.effectiveDeadline(job, now),
		Attempt:  1,
	}}
	if job.IdempotencyKey != "" {
		mj.key = idempotencyID(job)
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	switch {
	case q.closed:
		return ErrClosed
	case !q.started:
		return ErrNotStarted
	}
	if mj.key != "" {
		if _, pending := q.keys[mj.key]; pending {
			q.logger.Debug("job already pending, not queued again",
				"kind", job.Kind, "idempotency_key", job.IdempotencyKey)
			return ErrDuplicateJob
		}
		q.keys[mj.key] = struct{}{}
	}
	q.ready = append(q.ready, mj)
	q.cond.Signal()
	return nil
}

// Start implements [Queue]. It starts the workers; handlers registered later
// are still found, because a worker looks its handler up per job.
func (q *memQueue) Start(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return ErrClosed
	}
	if q.started {
		return errors.New("jobs: queue already started")
	}
	q.started = true
	// The workers keep the values of ctx but not its cancellation: the queue
	// lives until Close, not until whatever call happened to start it.
	base, cancel := context.WithCancel(context.WithoutCancel(ctx)) //nolint:gosec // G118: Close calls cancel
	q.cancel = cancel
	q.workers.Add(q.concurrency)
	for range q.concurrency {
		go q.work(base)
	}
	return nil
}

// work is one worker. It runs jobs until the queue closes.
func (q *memQueue) work(ctx context.Context) {
	defer q.workers.Done()
	for {
		q.mu.Lock()
		for len(q.ready) == 0 && !q.closed {
			q.cond.Wait()
		}
		if q.closed {
			q.mu.Unlock()
			return
		}
		mj := q.ready[0]
		q.ready[0] = memJob{} // release the payload to the collector
		q.ready = q.ready[1:]
		h := q.handlers[mj.job.Kind]
		q.mu.Unlock()

		q.run(ctx, h, mj)
	}
}

// run executes one attempt of mj and decides what follows it: done, another
// attempt later, or dropped.
//
// Enqueue only accepts registered kinds and handlers are never removed, so h
// is never nil here.
func (q *memQueue) run(ctx context.Context, h Handler, mj memJob) {
	job := mj.job
	if !job.Deadline.IsZero() && !time.Now().Before(job.Deadline) {
		q.logger.Warn("dropping job past its deadline",
			"kind", job.Kind, "attempt", job.Attempt, "deadline", job.Deadline)
		q.finish(mj)
		return
	}

	// Each attempt sees its own copy of the payload, as on the durable tier,
	// where every attempt re-reads the stored row. A handler that edits its
	// payload cannot change what its retry receives.
	job.Payload = clonePayload(mj.job.Payload)
	err := q.call(ctx, h, job)
	if err == nil {
		q.finish(mj)
		return
	}

	// A failure caused by Close is not the job's fault, and the queue is
	// gone, so there is nothing to retry on.
	if ctx.Err() != nil {
		q.finish(mj)
		return
	}
	// The payload is NOT logged: it may carry entity content.
	q.logger.Warn("job attempt failed", "kind", job.Kind, "attempt", job.Attempt, "error", err)
	if job.FinalAttempt() {
		q.logger.Error("dropping job, retry budget exhausted",
			"kind", job.Kind, "attempts", job.Attempt, "retry", job.Retry.String())
		q.finish(mj)
		return
	}

	delay := retryDelay(job.Attempt)
	if !job.Deadline.IsZero() && !time.Now().Add(delay).Before(job.Deadline) {
		q.logger.Warn("dropping job, its deadline passes before the next attempt",
			"kind", job.Kind, "attempt", job.Attempt, "deadline", job.Deadline)
		q.finish(mj)
		return
	}
	mj.job.Attempt++
	q.retryLater(mj, delay)
}

// call runs h with the per-attempt time cap, turning a panic into an error so
// a buggy handler costs one attempt rather than a worker.
func (q *memQueue) call(ctx context.Context, h Handler, job Job) (err error) {
	ctx, cancel := context.WithTimeout(ctx, handlerTimeout)
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			q.logger.Error("job handler panicked",
				"kind", job.Kind, "attempt", job.Attempt, "panic", panicSummary(r), "stack", string(debug.Stack()))
			err = fmt.Errorf("jobs: handler for %q panicked: %s", job.Kind, panicSummary(r))
		}
	}()
	return h(ctx, job)
}

// clonePayload copies p and the maps and slices nested in it, so no write
// through one copy reaches another. Other values are shared; a payload must
// round-trip through JSON, so they are scalars.
func clonePayload(p map[string]any) map[string]any {
	if p == nil {
		return nil
	}
	out := make(map[string]any, len(p))
	for k, v := range p {
		out[k] = cloneValue(v)
	}
	return out
}

func cloneValue(v any) any {
	switch v := v.(type) {
	case map[string]any:
		return clonePayload(v)
	case []any:
		out := make([]any, len(v))
		for i, e := range v {
			out[i] = cloneValue(e)
		}
		return out
	default:
		return v
	}
}

// panicSummary describes a recovered panic value without its content. A
// handler may panic with a value built from its payload, and payloads are
// never logged. A runtime error carries no payload, so its text is kept.
func panicSummary(v any) string {
	if rerr, ok := v.(runtime.Error); ok {
		return rerr.Error()
	}
	return fmt.Sprintf("%T", v)
}

// retryLater puts mj back on the ready list after delay.
func (q *memQueue) retryLater(mj memJob, delay time.Duration) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	// A pending retry counts as a worker, so Close waits for a callback that
	// has already fired rather than returning while it still runs. The
	// callback's Done balances this Add, or Close's when Stop prevents the
	// callback from running at all.
	q.workers.Add(1)
	var t *time.Timer
	// The callback takes mu, and mu is held until t is in the map, so the
	// callback always finds its own entry.
	t = time.AfterFunc(delay, func() {
		defer q.workers.Done()
		q.mu.Lock()
		defer q.mu.Unlock()
		delete(q.timers, t)
		if q.closed {
			return
		}
		q.ready = append(q.ready, mj)
		q.cond.Signal()
	})
	q.timers[t] = struct{}{}
}

// finish frees mj's idempotency key: the job will not run again.
func (q *memQueue) finish(mj memJob) {
	if mj.key == "" {
		return
	}
	q.mu.Lock()
	delete(q.keys, mj.key)
	q.mu.Unlock()
}

// Close implements [Queue].
//
// Queued jobs and jobs waiting for a retry are dropped, as the package doc
// says the ephemeral tier does. Once the workers have returned, no goroutine
// the queue started remains: retry timers are stopped, not left to fire.
//
// When ctx ends before the handlers return, the goroutine that waits for them
// lives on until they do.
func (q *memQueue) Close(ctx context.Context) error {
	q.mu.Lock()
	if !q.closed {
		q.closed = true
		if q.cancel != nil {
			q.cancel()
		}
		for t := range q.timers {
			if t.Stop() {
				q.workers.Done() // the callback will not run; see retryLater
			}
		}
		clear(q.timers)
		q.ready = nil
		clear(q.keys)
		q.cond.Broadcast()
		drained := make(chan struct{})
		go func() {
			q.workers.Wait()
			close(drained)
		}()
		q.drained = drained
	}
	drained := q.drained
	q.mu.Unlock()

	select {
	case <-drained:
		return nil
	case <-ctx.Done():
	}
	select {
	case <-drained: // both were ready; the handlers did finish
		return nil
	default:
		return fmt.Errorf("jobs: handlers still running at close: %w", ctx.Err())
	}
}
