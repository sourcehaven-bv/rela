package flowengine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
)

// Job steps (TKT-PY2G11W).
//
// A state with a `job:` runs its target once its outputs are written. The
// engine owns the retries; the queue runs each attempt once
// (jobs.RetryNever), so the retry policy lives in flows.yaml and not in
// the queue. The attempt is stored on the token's pending mark
// ([flowrun.JobAttempt]) and moves through these phases:
//
//   - waiting: the next attempt is due at NextAt. Once due, settling the
//     token queues it: Seq goes up, the deadline is set, the run is saved
//     and then the attempt is enqueued.
//   - queued: [AttemptHandler.HandleAttempt] moves it to started. Still queued at
//     its deadline, it never started, and it waits again without using
//     the budget.
//   - started: the worker runs the target, bounded by the deadline, and
//     stores the outcome. Still started at the deadline plus jobLostGrace,
//     the worker is lost.
//   - finished: settling acts on the outcome. Success follows `done`; a
//     retryable error waits for the next attempt or fails as exhausted; a
//     permanent error fails; an unknown outcome (timeout, lost) fails
//     unless the job is idempotent.
//   - failed: the token is parked until an operator retries the run.
//
// A token whose attempt is in flight or failed is parked: settling skips
// it, the join waits for it, and the other branches go on.

// AttemptRef names one attempt of a job step: the payload of its queue
// job. A handler never trusts it; it acts only when the stored attempt
// still matches.
type AttemptRef struct {
	Run flow.RunID `json:"run"`
	// Branch is the token: "" for the run itself.
	Branch     string `json:"branch,omitempty"`
	Generation int64  `json:"generation"`
	Seq        int64  `json:"seq"`
}

// JobQueue puts an attempt on the job queue. The queue must drop the job
// when it cannot start it by deadline.
type JobQueue interface {
	EnqueueAttempt(ctx context.Context, ref AttemptRef, key string, deadline time.Time) error
}

// JobCall is one attempt of a job step, as the runner receives it.
type JobCall struct {
	Flow  string
	Run   flow.RunID
	State string
	// Subject is the run's subject.
	Subject entity.Ref
	// RunAs is the flow's run_as: the identity the target runs as.
	RunAs string
	Job   flowcfg.Job
	// Attempt counts this execution among the attempts that ran, from 1.
	Attempt int
	// Key is the attempt's idempotency key: run, step and attempt.
	Key string
	// StepKey is the same for every attempt of the step, so a target can
	// drop a repeat across attempts.
	StepKey string
}

// JobRunner runs the target of a job step as call.RunAs. It returns an
// error wrapping [ErrPermanent] when another attempt cannot help.
type JobRunner interface {
	RunJob(ctx context.Context, call JobCall) error
}

// Errors of job steps.
var (
	// ErrPermanent marks a job error that another attempt cannot fix.
	ErrPermanent = errors.New("flowengine: permanent job failure")
	// ErrJobRunning: a retry of a run whose only pending work is an
	// attempt on the queue or running.
	ErrJobRunning = errors.New("flowengine: the run's job is running")
	// errParked: the token waits for its job. settle skips it.
	errParked = errors.New("flowengine: the step waits for its job")
)

// Permanent wraps err as a permanent job failure.
//
// Nil: accepted, returns nil.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrPermanent, err)
}

// jobLostGrace is how long after its deadline a started attempt still
// may store its outcome before it counts as lost.
const jobLostGrace = 30 * time.Second

// maxAttemptTries bounds the compare-and-set retries of one attempt write.
const maxAttemptTries = 8

// attemptSettleTimeout bounds the settle after an attempt's outcome.
var attemptSettleTimeout = time.Minute

// AttemptKey is the idempotency key of attempt seq of the step of state
// at generation gen in run.
func AttemptKey(run flow.RunID, state string, gen, seq int64) string {
	return fmt.Sprintf("%s/%d", StepKey(run, state, gen), seq)
}

// StepKey names one step of a run: the run, its state and the generation
// that entered it.
func StepKey(run flow.RunID, state string, gen int64) string {
	return fmt.Sprintf("%s/%s.%d", run, state, gen)
}

