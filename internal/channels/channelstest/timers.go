package channelstest

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// timerTests are the session timer tests (TKT-INM3AZ), registered in
// [RunAll].
var timerTests = []conformanceTest{
	{"SessionDueOnAppend", testSessionDueOnAppend},
	{"SessionDueOnCreateMany", testSessionDueOnCreateMany},
	{"SessionResumeOnReopen", testSessionResumeOnReopen},
	{"SessionResumeOnUntombstone", testSessionResumeOnUntombstone},
	{"SetSessionFire", testSetSessionFire},
	{"SetSessionReschedule", testSetSessionReschedule},
	{"SetSessionCAS", testSetSessionCAS},
	{"SetSessionRefusals", testSetSessionRefusals},
	{"ListSessionsDue", testListSessionsDue},
	{"SessionScheduleSurvivesMoves", testSessionScheduleSurvivesMoves},
}

// dueAt returns a schedule function whose due time is at, keeping the
// stored restart anchors.
func dueAt(at time.Time) channels.ScheduleFunc {
	return func(channels.Channel) channels.SessionSchedule { return channels.SessionSchedule{NextDueAt: at} }
}

// withRestarts returns a schedule function that stores due and restarts.
func withRestarts(due time.Time, restarts map[string]time.Time) channels.ScheduleFunc {
	return func(channels.Channel) channels.SessionSchedule {
		return channels.SessionSchedule{NextDueAt: due, Restarts: restarts}
	}
}

// timerStep returns a SetSession step with the suite's notice fields, a
// timer name and due as its schedule.
func timerStep(
	decide func(channels.Channel) (channels.SessionMove, bool), timer string, due channels.ScheduleFunc,
) channels.SessionStep {
	return channels.SessionStep{
		Decide: decide, NoticeType: "session", NoticeBy: noticeAuthor, TriggeredBy: "session:room",
		AuditTriggeredBy: "timer:room/contained/" + timer, Timer: timer, Due: due,
		Outcome: &channels.SessionOutcome{},
	}
}

// stay is a decider that never moves.
func stay(channels.Channel) (channels.SessionMove, bool) { return channels.SessionMove{}, false }

// containedToOverdue is a decider that moves from contained at generation 1
// to overdue, and returns no move otherwise.
func containedToOverdue(c channels.Channel) (channels.SessionMove, bool) {
	if c.Session.State != "contained" || c.Session.Generation != 1 {
		return channels.SessionMove{}, false
	}
	return channels.SessionMove{From: "contained", To: "overdue"}, true
}

// contained appends a message whose step moves k's unset session to
// contained (generation 1) with due as its schedule, and returns the head.
func contained(t *testing.T, s channels.Store, k channels.Key, due channels.ScheduleFunc) channels.Channel {
	t.Helper()
	step := sessionStep(fromUnset())
	step.Due = due
	mustAppend(t, s, k, stepped("detected", T0, step))
	head, err := s.Head(context.Background(), k)
	require.NoError(t, err)
	require.Equal(t, "contained", head.Session.State)
	return head
}

