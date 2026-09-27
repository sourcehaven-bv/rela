// Package schedulerstate holds scheduling bookkeeping for
// [github.com/Sourcehaven-BV/rela/internal/scheduler]: per task, when it last
// ran successfully, how many times it has failed since, and when it may be
// tried again; per run, where that run is in its lifecycle.
//
// # Runs are durable, not channels
//
// A scheduled run used to be tracked by an in-process channel the submitting
// goroutine blocked on. Any lost completion (a job executed by another node,
// redelivered after a restart, or stranded by the queue) stalled the whole
// scheduler for up to 20 minutes per attempt, and a stranded job held its
// idempotency key forever (BUG-TKL08E).
//
// Now each run is a record every process can read: queued, running, then
// succeeded, failed or abandoned. The node that executes a run records its
// outcome, together with the task's state, in one write. A run whose lease
// expires is abandoned by whichever process reaps it next. Nothing waits.
//
// At most one run per task is active (queued or running) at a time. That is
// the scheduler's non-overlap rule, and it holds across processes.
//
// # Why this is not in the graph
//
// A last-run timestamp is not a fact about an entity; it is a fact about a
// deployment's own operation. Storing it as an entity would put a write on
// every scheduler tick into the append-only audit log and (on postgres)
// through the version-capture sweep — a task on a one-minute cadence would
// generate more graph history than the work it performs. It is also
// DISPOSABLE: losing it costs a duplicated or delayed run, not data.
//
// So this is a separate service with its own backends, deliberately outside
// store.Store — the same argument internal/userstate makes for snoozes, and
// the same exemption from the "no repository abstractions" rule.
//
// # Why not one document in state.KV
//
// It used to be exactly that: a single scheduler-state.json holding every
// task's record, read once at startup and rewritten whole on every update
// through a KV whose Put is an unconditional upsert. Two schedulers against
// one database therefore clobbered each other WHOLESALE — not just the
// contended task, every task, because the unit of storage was the whole world.
//
// The fix is granularity, not locking: one record per task, so a write about
// one task cannot carry another task's state. Concurrent writers to DIFFERENT
// tasks then never interact at all, which is the common case.
//
// # Backends
//
// Implementations live in subpackages and are selected at wiring time the same
// way store backends are (see internal/appbuild). Every implementation must
// pass [schedulerstatetest.RunAll]: two backends without a shared contract test
// is two subtly different behaviors, and the one that diverges will be the one
// nobody runs locally.
//
// # Time is injected, never read
//
// No method reads the wall clock. Every write carries the run's START time and
// is guarded on it, which is what makes a stale writer harmless and what lets
// the conformance suite pin the ordering rules deterministically. This mirrors
// userstate's rule and exists for the same reason.
package schedulerstate

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrClosed is returned by every method once Close has been called.
var ErrClosed = errors.New("schedulerstate: store is closed")

// ErrNoTask is returned when a task name is empty. Defense in depth: the
// scheduler's own config parser already rejects one, so reaching this means a
// caller built a task by hand.
var ErrNoTask = errors.New("schedulerstate: task name must not be empty")

// ErrNoRun is returned when a run id is empty or names no stored run.
var ErrNoRun = errors.New("schedulerstate: no such run")

// ErrRunActive is returned by [Store.CreateRun] when the task already has a
// queued or running run. The caller skips the task: its previous run is still
// in progress, so the slot merges into it.
var ErrRunActive = errors.New("schedulerstate: task already has an active run")

// ErrStale is returned by [Store.CreateRun] when the task's state changed after
// the caller loaded it. Another process finished a run in between, so the
// caller's decision that the task is due may no longer hold.
var ErrStale = errors.New("schedulerstate: task state changed since it was loaded")

// ErrNotQueued is returned by [Store.StartRun] when the run is no longer
// queued: it already started (a duplicate delivery) or already finished.
var ErrNotQueued = errors.New("schedulerstate: run is not queued")

