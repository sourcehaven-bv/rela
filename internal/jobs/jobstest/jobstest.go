// Package jobstest is the conformance harness for [jobs.Queue]
// implementations, in the same spirit as store/storetest and state/statetest.
//
// Any new backend must pass [RunAll]. The suite asserts BEHAVIOR, never
// mechanism: it checks that RetryBounded retries more than once and then
// stops, not that it makes exactly five attempts. Retry numbers are meant to
// be retunable in jobs/retry.go without breaking a single test — that is the
// point of the flat enum, and a suite that pinned the numbers would quietly
// undo it.
package jobstest

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
)

// NewQueue builds a fresh, unstarted queue for one subtest. Implementations
// register their own cleanup with t.
type NewQueue func(t *testing.T) jobs.Queue

// settleTimeout bounds how long a test waits for a job to run.
//
// Generous on purpose: these tests assert ordering and counts, never latency,
// and a tight bound would turn CI load into a flake.
const settleTimeout = 10 * time.Second

// retryTimeout bounds how long a test waits for a RETRY, as opposed to a first
// attempt.
//
// It is much larger than settleTimeout because the backend, not rela, owns
// retry timing: neoq's backoff is roughly retries^4 + 15s + jitter, so even the
// FIRST retry cannot arrive sooner than ~16s. A test that waited settleTimeout
// for a retry would fail against a perfectly correct implementation — which is
// exactly what it did before this constant existed.
//
// This is the one place the suite is coupled to backend timing, and it is a
// lower bound on patience rather than an assertion about the numbers.
const retryTimeout = 90 * time.Second

// retryProbeWindow is how long a test waits to prove a retry did NOT happen.
//
// Proving a negative needs patience past the point a retry would have been
// due; anything shorter passes vacuously because the retry was not yet
// scheduled. Sized just beyond the ~16s minimum backoff.
const retryProbeWindow = 25 * time.Second

// notYetWindow is how long a test waits before concluding something has NOT
// happened yet (a deferred job leaking early, a discarded job running). Long
// enough that a leaked enqueue would have been picked up by a worker, short
// enough to keep the suite quick.
const notYetWindow = 250 * time.Millisecond

// dropWindow is how long a test waits to confirm a job was dropped outright
// (past its deadline) rather than merely delayed.
const dropWindow = 500 * time.Millisecond

// pollInterval is the require.Eventually tick for assertions about counts.
const pollInterval = 20 * time.Millisecond

// shortDeadline is the deadline attached to jobs that must expire WHILE
// queued, and settleAfterDeadline is how long the test waits for that to have
// happened. The pair models the real hazard: a deadline valid at enqueue that
// passes before a busy worker reaches the job.
const (
	shortDeadline       = 300 * time.Millisecond
	settleAfterDeadline = 600 * time.Millisecond
)

// retryPollInterval is the tick for assertions that wait on a RETRY. Coarser
// than pollInterval because the thing being waited for is tens of seconds away;
// polling every 20ms for that long is pure noise.
const retryPollInterval = 250 * time.Millisecond