// testSessionDueOnAppend pins the schedule of an append: the step's Due
// result is stored, a move clears the restart anchors, and a step without
// Due stores nothing.
func testSessionDueOnAppend(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	due := T0.Add(24 * time.Hour)
	head := contained(t, h.Store, k, dueAt(due))
	require.Equal(t, channels.NormTime(due), head.Session.NextDueAt)
	require.Nil(t, head.Session.Restarts)

	_, err := h.Store.SetLifecycle(ctx, k, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: T0.Add(time.Hour)})
	require.NoError(t, err)
	restart := T0.Add(3 * time.Hour)
	reopened, err := h.Store.SetLifecycle(ctx, k, channels.StateClosed, channels.Transition{
		To: channels.StateOpen, By: Bob, At: restart,
		SessionDue: withRestarts(due, map[string]time.Time{"notify-deadline": restart}),
	})
	require.NoError(t, err)
	require.Len(t, reopened.Session.Restarts, 1)

	stayStep := sessionStep(stay)
	stayStep.Due = dueAt(due.Add(time.Hour))
	mustAppend(t, h.Store, k, stepped("no move", T0, stayStep))
	head, err = h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, channels.NormTime(due.Add(time.Hour)), head.Session.NextDueAt, "no move still reschedules")
	require.Len(t, head.Session.Restarts, 1, "no move keeps the restart anchors")

	later := T0.Add(48 * time.Hour)
	moveStep := sessionStep(moveFrom("contained", "assessed"))
	moveStep.Due = dueAt(later)
	mustAppend(t, h.Store, k, stepped("move", T0.Add(4*time.Hour), moveStep))
	head, err = h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, "assessed", head.Session.State)
	require.Equal(t, channels.NormTime(later), head.Session.NextDueAt)
	require.Nil(t, head.Session.Restarts, "a move clears the restart anchors")

	before := head
	noDueStep := sessionStep(moveFrom("assessed", "notified"))
	noDueStep.Due = nil
	_, err = h.Store.Append(ctx, k, stepped("no due", T0, noDueStep))
	require.ErrorIs(t, err, channels.ErrInvalidMessage)
	head, err = h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, before, head, "a step without Due stores nothing")
}

// testSessionDueOnCreateMany pins that CreateMany stores its SessionDue
// result.
func testSessionDueOnCreateMany(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	due := T0.Add(time.Hour)
	c, err := h.Store.CreateMany(ctx, channels.Anchor{ID: "TKT-1"}, channels.NewChannel{
		Type: "topic", By: Alice, At: T0, SessionDue: dueAt(due),
	})
	require.NoError(t, err)
	require.Equal(t, channels.NormTime(due), c.Session.NextDueAt)
	head, err := h.Store.Head(ctx, c.Key)
	require.NoError(t, err)
	require.Equal(t, channels.NormTime(due), head.Session.NextDueAt)
}

// testSessionResumeOnReopen pins that a reopen with SessionDue stores its
// due time and restart anchors in the same step, and that a close without
// it leaves both as they were.
func testSessionResumeOnReopen(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	due := T0.Add(24 * time.Hour)
	contained(t, h.Store, k, dueAt(due))

	closed, err := h.Store.SetLifecycle(ctx, k, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: T0.Add(time.Hour)})
	require.NoError(t, err)
	require.Equal(t, channels.NormTime(due), closed.Session.NextDueAt, "a close keeps the due time")
	require.Nil(t, closed.Session.Restarts)

	at := T0.Add(30 * time.Hour)
	restarts := map[string]time.Time{"notify-deadline": at, "no-containment": at.Add(time.Nanosecond)}
	resumed := at.Add(72 * time.Hour)
	reopened, err := h.Store.SetLifecycle(ctx, k, channels.StateClosed, channels.Transition{
		To: channels.StateOpen, By: Bob, At: at, SessionDue: withRestarts(resumed, restarts),
	})
	require.NoError(t, err)
	want := map[string]time.Time{"notify-deadline": channels.NormTime(at), "no-containment": channels.NormTime(at)}
	require.Equal(t, channels.NormTime(resumed), reopened.Session.NextDueAt)
	require.Equal(t, want, reopened.Session.Restarts)
	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, reopened.Session, head.Session)

	closed, err = h.Store.SetLifecycle(ctx, k, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: at.Add(time.Hour)})
	require.NoError(t, err)
	require.Equal(t, reopened.Session, closed.Session, "a close without SessionDue keeps the schedule")
}

