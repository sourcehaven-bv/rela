package jobs

import (
	"math/rand/v2"
	"time"
)

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
// The memory queue counts it itself; the postgres queue hands it to River as
// the job's MaxAttempts.
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

// Backoff between attempts. The delay after failed attempt n (1-based) is
//
//	min(n⁴ seconds, retryDelayCap) + retryDelayBase + jitter
//
// where jitter is drawn from [0, n × retryJitterStep). The shape follows the
// curve rela ran under neoq: the first retry is no sooner than retryDelayBase,
// so a blip has time to clear, and later retries spread out fast enough that a
// RetryPersistent job does not pound an endpoint that is down for hours.
//
// The cap keeps a RetryPersistent job trying through its whole
// persistentWindow. Without it the twentieth attempt would wait two days,
// which is the window itself.
const (
	retryDelayBase  = 15 * time.Second
	retryJitterStep = 5 * time.Second
	retryDelayCap   = time.Hour
	// retryDelayMaxAttempt bounds n in retryDelay. Past it the growth term
	// is long capped (60⁴ seconds is 150 days), so clamping changes nothing
	// but keeps the arithmetic from overflowing.
	retryDelayMaxAttempt = 60
)

// retryDelay returns how long to wait before the attempt that follows failed
// attempt n. n below 1 is treated as 1.
func retryDelay(n int) time.Duration {
	n = min(max(n, 1), retryDelayMaxAttempt)
	jitter := rand.N(time.Duration(n) * retryJitterStep) //nolint:gosec // jitter, not a secret
	return retryDelayFloor(n) + jitter
}

// retryDelayFloor is retryDelay without its jitter: the soonest the attempt
// after failed attempt n can run. A job whose deadline passes before it will
// not run again.
func retryDelayFloor(n int) time.Duration {
	n = min(max(n, 1), retryDelayMaxAttempt)
	return min(time.Duration(n*n*n*n)*time.Second, retryDelayCap) + retryDelayBase
}
