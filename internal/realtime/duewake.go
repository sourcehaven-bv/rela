package realtime

import (
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// DueWake carries the due times that writes store to the timer loop, so
// the leader can run a round earlier than its next planned one. Note keeps
// the earliest time and pings C; Take returns and clears it. A ping is
// coalesced: C holds at most one.
//
// One DueWake is built at wiring and shared by every service built over
// the store, the trigger engine and the worker. A process that does not
// lead notes due times nobody takes, which costs nothing.
type DueWake struct {
	mu sync.Mutex
	at time.Time
	c  chan struct{}
}

// NewDueWake returns an empty DueWake.
func NewDueWake() *DueWake { return &DueWake{c: make(chan struct{}, 1)} }

// Note records the due time at and pings C. A zero at is ignored.
func (w *DueWake) Note(at time.Time) {
	if at.IsZero() {
		return
	}
	w.mu.Lock()
	if w.at.IsZero() || at.Before(w.at) {
		w.at = at
	}
	w.mu.Unlock()
	select {
	case w.c <- struct{}{}:
	default:
	}
}

// C receives a value after a Note.
func (w *DueWake) C() <-chan struct{} { return w.c }

// Take returns the earliest due time noted since the last Take, or zero,
// and clears it.
func (w *DueWake) Take() time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	at := w.at
	w.at = time.Time{}
	return at
}

// noteDue tells w the due time a session step stored, when it has one.
// Nil step: accepted, a post to a type without a session.
func noteDue(w *DueWake, step *channels.SessionStep) {
	if step == nil || step.Outcome == nil {
		return
	}
	w.Note(step.Outcome.Session.NextDueAt)
}

// TimerSchedule is when the leader runs its next timer round: at a target
// time, never closer than [realtimelimits.TimerFloor] after the last
// round. [NextTimerRound] sets it after a round, and [TimerSchedule.Wake]
// moves it earlier for a due time a [DueWake] reports.
type TimerSchedule struct {
	last, target time.Time
}

// NextTimerRound is the schedule after a round, ended at now, that
// reported rep (see [TimerDelay]).
func NextTimerRound(rep TimerReport, now time.Time) TimerSchedule {
	return TimerSchedule{last: now, target: now.Add(TimerDelay(rep.Next, rep.More, now))}
}

// TimerDelay is the wait before the next timer round after a round that
// reported next and more: the time until next, clamped to
// [realtimelimits.TimerFloor, realtimelimits.TimerCeiling]. more gives the
// floor; a zero next gives the ceiling.
func TimerDelay(next time.Time, more bool, now time.Time) time.Duration {
	switch {
	case more:
		return realtimelimits.TimerFloor
	case next.IsZero():
		return realtimelimits.TimerCeiling
	}
	return min(max(next.Sub(now), realtimelimits.TimerFloor), realtimelimits.TimerCeiling)
}

// Delay is the wait from now until the target, or zero once it has passed.
func (s TimerSchedule) Delay(now time.Time) time.Duration {
	return max(s.target.Sub(now), 0)
}

// Wake moves the target to the due time at when that is earlier, but not
// closer than the floor after the last round, and reports whether it
// moved. A zero at, or one at or after the target, starts no round.
func (s *TimerSchedule) Wake(at time.Time) bool {
	if at.IsZero() || !at.Before(s.target) {
		return false
	}
	floor := s.last.Add(realtimelimits.TimerFloor)
	if at.Before(floor) {
		at = floor
	}
	if !at.Before(s.target) {
		return false
	}
	s.target = at
	return true
}