// testSessionResumeOnUntombstone pins the resume callback of Untombstone:
// called for each restored open channel and stored, not called for a
// closed one, and a nil callback changes nothing.
func testSessionResumeOnUntombstone(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	open, shut := key("TKT-1", "room"), key("TKT-1", "topic")
	due := T0.Add(24 * time.Hour)
	contained(t, h.Store, open, dueAt(due))
	contained(t, h.Store, shut, dueAt(due))
	_, err := h.Store.SetLifecycle(ctx, shut, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: T0.Add(time.Hour)})
	require.NoError(t, err)

	th := tombstone(t, h.Store, "TKT-1", "", T0.Add(2*time.Hour))
	at := T0.Add(48 * time.Hour)
	var mu sync.Mutex
	seen := map[channels.Key]int{}
	resume := func(c channels.Channel) channels.SessionSchedule {
		mu.Lock()
		defer mu.Unlock()
		seen[c.Key]++
		return channels.SessionSchedule{
			NextDueAt: at.Add(24 * time.Hour), Restarts: map[string]time.Time{"no-containment": at},
		}
	}
	require.NoError(t, h.Store.Untombstone(ctx, th, at, resume))
	require.Positive(t, seen[open], "resume runs for the restored open channel")
	require.Zero(t, seen[shut], "resume does not run for a closed channel")
	require.Len(t, seen, 1)

	head, err := h.Store.Head(ctx, open)
	require.NoError(t, err)
	require.Equal(t, channels.NormTime(at.Add(24*time.Hour)), head.Session.NextDueAt)
	require.Equal(t, map[string]time.Time{"no-containment": channels.NormTime(at)}, head.Session.Restarts)
	head, err = h.Store.Head(ctx, shut)
	require.NoError(t, err)
	require.Equal(t, channels.NormTime(due), head.Session.NextDueAt)
	require.Nil(t, head.Session.Restarts)

	before, err := h.Store.Head(ctx, open)
	require.NoError(t, err)
	th = tombstone(t, h.Store, "TKT-1", "", at.Add(time.Hour))
	require.NoError(t, h.Store.Untombstone(ctx, th, at.Add(2*time.Hour), nil))
	head, err = h.Store.Head(ctx, open)
	require.NoError(t, err)
	require.Equal(t, before.Session, head.Session, "a nil resume changes nothing")
}

// testSetSessionFire pins a timer move: one notice at last_seq + 1, the
// session moved, restart anchors cleared, the Due result stored, the rev
// raised and one audit record, with the returned channel equal to Head.
func testSetSessionFire(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	contained(t, h.Store, k, dueAt(T0.Add(72*time.Hour)))
	_, err := h.Store.SetLifecycle(ctx, k, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: T0.Add(time.Hour)})
	require.NoError(t, err)
	_, err = h.Store.SetLifecycle(ctx, k, channels.StateClosed, channels.Transition{
		To: channels.StateOpen, By: Bob, At: T0.Add(2 * time.Hour),
		SessionDue: withRestarts(T0.Add(74*time.Hour), map[string]time.Time{"notify-deadline": T0.Add(2 * time.Hour)}),
	})
	require.NoError(t, err)
	before, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	before.Session.Restarts = nil
	trailBefore := sessionRecords(t, h.Store)

	at := T0.Add(74 * time.Hour)
	next := at.Add(time.Hour)
	step := timerStep(containedToOverdue, "notify-deadline", dueAt(next))
	got, err := h.Store.SetSession(ctx, k, at, step)
	require.NoError(t, err)

	want := channels.Session{
		State: "overdue", Generation: 2, EnteredAt: channels.NormTime(at), NextDueAt: channels.NormTime(next),
	}
	require.Equal(t, want, got.Session)
	require.Equal(t, before.LastSeq+1, got.LastSeq)
	require.Greater(t, got.Rev, before.Rev)
	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, head, got, "the returned channel and Head agree")
	require.Equal(t, channels.SessionOutcome{
		Session: want, CreatedAt: head.CreatedAt, Moved: true,
		Move: channels.SessionMove{From: "contained", To: "overdue"}, Notice: got.LastSeq,
	}, *step.Outcome)

	page, err := h.Store.Messages(ctx, k, channels.Page{AfterSeq: before.LastSeq})
	require.NoError(t, err)
	require.Len(t, page, 1)
	notice := page[0]
	require.Equal(t, got.LastSeq, notice.Seq)
	require.Equal(t, got.Rev, notice.Rev)
	require.Equal(t, "session", notice.Type)
	require.Equal(t, noticeAuthor, notice.Author)
	require.Equal(t, "session:room", notice.TriggeredBy)
	require.Equal(t, channels.NormTime(at), notice.At)
	require.Equal(t, map[string]any{
		"from": "contained", "to": "overdue", "generation": float64(2), "timer": "notify-deadline",
	}, notice.Props)

	recs := sessionRecords(t, h.Store)
	require.Len(t, recs, len(trailBefore)+1)
	rec := recs[len(recs)-1]
	require.Equal(t, "contained", rec.From)
	require.Equal(t, "overdue", rec.To)
	require.Equal(t, int64(2), rec.Generation)
	require.Equal(t, notice.Seq, rec.Seq)
	require.Equal(t, noticeAuthor, rec.By)
	require.Equal(t, "timer:room/contained/notify-deadline", rec.TriggeredBy)
	require.Equal(t, "room", rec.Type)
}