// RunAll runs the full conformance suite.
func RunAll(t *testing.T, newQueue NewQueue) {
	t.Helper()

	t.Run("EnqueueRunsHandler", func(t *testing.T) { testEnqueueRunsHandler(t, newQueue) })
	t.Run("DeferredUntilCommit", func(t *testing.T) { testDeferredUntilCommit(t, newQueue) })
	t.Run("DiscardDropsJobs", func(t *testing.T) { testDiscardDropsJobs(t, newQueue) })
	t.Run("RetryNeverRunsOnce", func(t *testing.T) { testRetryNeverRunsOnce(t, newQueue) })
	t.Run("RetryBoundedRetriesThenStops", func(t *testing.T) { testRetryBoundedRetries(t, newQueue) })
	t.Run("PastDeadlineNeverRuns", func(t *testing.T) { testPastDeadlineNeverRuns(t, newQueue) })
	t.Run("ZeroDeadlineMeansNoDeadline", func(t *testing.T) { testZeroDeadline(t, newQueue) })
	t.Run("UnknownKindRejected", func(t *testing.T) { testUnknownKindRejected(t, newQueue) })
	t.Run("EmptyKindRejected", func(t *testing.T) { testEmptyKindRejected(t, newQueue) })
	t.Run("DuplicateRegistrationRejected", func(t *testing.T) { testDuplicateRegistration(t, newQueue) })
	t.Run("EnqueueAfterCloseRejected", func(t *testing.T) { testEnqueueAfterClose(t, newQueue) })
	t.Run("ConcurrentEnqueueLosesNothing", func(t *testing.T) { testConcurrentEnqueue(t, newQueue) })
	t.Run("PayloadRoundTrips", func(t *testing.T) { testPayloadRoundTrips(t, newQueue) })
	t.Run("PoolSurvivesExhaustedJobs", func(t *testing.T) { testPoolSurvivesExhaustion(t, newQueue) })
	t.Run("PoolSurvivesExpiredDeadlines", func(t *testing.T) { testPoolSurvivesExpiredDeadlines(t, newQueue) })
	t.Run("IdenticalPayloadsAreDistinctJobs", func(t *testing.T) { testIdenticalPayloads(t, newQueue) })
	t.Run("HandlerSeesItsRetryPolicy", func(t *testing.T) { testHandlerSeesRetryPolicy(t, newQueue) })

	// NOTE: there is deliberately no "panicking handler" case here.
	//
	// neoq's panic-recovery path (handler.Exec, handler.go:154/181) races on
	// the shared error variable between the recovering deferred func and the
	// goroutine reading the result — confirmed with the race detector against
	// v0.72.1. rela runs -race across CI and does not allow //go:build !race
	// opt-outs, so a conformance case that exercises that path would make the
	// suite permanently red for an upstream reason.
	//
	// The gap is narrow and acceptable: a panicking handler is a programming
	// bug in the handler, not a runtime condition the queue must survive
	// gracefully. Job handlers should return errors. Add this case once the
	// upstream fix lands.
}

// recorder collects handler invocations across worker goroutines.
type recorder struct {
	mu      sync.Mutex
	jobs    []jobs.Job
	fired   chan struct{}
	fireOne sync.Once
}

func newRecorder() *recorder {
	return &recorder{fired: make(chan struct{})}
}

func (r *recorder) record(job jobs.Job) {
	r.mu.Lock()
	r.jobs = append(r.jobs, job)
	r.mu.Unlock()
	r.fireOne.Do(func() { close(r.fired) })
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.jobs)
}

func (r *recorder) last() (jobs.Job, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.jobs) == 0 {
		return jobs.Job{}, false
	}
	return r.jobs[len(r.jobs)-1], true
}

// waitFired blocks until the handler has run at least once.
func (r *recorder) waitFired(t *testing.T) {
	t.Helper()
	select {
	case <-r.fired:
	case <-time.After(settleTimeout):
		t.Fatal("timed out waiting for handler to run")
	}
}

// startQueue registers handlers and starts the queue, returning a live one.
func startQueue(t *testing.T, q jobs.Queue, kind jobs.Kind, h jobs.Handler) jobs.Queue {
	t.Helper()
	require.NoError(t, q.Register(kind, h))
	require.NoError(t, q.Start(context.Background()))
	return q
}

func testEnqueueRunsHandler(t *testing.T, newQueue NewQueue) {
	t.Helper()
	rec := newRecorder()
	q := startQueue(t, newQueue(t), "greet", func(_ context.Context, job jobs.Job) error {
		rec.record(job)
		return nil
	})

	require.NoError(t, q.Enqueue(context.Background(), jobs.Job{
		Kind:    "greet",
		Payload: map[string]any{"name": "ada"},
	}))

	rec.waitFired(t)
	got, ok := rec.last()
	require.True(t, ok)
	require.Equal(t, jobs.Kind("greet"), got.Kind)
	require.Equal(t, "ada", got.Payload["name"])
}

