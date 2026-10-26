package channelstest

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// reminderTests are the session reminder tests (TKT-36Z6X3), registered in
// [RunAll].
var reminderTests = []conformanceTest{
	{"SetSessionRemind", testSetSessionRemind},
	{"SetSessionMoveBeatsRemind", testSetSessionMoveBeatsRemind},
	{"SetSessionRemindCAS", testSetSessionRemindCAS},
	{"SetSessionRemindRefusals", testSetSessionRemindRefusals},
	{"SessionRemindersStored", testSessionRemindersStored},
	{"SessionRemindersMoved", testSessionRemindersMoved},
}

// reminderPost returns a post of the reminder status-update with rec.
func reminderPost(rec channels.ReminderRecord) channels.ReminderPost {
	return channels.ReminderPost{
		Name: "status-update", Record: rec, Type: "reminder",
		Props:       map[string]any{"body": "Post a status update"},
		TriggeredBy: "reminder:room/status-update",
	}
}

// remindStep returns a SetSession step that never moves, posts post when
// remind says so, and stores due.
func remindStep(
	remind func(channels.Channel) (channels.ReminderPost, bool), due channels.ScheduleFunc,
) channels.SessionStep {
	step := timerStep(stay, "", due)
	step.Remind = remind
	return step
}

// withReminders returns a schedule function that stores due and the
// reminder records rs.
func withReminders(due time.Time, rs map[string]channels.ReminderRecord) channels.ScheduleFunc {
	return func(channels.Channel) channels.SessionSchedule {
		return channels.SessionSchedule{NextDueAt: due, Reminders: rs}
	}
}

// testSetSessionRemind pins a reminder post: one message at last_seq + 1
// with the post's type, props and TriggeredBy, the step's NoticeBy as
// author and at as time; the post's record stored before Due runs; the Due
// result stored; rev raised; no audit record; state and generation kept.
func testSetSessionRemind(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	before := contained(t, h.Store, k, dueAt(T0.Add(time.Hour)))
	trail := sessionRecords(t, h.Store)

	at := T0.Add(40 * time.Minute)
	rec := channels.ReminderRecord{Since: T0, Last: at, Count: 1}
	var seen channels.ReminderRecord
	next := at.Add(30 * time.Minute)
	due := func(c channels.Channel) channels.SessionSchedule {
		seen = c.Session.Reminders["status-update"]
		return channels.SessionSchedule{NextDueAt: next, Reminders: c.Session.Reminders}
	}
	step := remindStep(func(channels.Channel) (channels.ReminderPost, bool) { return reminderPost(rec), true }, due)
	got, err := h.Store.SetSession(ctx, k, at, step)
	require.NoError(t, err)
	require.Equal(t, channels.NormReminder(rec), seen, "Due sees the posted record")

	require.Equal(t, "contained", got.Session.State)
	require.Equal(t, int64(1), got.Session.Generation)
	require.Equal(t, before.Session.EnteredAt, got.Session.EnteredAt)
	require.Equal(t, channels.NormTime(next), got.Session.NextDueAt)
	require.Equal(t, map[string]channels.ReminderRecord{"status-update": channels.NormReminder(rec)}, got.Session.Reminders)
	require.Equal(t, before.LastSeq+1, got.LastSeq)
	require.Greater(t, got.Rev, before.Rev)
	require.True(t, step.Outcome.Reminded)
	require.False(t, step.Outcome.Moved)
	require.Equal(t, got.Session, step.Outcome.Session)
	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, head, got, "the returned channel and Head agree")
	require.Equal(t, trail, sessionRecords(t, h.Store), "a reminder writes no audit record")

	page, err := h.Store.Messages(ctx, k, channels.Page{AfterSeq: before.LastSeq})
	require.NoError(t, err)
	require.Len(t, page, 1)
	m := page[0]
	require.Equal(t, got.LastSeq, m.Seq)
	require.Equal(t, got.Rev, m.Rev)
	require.Equal(t, "reminder", m.Type)
	require.Equal(t, noticeAuthor, m.Author)
	require.Equal(t, "reminder:room/status-update", m.TriggeredBy)
	require.Equal(t, channels.NormTime(at), m.At)
	require.Equal(t, map[string]any{"body": "Post a status update"}, m.Props)
}

// testSetSessionMoveBeatsRemind pins that Remind is not called when the
// decider moves: only the notice is appended.
func testSetSessionMoveBeatsRemind(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	before := contained(t, h.Store, k, dueAt(T0.Add(time.Hour)))
	var calls atomic.Int32
	step := timerStep(containedToOverdue, "late", dueAt(time.Time{}))
	step.Remind = func(channels.Channel) (channels.ReminderPost, bool) {
		calls.Add(1)
		return reminderPost(channels.ReminderRecord{Since: T0, Last: T0, Count: 1}), true
	}
	got, err := h.Store.SetSession(ctx, k, T0.Add(2*time.Hour), step)
	require.NoError(t, err)
	require.Zero(t, calls.Load(), "Remind runs only when nothing moves")
	require.True(t, step.Outcome.Moved)
	require.False(t, step.Outcome.Reminded)
	require.Nil(t, got.Session.Reminders)
	page, err := h.Store.Messages(ctx, k, channels.Page{AfterSeq: before.LastSeq})
	require.NoError(t, err)
	require.Len(t, page, 1)
	require.Equal(t, "session", page[0].Type)
}

