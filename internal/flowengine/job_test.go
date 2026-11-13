package flowengine

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
)

// jobFlow posts a webhook and ends when it succeeds.
const jobFlow = `
flows:
  ship:
    subject: person
    run_as: system:onboarding-bot
    start: [{ on: manual, permission: onboarding.start }]
    initial: send
    states:
      send:
        outputs:
          - set: { properties: { stage: sending } }
        job:
          kind: webhook
          url: https://hooks.example.com/ship
          retry: { attempts: 3, backoff: { initial: 10s, factor: 2, max: 1m }, timeout: 30s }
        on:
          done: { to: sent }
      sent: { final: true }
`

// jobForkFlow forks into a job branch and a branch a timer or a person
// moves on.
const jobForkFlow = `
flows:
  ship:
    subject: person
    run_as: system:onboarding-bot
    start: [{ on: manual, permission: onboarding.start }]
    initial: start
    states:
      start:
        on:
          done: { to: [send, review] }
      send:
        job:
          kind: webhook
          url: https://hooks.example.com/ship
          retry: false
        on:
          done: { to: joined }
      review:
        waiting_for: [hr]
        on:
          change: { to: joined, roles: [hr] }
        after:
          slow: { period: 1h, to: joined }
      joined:
        join: all
        on:
          done: { to: finished }
      finished: { final: true }
`

// fakeQueue records the attempts the engine queues.
type fakeQueue struct {
	mu        sync.Mutex
	refs      []AttemptRef
	keys      []string
	deadlines []time.Time
	err       error
}

func (q *fakeQueue) EnqueueAttempt(_ context.Context, ref AttemptRef, key string, deadline time.Time) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.err != nil {
		return q.err
	}
	q.refs = append(q.refs, ref)
	q.keys = append(q.keys, key)
	q.deadlines = append(q.deadlines, deadline)
	return nil
}

func (q *fakeQueue) last(t *testing.T) AttemptRef {
	t.Helper()
	q.mu.Lock()
	defer q.mu.Unlock()
	require.NotEmpty(t, q.refs, "nothing queued")
	return q.refs[len(q.refs)-1]
}

func (q *fakeQueue) count() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.refs)
}

// fakeRunner returns its results in order, then nil. block makes an
// attempt wait for its context; during runs inside an attempt.
type fakeRunner struct {
	mu      sync.Mutex
	calls   []JobCall
	results []error
	block   bool
	during  func()
}

func (r *fakeRunner) RunJob(ctx context.Context, call JobCall) error {
	r.mu.Lock()
	r.calls = append(r.calls, call)
	block, during := r.block, r.during
	var res error
	if len(r.results) > 0 {
		res, r.results = r.results[0], r.results[1:]
	}
	r.mu.Unlock()
	if during != nil {
		during()
	}
	if block {
		<-ctx.Done()
		return ctx.Err()
	}
	return res
}

func (r *fakeRunner) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

type jobHarness struct {
	harness
	q *fakeQueue
	r *fakeRunner
}

// startJob starts a run of src on PER-1 with the clock at the real time,
// so an attempt's deadline bounds its real context.
func startJob(t *testing.T, src string, r *fakeRunner) (jobHarness, flow.RunID) {
	t.Helper()
	q := &fakeQueue{}
	if r == nil {
		r = &fakeRunner{}
	}
	h := newHarness(t, src, func(d *Deps) { d.Jobs, d.JobRunner = q, r })
	*h.clock = time.Now().UTC().Truncate(time.Second)
	h.f.addPerson("PER-1", map[string]any{"name": "Ada"})
	id, err := h.eng.Start(context.Background(), "ship", entity.Ref{ID: "PER-1"})
	require.NoError(t, err)
	return jobHarness{harness: h, q: q, r: r}, id
}

func (h jobHarness) job(t *testing.T, id flow.RunID) flowrun.JobAttempt {
	t.Helper()
	run, err := h.runs.Get(context.Background(), id)
	require.NoError(t, err)
	require.NotNil(t, run.Pending)
	require.NotNil(t, run.Pending.Job)
	return *run.Pending.Job
}

func (h jobHarness) advance(t *testing.T, d time.Duration) {
	t.Helper()
	*h.clock = h.clock.Add(d)
	_, err := h.eng.Due(context.Background())
	require.NoError(t, err)
}

