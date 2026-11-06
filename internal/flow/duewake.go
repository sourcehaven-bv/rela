package flow

import (
	"sync"
	"time"
)

// DueWake carries the due times that writes store to the timer loop, so
// the leader can run a round earlier than its next planned one. Note keeps
// the earliest time and pings C; Take returns and clears it. A ping is
// coalesced: C holds at most one.
//
// One DueWake is built at wiring and shared by every writer. A process
// that does not lead notes due times nobody takes, which costs nothing.
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