// jobParked reports whether a token with pending mark p waits for its job
// at now: an attempt in flight before its check time, a wait not yet due,
// or a failure.
func jobParked(p *flowrun.Pending, now time.Time) bool {
	if p == nil || p.Job == nil {
		return false
	}
	due, ok := jobDue(p.Job)
	return !ok || now.Before(due)
}

// jobDue returns when an attempt needs the engine again; ok is false for
// a failed job, which needs an operator.
func jobDue(j *flowrun.JobAttempt) (time.Time, bool) {
	switch j.Phase {
	case flowrun.JobWaiting:
		return j.NextAt, true
	case flowrun.JobQueued:
		return j.Deadline, true
	case flowrun.JobStarted:
		return j.Deadline.Add(jobLostGrace), true
	case flowrun.JobFailed:
		return time.Time{}, false
	case flowrun.JobFinished:
		// Due at once, so a due round settles an outcome whose settle
		// did not run.
		return j.StartedAt, true
	default:
		return time.Time{}, true
	}
}

// settleable reports whether a token of run has a pending mark to settle
// at now: one that does not wait for its job.
func settleable(run flowrun.Run, now time.Time) bool {
	if !forked(run) {
		return run.Pending != nil && !jobParked(run.Pending, now)
	}
	for _, b := range run.Branches {
		if b.Pending != nil && !jobParked(b.Pending, now) {
			return true
		}
	}
	return false
}

// jobSteps runs the job steps of an [Engine]: it queues their attempts,
// runs the ones the queue delivers and acts on their outcomes.
type jobSteps struct{ *Engine }

// AttemptHandler runs the attempts the job queue delivers.
type AttemptHandler struct{ s jobSteps }

// NewAttemptHandler returns the handler of e's attempts.
//
// Nil: rejected, an attempt needs the engine that queued it.
func NewAttemptHandler(e *Engine) (*AttemptHandler, error) {
	if e == nil {
		return nil, errors.New("flowengine: no engine for the attempt handler")
	}
	return &AttemptHandler{s: jobSteps{e}}, nil
}

// HandleAttempt runs the attempt ref names; see jobSteps.HandleAttempt.
func (h *AttemptHandler) HandleAttempt(ctx context.Context, ref AttemptRef) error {
	return h.s.HandleAttempt(ctx, ref)
}

// jobStep drives the job of token tok, whose outputs are written. It
// returns the run with errParked while the job is not done, and the run
// as it is when the job succeeded, so the caller follows `done`.
func (e jobSteps) jobStep(ctx context.Context, f *flowcfg.Flow, run flowrun.Run, tok string, cur flow.Current,
	job flowcfg.Job) (flowrun.Run, error) {
	p := pendingOf(run, tok)
	now := e.now()
	j := flowrun.JobAttempt{Phase: flowrun.JobWaiting, NextAt: now}
	if p.Job != nil {
		j = *p.Job
	}
	switch j.Phase {
	case flowrun.JobQueued:
		if now.Before(j.Deadline) {
			return e.park(ctx, run, tok, j)
		}
		// It never started: the queue lost it or could not reach it in
		// time. Again, without using the budget.
		slog.WarnContext(ctx, "flowengine: a job attempt never started", "run", run.ID, "state", cur.State,
			"seq", j.Seq)
		j = unsent(j, now)
	case flowrun.JobStarted:
		if now.Before(j.Deadline.Add(jobLostGrace)) {
			return e.park(ctx, run, tok, j)
		}
		slog.WarnContext(ctx, "flowengine: the worker of a job attempt stopped without an outcome",
			"run", run.ID, "state", cur.State, "seq", j.Seq)
		j = e.decide(j, flowrun.JobTimeout, flowrun.JobLost, job, now)
	case flowrun.JobFinished:
		if j.Outcome == flowrun.JobOK {
			return run, nil
		}
		j = e.decide(j, j.Outcome, "", job, now)
	case flowrun.JobWaiting, flowrun.JobFailed:
		// Waiting: queued below when due. Failed: waits for an operator.
	}
	if j.Phase == flowrun.JobWaiting && !now.Before(j.NextAt) {
		return e.enqueue(ctx, f, run, tok, cur, job, j)
	}
	if j.Phase == flowrun.JobFailed {
		slog.WarnContext(ctx, "flowengine: a job step failed", "run", run.ID, "flow", f.Name(),
			"state", cur.State, "kind", j.FailKind)
	}
	return e.park(ctx, run, tok, j)
}

