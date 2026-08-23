// Package jobs is the background-job seam: one narrow interface every
// subsystem enqueues through, with durability tiered to the deployment model.
//
// Rela performs side effects against external systems — outbound mail, remote
// HTTP APIs, LLM calls. They are slow and fail intermittently, so running them
// inline on a user-facing write path makes latency unbounded and failure
// handling ad hoc. A job moves that work off the caller's goroutine.
//
// # Backends
//
// The backend is chosen at wiring time, per deployment tier:
//
//	fs / desktop (default build)   MemoryQueue    ephemeral — jobs vanish on exit
//	PostgreSQL   (postgres tag)    PostgresQueue  durable, survives restart
//
// Ephemeral is the CORRECT semantic for a single-user local app, not a
// degraded one: an unsent mail from a session that has ended is not worth
// persisting. Do not "fix" the memory backend to persist.
//
// # The retry contract is deliberately vague
//
// A job declares its retry appetite as a flat [Retry] enum, never a tuned
// policy object. The enum names INTENT; this package owns MECHANISM (attempt
// counts, backoff curves, the outer time bound — all in retry.go). That is
// what lets tuning change in one place instead of across every call site.
//
// Resist widening this into a policy struct or adding per-call knobs. A call
// site that genuinely needs different mechanics is evidence for a new intent
// value, not for exposing parameters.
//
// # Jobs never run before their enqueueing transaction closes
//
// A job enqueued inside an open store transaction must not become runnable
// until that transaction commits — otherwise a worker picks it up on a
// different connection and reads a snapshot that cannot yet see the writes,
// acting on the pre-write world. See [Deferred] and deferred.go.
//
// # Scheduling is not this package's concern
//
// The queue knows nothing about cadence or schedules. Callers that have one
// express it through [Job.Deadline]. See internal/scheduler for how a
// recurring task maps its interval onto that primitive.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Errors returned by [Queue] implementations. They are sentinel values so
// callers (and the jobstest conformance suite) can assert on the failure mode
// rather than on message text.
var (
	// ErrNoKind is returned when a job carries no Kind. A kind is how the
	// queue routes a payload to a handler, so an empty one can only ever be
	// a silent drop.
	ErrNoKind = errors.New("jobs: job kind must not be empty")

	// ErrUnknownKind is returned when enqueueing a kind with no registered
	// handler. Failing at enqueue is deliberate: the alternative is a job
	// that is accepted, stored, and then discarded by a worker that has
	// nothing to run it with — a drop discovered hours later, if at all.
	ErrUnknownKind = errors.New("jobs: no handler registered for kind")

	// ErrDuplicateKind is returned when registering a kind twice. Silently
	// replacing the first handler would make wiring order load-bearing.
	ErrDuplicateKind = errors.New("jobs: handler already registered for kind")

	// ErrClosed is returned by Enqueue after the queue has been closed.
	ErrClosed = errors.New("jobs: queue is closed")

	// ErrNotStarted is returned by Enqueue before Start has been called.
	//
	// A sentinel rather than a wrapped backend error because the backend's
	// version ("no processor configured for queue") describes its internals,
	// not the caller's mistake.
	ErrNotStarted = errors.New("jobs: queue has not been started")
)

// Retry is how much effort a job is worth. It names intent, not mechanism.
//
// The concrete meaning of each value — attempt counts, backoff, the outer
// time bound — lives in retry.go and is free to change without touching any
// call site. That freedom is the entire reason this is an enum and not a
// struct of tunables.
type Retry int

const (
	// RetryNever runs the job once. A failure is final.
	//
	// For work where a retry is worse than a miss: anything with a
	// user-visible side effect that would duplicate, or a payload whose
	// meaning expires immediately.
	RetryNever Retry = iota

	// RetryBounded tries a few times and then gives up.
	//
	// The default for ordinary external I/O — it rides out a blip without
	// hammering an endpoint that is genuinely down.
	RetryBounded

	// RetryPersistent keeps trying; the job is expected to get through
	// eventually.
	//
	// This does NOT mean literally forever. The implementation drops the
	// job after an outer time bound (see retry.go) with an error log,
	// because work that has failed for that long needs a human, not
	// another attempt.
	RetryPersistent
)

