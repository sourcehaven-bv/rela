package flow

import "time"

// Timers and reminders (TKT-INM3AZ, TKT-36Z6X3).
//
// An instance's due time is derived from the definition and the stored
// instance: the current state's timers, each counting from its anchor (see
// [TimerDue]), and its reminders' next slots (see [NextSlot]). The stored
// [Instance.NextDueAt] (see [NextDue]) is only an index hint: every write
// that changes the instance stores it, the leader's recompute repairs a
// stale one, and readers always derive the due time again.

// TimerDue is when t comes due for cur. A timer counts from when the
// instance entered its state (since: state; its creation for an instance
// that never moved) or from its creation (since: session). A restart
// anchor later than that, stored when the timer came due while the
// instance was paused, wins.
func TimerDue(t Timer, cur Current) time.Time {
	anchor := cur.EnteredAt
	if t.Since == SinceSession {
		anchor = cur.CreatedAt
	}
	if r, ok := cur.Restarts[t.Name]; ok && r.After(anchor) {
		anchor = r
	}
	return NormTime(t.Period.AddTo(anchor))
}

// NextTimer is the timer of cur's state that comes due first, and when. A
// tie goes to the first name. ok is false for a final or stranded state,
// or a state without timers.
func NextTimer(d *Definition, cur Current) (t Timer, due time.Time, ok bool) {
	st, isLive := live(d, cur)
	if !isLive {
		return t, due, false
	}
	for _, tm := range st.timers {
		at := TimerDue(tm, cur)
		if !ok || at.Before(due) {
			t, due, ok = tm, at, true
		}
	}
	return t, due, ok
}

// NextDue is the value of [Instance.NextDueAt] for cur: the earlier of the
// due time of [NextTimer] and the slot of [NextReminder] plus
// [ReminderSlack], normalized, or zero for neither. The
// slack lets one leader round serve several nearby slots; the round fires
// a reminder by its slot, never by this hint.
func NextDue(d *Definition, cur Current) time.Time {
	_, due, ok := NextTimer(d, cur)
	if _, slot, has := NextReminder(d, cur); has {
		if hint := slot.Add(ReminderSlack); !ok || hint.Before(due) {
			due, ok = hint, true
		}
	}
	if !ok {
		return time.Time{}
	}
	return NormTime(due)
}

// StepSchedule is the schedule of a step from from to next, both read
// under d. Without a move it keeps the stored records, pruned (see
// [PruneReminders]). A move clears the restart anchors and reminder
// records (see [Instance.Enter]); the anchor of a since: session timer is
// put back when the target state has a since: session timer of the same
// name, because that timer keeps counting across the move. A since: state
// timer starts again from the move. The records of reminders active in
// both states are put back (see [CarriedReminders]).
func StepSchedule(d *Definition, from, next Current) Schedule {
	if next.Generation == from.Generation {
		sch := Schedule{NextDueAt: NextDue(d, next)}
		if next.Reminders != nil {
			sch.Reminders = PruneReminders(d, next.Reminders)
		}
		return sch
	}
	carried := CarriedRestarts(d, from, next.State)
	reminders := CarriedReminders(d, from, next.State)
	next.Restarts, next.Reminders = carried, reminders
	return Schedule{NextDueAt: NextDue(d, next), Restarts: carried, Reminders: reminders}
}

// CarriedRestarts are the restart anchors of from that a move to state to
// keeps: those of since: session timers declared, with since: session, in
// both the state the instance left and to. Nil when there are none.
func CarriedRestarts(d *Definition, from Current, to string) map[string]time.Time {
	left, ok := d.State(from.State)
	target, ok2 := d.State(to)
	if !ok || !ok2 || len(from.Restarts) == 0 {
		return nil
	}
	sessionScoped := make(map[string]bool)
	for _, t := range left.timers {
		sessionScoped[t.Name] = t.Since == SinceSession
	}
	var out map[string]time.Time
	for _, t := range target.timers {
		r, anchored := from.Restarts[t.Name]
		if !anchored || t.Since != SinceSession || !sessionScoped[t.Name] {
			continue
		}
		if out == nil {
			out = make(map[string]time.Time)
		}
		out[t.Name] = r
	}
	return out
}

// Resume is the schedule of an instance that resumes at at, after a pause
// (a reopened or restored channel). Each timer of the current state that
// came due at or before at restarts from at (its restart anchor becomes
// at); a timer not yet due keeps its deadline. The anchors are kept per
// timer, so repeated pause and resume cycles never stretch a deadline that
// had not come due. Reminders resume the same way (see [ResumeReminders]).
// A final or stranded state gets the zero schedule.
func Resume(d *Definition, cur Current, at time.Time) Schedule {
	at = NormTime(at)
	st, ok := live(d, cur)
	if !ok {
		return Schedule{}
	}
	restarts := make(map[string]time.Time, len(st.timers))
	for _, t := range st.timers {
		if r, ok := cur.Restarts[t.Name]; ok {
			restarts[t.Name] = r
		}
		if due := TimerDue(t, cur); !due.After(at) {
			restarts[t.Name] = at
		}
	}
	reminders := ResumeReminders(d, cur, at)
	resumed := cur
	resumed.Restarts, resumed.Reminders = restarts, reminders
	return Schedule{NextDueAt: NextDue(d, resumed), Restarts: restarts, Reminders: reminders}
}
