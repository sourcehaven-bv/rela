package realtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// askRealtime is a run channel like the agent runner's: the runner asks,
// the session waits on the owner, the owner answers. A run that stalls an
// hour waits on the owner too, moved by a timer. A member's text moves
// nothing. A moderator sees who removed a message.
const askRealtime = `
version: 1
channel_types:
  run:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      owner:  { from: entity.owner }
      runner: { from: entity.watchers }
      member: { from: attached }
      mod:    { permission: chat.moderate }
    message_types:
      text:   { properties: { body: { type: string } }, post: [member] }
      ask:
        properties:
          question: { type: markdown }
          about:    { type: ref, to: [ticket] }
        post: [runner]
      answer: { properties: { body: { type: string } }, post: [owner] }
      note:   { properties: { body: { type: string } }, post: [owner] }
    message_ops:
      edit_own: { roles: [member], within: 15m }
      remove:   { roles: [mod] }
    session:
      initial: working
      states:
        working:
          on: { ask: { to: asking } }
          after: { stall: { period: 1h, to: asking } }
        asking:
          label: Question for the owner
          waiting_for: [owner]
          on:
            answer: { to: working }
            note:   { to: working, when: "message.body == 'stop'" }
`

func newAskHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t, withYAML(askRealtime))
	h.anchors.views = map[string]AnchorView{}
	h.refs.views["TKT-REF"] = RefView{ID: "TKT-REF", Type: "ticket", Title: "Referenced"}
	return h
}

// runTicket sets the view of a ticket owned by owner and run by the
// runner, a watcher.
func (h *harness) runTicket(id, owner string) {
	h.anchors.mu.Lock()
	defer h.anchors.mu.Unlock()
	h.anchors.views[id] = AnchorView{
		ID: id, Type: "ticket", Title: "Title of " + id,
		Props: map[string]any{"title": "Title of " + id, "owner": owner, "watchers": []any{"runner"}},
	}
}

func runAt(id string) RawAddress { return RawAddress{EntityType: "ticket", Anchor: id, Type: "run"} }

// ask posts a question on id's run channel as the runner and returns its
// seq.
func (h *harness) ask(t *testing.T, id, question string) int64 {
	t.Helper()
	res := h.postTo(t, user("runner"), runAt(id), "ask",
		props("question", question, "about", "TKT-REF"))
	return res.Message.Seq
}

// inboxPage is the inbox page as the HTTP route answers it: step one and
// the waiting messages.
func (h *harness) inboxPage(t *testing.T, a *fakeAccess) InboxView {
	t.Helper()
	v := h.inbox(t, a, InboxRequest{})
	out, err := WaitingMessages(as(a), h.svc, v)
	require.NoError(t, err)
	return out
}

// listedMessage returns the message of seq as the channel's message list
// renders it for a, without the per-message actions and deadlines.
func (h *harness) listedMessage(t *testing.T, a *fakeAccess, addr RawAddress, seq int64) string {
	t.Helper()
	for _, m := range h.readAll(t, a, addr).Messages {
		if m.Seq == seq {
			m.Actions, m.EditUntil, m.ReleaseAfter = nil, time.Time{}, time.Time{}
			b, err := json.Marshal(m)
			require.NoError(t, err)
			return string(b)
		}
	}
	t.Fatalf("message %d not listed", seq)
	return ""
}

func messageJSON(t *testing.T, m *MessageView) string {
	t.Helper()
	require.NotNil(t, m)
	b, err := json.Marshal(m)
	require.NoError(t, err)
	return string(b)
}

