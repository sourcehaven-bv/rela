package scheduler

import (
	"context"
	"errors"
	"fmt"

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
// "run this, retry it like so"; the scheduler owns "is this due, and did it
// succeed". Moving the bookkeeping too would have meant either duplicating the
// ladder in a handler or letting the queue learn what a schedule is — and the
// seam exists precisely so it never has to.
//
// # Why not neoq's own scheduler
//
// neoq offers StartCron, and it is deliberately unused. It is an in-process
// robfig/cron ticker that enqueues an empty job (memory_backend.go:160):
//
//   - No missed-run detection. It fires only while the process is alive, so a
//     daily task would never run on a desktop app that is closed at the
//     moment it was due. Running a task whose window passed while the app was
//     shut is the main reason this scheduler persists last-run times.
//   - No persistence, on any backend. cron is in-process, so several server
//     instances would each fire every task.
//   - Cron syntax, not schedules.yaml's "day" / "friday" / "30m".
//   - The enqueue error is discarded, so a task that fails to queue vanishes.
//
// Adopting it would also drop the retry ladder, the clock-jump guard
// (BUG-ZKK2UL) and run_as attribution. What neoq is good at — worker pools,
// retry mechanics, deduplication, durability — is what this file uses it for.

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
// script has run would let the next tick see stale state.
//
// Retry is [jobs.RetryNever] because the scheduler owns retrying: its ladder
// (5m→2h, suppressing the normal cadence) already encodes hard-won behavior
// including the BUG-ZKK2UL clock-jump guard. Two retry mechanisms stacked on
// one task would multiply, not compose.
//
// The task name is the idempotency key, which is how "do not stack two runs of
// the same task" is expressed. An earlier version used a deadline derived from
// the cadence; that was wrong in a way worth recording, because the reasoning
// looked sound. A deadline makes the job VANISH when it cannot be started in
// time — so under load, when the queue is exactly as busy as the operator most
// wants the work done, scheduled runs are silently dropped. The intent was
// never "expire this"; it was "if one is already pending, that is enough" —
// a daily report delayed six hours should not then send twice. That is
// deduplication by identity, and it degrades the right way: the run happens
// late instead of not at all.
func (s *Scheduler) enqueueTask(ctx context.Context, task TaskConfig) error {
	// A missing queue is a wiring bug, not a runtime condition: StartBackground
	// refuses to start without one. Reported rather than dereferenced so the
	// failure names its cause instead of surfacing as a nil panic in a
	// goroutine.
	if s.queue == nil {
		return errNoQueue
	}

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
		Retry: jobs.RetryNever,

		// Scoped to the task, so two tasks never suppress each other. On the
		// durable backend this holds ACROSS processes, which the in-process
		// claim above cannot: two rela-server instances sharing a database
		// will not both queue the same task.
		IdempotencyKey: task.Name,
	}

	if err := s.queue.Enqueue(ctx, job); err != nil {
		// Already pending. Not a failure — the work is scheduled — and not a
		// success either, since this run did not happen. Reported up so
		// doExecuteTask can leave the state alone.
		if errors.Is(err, jobs.ErrDuplicateJob) {
			return errTaskPending
		}
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

// errNoQueue reports that the scheduler has no job queue, so it cannot execute
// anything. Only reachable from a hand-built Scheduler that skipped UseQueue.
var errNoQueue = errors.New("scheduler: no job queue configured")

// errTaskPending reports that an identical job is already queued, so this run
// was collapsed into it rather than stacked behind it.
var errTaskPending = errors.New("scheduler: an identical task job is already pending")