// unsent returns j, which never started, waiting again after a backoff by
// the attempts in a row that never started.
func unsent(j flowrun.JobAttempt, now time.Time) flowrun.JobAttempt {
	delay := backoffDelay(j.Unsent)
	j.Unsent = min(j.Unsent+1, flowrun.MaxPendingFailures)
	return flowrun.JobAttempt{Seq: j.Seq, Failed: j.Failed, Unsent: j.Unsent, Phase: flowrun.JobWaiting,
		NextAt: now.Add(delay)}
}

// decide acts on an attempt that ended with outcome: wait for the next
// attempt, or fail. unknown is the fail kind of an unknown outcome:
// timeout, or lost for a worker that stopped.
func (e jobSteps) decide(j flowrun.JobAttempt, outcome flowrun.JobOutcome, unknown flowrun.JobFailKind,
	job flowcfg.Job, now time.Time) flowrun.JobAttempt {
	next := flowrun.JobAttempt{Seq: j.Seq, Failed: min(j.Failed+1, flowrun.MaxPendingFailures)}
	if unknown == "" {
		unknown = flowrun.JobFailedTimeout
	}
	switch {
	case outcome == flowrun.JobPermanent:
		next.Phase, next.FailKind = flowrun.JobFailed, flowrun.JobFailedPermanent
	case outcome == flowrun.JobTimeout && !job.Idempotent:
		next.Phase, next.FailKind = flowrun.JobFailed, unknown
	case next.Failed >= job.Retry.Attempts:
		next.Phase, next.FailKind = flowrun.JobFailed, flowrun.JobExhausted
	default:
		next.Phase, next.NextAt = flowrun.JobWaiting, now.Add(job.Retry.Delay(next.Failed))
	}
	return next
}

// enqueue queues the next attempt of token tok: it stores the attempt as
// queued and then puts it on the queue. Without a queue (a one-shot
// process) the attempt waits for a process that has one. When the enqueue
// fails, the attempt waits again at once, as one that never started.
func (e jobSteps) enqueue(ctx context.Context, f *flowcfg.Flow, run flowrun.Run, tok string, cur flow.Current,
	job flowcfg.Job, j flowrun.JobAttempt) (flowrun.Run, error) {
	if e.d.Jobs == nil {
		return e.park(ctx, run, tok, j)
	}
	now := e.now()
	if j.Seq >= flowrun.MaxJobSeq {
		j = flowrun.JobAttempt{Seq: j.Seq, Failed: j.Failed, Phase: flowrun.JobFailed,
			FailKind: flowrun.JobExhausted}
		return e.park(ctx, run, tok, j)
	}
	q := flowrun.JobAttempt{Seq: j.Seq + 1, Failed: j.Failed, Unsent: j.Unsent, Phase: flowrun.JobQueued,
		QueuedAt: now, Deadline: now.Add(job.Retry.Timeout)}
	saved, err := e.storeJob(ctx, run, tok, q)
	if err != nil || saved.ID == "" {
		return saved, err
	}
	ref := AttemptRef{Run: run.ID, Branch: tok, Generation: cur.Generation, Seq: q.Seq}
	key := AttemptKey(run.ID, cur.State, cur.Generation, q.Seq)
	if qerr := e.d.Jobs.EnqueueAttempt(ctx, ref, key, q.Deadline); qerr != nil {
		slog.WarnContext(ctx, "flowengine: a job attempt was not queued", "run", run.ID, "flow", f.Name(),
			"state", cur.State, "error", qerr)
		back, berr := e.storeJob(ctx, saved, tok, unsent(q, now))
		if berr != nil || back.ID == "" {
			return back, berr
		}
		return back, errParked
	}
	return saved, errParked
}

