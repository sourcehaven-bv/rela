package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"sync"
	"time"

	"github.com/acaloiaro/neoq"
	"github.com/acaloiaro/neoq/handler"
	neoqjobs "github.com/acaloiaro/neoq/jobs"
	"github.com/google/uuid"
)

// neoqQueue adapts neoq to [Queue]. Both the memory and postgres backends are
// this type with a different neoq backend injected, so the seam's semantics —
// validation, commit deferral, retry mapping, principal propagation — are
// implemented exactly once and cannot drift between tiers.
//
// Backend construction lives in memqueue.go / pgqueue.go, which are build-
// tagged; this file must stay backend-agnostic so the default build never
// links pgx.
type neoqQueue struct {
	nq     neoq.Neoq
	logger *slog.Logger

	// queueName is the neoq queue every kind is submitted to. rela routes by
	// Kind inside a single queue rather than mapping kind→neoq queue: neoq
	// starts a worker per queue, and one worker pool with a dispatch table is
	// both cheaper and keeps concurrency governable in one place.
	queueName string

	// concurrency is the worker count for the dispatch handler.
	concurrency int

	mu       sync.RWMutex
	handlers map[string]Handler
	started  bool
	closed   bool

	// enqueueMu serializes calls into the backend's Enqueue.
	//
	// WORKAROUND for an upstream data race: neoq's memory backend assigns
	// `job.ID = m.jobCount` while reading m.jobCount OUTSIDE the mutex that
	// guards its increment (memory_backend.go:116 in v0.72.1). Under
	// concurrent enqueues that races, and jobs collide on a duplicate ID and
	// are silently dropped — an enqueue returns nil having queued nothing.
	//
	// It is reproducible with pure neoq and no rela code, and rela runs the
	// race detector across CI, so containing it here is not optional. The
	// cost is small: enqueue is a channel send, and jobs are I/O-bound in the
	// handler, not at submission.
	//
	// Remove this once the upstream fix lands (a local read inside the lock)
	// and the pinned version moves past it.
	enqueueMu sync.Mutex
}

// payloadKindKey is the payload field carrying [Job.Kind] through neoq.
//
// Prefixed to avoid colliding with a caller's own payload keys — the whole
// payload map is round-tripped through JSON, and a job whose payload happened
// to contain "kind" would otherwise hijack dispatch.
// handlerTimeout bounds a single handler execution.
//
// Set explicitly because neoq's default is 30s, which is far too short for the
// work this package exists to carry (LLM calls, slow SMTP). Generous, since a
// job that genuinely hangs is caught by its deadline and its retry budget
// rather than by this. It is a backstop against a wedged handler holding a
// worker forever, not a latency target.
const handlerTimeout = 15 * time.Minute

const payloadKindKey = "__rela_kind"

// payloadRetryKey carries [Job.Retry] through neoq so the dispatcher can
// enforce the policy itself.
//
// Needed because neoq's own MaxRetries cannot express "never": see
// Retry.maxRetries for why zero is unusable. The value round-trips through
// JSON, so it is read back as a number of any concrete type.
const payloadRetryKey = "__rela_retry"

// payloadDeadlineKey carries the job's effective deadline (Unix nanoseconds)
// through neoq so the dispatcher can enforce it.
//
// Needed because neoq's own Deadline field is fatal to the worker when it
// expires: see backendRetryBudget.
const payloadDeadlineKey = "__rela_deadline"

// payloadAttemptKey carries the attempt count this package maintains itself.
//
// neoq tracks its own Retries, but reaching ITS limit kills the worker, so
// rela keeps a parallel count and stops the job before neoq's budget is
// approached.
const payloadAttemptKey = "__rela_attempt"

// newNeoqQueue wraps an initialized neoq backend.
//
// Nil: rejected — a nil backend or logger is a wiring bug, and substituting a
// no-op would turn every enqueue into a silent drop discovered much later.
func newNeoqQueue(nq neoq.Neoq, logger *slog.Logger, queueName string, concurrency int) (*neoqQueue, error) {
	if nq == nil {
		return nil, errors.New("jobs: neoq backend must not be nil")
	}
	if logger == nil {
		return nil, errors.New("jobs: logger must not be nil")
	}
	if queueName == "" {
		return nil, errors.New("jobs: queue name must not be empty")
	}
	if concurrency < 1 {
		return nil, fmt.Errorf("jobs: concurrency must be >= 1, got %d", concurrency)
	}
	return &neoqQueue{
		nq:          nq,
		logger:      logger,
		queueName:   queueName,
		concurrency: concurrency,
		handlers:    make(map[string]Handler),
	}, nil
}

