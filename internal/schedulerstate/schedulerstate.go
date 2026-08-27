// Package schedulerstate holds per-task scheduling bookkeeping for
// [github.com/Sourcehaven-BV/rela/internal/scheduler]: when a task last ran
// successfully, how many times it has failed since, and when it may be tried
// again.
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
	"time"
)

// ErrClosed is returned by every method once Close has been called.
var ErrClosed = errors.New("schedulerstate: store is closed")

// ErrNoTask is returned when a task name is empty. Defense in depth: the
// scheduler's own config parser already rejects one, so reaching this means a
// caller built a task by hand.
var ErrNoTask = errors.New("schedulerstate: task name must not be empty")

// RunState is one task's scheduling record.
//
// The three fields are independent on purpose. Encoding "is this failing?" into
// LastRun alone is what caused BUG-ZKK2UL: a failure left no trace, so the task
// stayed perpetually due and retried at the tick rate.
type RunState struct {
	// LastRun is the START time of the last SUCCESSFUL run, and the value the
	// schedule is evaluated against. Zero means the task has never succeeded.
	//
	// Start rather than completion, deliberately: a task that begins at 23:59
	// and finishes past midnight would otherwise land on the next day and
	// silently skip that day's execution. It also keeps interval schedules
	// from drifting forward by each run's duration.
	LastRun time.Time

	// Failures counts CONSECUTIVE failures. Zero means healthy. The caller
	// derives the retry delay from it; this package never interprets it.
	Failures int

	// NextRetry is when a failing task may next be attempted. Zero means no
	// retry is pending. While it is set the ordinary schedule is suppressed,
	// so a failing task fires on the ladder and never on its cadence.
	NextRetry time.Time
}

// Failing reports whether a retry is pending.
func (r RunState) Failing() bool { return !r.NextRetry.IsZero() }

// Store persists run-state one task at a time.
//
// Nil: rejected — the wiring site constructs a backend and returns an error
// rather than handing out a nil Store, because a silently absent store would
// make every task look permanently due.
type Store interface {
	// Load reads the state of the named tasks. Names with no stored record are
	// absent from the result rather than present-and-zero, so a caller can tell
	// "never ran" from "ran at the zero time".
	//
	// Scoped to names rather than returning everything: on a shared database
	// an unbounded read would return rows belonging to another deployment, and
	// scoping means a record for a task that no longer exists is simply never
	// read — which is what keeps Prune a pure optimisation.
	Load(ctx context.Context, tasks []string) (map[string]RunState, error)

	// RecordSuccess stamps start as the last successful run AND clears the
	// retry ladder, atomically.
	//
	// The clear is load-bearing, not incidental: it is the ONLY reset. A
	// success that left NextRetry set would leave the task ladder-driven
	// forever, because a pending retry suppresses the ordinary schedule —
	// BUG-ZKK2UL from the other direction.
	//
	// Ignored when a success with a later start is already recorded, so a
	// slow node cannot regress a newer result.
	RecordSuccess(ctx context.Context, task string, start time.Time) error

	// RecordFailure increments the consecutive-failure count and returns the
	// new value.
	//
	// The store owns the increment so it is atomic; the caller owns what the
	// count MEANS (the backoff curve is scheduling policy). Passing a count in
	// would be a read-modify-write over the wire, which loses updates and can
	// walk the ladder backwards.
	//
	// Ignored, returning the stored count, when a success with a later start
	// is already recorded: a failure must not resurrect a ladder for a task
	// that has since succeeded.
	RecordFailure(ctx context.Context, task string, start time.Time) (failures int, err error)

	// SetNextRetry records when a failing task may next be attempted. Guarded
	// by the same rule as RecordFailure.
	SetNextRetry(ctx context.Context, task string, start, retryAt time.Time) error

	// Prune drops records not touched since before, returning the task names
	// removed.
	//
	// Purely housekeeping: Load is scoped to configured names, so an orphaned
	// record is never read and a backend that never prunes stays CORRECT while
	// growing. That split is deliberate — correctness must not depend on a
	// sweeper an operator might disable.
	//
	// Age-based rather than "delete what is not in this config": two nodes
	// mid-rollout hold different schedules.yaml, and a config-driven prune
	// would have each erase the other's tasks on every startup.
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
func ClampRetry(rs RunState, now time.Time, maxDelay time.Duration) (RunState, bool) {
	if rs.NextRetry.IsZero() || rs.NextRetry.Sub(now) <= maxDelay {
		return rs, false
	}
	rs.NextRetry = now
	return rs, true
}