// park stores j on token tok when it changed, with the token due when the
// job needs the engine again, and returns errParked.
func (e jobSteps) park(ctx context.Context, run flowrun.Run, tok string, j flowrun.JobAttempt) (flowrun.Run, error) {
	p := pendingOf(run, tok)
	due, _ := jobDue(&j)
	if p.Job != nil && *p.Job == j && instOf(run, tok).NextDueAt.Equal(due) {
		return run, errParked
	}
	saved, err := e.storeJob(ctx, run, tok, j)
	if err != nil || saved.ID == "" {
		return saved, err
	}
	return saved, errParked
}

// storeJob stores j on the pending mark of token tok, with the token due
// when the job needs the engine again. A lost compare-and-set returns a
// zero run.
func (e jobSteps) storeJob(ctx context.Context, run flowrun.Run, tok string, j flowrun.JobAttempt) (flowrun.Run,
	error) {
	p := *pendingOf(run, tok)
	p.Job = &j
	inst := instOf(run, tok)
	inst.NextDueAt, _ = jobDue(&j)
	next, err := e.save(ctx, run, tok, inst, &p, nil, e.now())
	if isLost(err) {
		return flowrun.Run{}, nil
	}
	return next, err
}

// HandleAttempt runs the attempt ref names, when the stored attempt still
// matches it and is queued before its deadline: it moves it to started,
// runs the target bounded by the deadline, stores the outcome and settles
// the run. Any other case (a redelivery, a cancelled run, an attempt
// already given up) does nothing and returns nil.
//
// A cancel of ctx that is not the attempt's deadline (a shutdown) stores
// no outcome: the attempt stays started, and the lost path decides.
func (e jobSteps) HandleAttempt(ctx context.Context, ref AttemptRef) error {
	if e.d.JobRunner == nil {
		return errors.New("flowengine: no job runner")
	}
	call, deadline, f, ok, err := e.startAttempt(ctx, ref)
	if err != nil || !ok {
		return err
	}
	ctx = audit.WithFlowStep(ctx, audit.FlowStep{Flow: call.Flow, Run: string(call.Run), Step: call.State})
	// The deadline is on the engine's clock, which a test may pin; the
	// context runs on the wall clock, so it gets the time left. A deadline
	// already past expires the context at once, as WithDeadline did.
	actx, cancel := context.WithTimeout(ctx, deadline.Sub(e.now()))
	rerr := e.d.JobRunner.RunJob(actx, call)
	timedOut := errors.Is(actx.Err(), context.DeadlineExceeded)
	cancel()
	var outcome flowrun.JobOutcome
	switch {
	case rerr == nil:
		outcome = flowrun.JobOK
	case errors.Is(rerr, ErrPermanent):
		outcome = flowrun.JobPermanent
	case timedOut:
		outcome = flowrun.JobTimeout
	case ctx.Err() != nil:
		return ctx.Err()
	default:
		outcome = flowrun.JobRetryable
	}
	if rerr != nil {
		slog.WarnContext(ctx, "flowengine: a job attempt failed", "run", call.Run, "flow", call.Flow,
			"state", call.State, "attempt", call.Attempt, "outcome", outcome, "error", rerr)
	}
	sctx, scancel := context.WithTimeout(context.WithoutCancel(ctx), attemptSettleTimeout)
	defer scancel()
	run, ok, err := e.finishAttempt(sctx, ref, outcome)
	if err != nil || !ok {
		return err
	}
	if left, err := e.settle(sctx, f, run, ref.Branch); err != nil {
		e.backoff(sctx, left)
		return err
	}
	return nil
}

// matchAttempt returns the token's pending mark and job when run still
// carries the attempt ref names in phase.
func matchAttempt(run flowrun.Run, ref AttemptRef, phase flowrun.JobPhase) (flowrun.Pending, bool) {
	inst, p, _, ok := slot(run, ref.Branch)
	if !ok || p == nil || p.Job == nil || p.Generation != ref.Generation || inst.Generation != ref.Generation ||
		p.Job.Seq != ref.Seq || p.Job.Phase != phase {

		return flowrun.Pending{}, false
	}
	return *p, true
}

