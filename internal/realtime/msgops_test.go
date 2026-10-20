package realtime

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// TestMessageActions_EditWindowPending pins that a message offers edit and
// delete to its author while edit_own and delete_own admit them, remove to
// a moderator, and that the edit window, a closed channel and a pending
// action take the offers away as the operations refuse them.
func TestMessageActions_EditWindowPending(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.post(t, user("bob"), text("hi"))

	require.Equal(t, []string{OpDelete, OpEdit}, h.offeredOn(t, user("bob"), room, m.Seq))
	require.Nil(t, h.offeredOn(t, user("cat"), room, m.Seq), "another member may not edit or delete it")
	require.Equal(t, []string{OpRemove}, h.offeredOn(t, user("mo", "chat.moderate"), room, m.Seq))
	require.Nil(t, h.offeredOn(t, readOnlyClient(user("bob")), room, m.Seq),
		"a read-only client passes no gate")

	h.clock.Add(16 * time.Minute)
	require.Equal(t, []string{OpDelete}, h.offeredOn(t, user("bob"), room, m.Seq), "the edit window closed")
	_, err := h.svc.Edit(as(user("bob")), room, m.Seq, map[string]any{"body": "late"})
	requireDenial(t, err, http.StatusForbidden, TypeEditWindowClosed)

	fresh := h.post(t, user("bob"), text("again"))
	_, err = h.store.SetLifecycle(context.Background(), roomKey(), channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: channels.Author{User: "x"}, At: h.clock.Now()})
	require.NoError(t, err)
	require.Nil(t, h.offeredOn(t, user("bob"), room, fresh.Seq), "delete_own is from: open, edit needs open")
	require.Equal(t, []string{OpRemove}, h.offeredOn(t, user("mo", "chat.moderate"), room, fresh.Seq))
}

// TestMessageActions_Pending pins that a pending action offers only
// release, and only once the operation would accept it.
func TestMessageActions_Pending(t *testing.T) {
	t.Parallel()
	h := newActionHarness(t, &fakePromoter{err: errors.New("crash")})
	m := h.post(t, user("ann"), proposal("x"))
	require.Equal(t, []string{"ack", OpEdit}, h.offeredOn(t, user("ann"), room, m.Seq))
	_, err := h.svc.RunAction(as(decider()), room, m.Seq, "accept")
	require.Error(t, err)

	require.Nil(t, h.offeredOn(t, user("ann"), room, m.Seq), "a pending message cannot be edited")
	require.Nil(t, h.offeredOn(t, moderator(), room, m.Seq), "release waits for PendingRelease")
	h.clock.Add(realtimelimits.PendingRelease + time.Second)
	require.Equal(t, []string{OpRelease}, h.offeredOn(t, moderator(), room, m.Seq))
	require.Nil(t, h.offeredOn(t, decider(), room, m.Seq), "the remove gate releases in this type")
}

// TestMessageActions_NoEditWithRestrictedRef pins that edit is not offered
// on a message holding a ref the reader cannot read: the edit replaces
// every property, and the reader cannot send that one back.
func TestMessageActions_NoEditWithRestrictedRef(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.refs.views["TKT-3"] = RefView{ID: "TKT-3", Type: "ticket", Title: "Hidden later"}
	m := h.post(t, user("bob"), PostInput{Type: "note", Props: map[string]any{"level": 1, "about": "TKT-3"}})
	require.Contains(t, h.offeredOn(t, user("bob"), room, m.Seq), OpEdit)

	delete(h.refs.views, "TKT-3")
	require.Equal(t, []string{OpDelete}, h.offeredOn(t, user("bob"), room, m.Seq))
}

// TestMessageActions_GatesOncePerPage pins that the operation gates are
// asked once per read, however many messages the page holds.
func TestMessageActions_GatesOncePerPage(t *testing.T) {
	t.Parallel()
	count := func(n int) int64 {
		h := newHarness(t)
		for range n {
			h.post(t, user("bob"), text("x"))
		}
		bob := user("bob", "chat.moderate")
		_, err := h.svc.Read(as(bob), room, ReadRequest{})
		require.NoError(t, err)
		bob.permCalls.Store(0)
		page, err := h.svc.Read(as(bob), room, ReadRequest{})
		require.NoError(t, err)
		require.Len(t, page.Messages, n)
		return bob.permCalls.Load()
	}
	require.Equal(t, count(2), count(20))
}