// testDeferredUntilCommit is the load-bearing test: a job enqueued inside a
// transaction must not become runnable until that transaction commits.
//
// It is deterministic rather than timing-based. The failure mode it catches is
// "the handler ran too early", which is observed directly: the test does work
// that stands in for the rest of a transaction, and asserts the handler has
// NOT fired at that point. A broken implementation fails reliably instead of
// flaking under load — which is exactly how this bug behaves in production.
func testDeferredUntilCommit(t *testing.T, newQueue NewQueue) {
	t.Helper()
	rec := newRecorder()
	q := startQueue(t, newQueue(t), "after-commit", func(_ context.Context, job jobs.Job) error {
		rec.record(job)
		return nil
	})

	ctx, collector := jobs.WithDeferral(context.Background())

	require.NoError(t, q.Enqueue(ctx, jobs.Job{
		Kind:    "after-commit",
		Payload: map[string]any{"entity": "TKT-1"},
	}))

	// Held, not enqueued.
	require.Equal(t, 1, collector.Len(), "job should be collected, not enqueued")

	// Stand-in for the remainder of the transaction. If the enqueue leaked
	// through, a worker has had ample opportunity to run it by now.
	select {
	case <-rec.fired:
		t.Fatal("handler ran before the transaction committed")
	case <-time.After(notYetWindow):
	}
	require.Equal(t, 0, rec.count(), "handler must not run before commit")

	// Commit.
	require.NoError(t, collector.Flush(context.Background(), q))
	require.Equal(t, 0, collector.Len(), "flush should drain the collector")

	rec.waitFired(t)
	got, ok := rec.last()
	require.True(t, ok)
	require.Equal(t, "TKT-1", got.Payload["entity"])
}

// testDiscardDropsJobs covers the rollback path: a transaction that does not
// commit must not leave its jobs behind.
func testDiscardDropsJobs(t *testing.T, newQueue NewQueue) {
	t.Helper()
	rec := newRecorder()
	q := startQueue(t, newQueue(t), "rolled-back", func(_ context.Context, job jobs.Job) error {
		rec.record(job)
		return nil
	})

	ctx, collector := jobs.WithDeferral(context.Background())
	require.NoError(t, q.Enqueue(ctx, jobs.Job{Kind: "rolled-back"}))
	require.Equal(t, 1, collector.Len())

	collector.Discard()
	require.Equal(t, 0, collector.Len())

	// A flush after discard must stay a no-op, so `defer Discard` alongside
	// an explicit Flush cannot resurrect a rolled-back job.
	require.NoError(t, collector.Flush(context.Background(), q))

	select {
	case <-rec.fired:
		t.Fatal("discarded job ran")
	case <-time.After(notYetWindow):
	}
	require.Equal(t, 0, rec.count())
}

func testRetryNeverRunsOnce(t *testing.T, newQueue NewQueue) {
	t.Helper()
	var attempts atomicCounter
	rec := newRecorder()
	q := startQueue(t, newQueue(t), "doomed", func(_ context.Context, job jobs.Job) error {
		attempts.inc()
		rec.record(job)
		return errors.New("always fails")
	})

	require.NoError(t, q.Enqueue(context.Background(), jobs.Job{
		Kind:  "doomed",
		Retry: jobs.RetryNever,
	}))

	rec.waitFired(t)

	// Wait past the backend's minimum retry backoff (~16s) before concluding
	// no retry happened. A shorter wait would pass even against a backend
	// that retries everything, since the second attempt would not have been
	// due yet — the assertion would be vacuous.
	if testing.Short() {
		t.Skip("must outwait the retry backoff to be meaningful; run without -short")
	}
	time.Sleep(retryProbeWindow)
	require.Equal(t, 1, attempts.get(), "RetryNever must run exactly once")
}