// testSetSessionRemindCAS pins that concurrent SetSession calls whose
// Remind posts only while the committed record has no last post append
// exactly one reminder message.
func testSetSessionRemindCAS(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	stores := []channels.Store{h.Store}
	if h.Second != nil {
		stores = append(stores, h.Second(t))
	}
	k := key("TKT-1", "room")
	before := contained(t, h.Store, k, dueAt(T0.Add(time.Hour)))
	at := T0.Add(2 * time.Hour)
	remind := func(c channels.Channel) (channels.ReminderPost, bool) {
		if !c.Session.Reminders["status-update"].Last.IsZero() {
			return channels.ReminderPost{}, false
		}
		return reminderPost(channels.ReminderRecord{Since: T0, Last: at, Count: 1}), true
	}
	keep := func(c channels.Channel) channels.SessionSchedule {
		return channels.SessionSchedule{Reminders: c.Session.Reminders}
	}
	const n = 8
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := range n {
		wg.Go(func() {
			_, err := stores[i%len(stores)].SetSession(ctx, k, at, remindStep(remind, keep))
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	page, err := h.Store.Messages(ctx, k, channels.Page{AfterSeq: before.LastSeq})
	require.NoError(t, err)
	require.Len(t, page, 1, "exactly one reminder message")
	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, int64(1), head.Session.Reminders["status-update"].Count)
}

// testSetSessionRemindRefusals pins the refused reminder posts, and the
// Append of a reminder marker or with Remind: nothing is stored.
func testSetSessionRemindRefusals(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	before := contained(t, h.Store, k, dueAt(T0.Add(time.Hour)))
	at := T0.Add(time.Hour)
	good := channels.ReminderRecord{Since: T0, Last: at, Count: 1}
	post := func(edit func(*channels.ReminderPost)) func() error {
		return func() error {
			p := reminderPost(good)
			edit(&p)
			step := remindStep(func(channels.Channel) (channels.ReminderPost, bool) { return p, true }, dueAt(at))
			_, err := h.Store.SetSession(ctx, k, at, step)
			return err
		}
	}
	cases := []struct {
		name string
		want error
		call func() error
	}{
		{"triggered_by without marker", channels.ErrInvalidMessage,
			post(func(p *channels.ReminderPost) { p.TriggeredBy = "status-update" })},
		{"invalid type name", channels.ErrInvalidMessage,
			post(func(p *channels.ReminderPost) { p.Type = "Bad Type" })},
		{"invalid reminder name", channels.ErrInvalidMessage,
			post(func(p *channels.ReminderPost) { p.Name = "Bad Name" })},
		{"negative count", channels.ErrInvalidMessage,
			post(func(p *channels.ReminderPost) { p.Record.Count = -1 })},
		{"last after at", channels.ErrInvalidMessage,
			post(func(p *channels.ReminderPost) { p.Record.Last = at.Add(time.Second) })},
		{"props over the cap", channels.ErrTooLarge, post(func(p *channels.ReminderPost) {
			p.Props = map[string]any{"body": strings.Repeat("x", channels.MaxPropsBytes)}
		})},
		{"append with the marker", channels.ErrInvalidMessage, func() error {
			m := text("forged")
			m.TriggeredBy = "reminder:room/status-update"
			_, err := h.Store.Append(ctx, k, m)
			return err
		}},
		{"append step with Remind", channels.ErrInvalidMessage, func() error {
			step := sessionStep(stay)
			step.Remind = func(channels.Channel) (channels.ReminderPost, bool) { return reminderPost(good), true }
			_, err := h.Store.Append(ctx, k, stepped("posing", T0, step))
			return err
		}},
	}
	for _, tc := range cases {
		require.ErrorIs(t, tc.call(), tc.want, tc.name)
		head, err := h.Store.Head(ctx, k)
		require.NoError(t, err)
		require.Equal(t, before, head, "%s stored something", tc.name)
	}
}

// storedRecords are reminder records in another zone and with nanoseconds,
// so a store that does not normalize them fails [wantStored].
func storedRecords() map[string]channels.ReminderRecord {
	amsterdam := time.FixedZone("CEST", 2*60*60)
	since := T0.Add(123456789 * time.Nanosecond).In(amsterdam)
	return map[string]channels.ReminderRecord{
		"status-update": {Since: since, Last: since.Add(30*time.Minute + time.Nanosecond), Count: 2},
		"other":         {Since: since, Count: 0},
	}
}

// wantStored fails unless got holds the records of [storedRecords],
// normalized.
func wantStored(t *testing.T, got map[string]channels.ReminderRecord) {
	t.Helper()
	rs := storedRecords()
	require.Len(t, got, len(rs))
	for name, r := range rs {
		g := got[name]
		n := channels.NormReminder(r)
		require.True(t, g.Since.Equal(n.Since), "%s since %v, want %v", name, g.Since, n.Since)
		require.True(t, g.Last.Equal(n.Last), "%s last %v, want %v", name, g.Last, n.Last)
		require.Equal(t, n.Count, g.Count, name)
	}
}

// testSessionRemindersStored pins that the reminder records round-trip
// through every write that stores a schedule, normalized on every backend,
// and that a nil Reminders keeps them.
func testSessionRemindersStored(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	rs := storedRecords()

	k := key("TKT-1", "room")
	head := contained(t, h.Store, k, withReminders(T0.Add(time.Hour), rs))
	wantStored(t, head.Session.Reminders)

	keepStep := sessionStep(stay)
	keepStep.Due = dueAt(T0.Add(2 * time.Hour))
	mustAppend(t, h.Store, k, stepped("keeps", T0, keepStep))
	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	wantStored(t, head.Session.Reminders)

	c, err := h.Store.CreateMany(ctx, channels.Anchor{ID: "TKT-1"}, channels.NewChannel{
		Type: "topic", By: Alice, At: T0, SessionDue: withReminders(T0.Add(time.Hour), rs),
	})
	require.NoError(t, err)
	wantStored(t, c.Session.Reminders)
	got, err := h.Store.Head(ctx, c.Key)
	require.NoError(t, err)
	wantStored(t, got.Session.Reminders)

	_, err = h.Store.SetLifecycle(ctx, k, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: T0.Add(time.Hour)})
	require.NoError(t, err)
	_, err = h.Store.SetLifecycle(ctx, k, channels.StateClosed, channels.Transition{
		To: channels.StateOpen, By: Bob, At: T0.Add(2 * time.Hour),
		SessionDue: withReminders(T0.Add(3*time.Hour), map[string]channels.ReminderRecord{}),
	})
	require.NoError(t, err)
	head, err = h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Nil(t, head.Session.Reminders, "an empty map stores none")
	reopened, err := h.Store.SetLifecycle(ctx, k, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: T0.Add(3 * time.Hour)})
	require.NoError(t, err)
	require.Nil(t, reopened.Session.Reminders)
	_, err = h.Store.SetLifecycle(ctx, k, channels.StateClosed, channels.Transition{
		To: channels.StateOpen, By: Bob, At: T0.Add(4 * time.Hour), SessionDue: withReminders(T0.Add(5*time.Hour), rs),
	})
	require.NoError(t, err)
	head, err = h.Store.Head(ctx, k)
	require.NoError(t, err)
	wantStored(t, head.Session.Reminders)

	th := tombstone(t, h.Store, "TKT-1", "", T0.Add(5*time.Hour))
	require.NoError(t, h.Store.Untombstone(ctx, th, T0.Add(6*time.Hour), func(
		c channels.Channel,
	) channels.SessionSchedule {
		if c.Key != k {
			return channels.SessionSchedule{NextDueAt: c.Session.NextDueAt}
		}
		return channels.SessionSchedule{NextDueAt: T0.Add(7 * time.Hour), Reminders: rs}
	}))
	head, err = h.Store.Head(ctx, k)
	require.NoError(t, err)
	wantStored(t, head.Session.Reminders)
}

