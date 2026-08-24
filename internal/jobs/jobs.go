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
//
// # Semantics worth knowing before you write a handler
//
// A job is delivered AT LEAST once in principle and, in the ordinary case,
// exactly once. Two properties are worth stating because the opposite is a
// common default elsewhere:
//
//   - Submitting the same payload twice produces two jobs and two executions.
//     There is no deduplication by payload contents. Two triggers that both
//     decide to notify someone send two notifications.
//   - Job submission is NOT atomic with the database write. If the process
//     dies between a commit and the job reaching the backend, that job is
//     lost. Redis- and AMQP-backed queues behave the same way alongside a
//     database; it is the right trade for payloads that are notifications and
//     API calls, where a rare miss is an annoyance rather than a correctness
//     problem.
//
// So a handler should re-check current state rather than trusting its payload
// to still be accurate, and repeated delivery should be harmless where it
// matters.
//
// A single execution is capped at [handlerTimeout]. That is a backstop against
// a wedged handler holding a worker forever, not a latency target — real
// bounding comes from the job's deadline and its retry budget.
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

// Kind identifies which handler runs a job.
//
// A named type rather than a bare string because the kind namespace is global
// across the process: every subsystem registers into one queue, so two packages
// that both picked "sync" would collide — the second Register would fail at
// startup, or worse, route one subsystem's jobs into another's handler. A
// string parameter gives no hint that the value has to be unique repo-wide.
//
// Construct one with [NewKind], which namespaces it by owner.
type Kind string

// NewKind builds a [Kind] owned by a subsystem.
//
// The owner is conventionally the package that registers the handler
// ("scheduler", "mail"), which is what makes collisions between subsystems
// structurally unlikely rather than a matter of everyone remembering to prefix.
//
//	var taskKind = jobs.NewKind("scheduler", "run-task") // "scheduler:run-task"
func NewKind(owner, name string) Kind {
	return Kind(owner + ":" + name)
}

// String implements [fmt.Stringer].
func (k Kind) String() string { return string(k) }

// Job is a unit of deferred work.
//
// Payload must be JSON-serializable: a durable backend round-trips it through
// the database, so a value that survives in memory but not through JSON would
// work on the fs build and fail on postgres.
type Job struct {
	// Kind routes the job to a registered handler. Required.
	Kind Kind

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

// Enqueuer submits work. This is the seam the vast majority of code should
// depend on — a subsystem that produces jobs has no business starting or
// stopping the queue.
//
// Nil: rejected — constructors validate their collaborators and never return
// a nil Enqueuer alongside a nil error.
type Enqueuer interface {
	// Enqueue submits a job. It returns once the job is accepted, not once
	// it has run.
	//
	// The queue must be running; otherwise it returns [ErrNotStarted]. The
	// wiring site starts it (see appbuild), so ordinary consumers can treat
	// the queue they are handed as live.
	//
	// If ctx carries an open transaction collector (see [WithDeferral]),
	// the enqueue is held until that transaction commits. That path does NOT
	// require a started queue: the enqueue happens at commit time.
	Enqueue(ctx context.Context, job Job) error
}

// Registrar binds handlers to kinds.
//
// Separate from [Enqueuer] because producing and consuming are different
// capabilities: most code only submits work, and a subsystem that registers a
// handler (the scheduler, a mail sender) does so once, at wiring time, with the
// queue injected — it does not need Enqueue to do it.
type Registrar interface {
	// Register binds a handler to a kind. Registering the same kind twice
	// returns [ErrDuplicateKind].
	//
	// May be called before or after the queue starts; the dispatcher resolves
	// a handler per job. Registering at wiring time is the norm.
	Register(kind Kind, h Handler) error
}

// Lifecycle starts and stops a queue.
//
// This is a FRAMEWORK concern, not a consumer one. It is a separate interface
// so that handing a subsystem the ability to submit work does not also hand it
// the ability to shut the queue down for everyone else. Only the composition
// root (appbuild) should depend on it.
type Lifecycle interface {
	// Start begins processing. It does not block; workers run until the
	// queue is closed.
	Start(ctx context.Context) error

	// Close stops processing and releases resources. It is idempotent.
	//
	// On an ephemeral backend, jobs still queued are lost — by design.
	Close(ctx context.Context) error
}

// Client is what a subsystem is handed: it can submit work and register a
// handler for the kinds it owns, but cannot start or stop the queue.
//
// This is the type the composition root exposes (see appbuild.Services.Jobs).
// Prefer depending on [Enqueuer] alone where a subsystem only produces work.
type Client interface {
	Enqueuer
	Registrar
}

// ClientOf returns a [Client] view of q that does NOT type-assert back to
// [Lifecycle].
//
// Returning the queue itself as a Client would narrow only the static type: a
// consumer could recover Start and Close with a type assertion and shut
// background work down for every other subsystem. Wrapping makes the narrowing
// real, which is the difference between a convention and a boundary.
func ClientOf(q Queue) Client {
	return clientView{q}
}

// clientView deliberately embeds the two narrow interfaces rather than Queue,
// so Lifecycle's methods are not promoted onto it.
type clientView struct {
	q Queue
}

func (c clientView) Enqueue(ctx context.Context, job Job) error {
	return c.q.Enqueue(ctx, job)
}

func (c clientView) Register(kind Kind, h Handler) error {
	return c.q.Register(kind, h)
}

// Queue is the full surface a backend implements: production, registration and
// lifecycle together.
//
// Backends satisfy this; CONSUMERS SHOULD NOT DEPEND ON IT. Take the narrowest
// interface that does the job — [Enqueuer] to submit work, [Registrar] to bind
// a handler, and [Lifecycle] only at the composition root.
type Queue interface {
	Enqueuer
	Registrar
	Lifecycle
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