// Register implements [Queue].
func (q *neoqQueue) Register(kind string, h Handler) error {
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
func (q *neoqQueue) Enqueue(ctx context.Context, job Job) error {
	if err := job.validate(); err != nil {
		return err
	}

	q.mu.RLock()
	closed := q.closed
	started := q.started
	_, known := q.handlers[job.Kind]
	q.mu.RUnlock()

	if closed {
		return ErrClosed
	}
	// Reject an unroutable job at the door. Accepting it would store work
	// that no worker can ever run — a drop discovered hours later, if at all.
	if !known {
		return fmt.Errorf("%w: %q", ErrUnknownKind, job.Kind)
	}

	now := time.Now()

	// A job already past its deadline is dropped here rather than enqueued
	// and failed by a worker: the outcome is identical and this way it costs
	// no storage, no attempt, and no error log that looks like a real fault.
	if job.expired(now) {
		q.logger.Debug("job dropped, deadline already passed",
			"kind", job.Kind, "deadline", job.Deadline)
		return nil
	}

	// Commit deferral. Must come after validation — a caller should learn
	// about a malformed job at the call site, not at commit time, where the
	// error surfaces far from its cause.
	//
	// Checked BEFORE the started check: a transaction may legitimately
	// collect jobs before the queue is running, since the actual enqueue
	// happens later, at commit.
	if deferEnqueue(ctx, job) {
		return nil
	}

	// The backend only creates its queue channel in Start, so enqueueing
	// first fails with an upstream error describing neoq's internals. Report
	// the caller's actual mistake instead.
	if !started {
		return ErrNotStarted
	}

	payload := make(map[string]any, len(job.Payload)+3)
	maps.Copy(payload, job.Payload)
	payload[payloadKindKey] = job.Kind
	payload[payloadRetryKey] = int(job.Retry)
	if dl := job.Retry.effectiveDeadline(job, now); !dl.IsZero() {
		payload[payloadDeadlineKey] = dl.UnixNano()
	}

	// MaxRetries is a budget neoq will never reach, and Deadline is left
	// UNSET, because neoq treats exceeding either as fatal to the worker
	// goroutine. Both are enforced in dispatch instead — see
	// backendRetryBudget for the full reasoning.
	maxRetries := backendRetryBudget
	nj := &neoqjobs.Job{
		Queue:      q.queueName,
		Payload:    payload,
		MaxRetries: &maxRetries,

		// A unique fingerprint per enqueue. neoq otherwise derives one from
		// md5(queue + payload) and silently DROPS a job matching an
		// unprocessed one — returning a nil error having queued nothing. As
		// every rela job shares one queue, two legitimately identical
		// payloads ("notify alice" twice) would collapse into one. Worse,
		// the tiers disagree: the memory backend drops silently while
		// postgres has a unique index and returns an error. An explicit
		// fingerprint disables the behavior on both. If deduplication is
		// ever wanted it should be an opt-in field on Job, not an accident
		// of payload equality.
		Fingerprint: newFingerprint(),
	}

	// Serialized: see the enqueueMu field comment for the upstream race this
	// contains.
	q.enqueueMu.Lock()
	_, err := q.nq.Enqueue(ctx, nj)
	q.enqueueMu.Unlock()
	if err != nil {
		return fmt.Errorf("jobs: enqueue %q: %w", job.Kind, err)
	}
	return nil
}

// Start implements [Queue]. It registers a single dispatch handler that routes
// by kind, so handlers registered before Start all share one worker pool.
func (q *neoqQueue) Start(ctx context.Context) error {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return ErrClosed
	}
	if q.started {
		q.mu.Unlock()
		return errors.New("jobs: queue already started")
	}
	q.started = true
	// The lock is deliberately held ACROSS the call into neoq. Its memory
	// backend mutates cancelFuncs from Start under its mutex but reads and
	// nils the same slice from Shutdown WITHOUT it, so a concurrent
	// Start/Close is an upstream data race. Serializing here is the only
	// containment available to us, and CI runs -race with no opt-out.
	defer q.mu.Unlock()

	// JobTimeout is set EXPLICITLY. neoq defaults it to 30s when unset, which
	// would silently kill exactly the work this package exists for — an LLM
	// call or a slow SMTP send — and count it as a failed attempt. The value
	// is ours to choose and document rather than inherit.
	h := handler.New(q.queueName, q.dispatch,
		handler.Concurrency(q.concurrency),
		handler.JobTimeout(handlerTimeout),
	)
	if err := q.nq.Start(ctx, h); err != nil {
		return fmt.Errorf("jobs: start queue %q: %w", q.queueName, err)
	}
	return nil
}