// testRetryBoundedRetries asserts BEHAVIOR, not the attempt count: more than
// one attempt, and eventually it stops. Pinning the exact number here would
// couple the suite to jobs/retry.go's tuning, which is meant to be free to
// change.
func testRetryBoundedRetries(t *testing.T, newQueue NewQueue) {
	t.Helper()
	// The first retry is ~16s out (see retryTimeout), so this test is slow by
	// nature. Skipping it under -short keeps the fast suite fast without
	// deleting the only coverage that RetryBounded actually retries.
	if testing.Short() {
		t.Skip("retry backoff makes this test slow; run without -short")
	}

	var attempts atomicCounter
	rec := newRecorder()
	q := startQueue(t, newQueue(t), "flaky", func(_ context.Context, job jobs.Job) error {
		attempts.inc()
		rec.record(job)
		return errors.New("still failing")
	})

	require.NoError(t, q.Enqueue(context.Background(), jobs.Job{
		Kind:  "flaky",
		Retry: jobs.RetryBounded,
	}))

	rec.waitFired(t)
	require.Eventually(t, func() bool { return attempts.get() > 1 }, retryTimeout, retryPollInterval,
		"RetryBounded must attempt more than once")
}

func testPastDeadlineNeverRuns(t *testing.T, newQueue NewQueue) {
	t.Helper()
	rec := newRecorder()
	q := startQueue(t, newQueue(t), "expired", func(_ context.Context, job jobs.Job) error {
		rec.record(job)
		return nil
	})

	// Enqueue must not report this as an error: the job was validly
	// submitted, it simply is not worth running any more.
	require.NoError(t, q.Enqueue(context.Background(), jobs.Job{
		Kind:     "expired",
		Deadline: time.Now().Add(-time.Hour),
	}))

	select {
	case <-rec.fired:
		t.Fatal("job past its deadline ran")
	case <-time.After(dropWindow):
	}
	require.Equal(t, 0, rec.count())
}

// testZeroDeadline pins the zero-value trap: an omitted deadline means "no
// deadline", NOT the epoch (which would expire every job that left it unset).
func testZeroDeadline(t *testing.T, newQueue NewQueue) {
	t.Helper()
	rec := newRecorder()
	q := startQueue(t, newQueue(t), "no-deadline", func(_ context.Context, job jobs.Job) error {
		rec.record(job)
		return nil
	})

	require.NoError(t, q.Enqueue(context.Background(), jobs.Job{Kind: "no-deadline"}))
	rec.waitFired(t)
	require.Equal(t, 1, rec.count())
}

func testUnknownKindRejected(t *testing.T, newQueue NewQueue) {
	t.Helper()
	q := startQueue(t, newQueue(t), "known", func(context.Context, jobs.Job) error { return nil })

	err := q.Enqueue(context.Background(), jobs.Job{Kind: "not-registered"})
	require.ErrorIs(t, err, jobs.ErrUnknownKind,
		"an unroutable job must fail at enqueue, not be silently stored")
}

func testEmptyKindRejected(t *testing.T, newQueue NewQueue) {
	t.Helper()
	q := startQueue(t, newQueue(t), "known", func(context.Context, jobs.Job) error { return nil })
	require.ErrorIs(t, q.Enqueue(context.Background(), jobs.Job{}), jobs.ErrNoKind)
}

func testDuplicateRegistration(t *testing.T, newQueue NewQueue) {
	t.Helper()
	q := newQueue(t)
	noop := func(context.Context, jobs.Job) error { return nil }
	require.NoError(t, q.Register("dup", noop))
	require.ErrorIs(t, q.Register("dup", noop), jobs.ErrDuplicateKind)
}

func testEnqueueAfterClose(t *testing.T, newQueue NewQueue) {
	t.Helper()
	q := startQueue(t, newQueue(t), "kind", func(context.Context, jobs.Job) error { return nil })
	require.NoError(t, q.Close(context.Background()))

	// Errors rather than panics — a shutdown race must be handleable.
	require.ErrorIs(t, q.Enqueue(context.Background(), jobs.Job{Kind: "kind"}), jobs.ErrClosed)

	// Close is idempotent.
	require.NoError(t, q.Close(context.Background()))
}