// testSessionRemindersMoved pins that a move clears the reminder records
// unless Due returns them, and that invalid records are refused.
func testSessionRemindersMoved(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	rs := storedRecords()
	k := key("TKT-1", "room")
	contained(t, h.Store, k, withReminders(T0.Add(time.Hour), rs))

	moveStep := sessionStep(moveFrom("contained", "assessed"))
	moveStep.Due = dueAt(T0.Add(8 * time.Hour))
	mustAppend(t, h.Store, k, stepped("move", T0.Add(7*time.Hour), moveStep))
	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Nil(t, head.Session.Reminders, "a move clears the records unless Due returns them")
	carry := sessionStep(moveFrom("assessed", "overdue"))
	carry.Due = withReminders(T0.Add(9*time.Hour), rs)
	mustAppend(t, h.Store, k, stepped("carry", T0.Add(8*time.Hour), carry))
	before, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	wantStored(t, before.Session.Reminders)

	nine := map[string]channels.ReminderRecord{}
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"} {
		nine[name] = channels.ReminderRecord{Since: T0}
	}
	for name, bad := range map[string]map[string]channels.ReminderRecord{
		"too many":       nine,
		"invalid name":   {"Bad Name": {Since: T0}},
		"negative count": {"status-update": {Since: T0, Count: -1}},
	} {
		step := sessionStep(stay)
		step.Due = withReminders(T0, bad)
		_, err := h.Store.Append(ctx, k, stepped(name, T0, step))
		require.ErrorIs(t, err, channels.ErrInvalidMessage, name)
		head, err := h.Store.Head(ctx, k)
		require.NoError(t, err)
		require.Equal(t, before, head, "%s stored something", name)
	}
}