func (h jobHarness) ended(t *testing.T, id flow.RunID) {
	t.Helper()
	_, err := h.runs.Get(context.Background(), id)
	require.ErrorIs(t, err, flowrun.ErrNotFound)
}

func TestJob_SucceedsAndMovesOn(t *testing.T) {
	h, id := startJob(t, jobFlow, nil)
	ctx := context.Background()
	j := h.job(t, id)
	require.Equal(t, flowrun.JobQueued, j.Phase)
	require.Equal(t, int64(1), j.Seq)
	require.Equal(t, []map[string]any{{"stage": "sending"}}, h.f.sets, "outputs are written before the job")

	ref := h.q.last(t)
	require.Equal(t, h.clock.Add(30*time.Second), h.q.deadlines[0])
	require.NoError(t, jobSteps{h.eng}.HandleAttempt(ctx, ref))
	require.Equal(t, 1, h.r.callCount())
	call := h.r.calls[0]
	require.Equal(t, "ship", call.Flow)
	require.Equal(t, "send", call.State)
	require.Equal(t, "system:onboarding-bot", call.RunAs)
	require.Equal(t, entity.Ref{ID: "PER-1"}, call.Subject)
	require.Equal(t, 1, call.Attempt)
	h.ended(t, id)
	require.Equal(t, "sent", h.f.header(id)["state"])

	// A redelivery of the attempt does nothing.
	require.NoError(t, jobSteps{h.eng}.HandleAttempt(ctx, ref))
	require.Equal(t, 1, h.r.callCount())
}

func TestJob_ExhaustsItsAttemptsThenOperatorRetryStartsAFreshCount(t *testing.T) {
	boom := errors.New("503")
	h, id := startJob(t, jobFlow, &fakeRunner{results: []error{boom, boom, boom}})
	ctx := context.Background()

	require.NoError(t, jobSteps{h.eng}.HandleAttempt(ctx, h.q.last(t)))
	j := h.job(t, id)
	require.Equal(t, flowrun.JobWaiting, j.Phase)
	require.Equal(t, 1, j.Failed)
	require.Equal(t, h.clock.Add(10*time.Second), j.NextAt)

	// Not yet due: nothing is queued.
	h.advance(t, 9*time.Second)
	require.Equal(t, 1, h.q.count())
	h.advance(t, time.Second)
	require.Equal(t, 2, h.q.count())
	require.NoError(t, jobSteps{h.eng}.HandleAttempt(ctx, h.q.last(t)))
	require.Equal(t, h.clock.Add(20*time.Second), h.job(t, id).NextAt, "the backoff doubles")

	h.advance(t, 20*time.Second)
	require.NoError(t, jobSteps{h.eng}.HandleAttempt(ctx, h.q.last(t)))
	j = h.job(t, id)
	require.Equal(t, flowrun.JobFailed, j.Phase)
	require.Equal(t, flowrun.JobExhausted, j.FailKind)
	require.Equal(t, 3, j.Failed)
	require.Equal(t, 3, h.r.callCount())

	run, err := h.eng.Run(ctx, id)
	require.NoError(t, err)
	xs, err := h.eng.Explain(ctx, run)
	require.NoError(t, err)
	require.Equal(t, flow.StatusFailed, xs[0].Status)
	require.Equal(t, &JobStep{Phase: flowrun.JobFailed, Failed: 3, Attempts: 3, FailKind: flowrun.JobExhausted},
		xs[0].Pending.Job)

	// A failed job waits for an operator.
	h.advance(t, time.Hour)
	require.Equal(t, 3, h.q.count())

	require.NoError(t, h.eng.Retry(ctx, id))
	j = h.job(t, id)
	require.Equal(t, flowrun.JobQueued, j.Phase)
	require.Zero(t, j.Failed, "a retry starts a fresh count")
	require.NoError(t, jobSteps{h.eng}.HandleAttempt(ctx, h.q.last(t)))
	require.Equal(t, 1, h.r.calls[3].Attempt)
	h.ended(t, id)
}

