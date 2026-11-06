package flow

import "time"

// A reminder posts every Every while the instance stays inside its states
// (its span) and is not paused. Only a record per reminder is stored (see
// [ReminderRecord]): the time its slots count from, the time of its last
// post and its count. The slots themselves, since + first + k*every, are
// derived from the definition at every read, so a reload that changes
// every or first takes effect at the next slot after the last post.

// RecordOf is r's record for cur: the stored one, or for a reminder
// active in the current state without one, a span that started when the
// instance entered its state. Every move inside the span stores the record
// explicitly (see [CarriedReminders]), so an absent record means the span
// began at the state's entry. ok is false when r is not active in the
// current state.
func RecordOf(r Reminder, cur Current) (ReminderRecord, bool) {
	if !r.Active(cur.State) {
		return ReminderRecord{}, false
	}
	if rec, ok := cur.Reminders[r.Name]; ok {
		return rec, true
	}
	return ReminderRecord{Since: cur.EnteredAt}, true
}

// NextSlot is r's next slot for cur: since + first when the reminder has
// not posted in this span, else the first slot since + first + k*every
// strictly after its last post, with k negative when a reload moved first
// past the last post. So the next slot is at most one every after the last
// post, slots missed while no leader ran are skipped, not replayed, and
// the phase stays on the grid. ok is false when r is not active in the
// state, its max is reached, or the state is stranded.
func NextSlot(r Reminder, cur Current) (time.Time, bool) {
	if cur.Stranded {
		return time.Time{}, false
	}
	rec, ok := RecordOf(r, cur)
	if !ok || (r.Max > 0 && rec.Count >= r.Max) || r.Every <= 0 {
		return time.Time{}, false
	}
	first := rec.Since.Add(r.First)
	if rec.Last.IsZero() {
		return NormTime(first), true
	}
	diff := rec.Last.Sub(first)
	k := int64(diff / r.Every)
	if diff%r.Every < 0 {
		k-- // floor, not truncation, for a last post before first
	}
	return NormTime(first.Add(time.Duration(k+1) * r.Every)), true
}

// NextReminder is the reminder of cur's state whose next slot comes first,
// and that slot. A tie goes to the first name. A paused reminder (see
// [Definition.Pausing]) has no slot. ok is false when no reminder has a
// next slot.
func NextReminder(d *Definition, cur Current) (r Reminder, slot time.Time, ok bool) {
	if _, isLive := live(d, cur); !isLive {
		return r, slot, false
	}
	for _, rm := range d.RemindersIn(cur.State) {
		if d.Paused(rm.Name) {
			continue
		}
		at, has := NextSlot(rm, cur)
		if has && (!ok || at.Before(slot)) {
			r, slot, ok = rm, at, true
		}
	}
	return r, slot, ok
}

// CarriedReminders are the records a move from from to state to keeps:
// for every reminder active both in the state the instance left and in to,
// its record, an implicit one made explicit. A reminder active only in to
// starts a new span with no record. Never nil, so the schedule replaces
// the records [Instance.Enter] cleared.
func CarriedReminders(d *Definition, from Current, to string) map[string]ReminderRecord {
	out := map[string]ReminderRecord{}
	for _, r := range d.reminders {
		if !r.Active(to) {
			continue
		}
		if rec, ok := RecordOf(r, from); ok {
			out[r.Name] = rec
		}
	}
	return out
}

// PruneReminders returns the records of rs whose reminder d declares. A
// record of a declared reminder not active in the current state is kept
// until the next move. Never nil, so a schedule that returns it replaces
// the stored records, and a reload that renames every reminder never grows
// the map past the reminders d declares.
func PruneReminders(d *Definition, rs map[string]ReminderRecord) map[string]ReminderRecord {
	out := map[string]ReminderRecord{}
	for name, rec := range rs {
		if _, ok := d.Reminder(name); ok {
			out[name] = rec
		}
	}
	return out
}

// ResumeReminders returns the records of cur after it resumes at at: every
// reminder of the current state whose next slot came at or before at
// restarts from at, with no last post and its count kept, so the next
// reminder comes First after the resume. A reminder whose slot was not yet
// due keeps its record. The map is pruned (see [PruneReminders]).
func ResumeReminders(d *Definition, cur Current, at time.Time) map[string]ReminderRecord {
	out := PruneReminders(d, cur.Reminders)
	for _, r := range d.RemindersIn(cur.State) {
		slot, ok := NextSlot(r, cur)
		if !ok || slot.After(at) {
			continue
		}
		rec, _ := RecordOf(r, cur)
		out[r.Name] = ReminderRecord{Since: at, Count: rec.Count}
	}
	return out
}

// ReminderCounts are the reminder counters a guard reads for cur: for each
// declared reminder, its count in the current span when it is active in
// the current state, else 0.
func ReminderCounts(d *Definition, cur Current) map[string]int64 {
	out := make(map[string]int64, len(d.reminders))
	for _, r := range d.reminders {
		var n int64
		if r.Active(cur.State) {
			n = cur.Reminders[r.Name].Count
		}
		out[r.Name] = n
	}
	return out
}