// testSetSessionReschedule pins a SetSession whose decider returns no
// move: no message, no audit record, the Due result stored, the rev kept.
func testSetSessionReschedule(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	before := contained(t, h.Store, k, dueAt(T0.Add(time.Hour)))
	trail := sessionRecords(t, h.Store)

	next := T0.Add(5 * time.Hour)
	step := timerStep(stay, "", dueAt(next))
	got, err := h.Store.SetSession(ctx, k, T0.Add(2*time.Hour), step)
	require.NoError(t, err)
	require.Equal(t, channels.NormTime(next), got.Session.NextDueAt)
	require.Equal(t, before.Rev, got.Rev)
	require.Equal(t, before.LastSeq, got.LastSeq)
	require.False(t, step.Outcome.Moved)

	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, got, head)
	require.Equal(t, trail, sessionRecords(t, h.Store))
	page, err := h.Store.Messages(ctx, k, channels.Page{AfterSeq: before.LastSeq})
	require.NoError(t, err)
	require.Empty(t, page)

	cleared, err := h.Store.SetSession(ctx, k, T0.Add(3*time.Hour), timerStep(stay, "", dueAt(time.Time{})))
	require.NoError(t, err)
	require.True(t, cleared.Session.NextDueAt.IsZero(), "a zero Due result clears the due time")
}

// testSetSessionCAS pins that concurrent SetSession calls whose deciders
// move only from (contained, 1) move the session once.
func testSetSessionCAS(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	stores := []channels.Store{h.Store}
	if h.Second != nil {
		stores = append(stores, h.Second(t))
	}
	k := key("TKT-1", "room")
	before := contained(t, h.Store, k, dueAt(T0.Add(time.Hour)))
	const n = 8
	var wg sync.WaitGroup
	errs := make(chan error, n)
	moved := make(chan bool, n)
	for i := range n {
		wg.Go(func() {
			step := timerStep(containedToOverdue, "notify-deadline", dueAt(time.Time{}))
			_, err := stores[i%len(stores)].SetSession(ctx, k, T0.Add(2*time.Hour), step)
			errs <- err
			moved <- err == nil && step.Outcome.Moved
		})
	}
	wg.Wait()
	close(errs)
	close(moved)
	for err := range errs {
		require.NoError(t, err)
	}
	count := 0
	for m := range moved {
		if m {
			count++
		}
	}
	require.Equal(t, 1, count, "exactly one call moves the session")
	page, err := h.Store.Messages(ctx, k, channels.Page{AfterSeq: before.LastSeq})
	require.NoError(t, err)
	require.Len(t, page, 1, "exactly one notice")
	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, int64(2), head.Session.Generation)
}