// TestInbox_WaitingMessage pins AC 3: a waiting entry carries the message
// that moved its session, equal to the channel's message list without the
// per-message actions; the cause and last_seq are on the entry; a removed
// message is its stub, with who removed it for a moderator only. The
// waiting message cannot be edited: it moved the session.
func TestInbox_WaitingMessage(t *testing.T) {
	t.Parallel()
	alice, mod := user("alice"), user("alice", "chat.moderate")
	for _, tc := range []struct {
		name string
		// seed seeds TKT-1 and returns the cause, 0 for none.
		seed func(t *testing.T, h *harness) int64
		as   *fakeAccess
		want func(t *testing.T, h *harness, e InboxEntry, cause int64)
	}{
		{
			name: "ask",
			seed: func(t *testing.T, h *harness) int64 {
				t.Helper()
				h.postTo(t, user("bob"), runAt("TKT-1"), "text", props("body", "before"))
				return h.ask(t, "TKT-1", "Use **Postgres** or SQLite?")
			},
			as: alice,
			want: func(t *testing.T, h *harness, e InboxEntry, cause int64) {
				t.Helper()
				require.Equal(t, h.listedMessage(t, alice, runAt("TKT-1"), cause), messageJSON(t, e.Message))
				require.Equal(t, "ask", e.Message.Type)
			},
		},
		{
			name: "newer messages",
			seed: func(t *testing.T, h *harness) int64 {
				t.Helper()
				cause := h.ask(t, "TKT-1", "Which one?")
				h.postTo(t, user("bob"), runAt("TKT-1"), "text", props("body", "after"))
				return cause
			},
			as: alice,
			want: func(t *testing.T, _ *harness, e InboxEntry, cause int64) {
				t.Helper()
				require.Equal(t, cause+2, e.LastSeq, "the notice and one message follow the cause")
				require.Equal(t, cause, e.Message.Seq)
			},
		},
		{
			name: "removed, seen by a reader",
			seed: func(t *testing.T, h *harness) int64 {
				t.Helper()
				cause := h.ask(t, "TKT-1", "gone")
				_, err := h.svc.Delete(as(mod), runAt("TKT-1"), cause, "spam")
				require.NoError(t, err)
				return cause
			},
			as: alice,
			want: func(t *testing.T, h *harness, e InboxEntry, cause int64) {
				t.Helper()
				require.NotNil(t, e.Message.Removed)
				require.Nil(t, e.Message.Removed.By, "only a moderator sees who removed it")
				require.Empty(t, e.Message.Props)
				require.Equal(t, h.listedMessage(t, alice, runAt("TKT-1"), cause), messageJSON(t, e.Message))
			},
		},
		{
			name: "removed, seen by a moderator",
			seed: func(t *testing.T, h *harness) int64 {
				t.Helper()
				cause := h.ask(t, "TKT-1", "gone")
				_, err := h.svc.Delete(as(mod), runAt("TKT-1"), cause, "spam")
				require.NoError(t, err)
				return cause
			},
			as: mod,
			want: func(t *testing.T, h *harness, e InboxEntry, cause int64) {
				t.Helper()
				require.NotNil(t, e.Message.Removed.By)
				require.Equal(t, "spam", e.Message.Removed.Reason)
				require.Equal(t, h.listedMessage(t, mod, runAt("TKT-1"), cause), messageJSON(t, e.Message))
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newAskHarness(t)
			h.runTicket("TKT-1", "alice")
			cause := tc.seed(t, h)
			v := h.inboxPage(t, tc.as)
			require.Len(t, v.Entries, 1)
			e := v.Entries[0]
			require.Equal(t, cause, e.Session.Cause)
			head, err := h.svc.Head(as(tc.as), runAt("TKT-1"))
			require.NoError(t, err)
			require.Equal(t, head.Channel.LastSeq, e.LastSeq)
			require.Equal(t, cause, head.Channel.Session.Cause, "the head serves the same cause")
			tc.want(t, h, e, cause)
		})
	}
}

// TestInbox_WaitingWithoutCause pins that a session waiting without a
// cause (it never moved, here the initial state of the inbox harness's
// case type) has no message, and that the step reads nothing for it.
func TestInbox_WaitingWithoutCause(t *testing.T) {
	t.Parallel()
	h := newInboxHarness(t)
	h.seedVisible(t, 3)
	alice := user("alice")
	v := h.inbox(t, alice, InboxRequest{})
	require.Len(t, v.Entries, 3)
	h.store.getMessages.Store(0)
	out, err := WaitingMessages(as(alice), h.svc, v)
	require.NoError(t, err)
	require.Zero(t, h.store.getMessages.Load(), "no cause, no read")
	for _, e := range out.Entries {
		require.Zero(t, e.Session.Cause)
		require.Nil(t, e.Message)
	}
	require.Equal(t, v.Entries, out.Entries)
}

// TestInbox_WaitingMessageRefs pins AC 5 for refs: a ref in the waiting
// message resolves through the caller's gated resolver, and renders
// restricted for a caller who cannot read it.
func TestInbox_WaitingMessageRefs(t *testing.T) {
	t.Parallel()
	h := newAskHarness(t)
	h.runTicket("TKT-1", "alice")
	h.runTicket("TKT-2", "carol")
	h.refs.views["TKT-REF"] = RefView{ID: "TKT-REF", Type: "ticket", Title: "Referenced"}
	h.refs.hide = map[string][]string{"carol": {"TKT-REF"}}
	h.ask(t, "TKT-1", "q1")
	h.ask(t, "TKT-2", "q2")

	about := func(a *fakeAccess) *RefValue {
		v := h.inboxPage(t, a)
		require.Len(t, v.Entries, 1)
		ref, ok := v.Entries[0].Message.Props["about"].(*RefValue)
		require.True(t, ok)
		return ref
	}
	require.Equal(t, &RefValue{ID: "TKT-REF", Type: "ticket", Title: "Referenced"}, about(user("alice")))
	require.Equal(t, &RefValue{Restricted: true}, about(user("carol")))
}

