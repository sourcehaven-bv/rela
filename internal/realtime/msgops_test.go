package realtime

import (
	"context"
	"errors"
	"net/http"
	"slices"
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

// TestEdit_KeepsUnreadableRef pins that edit stays offered on a message
// holding a ref the reader cannot read, and that an edit which does not
// name that property keeps the stored id rather than erasing it. Naming
// the property replaces it, and naming an unreadable id is refused as on a
// post.
func TestEdit_KeepsUnreadableRef(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.refs.views["TKT-3"] = RefView{ID: "TKT-3", Type: "ticket", Title: "Hidden later"}
	m := h.post(t, user("bob"), PostInput{Type: "note", Props: map[string]any{"level": 1, "about": "TKT-3"}})
	delete(h.refs.views, "TKT-3")
	require.Equal(t, []string{OpDelete, OpEdit}, h.offeredOn(t, user("bob"), room, m.Seq))

	got, err := h.svc.Edit(as(user("bob")), room, m.Seq, map[string]any{"level": 2})
	require.NoError(t, err)
	require.Equal(t, &RefValue{Restricted: true}, got.Props["about"])
	stored, err := h.store.Get(context.Background(), roomKey(), m.Seq)
	require.NoError(t, err)
	require.Equal(t, "TKT-3", stored.Props["about"], "the unreadable ref is kept")
	require.EqualValues(t, 2, stored.Props["level"])

	_, err = h.svc.Edit(as(user("bob")), room, m.Seq, map[string]any{"level": 3, "about": "TKT-3"})
	requireDenial(t, err, http.StatusUnprocessableEntity, TypeInvalidRef)

	h.refs.views["TKT-4"] = RefView{ID: "TKT-4", Type: "ticket", Title: "Readable"}
	_, err = h.svc.Edit(as(user("bob")), room, m.Seq, map[string]any{"level": 3, "about": "TKT-4"})
	require.NoError(t, err)
	stored, err = h.store.Get(context.Background(), roomKey(), m.Seq)
	require.NoError(t, err)
	require.Equal(t, "TKT-4", stored.Props["about"], "a named ref replaces the stored one")
}

// TestMessageActions_Deadlines pins edit_until and release_after: the times
// at which a message's operations change without the message changing.
func TestMessageActions_Deadlines(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.post(t, user("bob"), text("hi"))
	view := func(a *fakeAccess, seq int64) MessageView {
		t.Helper()
		page, err := h.svc.Read(as(a), room, ReadRequest{})
		require.NoError(t, err)
		for _, v := range page.Messages {
			if v.Seq == seq {
				return v
			}
		}
		t.Fatalf("message %d not on the page", seq)
		return MessageView{}
	}
	require.Equal(t, m.At.Add(15*time.Minute), view(user("bob"), m.Seq).EditUntil)
	require.True(t, view(user("cat"), m.Seq).EditUntil.IsZero(), "edit is not offered to another member")

	pendingAt := h.clock.Now()
	_, err := h.store.SetAction(context.Background(), roomKey(), m.Seq, "x", channels.ActionNone,
		channels.Action{Status: channels.ActionPending, By: channels.Author{User: "x"}, At: pendingAt, PendingAt: pendingAt})
	require.NoError(t, err)
	require.True(t, view(user("bob"), m.Seq).EditUntil.IsZero(), "a pending message cannot be edited")
	require.Equal(t, pendingAt.Add(realtimelimits.PendingRelease), view(moderator(), m.Seq).ReleaseAfter)
	require.True(t, view(user("bob"), m.Seq).ReleaseAfter.IsZero(), "bob may never release it")
	h.clock.Add(realtimelimits.PendingRelease)
	v := view(moderator(), m.Seq)
	require.True(t, v.ReleaseAfter.IsZero())
	require.Contains(t, v.Actions, OpRelease)
}

// TestMessageActions_AffordanceAndOperationAgree pins that for every
// caller and message state each message operation is offered exactly when
// running it succeeds.
func TestMessageActions_AffordanceAndOperationAgree(t *testing.T) {
	t.Parallel()
	callers := map[string]func() *fakeAccess{
		"author": func() *fakeAccess { return user("bob") }, "member": func() *fakeAccess { return user("cat") },
		"moderator": moderator, "moderating author": func() *fakeAccess { return user("bob", "chat.moderate") },
		"read-only author": func() *fakeAccess { return readOnlyClient(user("bob")) },
	}
	states := []string{"fresh", "late", "closed", "pending", "pending long"}
	ops := []string{OpEdit, OpDelete, OpRemove, OpRelease}
	for cname, caller := range callers {
		for _, state := range states {
			for _, op := range ops {
				t.Run(cname+"/"+state+"/"+op, func(t *testing.T) {
					t.Parallel()
					h := newHarness(t)
					m := h.post(t, user("bob"), text("hi"))
					ctx := context.Background()
					switch state {
					case "late":
						h.clock.Add(16 * time.Minute)
					case "closed":
						_, err := h.store.SetLifecycle(ctx, roomKey(), channels.StateOpen,
							channels.Transition{To: channels.StateClosed, By: channels.Author{User: "x"}, At: h.clock.Now()})
						require.NoError(t, err)
					case "pending", "pending long":
						_, err := h.store.SetAction(ctx, roomKey(), m.Seq, "x", channels.ActionNone, channels.Action{
							Status: channels.ActionPending, By: channels.Author{User: "x"},
							At: h.clock.Now(), PendingAt: h.clock.Now(),
						})
						require.NoError(t, err)
						if state == "pending long" {
							h.clock.Add(realtimelimits.PendingRelease)
						}
					}
					offered := slices.Contains(h.offeredOn(t, caller(), room, m.Seq), op)
					var got MessageView
					var err error
					switch op {
					case OpEdit:
						got, err = h.svc.Edit(as(caller()), room, m.Seq, map[string]any{"body": "new"})
					case OpDelete:
						got, err = h.svc.Delete(as(caller()), room, m.Seq, "")
					case OpRemove:
						got, err = h.svc.Delete(as(caller()), room, m.Seq, "spam")
					case OpRelease:
						got, err = h.svc.ReleaseAction(as(caller()), room, m.Seq)
					}
					ran := err == nil
					// A moderator's DELETE without a reason, and an author's
					// with one, run as the other operation.
					switch {
					case ran && op == OpDelete:
						ran = got.Removed.Kind == channels.RemovalAuthor
					case ran && op == OpRemove:
						ran = got.Removed.Kind == channels.RemovalModerator
					}
					require.Equal(t, offered, ran, "offered %v, error %v", offered, err)
				})
			}
		}
	}
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
