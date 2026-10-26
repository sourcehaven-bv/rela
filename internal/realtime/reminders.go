package realtime

import (
	"context"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// Session reminders (`session.reminders`, TKT-36Z6X3).
//
// A reminder posts a message every Every while the session stays inside
// its `in:` states (its span) and the channel is open. Only a record per
// reminder is stored (see [channels.ReminderRecord]): the time its slots
// count from, the time of its last post and its count. The slots
// themselves, since + first + k*every, are derived from the configuration
// at every read, so a reload that changes every or first takes effect at
// the next slot after the last post.

// reminderRecord is r's record for ch, whose current session is cur: the
// stored one, or for a reminder active in the current state without one, a
// span that started when the session entered its state. Every move inside
// the span stores the record explicitly (see [carriedReminders]), so an
// absent record means the span began at the state's entry. ok is false
// when r is not active in the current state.
func reminderRecord(r realtimecfg.SessionReminder, cur current, ch channels.Channel) (channels.ReminderRecord, bool) {
	if !r.Active(cur.State) {
		return channels.ReminderRecord{}, false
	}
	if rec, ok := ch.Session.Reminders[r.Name]; ok {
		return rec, true
	}
	return channels.ReminderRecord{Since: cur.EnteredAt}, true
}

// nextSlot is r's next slot for ch, whose current session is cur: since +
// first when the reminder has not posted in this span, else the first slot
// since + first + k*every strictly after its last post, with k negative
// when a reload moved first past the last post. So the next slot is at
// most one every after the last post, slots missed while no leader ran are
// skipped, not replayed, and the phase stays on the grid. ok is false when r is not active in the state, its max is reached,
// or the state is final or stranded.
func nextSlot(r realtimecfg.SessionReminder, cur current, ch channels.Channel) (time.Time, bool) {
	if cur.stranded {
		return time.Time{}, false
	}
	rec, ok := reminderRecord(r, cur, ch)
	if !ok || (r.Max > 0 && rec.Count >= r.Max) || r.Every <= 0 {
		return time.Time{}, false
	}
	first := rec.Since.Add(r.First)
	if rec.Last.IsZero() {
		return channels.NormTime(first), true
	}
	diff := rec.Last.Sub(first)
	k := int64(diff / r.Every)
	if diff%r.Every < 0 {
		k-- // floor, not truncation, for a last post before first
	}
	return channels.NormTime(first.Add(time.Duration(k+1) * r.Every)), true
}

// nextReminder is the reminder of ch's current state whose next slot comes
// first, and that slot. A tie goes to the first name. A paused reminder
// (see [realtimecfg.Session.Pausing]) has no slot. ok is false when no
// reminder has a next slot.
func nextReminder(
	s *realtimecfg.Session, ch channels.Channel,
) (r realtimecfg.SessionReminder, slot time.Time, ok bool) {
	cur := currentSession(s, ch)
	st, declared := s.State(cur.State)
	if !declared || st.Final {
		return r, slot, false
	}
	for _, rm := range s.RemindersIn(cur.State) {
		if s.Paused(rm.Name) {
			continue
		}
		at, has := nextSlot(rm, cur, ch)
		if has && (!ok || at.Before(slot)) {
			r, slot, ok = rm, at, true
		}
	}
	return r, slot, ok
}

// nextDue is the value of Session.NextDueAt for ch: the earlier of the due
// time of [nextTimer] and the slot of [nextReminder] plus
// [realtimelimits.ReminderSlack], normalized, or zero for neither. The
// slack lets one leader round serve several nearby slots; the round fires
// a reminder by its slot, never by this hint.
func nextDue(s *realtimecfg.Session, ch channels.Channel) time.Time {
	_, due, ok := nextTimer(s, ch)
	if _, slot, has := nextReminder(s, ch); has {
		if hint := slot.Add(realtimelimits.ReminderSlack); !ok || hint.Before(due) {
			due, ok = hint, true
		}
	}
	if !ok {
		return time.Time{}
	}
	return channels.NormTime(due)
}

// carriedReminders are the records a move from channel from to state to
// keeps: for every reminder active both in the state the session left and
// in to, its record, an implicit one made explicit. A reminder active only
// in to starts a new span with no record. Never nil, so the schedule
// replaces the records [channels.ApplySession] cleared.
func carriedReminders(s *realtimecfg.Session, from channels.Channel, to string) map[string]channels.ReminderRecord {
	out := map[string]channels.ReminderRecord{}
	left := currentSession(s, from)
	for _, r := range s.Reminders() {
		if !r.Active(to) {
			continue
		}
		if rec, ok := reminderRecord(r, left, from); ok {
			out[r.Name] = rec
		}
	}
	return out
}

// pruneReminders returns the records of rs whose reminder s declares. A
// record of a declared reminder not active in the current state is kept
// until the next move. Never nil, so a schedule that returns it replaces
// the stored records, and a reload that renames every reminder never pushes
// the map over [channels.MaxSessionReminders].
func pruneReminders(s *realtimecfg.Session, rs map[string]channels.ReminderRecord) map[string]channels.ReminderRecord {
	out := map[string]channels.ReminderRecord{}
	for name, rec := range rs {
		if _, ok := s.Reminder(name); ok {
			out[name] = rec
		}
	}
	return out
}

// resumeReminders returns the records of ch after it resumes at at: every
// reminder of the current state whose next slot came at or before at
// restarts from at, with no last post and its count kept, so the next
// reminder comes First after the resume. A reminder whose slot was not yet
// due keeps its record. The map is pruned (see [pruneReminders]).
func resumeReminders(
	s *realtimecfg.Session, cur current, ch channels.Channel, at time.Time,
) map[string]channels.ReminderRecord {
	out := pruneReminders(s, ch.Session.Reminders)
	for _, r := range s.RemindersIn(cur.State) {
		slot, ok := nextSlot(r, cur, ch)
		if !ok || slot.After(at) {
			continue
		}
		rec, _ := reminderRecord(r, cur, ch)
		out[r.Name] = channels.ReminderRecord{Since: at, Count: rec.Count}
	}
	return out
}

// reminderCounters binds `session.reminders` for a guard over ch: for each
// declared reminder, its count in the current span when it is active in
// the current state, else 0. It reads the stored records, because
// [currentSession] replaces an unset session.
func reminderCounters(s *realtimecfg.Session, cur current, ch channels.Channel) predicate.Value {
	fields := map[string]predicate.Value{}
	for _, r := range s.Reminders() {
		var n int64
		if r.Active(cur.State) {
			n = ch.Session.Reminders[r.Name].Count
		}
		fields[r.Name] = predicate.NewInt(n)
	}
	return predicate.NewRecord(fields)
}

// reminderLabel is the TriggeredBy of a message posted by reminder name of
// channel type typ.
func reminderLabel(typ, name string) string { return channels.ReminderMarker + typ + "/" + name }

// reminderProps returns the props of r's message, checked as a trigger's
// are at post time (see [systemWriter.post]): the literal values after
// [validateProps], and the type's `when:` holding. The configuration checked
// both at load; a later metamodel change may break them.
func reminderProps(
	ctx context.Context, cfg *realtimecfg.Compiled, ct realtimecfg.ChannelType, r realtimecfg.SessionReminder,
	now time.Time,
) (map[string]any, error) {
	mt, ok := ct.MessageTypes[r.PostType]
	if !ok {
		return nil, fmt.Errorf("realtime: reminder %s posts message type %s, which %s does not declare",
			r.Name, r.PostType, ct.Name)
	}
	in := make(map[string]any, len(r.Props()))
	for k, v := range r.Props() {
		in[k] = v
	}
	vp, err := validateProps(mt, in, cfg.Limits().MessageBytes)
	if err != nil {
		return nil, fmt.Errorf("realtime: reminder %s: %w", r.Name, err)
	}
	if !whenHolds(ctx, mt, vp.props, now) {
		return nil, fmt.Errorf("realtime: reminder %s: the message does not meet the when: condition of %s",
			r.Name, mt.Name)
	}
	return vp.props, nil
}

// reminderStep is the SetSession step that posts r's reminder for a
// channel of type ct with session s, listed with the record listed. Its
// decider never moves. Remind posts only when, under the store's lock, the
// channel is open, r is active in the committed state, its committed record
// has the listed Since and Last, its max is not reached, and its next slot
// is at or before now: a compare-and-set, so a slot posts at most once.
// Otherwise the step only rewrites the due time. Its Due keeps the
// committed records, pruned (see [pruneReminders]).
func reminderStep(
	ct realtimecfg.ChannelType, s *realtimecfg.Session, r realtimecfg.SessionReminder,
	listed channels.ReminderRecord, props map[string]any, now time.Time,
) channels.SessionStep {
	listed = channels.NormReminder(listed)
	step := channels.SessionStep{
		Decide:     func(channels.Channel) (channels.SessionMove, bool) { return channels.SessionMove{}, false },
		NoticeType: realtimecfg.SessionNoticeType, NoticeBy: systemAuthor(),
		TriggeredBy: channels.SessionMarker + ct.Name, Outcome: &channels.SessionOutcome{},
	}
	step.Remind = func(c channels.Channel) (channels.ReminderPost, bool) {
		if c.State != channels.StateOpen {
			return channels.ReminderPost{}, false
		}
		cur := currentSession(s, c)
		rec, ok := reminderRecord(r, cur, c)
		if !ok {
			return channels.ReminderPost{}, false
		}
		rec = channels.NormReminder(rec)
		if !rec.Since.Equal(listed.Since) || !rec.Last.Equal(listed.Last) {
			return channels.ReminderPost{}, false
		}
		slot, ok := nextSlot(r, cur, c)
		if !ok || slot.After(now) {
			return channels.ReminderPost{}, false
		}
		return channels.ReminderPost{
			Name: r.Name, Record: channels.ReminderRecord{Since: rec.Since, Last: now, Count: rec.Count + 1},
			Type: r.PostType, Props: props, TriggeredBy: reminderLabel(ct.Name, r.Name),
		}, true
	}
	step.Due = func(next channels.Channel) channels.SessionSchedule {
		next.Session.Reminders = pruneReminders(s, next.Session.Reminders)
		return channels.SessionSchedule{NextDueAt: nextDue(s, next), Reminders: next.Session.Reminders}
	}
	return step
}