// String implements [fmt.Stringer] so log lines and test failures name the
// intent rather than printing a bare integer.
func (r Retry) String() string {
	switch r {
	case RetryNever:
		return "never"
	case RetryBounded:
		return "bounded"
	case RetryPersistent:
		return "persistent"
	default:
		return fmt.Sprintf("Retry(%d)", int(r))
	}
}

// valid reports whether r is a defined enum value. Guards against an
// out-of-range int being silently treated as RetryNever.
func (r Retry) valid() bool {
	return r >= RetryNever && r <= RetryPersistent
}

// Job is a unit of deferred work.
//
// Payload must be JSON-serializable: a durable backend round-trips it through
// the database, so a value that survives in memory but not through JSON would
// work on the fs build and fail on postgres.
type Job struct {
	// Kind routes the job to a registered handler. Required.
	Kind string

	// Payload is the job's input. It names WHAT to act on — never what is
	// permitted. A handler must re-derive authority from the principal on
	// its context, never trust a payload field claiming it.
	Payload map[string]any

	// Retry is the job's retry appetite. The zero value is RetryNever,
	// which is the safe default: work that must survive failure has to say
	// so explicitly.
	Retry Retry

	// Deadline is when the job stops being worth running. Zero means no
	// deadline.
	//
	// This is the generic primitive callers with a schedule use to express
	// "do not retry past my next run" — see the package doc. A job already
	// past its deadline at enqueue is dropped rather than run once and
	// failed.
	Deadline time.Time
}

// Handler runs a job. Returning a non-nil error marks the attempt failed and
// subjects it to the job's [Retry] policy.
//
// The context carries the enqueueing principal and audit attribution, and is
// cancelled when the job's timeout expires or the queue shuts down. A handler
// that ignores cancellation keeps running after the queue has given up on it,
// so long operations should select on ctx.Done.
type Handler func(ctx context.Context, job Job) error

// Queue is the seam. Consumers depend on this interface, not on a backend.
//
// Nil: rejected — constructors validate their collaborators and never return
// a nil Queue alongside a nil error.
type Queue interface {
	// Register binds a handler to a kind. It must be called before Start;
	// registering the same kind twice returns [ErrDuplicateKind].
	Register(kind string, h Handler) error

	// Enqueue submits a job. It returns once the job is accepted, not once
	// it has run.
	//
	// Start must have been called first; otherwise it returns
	// [ErrNotStarted]. The wiring site is responsible for starting the queue
	// — see appbuild.
	//
	// If ctx carries an open transaction collector (see [WithDeferral]),
	// the enqueue is held until that transaction commits. That path does NOT
	// require a started queue: the enqueue happens at commit time.
	Enqueue(ctx context.Context, job Job) error

	// Start begins processing. It does not block; workers run until the
	// queue is closed.
	Start(ctx context.Context) error

	// Close stops processing and releases resources. It is idempotent.
	//
	// On an ephemeral backend, jobs still queued are lost — by design.
	Close(ctx context.Context) error
}

// validate checks the parts of a job that no backend can sensibly accept.
// Shared by every implementation so the contract cannot drift between them.
func (j Job) validate() error {
	if j.Kind == "" {
		return ErrNoKind
	}
	if !j.Retry.valid() {
		return fmt.Errorf("jobs: invalid retry policy %d for kind %q", int(j.Retry), j.Kind)
	}
	return nil
}

// expired reports whether the job's deadline has already passed at time now.
//
// A zero Deadline means "no deadline" — this must not be confused with the
// epoch, which would expire every job that omitted the field.
func (j Job) expired(now time.Time) bool {
	return !j.Deadline.IsZero() && !now.Before(j.Deadline)
}
