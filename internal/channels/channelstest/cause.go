package channelstest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// causeTests are the session cause (TKT-V05706X) and previous state
// (TKT-56TPOTE) tests, registered in [RunAll]. Both are facts of the last
// move, so one test pins that nothing else changes them.
var causeTests = []conformanceTest{
	{"SessionCause", testSessionCause},
	{"SessionPrevious", testSessionPrevious},
	{"SessionMoveFactsKept", testSessionMoveFactsKept},
}

// requireCause asserts k's stored session cause on Head and List.
func requireCause(t *testing.T, s channels.Store, k channels.Key, want int64, msg string) {
	t.Helper()
	ctx := context.Background()
	head, err := s.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, want, head.Session.Cause, msg)
	list, err := s.List(ctx, k.Anchor, k.Type, "")
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, want, list[0].Session.Cause, msg+" (list)")
}

// testSessionCause pins [channels.Session.Cause] (AC 1): a post that moves
// the session stores its own seq, which equals the notice's cause prop; a
// later move by post replaces it; a timer move stores 0.
func testSessionCause(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	mustAppend(t, h.Store, k, text("before"))
	msg := mustAppend(t, h.Store, k, stepped("move", T0, sessionStep(fromUnset())))
	requireCause(t, h.Store, k, msg.Seq, "a post that moves stores its seq")
	notice, err := h.Store.Get(ctx, k, msg.Seq+1)
	require.NoError(t, err)
	require.EqualValues(t, msg.Seq, notice.Props["cause"], "the notice names the same cause")

	again := mustAppend(t, h.Store, k, stepped("again", T0.Add(time.Minute),
		sessionStep(moveFrom("contained", "overdue"))))
	requireCause(t, h.Store, k, again.Seq, "the next move replaces it")

	got, err := h.Store.SetSession(ctx, k, T0.Add(time.Hour),
		timerStep(moveFrom("overdue", "late"), "late", dueAt(time.Time{})))
	require.NoError(t, err)
	require.Zero(t, got.Session.Cause, "a timer move has no cause")
	requireCause(t, h.Store, k, 0, "a timer move stores 0")
}

// requirePrevious asserts k's stored previous state on Head and List.
func requirePrevious(t *testing.T, s channels.Store, k channels.Key, want, msg string) {
	t.Helper()
	ctx := context.Background()
	head, err := s.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, want, head.Session.Previous, msg)
	list, err := s.List(ctx, k.Anchor, k.Type, "")
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, want, list[0].Session.Previous, msg+" (list)")
}

// requireMoveFacts asserts k's stored cause and previous state.
func requireMoveFacts(t *testing.T, s channels.Store, k channels.Key, cause int64, previous, msg string) {
	t.Helper()
	requireCause(t, s, k, cause, msg)
	requirePrevious(t, s, k, previous, msg)
}

// testSessionPrevious pins [channels.Session.Previous]: a session that
// never moved has none, the first move stores the initial state, a re-entry
// keeps it, and a later move by post or timer stores the state it left.
func testSessionPrevious(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	mustAppend(t, h.Store, k, text("before"))
	requirePrevious(t, h.Store, k, "", "a session that never moved has none")
	mustAppend(t, h.Store, k, stepped("move", T0, sessionStep(fromUnset())))
	requirePrevious(t, h.Store, k, "detected", "the first move stores the initial state")
	mustAppend(t, h.Store, k, stepped("again", T0.Add(time.Minute),
		sessionStep(moveFrom("contained", "contained"))))
	requirePrevious(t, h.Store, k, "detected", "a re-entry keeps it")
	mustAppend(t, h.Store, k, stepped("on", T0.Add(2*time.Minute),
		sessionStep(moveFrom("contained", "overdue"))))
	requirePrevious(t, h.Store, k, "contained", "a move by post stores the state it left")
	_, err := h.Store.SetSession(ctx, k, T0.Add(time.Hour),
		timerStep(moveFrom("overdue", "late"), "late", dueAt(time.Time{})))
	require.NoError(t, err)
	requirePrevious(t, h.Store, k, "overdue", "a timer move stores it too")
}

// testSessionMoveFactsKept pins that only a move changes the cause and the
// previous state: a post that does not move, a reminder post, close,
// reopen, rekey, tombstone and untombstone with a resume schedule keep them.
func testSessionMoveFactsKept(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("OLD-1", "room")
	msg := mustAppend(t, h.Store, k, stepped("move", T0, sessionStep(fromUnset())))
	want, previous := msg.Seq, "detected"

	stayStep := sessionStep(stay)
	stayStep.Due = dueAt(T0.Add(time.Hour))
	mustAppend(t, h.Store, k, stepped("no move", T0, stayStep))
	requireMoveFacts(t, h.Store, k, want, previous, "a post that does not move keeps them")

	post := reminderPost(channels.ReminderRecord{Since: T0, Last: T0.Add(time.Minute), Count: 1})
	_, err := h.Store.SetSession(ctx, k, T0.Add(time.Minute), remindStep(
		func(channels.Channel) (channels.ReminderPost, bool) { return post, true }, dueAt(T0.Add(time.Hour))))
	require.NoError(t, err)
	requireMoveFacts(t, h.Store, k, want, previous, "a reminder post keeps them")

	_, err = h.Store.SetLifecycle(ctx, k, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: T0.Add(2 * time.Minute)})
	require.NoError(t, err)
	requireMoveFacts(t, h.Store, k, want, previous, "close keeps them")
	_, err = h.Store.SetLifecycle(ctx, k, channels.StateClosed, channels.Transition{
		To: channels.StateOpen, By: Bob, At: T0.Add(3 * time.Minute), SessionDue: dueAt(T0.Add(time.Hour)),
	})
	require.NoError(t, err)
	requireMoveFacts(t, h.Store, k, want, previous, "reopen keeps them")

	require.NoError(t, h.Store.Rekey(ctx, "OLD-1", "NEW-1", T0.Add(4*time.Minute)))
	k = key("NEW-1", "room")
	requireMoveFacts(t, h.Store, k, want, previous, "rekey keeps them")

	th := tombstone(t, h.Store, "NEW-1", "", T0.Add(5*time.Minute))
	require.NoError(t, h.Store.Untombstone(ctx, th, T0.Add(6*time.Minute), dueAt(T0.Add(2*time.Hour))))
	requireMoveFacts(t, h.Store, k, want, previous, "tombstone and untombstone keep them")
}