func TestJob_PermanentErrorFailsAtOnce(t *testing.T) {
	h, id := startJob(t, jobFlow, &fakeRunner{results: []error{Permanent(errors.New("404"))}})
	require.NoError(t, jobSteps{h.eng}.HandleAttempt(context.Background(), h.q.last(t)))
	j := h.job(t, id)
	require.Equal(t, flowrun.JobFailed, j.Phase)
	require.Equal(t, flowrun.JobFailedPermanent, j.FailKind)
	require.Equal(t, 1, h.r.callCount())
}

func TestJob_RetryFalseFailsOnTheFirstError(t *testing.T) {
	src := strings.Replace(jobFlow, "retry: { attempts: 3, backoff: { initial: 10s, factor: 2, max: 1m }, timeout: 30s }",
		"retry: false", 1)
	h, id := startJob(t, src, &fakeRunner{results: []error{errors.New("503")}})
	require.NoError(t, jobSteps{h.eng}.HandleAttempt(context.Background(), h.q.last(t)))
	j := h.job(t, id)
	require.Equal(t, flowrun.JobFailed, j.Phase)
	require.Equal(t, flowrun.JobExhausted, j.FailKind)
}

// shortTimeout gives an attempt one second.
func shortTimeout(t *testing.T, idempotent bool) string {
	t.Helper()
	src := strings.Replace(jobFlow, "timeout: 30s", "timeout: 1s", 1)
	if idempotent {
		src = strings.Replace(src, "url: https://hooks.example.com/ship",
			"url: https://hooks.example.com/ship\n          idempotent: true", 1)
	}
	return src
}

func TestJob_TimedOutAttempt(t *testing.T) {
	t.Run("fails when the job is not idempotent", func(t *testing.T) {
		h, id := startJob(t, shortTimeout(t, false), &fakeRunner{block: true})
		require.NoError(t, jobSteps{h.eng}.HandleAttempt(context.Background(), h.q.last(t)))
		j := h.job(t, id)
		require.Equal(t, flowrun.JobFailed, j.Phase)
		require.Equal(t, flowrun.JobFailedTimeout, j.FailKind)
	})
	t.Run("tries again when the job is idempotent", func(t *testing.T) {
		h, id := startJob(t, shortTimeout(t, true), &fakeRunner{block: true})
		require.NoError(t, jobSteps{h.eng}.HandleAttempt(context.Background(), h.q.last(t)))
		j := h.job(t, id)
		require.Equal(t, flowrun.JobWaiting, j.Phase)
		require.Equal(t, 1, j.Failed)
	})
}

func TestJob_LostWorker(t *testing.T) {
	for _, idempotent := range []bool{false, true} {
		t.Run(map[bool]string{false: "fails", true: "idempotent tries again"}[idempotent], func(t *testing.T) {
			src := strings.Replace(jobFlow, "timeout: 30s", "timeout: 1m", 1)
			if idempotent {
				src = shortTimeout(t, true)
				src = strings.Replace(src, "timeout: 1s", "timeout: 1m", 1)
			}
			h, id := startJob(t, src, &fakeRunner{block: true})
			// The process shuts down while the attempt runs: no outcome.
			ctx, cancel := context.WithCancel(context.Background())
			h.r.during = cancel
			require.ErrorIs(t, jobSteps{h.eng}.HandleAttempt(ctx, h.q.last(t)), context.Canceled)
			require.Equal(t, flowrun.JobStarted, h.job(t, id).Phase)

			// Within the grace the engine waits for the worker.
			h.advance(t, time.Minute+jobLostGrace-time.Second)
			require.Equal(t, flowrun.JobStarted, h.job(t, id).Phase)

			h.advance(t, time.Second)
			j := h.job(t, id)
			if !idempotent {
				require.Equal(t, flowrun.JobFailed, j.Phase)
				require.Equal(t, flowrun.JobLost, j.FailKind)
				return
			}
			require.Equal(t, flowrun.JobWaiting, j.Phase)
			require.Equal(t, 1, j.Failed)
		})
	}
}