// startAttempt moves the attempt ref names from queued to started and
// returns its call. ok is false when there is nothing to run.
func (e jobSteps) startAttempt(ctx context.Context, ref AttemptRef) (call JobCall, deadline time.Time,
	f *flowcfg.Flow, ok bool, err error) {
	for range maxAttemptTries {
		run, gerr := e.d.Runs.Get(ctx, ref.Run)
		if errors.Is(gerr, flowrun.ErrNotFound) {
			return call, deadline, nil, false, nil
		}
		if gerr != nil {
			return call, deadline, nil, false, gerr
		}
		p, match := matchAttempt(run, ref, flowrun.JobQueued)
		now := e.now()
		if !match || !now.Before(p.Job.Deadline) {
			return call, deadline, nil, false, nil
		}
		if f, err = e.flowOf(ctx, run); err != nil {
			return call, deadline, nil, false, err
		}
		st, _ := f.State(p.State)
		job, hasJob := st.Job()
		if !hasJob {
			return call, deadline, nil, false, fmt.Errorf("%w: state %q runs no job", ErrStranded, p.State)
		}
		// The timeout counts from the start: the queue deadline only
		// bounds the wait for a worker.
		j := *p.Job
		j.Phase, j.StartedAt, j.Deadline = flowrun.JobStarted, now, now.Add(job.Retry.Timeout)
		saved, serr := e.storeJob(ctx, run, ref.Branch, j)
		if serr != nil {
			return call, deadline, nil, false, serr
		}
		if saved.ID == "" {
			continue // another write won: read again
		}
		call = JobCall{
			Flow: f.Name(), Run: run.ID, State: p.State, Subject: run.Subject, RunAs: f.RunAs(), Job: job,
			Attempt: j.Failed + 1, Key: AttemptKey(run.ID, p.State, p.Generation, j.Seq),
			StepKey: StepKey(run.ID, p.State, p.Generation),
		}
		return call, j.Deadline, f, true, nil
	}
	return call, deadline, nil, false, fmt.Errorf("flowengine: run %s kept changing; attempt %d not started",
		ref.Run, ref.Seq)
}

// finishAttempt stores the outcome of the started attempt ref names, due
// at once. ok is false when the run no longer carries it.
func (e jobSteps) finishAttempt(ctx context.Context, ref AttemptRef, outcome flowrun.JobOutcome) (flowrun.Run,
	bool, error) {
	for range maxAttemptTries {
		run, err := e.d.Runs.Get(ctx, ref.Run)
		if errors.Is(err, flowrun.ErrNotFound) {
			return flowrun.Run{}, false, nil
		}
		if err != nil {
			return flowrun.Run{}, false, err
		}
		p, match := matchAttempt(run, ref, flowrun.JobStarted)
		if !match {
			return flowrun.Run{}, false, nil
		}
		j := *p.Job
		j.Phase, j.Outcome = flowrun.JobFinished, outcome
		saved, err := e.storeJob(ctx, run, ref.Branch, j)
		if err != nil {
			return flowrun.Run{}, false, err
		}
		if saved.ID != "" {
			return saved, true, nil
		}
	}
	return flowrun.Run{}, false, fmt.Errorf("flowengine: run %s kept changing; the outcome of attempt %d is lost",
		ref.Run, ref.Seq)
}

// retryJobs returns run's state with every failed job of the run waiting
// for a fresh attempt count at now, and every waiting job due at now.
// changed reports a change; running reports a job on the queue or running.
func retryJobs(run flowrun.Run, now time.Time) (st flowrun.State, changed, running bool) {
	st = cloneState(run)
	one := func(inst *flow.Instance, p **flowrun.Pending) {
		if *p == nil || (*p).Job == nil {
			return
		}
		cp := **p
		j := *cp.Job
		switch j.Phase {
		case flowrun.JobFailed:
			j = flowrun.JobAttempt{Seq: j.Seq, Phase: flowrun.JobWaiting, NextAt: now}
		case flowrun.JobWaiting:
			j.NextAt = now
		case flowrun.JobQueued, flowrun.JobStarted:
			running = true
			return
		default:
			return
		}
		cp.Job, cp.Failures = &j, 0
		*p = &cp
		inst.NextDueAt = now
		changed = true
	}
	one(&st.Instance, &st.Pending)
	for i := range st.Branches {
		one(&st.Branches[i].Instance, &st.Branches[i].Pending)
	}
	return st, changed, running
}
