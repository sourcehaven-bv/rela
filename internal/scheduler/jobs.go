package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
)

// This file is the scheduler's half of the job seam.
//
// What moves onto the queue is SCRIPT EXECUTION — the slow part, which calls
// out to Lua and from there to HTTP, mail and LLM providers. What stays here is
// everything about *scheduling*: which tasks are due, the retry ladder, the
// clock-jump guard, and the last-successful-run bookkeeping in
// .rela/scheduler-state.json.
//
// That split is deliberate and is the whole point of the port. The queue owns
// "run this, retry it like so, give up by then"; the scheduler owns "is this
// due, and did it succeed". Moving the bookkeeping too would have meant either
// duplicating the ladder in a handler or letting the queue learn what a
// schedule is — and the seam exists precisely so it never has to.

// TaskKind is the job kind for a scheduled Lua script.
//
// Namespaced by owner so it cannot collide with another subsystem's kind: every
// subsystem registers into one process-wide queue.
var TaskKind = jobs.NewKind("scheduler", "run-task")

// Payload keys for a task job. The payload names WHAT to run; authority comes
// from the principal the handler stamps on its context, never from these.
const (
	payloadTaskName = "task"
	payloadScript   = "script"
	payloadRunAs    = "run_as"
)

// JobQueue is the slice of the job seam the scheduler needs.
//
// Declared here, at the call site, rather than taking jobs.Client: the
// scheduler both registers its handler and submits work, but it has no business
// starting or stopping the queue, and naming the two methods it uses documents
// that better than a wider interface would.
//
// Nil: rejected — [Scheduler.UseQueue] returns an error rather than silently
// falling back to inline execution, since a scheduler that quietly stopped
// using the queue would be indistinguishable from one that never had it.
type JobQueue interface {
	Enqueue(ctx context.Context, job jobs.Job) error
	Register(kind jobs.Kind, h jobs.Handler) error
}

// UseQueue routes script execution through q.
//
// Call it once at wiring time, before Run. It registers the task handler and
// switches [Scheduler.doExecuteTask] onto the queue; without it the scheduler
// executes inline exactly as before, which is what keeps every existing test
// meaningful.
//
// The handler is registered here rather than by the wiring site because the
// scheduler owns [TaskKind] — a subsystem that owns a kind should be the thing
// that binds it.
func (s *Scheduler) UseQueue(q JobQueue) error {
	if q == nil {
		return errors.New("scheduler: job queue must not be nil")
	}
	if err := q.Register(TaskKind, s.runTaskJob); err != nil {
		return fmt.Errorf("scheduler: register task handler: %w", err)
	}
	s.queue = q
	return nil
}

// runTaskJob is the handler that actually executes a scheduled script.
//
// It runs on a queue worker, so the principal and audit attribution have to be
// re-derived HERE from the payload rather than inherited from the enqueueing
// goroutine's context — a worker's ctx is not the submitter's. That is the same
// stamping the inline path does (see stampTaskAuditContext); doing it in one
// place would be nicer but the two contexts are genuinely different.
func (s *Scheduler) runTaskJob(ctx context.Context, job jobs.Job) error {
	name, _ := job.Payload[payloadTaskName].(string)
	script, _ := job.Payload[payloadScript].(string)
	runAs, _ := job.Payload[payloadRunAs].(string)

	if script == "" {
		// Unrunnable, and no retry would make a script path appear.
		return fmt.Errorf("scheduler: job %q carries no script", name)
	}

	taskCtx := stampTaskAuditContext(ctx, name, runAs)
	err := s.runEngine(taskCtx, TaskConfig{Name: name, Script: script, RunAs: runAs})

	// Hand the outcome back to the waiting submitter, which owns the state
	// bookkeeping. Reported before returning so the scheduler observes the
	// result whether or not the queue chooses to retry (it will not — the job
	// is submitted RetryNever).
	s.reportInFlight(name, err)
	return err
}

