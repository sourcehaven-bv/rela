package channelstest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// causeTests are the session cause tests (TKT-V05706X), registered in
// [RunAll].
var causeTests = []conformanceTest{
	{"SessionCause", testSessionCause},
	{"SessionCauseKept", testSessionCauseKept},
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

// testSessionCauseKept pins that only a move changes the cause: a post
// that does not move, a reminder post, close, reopen, rekey, tombstone and
// untombstone with a resume schedule keep it.
func testSessionCauseKept(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("OLD-1", "room")
	msg := mustAppend(t, h.Store, k, stepped("move", T0, sessionStep(fromUnset())))
	want := msg.Seq

	stayStep := sessionStep(stay)
	stayStep.Due = dueAt(T0.Add(time.Hour))
	mustAppend(t, h.Store, k, stepped("no move", T0, stayStep))
	requireCause(t, h.Store, k, want, "a post that does not move keeps it")

	post := reminderPost(channels.ReminderRecord{Since: T0, Last: T0.Add(time.Minute), Count: 1})
	_, err := h.Store.SetSession(ctx, k, T0.Add(time.Minute), remindStep(
		func(channels.Channel) (channels.ReminderPost, bool) { return post, true }, dueAt(T0.Add(time.Hour))))
	require.NoError(t, err)
	requireCause(t, h.Store, k, want, "a reminder post keeps it")

	_, err = h.Store.SetLifecycle(ctx, k, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: T0.Add(2 * time.Minute)})
	require.NoError(t, err)
	requireCause(t, h.Store, k, want, "close keeps it")
	_, err = h.Store.SetLifecycle(ctx, k, channels.StateClosed, channels.Transition{
		To: channels.StateOpen, By: Bob, At: T0.Add(3 * time.Minute), SessionDue: dueAt(T0.Add(time.Hour)),
	})
	require.NoError(t, err)
	requireCause(t, h.Store, k, want, "reopen keeps it")

	require.NoError(t, h.Store.Rekey(ctx, "OLD-1", "NEW-1", T0.Add(4*time.Minute)))
	k = key("NEW-1", "room")
	requireCause(t, h.Store, k, want, "rekey keeps it")

	th := tombstone(t, h.Store, "NEW-1", "", T0.Add(5*time.Minute))
	require.NoError(t, h.Store.Untombstone(ctx, th, T0.Add(6*time.Minute), dueAt(T0.Add(2*time.Hour))))
	requireCause(t, h.Store, k, want, "tombstone and untombstone keep it")
}