// dispatch is the single neoq handler. It resolves the rela handler by kind
// and invokes it.
func (q *neoqQueue) dispatch(ctx context.Context) error {
	nj, err := neoqjobs.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("jobs: no job on context: %w", err)
	}

	kind, _ := nj.Payload[payloadKindKey].(string)
	if kind == "" {
		// Unroutable. Returning an error would retry it to no purpose, so
		// log and consume — nothing about a later attempt would differ.
		q.logger.Error("job has no kind, dropping", "job_id", nj.ID)
		return nil
	}

	q.mu.RLock()
	h, ok := q.handlers[kind]
	q.mu.RUnlock()
	if !ok {
		// Reachable on a durable backend: a job enqueued by a previous
		// build whose kind this binary no longer registers. Retrying will
		// not make the handler appear.
		q.logger.Error("no handler registered for kind, dropping", "kind", kind, "job_id", nj.ID)
		return nil
	}

	// Terminal conditions are evaluated HERE, never by neoq.
	//
	// neoq's worker returns from its goroutine when a job exceeds its
	// MaxRetries or its Deadline, so letting it reach either would silently
	// shrink the worker pool to nothing under exactly the failure load the
	// retry policy exists to absorb. Both budgets are therefore withheld
	// from the backend and enforced at this chokepoint — see
	// backendRetryBudget.
	//
	// Returning nil (consuming the attempt) rather than an error is the
	// point: an error would schedule yet another retry of a job that is
	// already spent.
	retry := retryOf(nj.Payload)
	attempt := attemptOf(nj.Payload) + 1

	if dl, ok := deadlineOf(nj.Payload); ok && !time.Now().Before(dl) {
		q.logger.Warn("dropping job past its deadline",
			"kind", kind, "job_id", nj.ID, "attempt", attempt, "deadline", dl)
		return nil
	}
	if attempt > retry.maxAttempts() {
		q.logger.Error("dropping job, retry budget exhausted",
			"kind", kind, "job_id", nj.ID, "attempts", attempt-1, "retry", retry.String())
		return nil
	}

	// The attempt counter lives on the job neoq re-queues, so it must be
	// written back onto the payload it carries — not onto our copy.
	nj.Payload[payloadAttemptKey] = attempt

	payload := make(map[string]any, len(nj.Payload))
	for k, v := range nj.Payload {
		switch k {
		case payloadKindKey, payloadRetryKey, payloadDeadlineKey, payloadAttemptKey:
			continue
		}
		payload[k] = v
	}

	// Retry is carried through so a handler inspecting it sees the policy the
	// caller actually chose, not the zero value.
	job := Job{Kind: kind, Payload: payload, Retry: retry}
	if dl, ok := deadlineOf(nj.Payload); ok {
		job.Deadline = dl
	}

	if err := h(ctx, job); err != nil {
		// Logged at Warn, not Error: a failure that the retry policy will
		// absorb is not yet a fault. The payload is NOT logged — it may
		// carry entity content.
		q.logger.Warn("job attempt failed",
			"kind", kind, "job_id", nj.ID, "attempt", nj.Retries+1, "error", err)
		return err
	}
	return nil
}

// Close implements [Queue].
//
// KNOWN LEAK (upstream): neoq starts a robfig/cron goroutine per backend and
// never stops it — its Shutdown cancels contexts but does not call cron.Stop.
// Verified: 20 create/close cycles leak exactly 20 goroutines. There is no
// handle exposed for us to stop it.
//
// Bounded and harmless for a single long-lived process, which is every current
// caller. It is NOT harmless if rela ever assembles one Services per tenant
// and evicts them (appbuild.SharedBase documents that shape): each eviction
// would leak a goroutine permanently. Fix upstream before relying on tenant
// churn.
func (q *neoqQueue) Close(ctx context.Context) error {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return nil
	}
	q.closed = true
	// Held across Shutdown for the same reason Start holds it — see there.
	defer q.mu.Unlock()

	q.nq.Shutdown(ctx)
	return nil
}

// retryOf reads the retry policy back out of a neoq payload.
//
// The payload round-trips through JSON on a durable backend, so the value may
// come back as float64 rather than the int that was written; both are handled.
//
// An absent or unrecognized value falls back to RetryNever, the SAFEST policy
// — not the most forgiving one. The only way the key goes missing is a corrupt
// or foreign job, and guessing "retry it" there would re-run work whose whole
// declaration may have been "never repeat this side effect". A missed retry is
// recoverable; a duplicated payment or mail is not.
func retryOf(payload map[string]any) Retry {
	r, ok := intFromPayload(payload, payloadRetryKey)
	if !ok {
		return RetryNever
	}
	out := Retry(r)
	if !out.valid() {
		return RetryNever
	}
	return out
}

// attemptOf reads the attempt count this package maintains. Absent means the
// job has not run yet.
func attemptOf(payload map[string]any) int {
	n, ok := intFromPayload(payload, payloadAttemptKey)
	if !ok || n < 0 {
		return 0
	}
	return n
}

// deadlineOf reads the effective deadline. The second result is false when the
// job carries none.
func deadlineOf(payload map[string]any) (time.Time, bool) {
	n, ok := intFromPayload(payload, payloadDeadlineKey)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(0, int64(n)), true
}

// intFromPayload reads an integer that may have round-tripped through JSON,
// where it comes back as float64.
func intFromPayload(payload map[string]any, key string) (int, bool) {
	switch v := payload[key].(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

// newFingerprint returns a value unique to one enqueue.
//
// See the Fingerprint field in Enqueue: this exists to DISABLE neoq's
// payload-equality deduplication, which would otherwise silently drop a job
// that looks like one already in flight.
func newFingerprint() string {
	return uuid.NewString()
}