func TestJob_AttemptThatNeverStartedIsQueuedAgainWithoutUsingTheBudget(t *testing.T) {
	h, id := startJob(t, jobFlow, nil)
	stale := h.q.last(t)
	h.advance(t, 30*time.Second)
	j := h.job(t, id)
	require.Equal(t, flowrun.JobWaiting, j.Phase)
	require.Zero(t, j.Failed)
	require.Equal(t, 1, j.Unsent)

	h.advance(t, time.Minute)
	require.Equal(t, 2, h.q.count())
	j = h.job(t, id)
	require.Equal(t, flowrun.JobQueued, j.Phase)
	require.Equal(t, int64(2), j.Seq)

	// The first attempt, delivered late, does nothing.
	require.NoError(t, jobSteps{h.eng}.HandleAttempt(context.Background(), stale))
	require.Zero(t, h.r.callCount())
	require.NoError(t, jobSteps{h.eng}.HandleAttempt(context.Background(), h.q.last(t)))
	h.ended(t, id)
}

func TestJob_EnqueueErrorWaitsAsAnAttemptThatNeverStarted(t *testing.T) {
	q := &fakeQueue{err: errors.New("queue down")}
	h := newHarness(t, jobFlow, func(d *Deps) { d.Jobs, d.JobRunner = q, &fakeRunner{} })
	h.f.addPerson("PER-1", map[string]any{"name": "Ada"})
	id, err := h.eng.Start(context.Background(), "ship", entity.Ref{ID: "PER-1"})
	require.NoError(t, err)
	jh := jobHarness{harness: h, q: q}
	j := jh.job(t, id)
	require.Equal(t, flowrun.JobWaiting, j.Phase)
	require.Equal(t, 1, j.Unsent)
	require.Zero(t, j.Failed)
	require.True(t, j.NextAt.After(*h.clock), "it waits before it tries again")
}

func TestJob_WithoutAQueueTheStepWaits(t *testing.T) {
	h := newHarness(t, jobFlow)
	h.f.addPerson("PER-1", map[string]any{"name": "Ada"})
	id, err := h.eng.Start(context.Background(), "ship", entity.Ref{ID: "PER-1"})
	require.NoError(t, err)
	j := jobHarness{harness: h}.job(t, id)
	require.Equal(t, flowrun.JobWaiting, j.Phase)
	require.Zero(t, j.Seq)
}

func TestJob_IdempotencyKeys(t *testing.T) {
	boom := errors.New("503")
	h, id := startJob(t, jobFlow, &fakeRunner{results: []error{boom}})
	ctx := context.Background()
	require.NoError(t, jobSteps{h.eng}.HandleAttempt(ctx, h.q.last(t)))
	h.advance(t, 10*time.Second)
	require.NoError(t, jobSteps{h.eng}.HandleAttempt(ctx, h.q.last(t)))

	gen := h.q.refs[0].Generation
	require.Equal(t, []string{AttemptKey(id, "send", gen, 1), AttemptKey(id, "send", gen, 2)}, h.q.keys)
	require.NotEqual(t, h.q.keys[0], h.q.keys[1], "each attempt has its own key")
	c1, c2 := h.r.calls[0], h.r.calls[1]
	require.Equal(t, h.q.keys[0], c1.Key)
	require.Equal(t, h.q.keys[1], c2.Key)
	require.Equal(t, StepKey(id, "send", gen), c1.StepKey)
	require.Equal(t, c1.StepKey, c2.StepKey, "the step key is the same for every attempt")
	require.Equal(t, 2, c2.Attempt)
	require.Equal(t, string(id)+"/send."+strconv.FormatInt(gen, 10)+"/2", c2.Key)
}

func TestJob_ForkOtherBranchesGoOnWhileTheJobFails(t *testing.T) {
	h, id := startJob(t, jobForkFlow, &fakeRunner{results: []error{Permanent(errors.New("410"))}})
	ctx := context.Background()
	require.NoError(t, jobSteps{h.eng}.HandleAttempt(ctx, h.q.last(t)))
	run, err := h.eng.Run(ctx, id)
	require.NoError(t, err)
	send := run.Branches[branchAt(run, "send")]
	require.Equal(t, flowrun.JobFailed, send.Pending.Job.Phase)

	// The review branch's timer still fires; the join waits for the job.
	h.advance(t, time.Hour)
	run, err = h.eng.Run(ctx, id)
	require.NoError(t, err)
	require.True(t, run.Branches[branchAt(run, "review")].Arrived)
	require.Equal(t, flowrun.JobFailed, run.Branches[branchAt(run, "send")].Pending.Job.Phase)

	require.NoError(t, h.eng.Retry(ctx, id))
	require.NoError(t, jobSteps{h.eng}.HandleAttempt(ctx, h.q.last(t)))
	h.ended(t, id)
	require.Equal(t, "finished", h.f.header(id)["state"])
}

