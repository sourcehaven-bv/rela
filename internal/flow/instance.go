package flow

import (
	"maps"
	"time"
)

// Instance is the stored state of one run of a definition: the state it is
// in, how often it moved, and when it entered the state. The zero value is
// unset: the instance never moved, and [Read] reads it as the definition's
// initial state.
//
// NextDueAt, Restarts and Reminders are the instance's timer schedule (see
// [Schedule]). A store keeps them, but never derives them.
type Instance struct {
	State      string    `json:"state,omitempty"`
	Generation int64     `json:"generation,omitempty"`
	EnteredAt  time.Time `json:"entered_at,omitzero"`
	// Cause is the sequence number of the event that moved the instance
	// into its current state (for a channel session, the waiting message).
	// Zero: the initial state, a timer move, or a move stored before the
	// field existed.
	Cause int64 `json:"cause,omitempty"`
	// Previous is the state the instance was in before it entered its
	// current one (TKT-56TPOTE). A re-entry keeps it. Empty: the instance
	// never left its initial state, or last moved before the field existed.
	Previous string `json:"previous,omitempty"`
	// NextDueAt is the index hint a store lists due instances by: the
	// earliest due time of the current state's timers and reminders. Zero:
	// no due time.
	NextDueAt time.Time `json:"next_due_at,omitzero"`
	// Restarts holds, per timer name, the restart anchor of a timer that
	// came due while the instance was paused. A move clears it.
	Restarts map[string]time.Time `json:"restarts,omitempty"`
	// Reminders holds, per reminder name, the reminder's record in the
	// current span. A move clears it; the step's schedule decides which
	// records carry over.
	Reminders map[string]ReminderRecord `json:"reminders,omitempty"`
}

// IsZero reports an unset instance.
func (i Instance) IsZero() bool { return i.Generation == 0 && i.State == "" }

// Enter returns the instance after move, made at at: the generation rises
// by one and the instance enters move.To at at. The due time, restart
// anchors and reminder records are cleared; the step's [Schedule] sets the
// new ones. cause is the sequence number of the event that made the move,
// or 0 for a timer move. The previous state becomes move.From, except on a
// re-entry, which keeps it.
func (i Instance) Enter(move Move, at time.Time, cause int64) Instance {
	previous := move.From
	if move.From == move.To {
		previous = i.Previous
	}
	return Instance{
		State: move.To, Generation: i.Generation + 1, EnteredAt: NormTime(at), Cause: cause,
		Previous: previous,
	}
}

// ReminderRecord is one reminder's state in a span: the time its slots
// count from, the time of its last post and how many it posted. The slot
// grid itself is configuration, so only these three are stored.
type ReminderRecord struct {
	// Since is the start of the span, or the resume time after a restart.
	Since time.Time `json:"since"`
	// Last is the time of the last post in this span; zero for none.
	Last time.Time `json:"last,omitzero"`
	// Count is the number of reminders posted in this span.
	Count int64 `json:"count"`
}

// NormReminder returns r with its times normalized ([NormTime]), as every
// store keeps them, so a compare-and-set on a record read back compares
// equal values.
func NormReminder(r ReminderRecord) ReminderRecord {
	return ReminderRecord{Since: NormTime(r.Since), Last: NormTime(r.Last), Count: r.Count}
}

// CloneReminders returns a copy of rs, or nil for none.
func CloneReminders(rs map[string]ReminderRecord) map[string]ReminderRecord {
	if len(rs) == 0 {
		return nil
	}
	out := make(map[string]ReminderRecord, len(rs))
	maps.Copy(out, rs)
	return out
}

// NormTime returns t in UTC, truncated to the microsecond: the precision
// every store keeps. The zero time stays zero.
func NormTime(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return t.UTC().Truncate(time.Microsecond)
}

// Move is one transition: the state the instance leaves and the state it
// enters. From equals To for a re-entry.
type Move struct{ From, To string }

// Schedule is an instance's timer schedule after a step: the next due time
// and, when non-nil, the restart anchors and reminder records to store. A
// nil Restarts keeps the stored anchors; a nil Reminders keeps the stored
// records, and an empty one stores none.
type Schedule struct {
	NextDueAt time.Time
	Restarts  map[string]time.Time
	Reminders map[string]ReminderRecord
}