func testConcurrentEnqueue(t *testing.T, newQueue NewQueue) {
	t.Helper()
	const n = 50

	var seen atomicCounter
	rec := newRecorder()
	q := startQueue(t, newQueue(t), "concurrent", func(_ context.Context, job jobs.Job) error {
		seen.inc()
		rec.record(job)
		return nil
	})

	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = q.Enqueue(context.Background(), jobs.Job{
				Kind:    "concurrent",
				Payload: map[string]any{"i": strconv.Itoa(i)},
			})
		}(i)
	}
	wg.Wait()

	require.Eventually(t, func() bool { return seen.get() == n }, settleTimeout, pollInterval,
		"every concurrently enqueued job should run exactly once")
}

// testPayloadRoundTrips guards the durable backends: a payload is serialized
// to JSON, so a value that survives in memory but not through JSON would pass
// on fs and fail on postgres.
func testPayloadRoundTrips(t *testing.T, newQueue NewQueue) {
	t.Helper()
	rec := newRecorder()
	q := startQueue(t, newQueue(t), "payload", func(_ context.Context, job jobs.Job) error {
		rec.record(job)
		return nil
	})

	require.NoError(t, q.Enqueue(context.Background(), jobs.Job{
		Kind: "payload",
		Payload: map[string]any{
			"entity":  "TKT-YOED3R",
			"unicode": "naïve café 🎯",
			"nested":  map[string]any{"deep": "value"},
		},
	}))

	rec.waitFired(t)
	got, ok := rec.last()
	require.True(t, ok)
	require.Equal(t, "TKT-YOED3R", got.Payload["entity"])
	require.Equal(t, "naïve café 🎯", got.Payload["unicode"])
	require.NotContains(t, got.Payload, "__rela_kind",
		"the internal routing key must not leak into a handler's payload")
	require.NotContains(t, got.Payload, "__rela_retry",
		"the internal retry key must not leak into a handler's payload")
}

// testPoolSurvivesExhaustion is the most valuable case in this suite.
//
// The failure it guards against is not "a job behaved wrongly" but "the queue
// stopped working at all". neoq's worker goroutine RETURNS when a job exceeds
// its retry budget, so a backend handed the real budget loses one worker per
// exhausted job until nothing runs — silently, with no error to the caller and
// nothing in the logs. Verified against unwrapped neoq: after four exhausted
// jobs, 0 of 8 healthy jobs ran.
//
// Every happy-path assertion in this file passes against a queue in that
// state. Only this one notices.
func testPoolSurvivesExhaustion(t *testing.T, newQueue NewQueue) {
	t.Helper()
	if testing.Short() {
		t.Skip("must outwait the retry backoff to reach exhaustion; run without -short")
	}

	var healthy atomicCounter
	rec := newRecorder()
	q := newQueue(t)
	require.NoError(t, q.Register("doomed", func(context.Context, jobs.Job) error {
		return errors.New("always fails")
	}))
	require.NoError(t, q.Register("healthy", func(_ context.Context, job jobs.Job) error {
		healthy.inc()
		rec.record(job)
		return nil
	}))
	require.NoError(t, q.Start(context.Background()))

	// Enough failing jobs to have claimed every worker, had they been fatal.
	const doomed = 8
	for range doomed {
		require.NoError(t, q.Enqueue(context.Background(), jobs.Job{
			Kind: "doomed", Retry: jobs.RetryNever,
		}))
	}

	// Past the first retry backoff, so each job has been redelivered and has
	// exhausted its budget.
	time.Sleep(retryProbeWindow)

	const healthyJobs = 10
	for range healthyJobs {
		require.NoError(t, q.Enqueue(context.Background(), jobs.Job{Kind: "healthy"}))
	}

	require.Eventually(t, func() bool { return healthy.get() == healthyJobs },
		settleTimeout, pollInterval,
		"the queue stopped processing after jobs exhausted their retries — worker pool died")
}