// testSetSessionRefusals pins the refused SetSession calls and the Append
// that names a timer: nothing is stored in any of them.
func testSetSessionRefusals(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	missing := key("TKT-9", "room")
	_, err := h.Store.SetSession(ctx, missing, T0,
		timerStep(containedToOverdue, "late", dueAt(T0)))
	require.ErrorIs(t, err, channels.ErrNotFound)
	_, err = h.Store.Head(ctx, missing)
	require.ErrorIs(t, err, channels.ErrNotFound, "a refused SetSession created the channel")

	k := key("TKT-1", "room")
	before := contained(t, h.Store, k, dueAt(T0.Add(time.Hour)))
	trail := sessionRecords(t, h.Store)
	cases := []struct {
		name string
		call func() error
	}{
		{"move without timer", func() error {
			_, err := h.Store.SetSession(ctx, k, T0, timerStep(moveFrom("contained", "overdue"), "", dueAt(T0)))
			return err
		}},
		{"invalid state name", func() error {
			_, err := h.Store.SetSession(ctx, k, T0, timerStep(moveFrom("contained", "Bad State"), "late", dueAt(T0)))
			return err
		}},
		{"no schedule", func() error {
			_, err := h.Store.SetSession(ctx, k, T0, timerStep(moveFrom("contained", "overdue"), "late", nil))
			return err
		}},
		{"append naming a timer", func() error {
			step := sessionStep(moveFrom("contained", "overdue"))
			step.Timer = "late"
			_, err := h.Store.Append(ctx, k, stepped("posing", T0, step))
			return err
		}},
	}
	for _, tc := range cases {
		require.ErrorIs(t, tc.call(), channels.ErrInvalidMessage, tc.name)
		head, err := h.Store.Head(ctx, k)
		require.NoError(t, err)
		require.Equal(t, before, head, "%s stored something", tc.name)
		require.Equal(t, trail, sessionRecords(t, h.Store), tc.name)
	}
}

// testListSessionsDue pins the due listing: the live, open channels of
// the given types with a due time at or before until, ordered by due time
// and then key, at most limit.
func testListSessionsDue(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	s := h.Store
	due := T0.Add(time.Hour)
	// Ties on due: B-1 before B-2, and on B-2 room before topic.
	tieB2Topic := key("B-2", "topic")
	tieB1 := key("B-1", "room")
	tieB2Room := key("B-2", "room")
	early := key("Z-1", "room")
	for _, k := range []channels.Key{tieB2Topic, tieB1, tieB2Room} {
		contained(t, s, k, dueAt(due))
	}
	contained(t, s, early, dueAt(T0.Add(time.Minute)))

	contained(t, s, key("C-1", "other"), dueAt(due))
	contained(t, s, key("C-2", "room"), dueAt(due.Add(time.Hour)))
	mustAppend(t, s, key("C-3", "room"), text("no session"))
	contained(t, s, key("C-4", "room"), dueAt(time.Time{}))
	shut := key("C-5", "room")
	contained(t, s, shut, dueAt(due))
	_, err := s.SetLifecycle(ctx, shut, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: T0})
	require.NoError(t, err)
	archived := key("C-6", "room")
	contained(t, s, archived, dueAt(due))
	_, err = s.SetLifecycle(ctx, archived, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: T0})
	require.NoError(t, err)
	_, err = s.SetLifecycle(ctx, archived, channels.StateClosed,
		channels.Transition{To: channels.StateArchived, By: Bob, At: T0})
	require.NoError(t, err)
	contained(t, s, key("C-7", "room"), dueAt(due))
	tombstone(t, s, "C-7", "", T0)

	got, err := s.ListSessionsDue(ctx, []string{"room", "topic"}, due, 10)
	require.NoError(t, err)
	keys := make([]channels.Key, len(got))
	for i, c := range got {
		keys[i] = c.Key
		require.Equal(t, channels.StateOpen, c.State)
	}
	require.Equal(t, []channels.Key{early, tieB1, tieB2Room, tieB2Topic}, keys)
	require.Equal(t, channels.NormTime(due), got[1].Session.NextDueAt)

	capped, err := s.ListSessionsDue(ctx, []string{"room", "topic"}, due, 2)
	require.NoError(t, err)
	require.Len(t, capped, 2)
	require.Equal(t, []channels.Key{early, tieB1}, []channels.Key{capped[0].Key, capped[1].Key})

	none, err := s.ListSessionsDue(ctx, []string{"room"}, T0, 10)
	require.NoError(t, err)
	require.Empty(t, none)

	_, err = s.ListSessionsDue(ctx, nil, due, 10)
	require.ErrorIs(t, err, channels.ErrInvalidKey)
	_, err = s.ListSessionsDue(ctx, []string{"Bad Type"}, due, 10)
	require.ErrorIs(t, err, channels.ErrInvalidKey)
	_, err = s.ListSessionsDue(ctx, []string{"room"}, due, 0)
	require.ErrorIs(t, err, channels.ErrInvalidMessage)
}