// TestInbox_DecisionInputs pins the realtime half of AC 4: each waiting
// entry's actions and session transitions equal the channel head's, for
// the owner who may answer. TestInbox_DecisionInputsWithoutPost covers a
// waiting role holder who may not post the answer.
func TestInbox_DecisionInputs(t *testing.T) {
	t.Parallel()
	h := newAskHarness(t)
	h.runTicket("TKT-1", "alice")
	h.ask(t, "TKT-1", "q")
	alice := user("alice")
	e := h.inbox(t, alice, InboxRequest{}).Entries[0]
	head, err := h.svc.Head(as(alice), runAt("TKT-1"))
	require.NoError(t, err)
	require.Equal(t, head.Actions.Post, e.Actions.Post)
	require.Equal(t, head.Actions.Transitions, e.Actions.Transitions)
	require.Equal(t, head.Actions.Entries, e.Actions.Entries)
	require.Equal(t, head.Channel.Session.Transitions, e.Session.Transitions)
	require.Equal(t, []string{"answer", "note", "text"}, e.Actions.Post)
	require.Equal(t, []string{"answer", "note"}, e.Actions.Transitions)
	require.Equal(t, e.Transitions, e.Actions.Transitions)
}

// TestWaitingMessages_Refusals pins the step's refusals: a view of
// another reader is refused, and a store error fails the step.
func TestWaitingMessages_Refusals(t *testing.T) {
	t.Parallel()
	h := newAskHarness(t)
	h.runTicket("TKT-1", "alice")
	h.ask(t, "TKT-1", "q")
	v := h.inbox(t, user("alice"), InboxRequest{})
	_, err := WaitingMessages(as(user("bob")), h.svc, v)
	require.ErrorIs(t, err, ErrInboxReader)

	h.store.getMessagesErr = errors.New("disk on fire")
	_, err = WaitingMessages(as(user("alice")), h.svc, v)
	require.ErrorContains(t, err, "disk on fire")
}

// TestInbox_WaitingMessagesBudget pins AC 6 for the message step: one
// GetMessages and at most one ref batch, and no Get, Head or Messages,
// whatever the number of entries; the inbox step itself reads no message.
func TestInbox_WaitingMessagesBudget(t *testing.T) {
	t.Parallel()
	for _, n := range []int{10, 50} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			t.Parallel()
			h := newAskHarness(t)
			h.refs.views["TKT-REF"] = RefView{ID: "TKT-REF", Type: "ticket", Title: "Referenced"}
			for i := range n {
				id := fmt.Sprintf("TKT-%d", i+1)
				h.runTicket(id, "alice")
				h.ask(t, id, "q")
			}
			alice := user("alice")
			h.store.getMessages.Store(0)
			h.store.gets.Store(0)
			h.store.heads.Store(0)
			h.store.messages.Store(0)
			h.refs.calls.Store(0)
			v := h.inbox(t, alice, InboxRequest{Limit: MaxInboxLimit})
			require.Zero(t, h.store.getMessages.Load(), "the inbox step reads no message")
			require.Zero(t, h.refs.calls.Load())
			out, err := WaitingMessages(as(alice), h.svc, v)
			require.NoError(t, err)
			require.Len(t, out.Entries, n)
			for _, e := range out.Entries {
				require.NotNil(t, e.Message)
			}
			require.Equal(t, int64(1), h.store.getMessages.Load())
			require.Equal(t, int64(1), h.refs.calls.Load())
			require.Zero(t, h.store.gets.Load())
			require.Zero(t, h.store.heads.Load())
			require.Zero(t, h.store.messages.Load())
		})
	}
}