// TaskState is one task's scheduling record.
//
// LastRun, Failures and NextRetry are independent on purpose. Encoding "is this
// failing?" into LastRun alone is what caused BUG-ZKK2UL: a failure left no
// trace, so the task stayed perpetually due and retried at the tick rate.
type TaskState struct {
	// LastRun is the START time of the last SUCCESSFUL run, and the value the
	// schedule is evaluated against. Zero means the task has never succeeded.
	//
	// Start rather than completion, deliberately: a task that begins at 23:59
	// and finishes past midnight would otherwise land on the next day and
	// silently skip that day's execution. It also keeps interval schedules
	// from drifting forward by each run's duration.
	LastRun time.Time

	// Failures counts CONSECUTIVE failures. Zero means healthy. The caller
	// derives the retry delay from it through a [RetryPolicy]; this package
	// never interprets it.
	Failures int

	// NextRetry is when a failing task may next be attempted. Zero means no
	// retry is pending. While it is set the ordinary schedule is suppressed,
	// so a failing task fires on the ladder and never on its cadence.
	NextRetry time.Time

	// Version changes on every write to LastRun, Failures or NextRetry.
	// [Store.CreateRun] is conditional on it.
	Version int64

	// Active is the task's queued or running run. Nil when there is none.
	Active *Run
}

// Failing reports whether a retry is pending.
func (t TaskState) Failing() bool { return !t.NextRetry.IsZero() }

// RunStatus is where a run is in its lifecycle.
type RunStatus string

// The lifecycle is queued → running → one of the three terminal states. A
// queued run can also end directly, when its job never reached the queue or
// never started before its lease expired.
const (
	RunQueued    RunStatus = "queued"
	RunRunning   RunStatus = "running"
	RunSucceeded RunStatus = "succeeded"
	RunFailed    RunStatus = "failed"
	// RunAbandoned means the run's lease expired before it reported an
	// outcome: its worker died, its job was lost, or it ran far past the
	// queue's own handler timeout. It counts as a failure.
	RunAbandoned RunStatus = "abandoned"
)

// Active reports whether a run in this status still occupies its task.
func (s RunStatus) Active() bool { return s == RunQueued || s == RunRunning }

// Run is one execution of a task.
type Run struct {
	ID     string
	Task   string
	Status RunStatus

	// CreatedAt is when the scheduler decided the task was due. It is the
	// run's START for scheduling purposes: success stamps it as the task's
	// LastRun.
	CreatedAt time.Time
	// StartedAt is when a worker began executing. Zero while queued.
	StartedAt time.Time
	// FinishedAt is when the run reached a terminal status.
	FinishedAt time.Time

	// LeaseUntil is when an active run is presumed lost. Set at creation (the
	// pickup bound), on start, and extended by progress.
	LeaseUntil time.Time

	// Node identifies the process executing the run. Empty while queued.
	Node string

	// Error is why a failed or abandoned run failed.
	Error string

	// Occurrence names the calendar slot a for_each run serves (see
	// scheduler.Schedule.Occurrence). Empty for a plain run. A retry of a
	// failed fan-out serves the same occurrence, which is how it finds the
	// subjects an earlier attempt already delivered.
	Occurrence string

	// Children is how many for_each subjects the run fans out to; zero for a
	// plain run. The run finishes when all of them have settled.
	Children        int
	ChildrenSettled int
	ChildrenFailed  int
}

// Outcome is how an execution ended.
type Outcome struct {
	// Error is empty on success.
	Error string
	// At is when the outcome was observed. A failure's retry is measured
	// from here.
	At time.Time
}

// RetryPolicy returns the delay before the next attempt after the given
// number of consecutive failures (1 or more).
//
// Supplied by the caller: the backoff curve is scheduling policy, and the
// store only applies it inside the same write that increments the count, so
// the increment and the retry time cannot disagree.
type RetryPolicy func(failures int) time.Duration