// testSessionScheduleSurvivesMoves pins that the due time, the restart
// anchors and the reminder records are kept across a close, a rekey and a
// tombstone.
func testSessionScheduleSurvivesMoves(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("OLD-1", "room")
	contained(t, h.Store, k, dueAt(T0.Add(24*time.Hour)))
	_, err := h.Store.SetLifecycle(ctx, k, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: T0.Add(time.Minute)})
	require.NoError(t, err)
	_, err = h.Store.SetLifecycle(ctx, k, channels.StateClosed, channels.Transition{
		To: channels.StateOpen, By: Bob, At: T0.Add(2 * time.Minute),
		SessionDue: func(channels.Channel) channels.SessionSchedule {
			return channels.SessionSchedule{
				NextDueAt: T0.Add(48 * time.Hour), Restarts: map[string]time.Time{"late": T0.Add(2 * time.Minute)},
				Reminders: map[string]channels.ReminderRecord{
					"status-update": {Since: T0, Last: T0.Add(time.Minute), Count: 3},
				},
			}
		},
	})
	require.NoError(t, err)
	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	want := head.Session
	require.Len(t, want.Restarts, 1)
	require.Len(t, want.Reminders, 1)

	c, err := h.Store.SetLifecycle(ctx, k, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: T0.Add(3 * time.Minute)})
	require.NoError(t, err)
	require.Equal(t, want, c.Session)

	require.NoError(t, h.Store.Rekey(ctx, "OLD-1", "NEW-1", T0.Add(4*time.Minute)))
	got, err := h.Store.ListEntity(ctx, "NEW-1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, want, got[0].Session)

	th := tombstone(t, h.Store, "NEW-1", "", T0.Add(5*time.Minute))
	require.NoError(t, h.Store.Untombstone(ctx, th, T0.Add(6*time.Minute), nil))
	got, err = h.Store.ListEntity(ctx, "NEW-1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, want, got[0].Session)
}

// RunDamagedDueRow pins that [channels.Store.ListSessionsDue] skips a
// due channel whose stored row cannot be decoded, and still returns the
// others: one damaged row must not stop the timers of every channel.
// corrupt damages the stored row of the channel at k in a way only the
// backend can, for example invalid JSON in a column.
func RunDamagedDueRow(t *testing.T, s channels.Store, corrupt func(t *testing.T, k channels.Key)) {
	t.Helper()
	ctx := context.Background()
	due := T0.Add(time.Hour)
	bad, good := key("TKT-1", "room"), key("TKT-2", "room")
	contained(t, s, bad, dueAt(due))
	contained(t, s, good, dueAt(due))
	corrupt(t, bad)
	got, err := s.ListSessionsDue(ctx, []string{"room"}, due, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, good, got[0].Key)
}