// TestInbox_WaitingMessageHiddenRows pins AC 5 for the message step:
// waiting channels the caller may not see (another owner, an unreadable
// anchor, an owner property hidden from the caller) change no byte of the
// page with its messages, and their messages are never read.
func TestInbox_WaitingMessageHiddenRows(t *testing.T) {
	t.Parallel()
	seed := func(h *harness, hidden bool) {
		for i := range 3 {
			id := fmt.Sprintf("TKT-%d", 10+i*2)
			h.runTicket(id, "alice")
			h.ask(t, id, "visible "+id)
			if !hidden {
				continue
			}
			other := fmt.Sprintf("TKT-%d", 11+i*2)
			h.runTicket(other, "alice")
			h.ask(t, other, "secret "+other)
			h.anchors.mu.Lock()
			switch i {
			case 0:
				delete(h.anchors.views, other)
			case 1:
				h.anchors.hideAt = map[string][]string{other: {"owner"}}
			default:
				v := h.anchors.views[other]
				v.Props["owner"] = "bob"
				h.anchors.views[other] = v
			}
			h.anchors.mu.Unlock()
		}
	}
	plain, mixed := newAskHarness(t), newAskHarness(t)
	seed(plain, false)
	seed(mixed, true)
	alice := user("alice")
	base := plain.inboxPage(t, alice)
	withHidden := mixed.inboxPage(t, alice)
	require.Len(t, base.Entries, 3)
	require.Equal(t, inboxJSON(t, base), inboxJSON(t, withHidden))
	require.Len(t, withHidden.Entries, len(base.Entries))
	for i := range base.Entries {
		b, w := base.Entries[i], withHidden.Entries[i]
		require.Equal(t, messageJSON(t, b.Message), messageJSON(t, w.Message))
		require.NotContains(t, messageJSON(t, w.Message), "secret")
		require.Equal(t, b.Actions, w.Actions)
		require.Equal(t, b.LastSeq, w.LastSeq)
		require.Equal(t, b.Session.Cause, w.Session.Cause)
	}
}

// TestWaitingMessages_ConfigReloaded pins that the message step renders
// nothing when the configuration changed after the inbox step gated the
// entries: a reload may have revoked the role they were judged by.
func TestWaitingMessages_ConfigReloaded(t *testing.T) {
	t.Parallel()
	h := newAskHarness(t)
	h.runTicket("TKT-1", "alice")
	h.ask(t, "TKT-1", "q")
	alice := user("alice")
	v := h.inbox(t, alice, InboxRequest{})
	require.Len(t, v.Entries, 1)
	h.cfg.set(Snapshot{Config: testCompiled(t, askRealtime)})
	h.store.getMessages.Store(0)
	out, err := WaitingMessages(as(alice), h.svc, v)
	require.NoError(t, err)
	require.Nil(t, out.Entries[0].Message)
	require.Zero(t, h.store.getMessages.Load())
}

// TestInbox_WaitingAfterTimerMove pins AC 3 for a timer move: the session
// waits with no cause, so the entry has no message and the step reads
// nothing, and the head agrees.
func TestInbox_WaitingAfterTimerMove(t *testing.T) {
	t.Parallel()
	h := newAskHarness(t)
	h.runTicket("TKT-1", "alice")
	h.postTo(t, user("bob"), runAt("TKT-1"), "text", props("body", "starting"))
	h.clock.mu.Lock()
	h.clock.now = h.clock.now.Add(time.Hour)
	h.clock.mu.Unlock()
	rep, err := h.timers(t).RunRound(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, rep.Fired)

	alice := user("alice")
	h.store.getMessages.Store(0)
	v := h.inboxPage(t, alice)
	require.Len(t, v.Entries, 1)
	e := v.Entries[0]
	require.Equal(t, "asking", e.Session.State)
	require.Zero(t, e.Session.Cause)
	require.Nil(t, e.Message)
	require.Zero(t, h.store.getMessages.Load())
	head, err := h.svc.Head(as(alice), runAt("TKT-1"))
	require.NoError(t, err)
	require.Zero(t, head.Channel.Session.Cause)
	require.Equal(t, head.Actions.Transitions, e.Actions.Transitions)
}

// TestInbox_DecisionInputsWithoutPost pins AC 4 and AC 12 for a waiting
// role holder without a post role: the entry lists no post types and no
// transitions, as the head does, and still carries the message.
func TestInbox_DecisionInputsWithoutPost(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(strings.Replace(askRealtime,
		"answer: { properties: { body: { type: string } }, post: [owner] }",
		"answer: { properties: { body: { type: string } }, post: [runner] }", 1)))
	h.anchors.views = map[string]AnchorView{}
	h.refs.views["TKT-REF"] = RefView{ID: "TKT-REF", Type: "ticket", Title: "Referenced"}
	h.runTicket("TKT-1", "alice")
	h.ask(t, "TKT-1", "q")
	alice := user("alice")
	v := h.inboxPage(t, alice)
	require.Len(t, v.Entries, 1)
	e := v.Entries[0]
	head, err := h.svc.Head(as(alice), runAt("TKT-1"))
	require.NoError(t, err)
	require.Equal(t, head.Actions.Post, e.Actions.Post)
	require.Equal(t, head.Actions.Transitions, e.Actions.Transitions)
	require.Equal(t, head.Actions.Entries, e.Actions.Entries)
	require.NotContains(t, e.Actions.Post, "answer")
	require.Equal(t, []string{"note"}, e.Actions.Transitions)
	require.NotNil(t, e.Message)
}