// Finished reports the effect of ending a run.
type Finished struct {
	// Run is the run as stored after the call.
	Run Run
	// Applied is false when the run had already ended, so its record did
	// not change. A late result for an abandoned run lands here.
	Applied bool
	// LateSuccess reports that a success arrived for a run already abandoned,
	// and was still stamped as the task's LastRun.
	LateSuccess bool
	// Failures and NextRetry are the task's ladder after a failure; both are
	// zero after a success.
	Failures  int
	NextRetry time.Time
}

// Store persists task state and runs.
//
// Nil: rejected — the wiring site constructs a backend and returns an error
// rather than handing out a nil Store, because a silently absent store would
// make every task look permanently due.
type Store interface {
	// Load reads the state of the named tasks, including each task's active
	// run. Names with no stored record are absent from the result rather than
	// present-and-zero, so a caller can tell "never ran" from "ran at the
	// zero time".
	//
	// Scoped to names rather than returning everything: on a shared database
	// an unbounded read would return rows belonging to another deployment, and
	// scoping means a record for a task that no longer exists is simply never
	// read, which is what keeps Prune a pure optimisation.
	Load(ctx context.Context, tasks []string) (map[string]TaskState, error)

	// Seed stores a task's state only if the task has no record yet. It
	// exists to import state written by an older release, and must never
	// overwrite newer state.
	Seed(ctx context.Context, task string, st TaskState) error

	// CreateRun records a new queued run.
	//
	// It returns [ErrRunActive] when the task already has an active run, and
	// [ErrStale] when the task's Version is no longer expectVersion. A task
	// with no record has version 0. Both checks are atomic with the insert,
	// so two processes deciding the same task is due create one run.
	CreateRun(ctx context.Context, run Run, expectVersion int64) error

	// StartRun moves a queued run to running, recording the node and a new
	// lease. It returns the run as stored. A run that is not queued returns
	// [ErrNotQueued] together with its current state: the caller is a
	// duplicate delivery and must not execute it.
	StartRun(ctx context.Context, id, node string, now, leaseUntil time.Time) (Run, error)

	// StartChild claims one for_each subject for execution and moves the
	// run's lease forward to leaseUntil (never backwards).
	//
	// It returns false, and changes nothing, when the run has ended or the
	// subject has already settled. The caller must then not execute the
	// subject: its run was abandoned and a retry owns the subject, or this is
	// a redelivery of a child that already ran. Settling is what makes a
	// subject final, so a claimed subject whose attempt failed stays
	// claimable for the queue's next attempt.
	//
	// A backend may skip a lease extension of under a minute, to avoid a
	// write per subject.
	StartChild(ctx context.Context, id, subject string, leaseUntil time.Time) (bool, error)

	// ExpectChildren records the for_each subjects a running run fans out to.
	// Call it before enqueueing any child, so a child can never settle
	// against a run that does not know about it yet.
	ExpectChildren(ctx context.Context, id string, subjects []string) error

	// SettleChild records one subject's final outcome and returns the run's
	// ending when this was the last subject to settle.
	//
	// Idempotent per subject: a second settle of the same subject (a
	// redelivered child) changes nothing and returns settled=false. When the
	// last subject settles, the run ends as a failure if any subject failed,
	// with the task's state updated as [Store.FinishRun] does.
	SettleChild(
		ctx context.Context, id, subject string, out Outcome, policy RetryPolicy,
	) (settled bool, done *Finished, err error)

	// SucceededSubjects returns the for_each subjects that settled without
	// error in any stored run of task for occurrence, sorted.
	//
	// A failed fan-out is retried as a whole run, and without this the retry
	// would repeat every subject's side effect (a reminder mail, say) for the
	// recipients that already got it. The expansion excludes these.
	SucceededSubjects(ctx context.Context, task, occurrence string) ([]string, error)

	// FinishRun ends an active run with out, and in the same write stamps the
	// task's LastRun (success: the run's CreatedAt, clearing the ladder) or
	// advances its ladder (failure: Failures+1, NextRetry = out.At plus the
	// policy's delay).
	//
	// A run that has already ended is left alone and reported with
	// Applied=false. One exception: a success for an ABANDONED run still
	// stamps the task's LastRun (LateSuccess). The work did happen, and
	// discarding it would run the task again. The run keeps its abandoned
	// status.
	FinishRun(ctx context.Context, id string, out Outcome, policy RetryPolicy) (Finished, error)

	// Reap abandons every active run whose lease is before now, advancing
	// each task's ladder as a failure observed at now.
	Reap(ctx context.Context, now time.Time, policy RetryPolicy) ([]Finished, error)

	// Prune drops ended runs that finished before before, and task records
	// untouched since before that have no active run. It returns the task
	// names removed.
	//
	// Purely housekeeping: Load is scoped to configured names, so an orphaned
	// record is never read and a backend that never prunes stays CORRECT while
	// growing. Age-based rather than "delete what is not in this config": two
	// nodes mid-rollout hold different schedules.yaml, and a config-driven
	// prune would have each erase the other's tasks on every startup.
	Prune(ctx context.Context, before time.Time) ([]string, error)

	// Close releases resources. Subsequent calls to any method return
	// [ErrClosed]; Close itself is idempotent.
	Close() error
}