// enqueueTask submits one task execution and waits for it to finish.
//
// Waiting looks odd for a job queue, and it is deliberate. The scheduler's
// contract is that a task's outcome updates its state before the next tick
// evaluates it: recordSuccess stamps the last successful run, recordFailure
// advances the retry ladder, and runDueTasks reads both. Returning before the
// script has run would let the next tick see stale state and re-fire a task
// that is already running — the pile-up the single-threaded design has always
// prevented.
//
// So what the queue buys here is not fire-and-forget. It is that execution now
// carries a retry policy and a deadline the scheduler no longer hand-rolls per
// call, and that the same seam serves the fire-and-forget producers (mail,
// HTTP) that come later.
//
// Retry is [jobs.RetryNever] because the scheduler owns retrying: its ladder
// (5m→2h, suppressing the normal cadence) already encodes hard-won behavior
// including the BUG-ZKK2UL clock-jump guard. Two retry mechanisms stacked on
// one task would multiply, not compose.
//
// The deadline is the task's next scheduled run measured FROM NOW, which is the
// cadence rule expressed with the queue's generic primitive — no cadence
// concept crosses the boundary.
//
// From now, not from the last run. A task fires because it is due, so its next
// run measured from lastRun has by definition already arrived: a 1m task that
// ticks on time gets a deadline of exactly now, and one that ticks a few
// seconds late gets one in the past. Every such job would be refused or
// dropped. Measuring forward from the moment of submission gives the window the
// rule actually intends — "keep trying until my next slot comes round".
func (s *Scheduler) enqueueTask(ctx context.Context, task TaskConfig, from time.Time) error {
	// The completion channel is registered before the enqueue and found again
	// by the handler through s.inflight, keyed by task name. It cannot ride in
	// the payload: that must stay JSON-serializable for the durable backend,
	// and a channel is neither serializable nor meaningful in another process.
	//
	// The key is safe because a task is single-in-flight by construction — the
	// claim below is what enforces it.
	done, err := s.claimInFlight(task.Name)
	if err != nil {
		return err
	}
	defer s.releaseInFlight(task.Name)

	job := jobs.Job{
		Kind: TaskKind,
		Payload: map[string]any{
			payloadTaskName: task.Name,
			payloadScript:   task.Script,
			payloadRunAs:    task.RunAs,
		},
		Retry:    jobs.RetryNever,
		Deadline: task.Every.NextRun(from),
	}

	// A deadline already past means the queue will drop the job without
	// running it — and nothing would ever report completion, so the wait below
	// would block forever. Refuse to submit instead of hanging the scheduler.
	//
	// Reachable through a clock that jumped backwards, or a state file whose
	// last-run stamp is in the future. Both are the same class of input the
	// retry ladder's clock-jump guard exists for.
	if !job.Deadline.IsZero() && !s.now().Before(job.Deadline) {
		return fmt.Errorf("%w: task %q deadline %v already passed",
			errDeadlinePassed, task.Name, job.Deadline)
	}

	if err := s.queue.Enqueue(ctx, job); err != nil {
		return fmt.Errorf("scheduler: enqueue task %q: %w", task.Name, err)
	}

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// claimInFlight reserves a task name and returns the channel its completion
// will be reported on.
//
// It returns [errTaskInFlight] when the task's previous run has not finished.
// The single-threaded loop used to give non-overlap for free; with execution on
// a worker pool it has to be stated, or a slow task on a short cadence queues
// behind itself and piles up.
func (s *Scheduler) claimInFlight(name string) (<-chan error, error) {
	s.inflightMu.Lock()
	defer s.inflightMu.Unlock()

	if s.inflight == nil {
		s.inflight = make(map[string]chan error)
	}
	if _, busy := s.inflight[name]; busy {
		return nil, errTaskInFlight
	}
	// Buffered: the handler must never block reporting a result, even if the
	// submitter has already given up on ctx cancellation.
	ch := make(chan error, 1)
	s.inflight[name] = ch
	return ch, nil
}

// releaseInFlight drops a task's claim, letting its next slot run.
func (s *Scheduler) releaseInFlight(name string) {
	s.inflightMu.Lock()
	defer s.inflightMu.Unlock()
	delete(s.inflight, name)
}

// reportInFlight delivers a finished run's outcome to whoever is waiting.
//
// A missing entry is not an error: the submitter may have been cancelled, or
// the job may have been redelivered by a durable backend after a restart, in
// which case nothing in this process is waiting. Dropping the result is right
// in both cases — the alternative is blocking a worker forever.
func (s *Scheduler) reportInFlight(name string, err error) {
	s.inflightMu.Lock()
	ch, ok := s.inflight[name]
	s.inflightMu.Unlock()
	if !ok {
		return
	}
	select {
	case ch <- err:
	default:
	}
}

// errTaskInFlight reports that a task's previous run has not finished.
var errTaskInFlight = errors.New("scheduler: task already in flight")

// errDeadlinePassed reports that a task's computed deadline is already in the
// past, so the queue would drop the job rather than run it.
var errDeadlinePassed = errors.New("scheduler: task deadline already passed")
