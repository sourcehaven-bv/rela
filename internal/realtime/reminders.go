package realtime

import (
	"context"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// Session reminders (`session.reminders`, TKT-36Z6X3). The slot rules are
// the engine's (see [flow.NextSlot]); this file adapts them to channels and
// posts the reminder messages.

// reminderRecord is r's record for cur (see [flow.RecordOf]). ok is false
// when r is not active in the current state.
func reminderRecord(r realtimecfg.SessionReminder, cur current) (channels.ReminderRecord, bool) {
	return flow.RecordOf(r.Reminder, cur)
}

// nextSlot is r's next slot for cur (see [flow.NextSlot]). The channel the
// session was read from adds nothing: cur carries the stored records.
func nextSlot(r realtimecfg.SessionReminder, cur current, _ channels.Channel) (time.Time, bool) {
	return flow.NextSlot(r.Reminder, cur)
}

// nextReminder is the reminder of ch's current state whose next slot comes
// first, and that slot (see [flow.NextReminder]). ok is false when no
// reminder has a next slot.
func nextReminder(
	s *realtimecfg.Session, ch channels.Channel,
) (r realtimecfg.SessionReminder, slot time.Time, ok bool) {
	fr, slot, ok := flow.NextReminder(s.Flow(), currentSession(s, ch))
	if !ok {
		return r, slot, false
	}
	// The definition is built from s's reminders, so the name is declared.
	r, _ = s.Reminder(fr.Name)
	return r, slot, true
}

// nextDue is the value of Session.NextDueAt for ch (see [flow.NextDue]).
func nextDue(s *realtimecfg.Session, ch channels.Channel) time.Time {
	return flow.NextDue(s.Flow(), currentSession(s, ch))
}

// reminderCounters binds `session.reminders` for a guard over cur (see
// [flow.ReminderCounts]). The channel adds nothing: cur carries the stored
// records.
func reminderCounters(s *realtimecfg.Session, cur current, _ channels.Channel) predicate.Value {
	counts := flow.ReminderCounts(s.Flow(), cur)
	fields := make(map[string]predicate.Value, len(counts))
	for name, n := range counts {
		fields[name] = predicate.NewInt(n)
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
// committed records, pruned (see [flow.PruneReminders]).
func reminderStep(
	ct realtimecfg.ChannelType, s *realtimecfg.Session, r realtimecfg.SessionReminder,
	listed channels.ReminderRecord, props map[string]any, now time.Time,
) channels.SessionStep {
	listed = channels.NormReminder(listed)
	// The load checks that the post type exists; the lookup stays nil-safe.
	noUnread := false
	if mt := ct.MessageTypes[r.PostType]; mt != nil {
		noUnread = mt.NoUnread
	}
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
		rec, ok := reminderRecord(r, cur)
		if !ok {
			return channels.ReminderPost{}, false
		}
		rec = channels.NormReminder(rec)
		if !rec.Since.Equal(listed.Since) || !rec.Last.Equal(listed.Last) {
			return channels.ReminderPost{}, false
		}
		slot, ok := flow.NextSlot(r.Reminder, cur)
		if !ok || slot.After(now) {
			return channels.ReminderPost{}, false
		}
		return channels.ReminderPost{
			Name: r.Name, Record: channels.ReminderRecord{Since: rec.Since, Last: now, Count: rec.Count + 1},
			Type: r.PostType, Props: props, TriggeredBy: reminderLabel(ct.Name, r.Name), NoUnread: noUnread,
		}, true
	}
	step.Due = func(next channels.Channel) channels.SessionSchedule {
		next.Session.Reminders = flow.PruneReminders(s.Flow(), next.Session.Reminders)
		return channels.SessionSchedule{NextDueAt: nextDue(s, next), Reminders: next.Session.Reminders}
	}
	return step
}