// ClampRetry corrects an implausible NextRetry to now.
//
// A pending retry can never legitimately be further out than maxDelay. Anything
// beyond that came from a clock that jumped (VM snapshot resume, NTP step, bad
// RTC) or a hand-edited state file, and because the stored value drives the
// schedule it would otherwise wedge the task FOREVER, silently.
//
// Applied at READ time on every load rather than written back. The stored value
// is untrusted input, so the correction has to survive a process that reads it
// and dies before writing anything — which a durable fix-up would not. It is
// idempotent, so re-applying it costs nothing.
func ClampRetry(ts TaskState, now time.Time, maxDelay time.Duration) (TaskState, bool) {
	if ts.NextRetry.IsZero() || ts.NextRetry.Sub(now) <= maxDelay {
		return ts, false
	}
	ts.NextRetry = now
	return ts, true
}

// ApplyOutcome returns ts after a run that was created at start ends with out.
//
// Shared by every backend so the ladder arithmetic has exactly one definition:
// success stamps start and clears the ladder (the ONLY reset: a success that
// left NextRetry set would keep the task ladder-driven forever); failure
// increments the count and schedules the retry from when it was observed.
//
// A failure of a run that began before an already-recorded success changes
// nothing: it must not resurrect a ladder for a task that has since
// succeeded. Likewise a success never regresses a newer LastRun.
func ApplyOutcome(ts TaskState, start time.Time, out Outcome, policy RetryPolicy) (TaskState, bool) {
	if out.Error == "" {
		if !ts.LastRun.IsZero() && !ts.LastRun.Before(start) {
			return ts, false
		}
		ts.LastRun = start
		ts.Failures = 0
		ts.NextRetry = time.Time{}
		ts.Version++
		return ts, true
	}
	if !ts.LastRun.IsZero() && ts.LastRun.After(start) {
		return ts, false
	}
	ts.Failures++
	ts.NextRetry = out.At.Add(policy(ts.Failures))
	ts.Version++
	return ts, true
}

// ChildOutcome is the run-level outcome once every for_each subject has
// settled: success when none failed, otherwise a failure naming how many.
func ChildOutcome(r Run, firstError string, at time.Time) Outcome {
	if r.ChildrenFailed == 0 {
		return Outcome{At: at}
	}
	msg := fmt.Sprintf("%d of %d for_each subjects failed", r.ChildrenFailed, r.Children)
	if firstError != "" {
		msg += "; first: " + firstError
	}
	return Outcome{Error: msg, At: at}
}