func TestJob_OtherBranchWritesWhileTheAttemptRuns(t *testing.T) {
	h, id := startJob(t, jobForkFlow, nil)
	ctx := context.Background()
	run, err := h.eng.Run(ctx, id)
	require.NoError(t, err)
	gen := genOf(t, run, "review")
	h.r.during = func() {
		_, cerr := h.eng.Change(ctx, id, gen, "alice", holdsRoles("hr"))
		require.NoError(t, cerr)
	}
	require.NoError(t, jobSteps{h.eng}.HandleAttempt(ctx, h.q.last(t)))
	h.ended(t, id)
}

func TestJob_RetryWhileTheAttemptRunsIsRefused(t *testing.T) {
	h, id := startJob(t, jobFlow, nil)
	require.ErrorIs(t, h.eng.Retry(context.Background(), id), ErrJobRunning)
}

// runnerFunc is a JobRunner from a function.
type runnerFunc func(ctx context.Context, call JobCall) error

func (f runnerFunc) RunJob(ctx context.Context, call JobCall) error { return f(ctx, call) }

func TestJob_FinishedOutcomeIsDueWhenItsSettleDidNotRun(t *testing.T) {
	h, id := startJob(t, jobFlow, nil)
	ctx := context.Background()
	ref := h.q.last(t)
	// The process stops between storing the outcome and settling it.
	call, _, _, ok, err := jobSteps{h.eng}.startAttempt(ctx, ref)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 1, call.Attempt)
	_, ok, err = jobSteps{h.eng}.finishAttempt(ctx, ref, flowrun.JobOK)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, flowrun.JobFinished, h.job(t, id).Phase)

	h.advance(t, 0)
	h.ended(t, id)
}

func TestJob_TimeoutCountsFromTheStart(t *testing.T) {
	h, id := startJob(t, jobFlow, nil)
	var left time.Duration
	h.r.during = func() { left = h.job(t, id).Deadline.Sub(*h.clock) }
	// A worker picks the attempt up just before the queue deadline.
	*h.clock = h.clock.Add(29 * time.Second)
	require.NoError(t, jobSteps{h.eng}.HandleAttempt(context.Background(), h.q.last(t)))
	require.Equal(t, 30*time.Second, left, "the attempt gets its whole timeout")
	h.ended(t, id)
}

func TestJob_PermanentErrorAtTheDeadlineIsPermanent(t *testing.T) {
	h, id := startJob(t, shortTimeout(t, true), nil)
	h.eng.d.JobRunner = runnerFunc(func(ctx context.Context, _ JobCall) error {
		<-ctx.Done()
		return Permanent(errors.New("410"))
	})
	require.NoError(t, jobSteps{h.eng}.HandleAttempt(context.Background(), h.q.last(t)))
	j := h.job(t, id)
	require.Equal(t, flowrun.JobFailed, j.Phase)
	require.Equal(t, flowrun.JobFailedPermanent, j.FailKind, "an idempotent job does not retry a permanent error")
}

func TestJob_ParkedBranchDoesNotHoldAJoinThatDoesNotNeedIt(t *testing.T) {
	src := strings.Replace(jobForkFlow, "join: all", "join: any", 1)
	h, id := startJob(t, src, &fakeRunner{results: []error{Permanent(errors.New("410"))}})
	ctx := context.Background()

	// The review branch's timer fires while the job waits for a worker:
	// one round queues the attempt again, the next fires the timer.
	h.advance(t, time.Hour)
	h.advance(t, 0)
	h.ended(t, id)
	require.Equal(t, "finished", h.f.header(id)["state"])

	// The attempts of the branch the join dropped do nothing.
	for _, ref := range h.q.refs {
		require.NoError(t, jobSteps{h.eng}.HandleAttempt(ctx, ref))
	}
	require.Zero(t, h.r.callCount())
}

func TestNewAttemptHandler_RejectsNil(t *testing.T) {
	_, err := NewAttemptHandler(nil)
	require.Error(t, err)
}
