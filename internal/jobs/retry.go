package jobs

import "time"

// This file is the ONE place retry mechanism is defined. Everything here is
// deliberately changeable: [Retry] is a statement of intent, and translating
// intent into attempt counts and time bounds is this package's job, not the
// caller's.
//
// If you are about to add a parameter to [Job] so a call site can override
// one of these numbers, stop — that is the coupling the enum exists to
// prevent. Add a new intent value instead, or retune here for everyone.

const (
	// neverAttempts is the total attempt budget for RetryNever.
	neverAttempts = 1

	// boundedAttempts is how many times RetryBounded tries in total.
	//
	// Enough to ride out a transient blip (a connection reset, a brief 503)
	// without pounding an endpoint that is genuinely down. The backoff
	// between attempts matters more than the count.
	boundedAttempts = 5

	// persistentAttempts is the attempt ceiling for RetryPersistent.
	//
	// Paired with persistentWindow below: whichever bound is hit first
	// stops the job. The count exists so a fast-failing job (one that
	// errors in milliseconds) cannot burn thousands of attempts inside the
	// time window.
	persistentAttempts = 200

	// persistentWindow is the outer time bound on RetryPersistent.
	//
	// "Persistent" does not mean forever. Work still failing after two days
	// needs a human — a queue that retries it indefinitely converts a
	// visible failure into invisible background noise, and on a durable
	// backend the row never leaves the table.
	persistentWindow = 48 * time.Hour
)

// maxAttempts returns the TOTAL number of executions a policy permits,
// including the first.
//
// This package enforces the budget itself, in the dispatcher — it is never
// handed to the backend. See [Retry.backendRetryBudget] for why.
func (r Retry) maxAttempts() int {
	switch r {
	case RetryNever:
		return neverAttempts
	case RetryBounded:
		return boundedAttempts
	case RetryPersistent:
		return persistentAttempts
	default:
		// Unreachable: Job.validate rejects out-of-range values before a
		// backend sees them. Fall back to the most conservative policy —
		// one attempt — rather than guessing in the direction of repeated
		// side effects.
		return neverAttempts
	}
}

// backendRetryBudget is the MaxRetries value handed to neoq.
//
// It is deliberately a large constant rather than the policy's real budget,
// and it is the same for every policy. The reason is a hard upstream
// constraint, not laziness:
//
// neoq's worker loop treats both ErrJobExceededMaxRetries and
// ErrJobExceededDeadline as FATAL — it returns from the goroutine
// (memory_backend.go:245-250). So a job that legitimately exhausts its retry
// budget does not merely stop: it takes a worker with it. Four such jobs and a
// four-worker pool is permanently silent, with no error surfaced to rela and
// nothing in the logs. Verified empirically: after four RetryNever jobs
// exhausted, 0 of 8 subsequent healthy jobs ran.
//
// The fix is to never let neoq evaluate its own terminal conditions. rela
// hands it a budget it will not reach, omits Deadline entirely, and enforces
// BOTH the attempt count and the deadline in [neoqQueue.dispatch], which
// returns nil (consuming the attempt) once a job is spent. Since dispatch is
// the single chokepoint every job passes through, that converts an open-ended
// upstream hazard into one invariant this package owns and tests.
//
// Pinned by the jobstest "pool survives exhausted jobs" conformance case.
const backendRetryBudget = 1 << 30

// effectiveDeadline returns when the job should stop being retried, given the
// time it was enqueued.
//
// A caller-supplied [Job.Deadline] always wins when it is sooner: the caller
// knows something the queue does not (a scheduler knows its next run makes
// further retries pointless). Otherwise RetryPersistent gets its own outer
// bound, and the other policies rely on their attempt count alone.
//
// The zero return means "no deadline".
func (r Retry) effectiveDeadline(job Job, enqueuedAt time.Time) time.Time {
	if r != RetryPersistent {
		return job.Deadline
	}
	window := enqueuedAt.Add(persistentWindow)
	if job.Deadline.IsZero() || window.Before(job.Deadline) {
		return window
	}
	return job.Deadline
}