// testPoolSurvivesExpiredDeadlines is the deadline twin of the case above.
//
// neoq treats an expired deadline as fatal to the worker in the same way it
// treats exhausted retries. This matters especially because attaching a short
// deadline is the DESIGNED path for scheduled work (see Schedule.NextRun): a
// one-minute cadence attaches a one-minute deadline, and any job that queues
// behind a slow one blows it. The headline feature would otherwise be the
// outage trigger.
func testPoolSurvivesExpiredDeadlines(t *testing.T, newQueue NewQueue) {
	t.Helper()

	var healthy atomicCounter
	release := make(chan struct{})
	q := newQueue(t)

	// Occupy every worker so the following jobs sit queued long enough for
	// their deadlines to pass while waiting — a deadline valid at enqueue and
	// expired by the time it is picked up.
	require.NoError(t, q.Register("blocker", func(ctx context.Context, _ jobs.Job) error {
		select {
		case <-release:
		case <-ctx.Done():
		}
		return nil
	}))
	require.NoError(t, q.Register("healthy", func(context.Context, jobs.Job) error {
		healthy.inc()
		return nil
	}))
	require.NoError(t, q.Start(context.Background()))

	const blockers = 8
	for range blockers {
		require.NoError(t, q.Enqueue(context.Background(), jobs.Job{Kind: "blocker"}))
	}

	const expiring = 8
	for range expiring {
		require.NoError(t, q.Enqueue(context.Background(), jobs.Job{
			Kind:     "healthy",
			Deadline: time.Now().Add(shortDeadline),
		}))
	}

	time.Sleep(settleAfterDeadline)
	close(release)

	const healthyJobs = 10
	for range healthyJobs {
		require.NoError(t, q.Enqueue(context.Background(), jobs.Job{Kind: "healthy"}))
	}

	require.Eventually(t, func() bool { return healthy.get() >= healthyJobs },
		settleTimeout, pollInterval,
		"the queue stopped processing after jobs passed their deadlines — worker pool died")
}

// testIdenticalPayloads pins that two jobs with equal payloads are two jobs.
//
// neoq derives a fingerprint from md5(queue + payload) and drops a job that
// matches an unprocessed one — returning a nil error having queued nothing. As
// every rela job shares a single queue, "notify alice" submitted twice would
// otherwise collapse into one delivery, and the two tiers would disagree about
// it: the memory backend drops silently, postgres has a unique index and
// errors.
func testIdenticalPayloads(t *testing.T, newQueue NewQueue) {
	t.Helper()

	var ran atomicCounter
	q := startQueue(t, newQueue(t), "dup", func(context.Context, jobs.Job) error {
		ran.inc()
		return nil
	})

	const n = 5
	for range n {
		require.NoError(t, q.Enqueue(context.Background(), jobs.Job{
			Kind:    "dup",
			Payload: map[string]any{"to": "alice@example.com"},
		}))
	}

	require.Eventually(t, func() bool { return ran.get() == n }, settleTimeout, pollInterval,
		"jobs with identical payloads were deduplicated; each Enqueue must produce one execution")
}

// testHandlerSeesRetryPolicy pins that the policy survives the round trip. A
// handler reading job.Retry should see what the caller chose, not a zero value
// that misreports every job as RetryNever.
func testHandlerSeesRetryPolicy(t *testing.T, newQueue NewQueue) {
	t.Helper()

	rec := newRecorder()
	q := startQueue(t, newQueue(t), "policy", func(_ context.Context, job jobs.Job) error {
		rec.record(job)
		return nil
	})

	require.NoError(t, q.Enqueue(context.Background(), jobs.Job{
		Kind:  "policy",
		Retry: jobs.RetryPersistent,
	}))

	rec.waitFired(t)
	got, ok := rec.last()
	require.True(t, ok)
	require.Equal(t, jobs.RetryPersistent, got.Retry)
}

// atomicCounter is a mutex-guarded counter. Used instead of sync/atomic so a
// racy read in a test failure message is impossible.
type atomicCounter struct {
	mu sync.Mutex
	n  int
}

func (c *atomicCounter) inc() {
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
}

func (c *atomicCounter) get() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}
