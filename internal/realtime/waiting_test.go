package realtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// waitStates is the default session of waitRealtime: waiting waits for
// lead (TKT-1's owner, alice) and watcher (carol and dave); a member's
// text moves it to working, an answer ends it.
const waitStates = `
        waiting:
          label: Waiting
          waiting_for: [lead, watcher]
          on:
            answer: { to: done }
            text:   { to: working }
        working:
          label: Working
          on:
            answer: { to: waiting }
        done: { final: true }
`

// waitRealtime declares case, a cardinality-one session type on tickets,
// review, whose lead is a watcher, and threads, a cardinality-many type
// whose lead is the channel's creator.
func waitRealtime(initial, states string) string {
	return `
version: 1
channel_types:
  case:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      lead:    { from: entity.owner, label: the lead }
      watcher: { from: entity.watchers, label: a watcher }
      member:  { from: attached }
    message_types:
      text:   { properties: { body: { type: string } }, post: [member] }
      chat:   { properties: { body: { type: string } }, post: [member] }
      answer: { properties: { note: { type: string } }, post: [lead, watcher] }
    lifecycle:
      close:  { roles: [member] }
      reopen: { roles: [member] }
    session:
      initial: ` + initial + `
      states:` + states + `
  review:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      lead:   { from: entity.watchers }
      member: { from: attached }
    message_types:
      chat:   { properties: { body: { type: string } }, post: [member] }
      answer: { properties: { note: { type: string } }, post: [lead] }
    session:
      initial: asked
      states:
        asked: { waiting_for: [lead], on: { answer: { to: answered } } }
        answered: { final: true }
  threads:
    attach_to: [ticket]
    cardinality: many
    read: { from: attached }
    create: { roles: [member] }
    roles:
      lead:   { creator: true, label: the starter }
      owner:  { from: entity.owner }
      member: { from: attached }
    message_types:
      reply: { properties: { body: { type: string } }, post: [lead, owner, member] }
    session:
      initial: asked
      states:
        asked: { waiting_for: [lead, owner], on: { reply: { to: answered, roles: [lead, owner] } } }
        answered: { final: true }
`
}

var waitCase = RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "case"}

func newWaitHarness(t *testing.T) *harness {
	t.Helper()
	return newHarness(t, withYAML(waitRealtime("waiting", waitStates)))
}

// session reads the session of addr as a.
func (h *harness) session(t *testing.T, a *fakeAccess, addr RawAddress) SessionView {
	t.Helper()
	got, err := h.svc.Session(as(a), addr)
	require.NoError(t, err)
	require.NotNil(t, got.Session)
	return *got.Session
}

// TestSessionView_Status pins AC 12: status follows the current config,
// a stranded state reads failed with no waiting roles, and declaring the
// state again restores it.
func TestSessionView_Status(t *testing.T) {
	t.Parallel()
	h := newWaitHarness(t)
	bob := user("bob")
	v := h.session(t, bob, waitCase)
	require.Equal(t, "input_required", v.Status)
	h.postTo(t, bob, waitCase, "text", props("body", "on it"))
	v = h.session(t, bob, waitCase)
	require.Equal(t, "working", v.State)
	require.Equal(t, "working", v.Status)
	require.Empty(t, v.WaitingFor)

	waitingWorking := `
        waiting:
          waiting_for: [lead, watcher]
          on:
            answer: { to: done }
            text:   { to: working }
        working:
          waiting_for: [lead]
          on:
            answer: { to: waiting }
        done: { final: true }
`
	h.cfg.set(Snapshot{Config: testCompiled(t, waitRealtime("waiting", waitingWorking))})
	v = h.session(t, bob, waitCase)
	require.Equal(t, "input_required", v.Status)
	require.Equal(t, []SessionWaitingView{{Role: "lead", Label: "the lead", Holders: []string{"alice"}}}, v.WaitingFor)

	stranding := `
        waiting:
          waiting_for: [lead]
          on:
            answer: { to: done }
        done: { final: true }
`
	h.cfg.set(Snapshot{Config: testCompiled(t, waitRealtime("waiting", stranding))})
	v = h.session(t, user("alice"), waitCase)
	require.True(t, v.Stranded)
	require.Equal(t, "failed", v.Status)
	require.Empty(t, v.WaitingFor)
	require.False(t, v.WaitingOnMe)

	h.cfg.set(Snapshot{Config: testCompiled(t, waitRealtime("waiting", waitStates))})
	v = h.session(t, bob, waitCase)
	require.False(t, v.Stranded)
	require.Equal(t, "working", v.Status)
}

// TestSessionView_UnsetIsInitial pins AC 13: a stored channel that never
// moved reads as the initial state of the config in force, including
// after a reload that changes initial.
func TestSessionView_UnsetIsInitial(t *testing.T) {
	t.Parallel()
	h := newWaitHarness(t)
	h.postTo(t, user("bob"), waitCase, "chat", props("body", "hi"))
	v := h.session(t, user("alice"), waitCase)
	require.Equal(t, "waiting", v.State)
	require.EqualValues(t, 0, v.Generation)
	require.Equal(t, "input_required", v.Status)
	require.True(t, v.WaitingOnMe)

	h.cfg.set(Snapshot{Config: testCompiled(t, waitRealtime("working", waitStates))})
	v = h.session(t, user("alice"), waitCase)
	require.Equal(t, "working", v.State)
	require.Equal(t, "working", v.Status)
	require.False(t, v.WaitingOnMe)
}

// TestSessionView_UnstoredNoEnteredAt pins AC 14: a channel that is not
// stored has no entered_at, so two reads at two clock times render the
// same; a stored channel that never moved keeps its created_at.
func TestSessionView_UnstoredNoEnteredAt(t *testing.T) {
	t.Parallel()
	h := newWaitHarness(t)
	alice := user("alice")
	head := func() []byte {
		t.Helper()
		got, err := h.svc.Head(as(alice), waitCase)
		require.NoError(t, err)
		b, err := json.Marshal(got)
		require.NoError(t, err)
		return b
	}
	first := head()
	h.clock.Add(time.Hour)
	require.Equal(t, string(first), string(head()))
	require.NotContains(t, string(first), "entered_at")
	sess, err := h.svc.Session(as(alice), waitCase)
	require.NoError(t, err)
	require.True(t, sess.Session.EnteredAt.IsZero())
	read, err := h.svc.Read(as(alice), waitCase, ReadRequest{})
	require.NoError(t, err)
	require.True(t, read.Channel.Session.EnteredAt.IsZero())
	list, err := h.svc.ListAnchor(as(alice), "ticket", "TKT-1", "")
	require.NoError(t, err)
	require.Empty(t, list.Types[0].Channels, "an unstored one-channel is not listed")

	created := h.clock.Now()
	h.postTo(t, user("bob"), waitCase, "chat", props("body", "hi"))
	h.clock.Add(time.Hour)
	v := h.session(t, alice, waitCase)
	require.True(t, v.EnteredAt.Equal(created), "entered_at %v", v.EnteredAt)
}

// TestSessionView_WaitingFor pins AC 15: waiting roles in config order,
// holders from the reader's redacted anchor view, none for a hidden
// property or a creator role.
func TestSessionView_WaitingFor(t *testing.T) {
	t.Parallel()
	h := newWaitHarness(t)
	h.anchors.hide = map[string][]string{"bob": {"owner"}}
	want := []SessionWaitingView{
		{Role: "lead", Label: "the lead", Holders: []string{"alice"}},
		{Role: "watcher", Label: "a watcher", Holders: []string{"carol", "dave"}},
	}
	require.Equal(t, want, h.session(t, user("erin"), waitCase).WaitingFor)
	want[0].Holders = nil
	require.Equal(t, want, h.session(t, user("bob"), waitCase).WaitingFor)

	ch, err := h.svc.Create(as(user("bob")), "ticket", "TKT-1", "threads", CreateInput{Title: "Q"})
	require.NoError(t, err)
	got := h.session(t, user("bob"), RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "threads", Name: ch.Name})
	require.Equal(t, []SessionWaitingView{{Role: "lead", Label: "the starter"}, {Role: "owner", Label: "owner"}},
		got.WaitingFor)
}

// TestSessionView_WaitingOnMe pins AC 16.
func TestSessionView_WaitingOnMe(t *testing.T) {
	t.Parallel()
	threadAs := func(t *testing.T, h *harness, creator string) RawAddress {
		t.Helper()
		ch, err := h.svc.Create(as(user(creator)), "ticket", "TKT-1", "threads", CreateInput{Title: "Q"})
		require.NoError(t, err)
		return RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "threads", Name: ch.Name}
	}
	stored := func(t *testing.T, h *harness) RawAddress {
		t.Helper()
		h.postTo(t, user("bob"), waitCase, "chat", props("body", "hi"))
		return waitCase
	}
	noCeiling := func() *fakeAccess {
		a := user("alice")
		a.ceiling = false
		return a
	}
	anonymous := &fakeAccess{q: predicatefns.QueryIdentity{Raw: "alice"}, ceiling: true}
	cases := []struct {
		name   string
		reader *fakeAccess
		setup  func(t *testing.T, h *harness) RawAddress
		want   bool
	}{
		{"holder of a named role", user("alice"), stored, true},
		{"holder of a list role", user("dave"), stored, true},
		{"creator on a many type", user("bob"), func(t *testing.T, h *harness) RawAddress {
			t.Helper()
			return threadAs(t, h, "bob")
		}, true},
		{"not the creator", user("erin"), func(t *testing.T, h *harness) RawAddress {
			t.Helper()
			return threadAs(t, h, "bob")
		}, false},
		{"non-holder", user("bob"), stored, false},
		{"ceiling denies update", noCeiling(), stored, false},
		{"closed channel", user("alice"), func(t *testing.T, h *harness) RawAddress {
			t.Helper()
			addr := stored(t, h)
			_, err := h.svc.Lifecycle(as(user("bob")), addr, realtimecfg.LifecycleClose)
			require.NoError(t, err)
			return addr
		}, false},
		{"unstored channel", user("alice"), func(*testing.T, *harness) RawAddress { return waitCase }, false},
		{"degraded snapshot", user("alice"), func(t *testing.T, h *harness) RawAddress {
			t.Helper()
			addr := stored(t, h)
			h.cfg.set(Snapshot{Config: h.cfg.Load().Config, Degraded: true})
			return addr
		}, false},
		{"anonymous reader", anonymous, stored, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newWaitHarness(t)
			addr := tc.setup(t, h)
			v := h.session(t, tc.reader, addr)
			require.Equal(t, "input_required", v.Status)
			require.Equal(t, tc.want, v.WaitingOnMe)
		})
	}
	t.Run("reopen restores it", func(t *testing.T) {
		t.Parallel()
		h := newWaitHarness(t)
		h.postTo(t, user("bob"), waitCase, "chat", props("body", "hi"))
		_, err := h.svc.Lifecycle(as(user("bob")), waitCase, realtimecfg.LifecycleClose)
		require.NoError(t, err)
		_, err = h.svc.Lifecycle(as(user("bob")), waitCase, realtimecfg.LifecycleReopen)
		require.NoError(t, err)
		require.True(t, h.session(t, user("alice"), waitCase).WaitingOnMe)
	})
}

// listWaiting maps channel type and name to waiting_on_me in a list.
func listWaiting(t *testing.T, h *harness, a *fakeAccess) map[string]bool {
	t.Helper()
	got, err := h.svc.ListAnchor(as(a), "ticket", "TKT-1", "")
	require.NoError(t, err)
	out := map[string]bool{}
	for _, tc := range got.Types {
		for _, ch := range tc.Channels {
			require.NotNil(t, ch.Session)
			out[tc.Type+"/"+ch.Name] = ch.Session.WaitingOnMe
		}
	}
	return out
}

// TestListAnchor_WaitingOnMeIsolation pins AC 17: role answers do not
// cross channel types that declare the same role name with different
// sources, and a creator role is answered per channel.
func TestListAnchor_WaitingOnMeIsolation(t *testing.T) {
	t.Parallel()
	h := newWaitHarness(t)
	h.postTo(t, user("bob"), waitCase, "chat", props("body", "hi"))
	h.postTo(t, user("bob"), RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "review"}, "chat",
		props("body", "hi"))
	byBob, err := h.svc.Create(as(user("bob")), "ticket", "TKT-1", "threads", CreateInput{Title: "B"})
	require.NoError(t, err)
	byErin, err := h.svc.Create(as(user("erin")), "ticket", "TKT-1", "threads", CreateInput{Title: "E"})
	require.NoError(t, err)

	require.Equal(t, map[string]bool{
		"case/": true, "review/": false, "threads/" + byBob.Name: true, "threads/" + byErin.Name: true,
	}, listWaiting(t, h, user("alice")), "alice owns TKT-1: lead of case, owner of every thread")
	require.Equal(t, map[string]bool{
		"case/": true, "review/": true, "threads/" + byBob.Name: false, "threads/" + byErin.Name: false,
	}, listWaiting(t, h, user("carol")), "carol watches TKT-1: watcher of case, lead of review")
	require.Equal(t, map[string]bool{
		"case/": false, "review/": false, "threads/" + byBob.Name: true, "threads/" + byErin.Name: false,
	}, listWaiting(t, h, user("bob")), "bob started one thread")
}

// budgetRealtime declares n-channel threads whose state waits for a named
// role and a creator role, or for nobody when waiting is empty.
func budgetRealtime(waiting string) string {
	return `
version: 1
channel_types:
  threads:
    attach_to: [ticket]
    cardinality: many
    read: { from: attached }
    create: { roles: [member] }
    roles:
      starter: { creator: true }
      lead:    { from: entity.owner }
      member:  { from: attached }
    message_types:
      reply: { properties: { body: { type: string } }, post: [starter, lead, member] }
    session:
      initial: asked
      states:
        asked: { ` + waiting + `on: { reply: { to: answered, roles: [starter, lead] } } }
        answered: { final: true }
`
}

// TestListAnchor_WaitingForBudget pins AC 18: waiting_for costs no store
// read and no permission check on the list path, at 10 and at 20 rows,
// for a reader who holds neither role, so the creator role is asked on
// every row.
func TestListAnchor_WaitingForBudget(t *testing.T) {
	t.Parallel()
	type cost struct{ anchors, lists, heads, perms int64 }
	measure := func(t *testing.T, waiting string, n int) cost {
		t.Helper()
		h := newHarness(t, withYAML(budgetRealtime(waiting)))
		for i := range n {
			_, err := h.svc.Create(as(user(fmt.Sprintf("u%d", i%3))), "ticket", "TKT-1", "threads", CreateInput{})
			require.NoError(t, err)
		}
		// erin neither owns TKT-1 nor created a thread, so every row asks
		// both the named role and the per-row creator role.
		reader := user("erin")
		a0, l0, h0 := h.anchors.calls.Load(), h.store.entityLists.Load(), h.store.heads.Load()
		got, err := h.svc.ListAnchor(as(reader), "ticket", "TKT-1", "")
		require.NoError(t, err)
		require.Len(t, got.Types[0].Channels, n)
		for _, ch := range got.Types[0].Channels {
			require.False(t, ch.Session.WaitingOnMe, "erin holds no waiting role")
			if waiting != "" {
				require.Len(t, ch.Session.WaitingFor, 2)
			}
		}
		return cost{
			anchors: h.anchors.calls.Load() - a0, lists: h.store.entityLists.Load() - l0,
			heads: h.store.heads.Load() - h0, perms: reader.permCalls.Load(),
		}
	}
	base := measure(t, "", 10)
	require.Equal(t, cost{anchors: 1, lists: 1}, base)
	const waiting = "waiting_for: [lead, starter], "
	require.Equal(t, base, measure(t, waiting, 10))
	require.Equal(t, base, measure(t, waiting, 20))
}

// handoffRealtime declares a case whose waiting state hands lead over
// with a set of owner, or records a status with a set that names no role.
const handoffRealtime = `
version: 1
channel_types:
  case:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      lead:   { from: entity.owner, label: the lead }
      member: { from: attached }
    message_types:
      handoff: { properties: { to: { type: string, required: true } }, post: [lead] }
      note:    { properties: { body: { type: string } }, post: [lead] }
      accept:  { properties: { body: { type: string } }, post: [lead] }
    session:
      initial: waiting
      states:
        waiting:
          waiting_for: [lead]
          on:
            handoff:
              to: handed
              roles: [lead]
              outputs: [ { set: { properties: { owner: "{{message.to}}" } } } ]
            note:
              to: noted
              roles: [lead]
              outputs: [ { set: { properties: { status: noted } } } ]
          after:
            deadline: { period: 72h, since: session, to: noted }
        handed:
          waiting_for: [lead]
          on:
            accept: { to: waiting, roles: [lead] }
            handoff:
              to: waiting
              roles: [lead]
              outputs: [ { set: { properties: { owner: "{{message.to}}" } } } ]
          after:
            deadline: { period: 72h, since: session, to: noted }
        noted: { final: true }
`

// newHandoffHarness writes each set into the fake anchor, as the graph
// write would.
func newHandoffHarness(t *testing.T) *harness {
	t.Helper()
	var h *harness
	w := &fakeOutputs{write: func(r OutputRequest) {
		for name, v := range r.Props {
			h.anchors.setProp(name, v)
		}
	}}
	h = newHarness(t, withYAML(handoffRealtime), withOutputs(w))
	return h
}

// TestPost_NoHandoffNoExtraRead pins the read count of AC 19: a post whose
// inline set hands a role over answers with the new holders from one more
// anchor read; a post whose set names no role property reads the anchor
// once.
func TestPost_NoHandoffNoExtraRead(t *testing.T) {
	t.Parallel()
	t.Run("handoff", func(t *testing.T) {
		t.Parallel()
		h := newHandoffHarness(t)
		before := h.anchors.calls.Load()
		res := h.postTo(t, user("alice"), waitCase, "handoff", props("to", "bob"))
		require.EqualValues(t, 2, h.anchors.calls.Load()-before)
		require.Equal(t, "handed", res.Session.State)
		require.Equal(t, []SessionWaitingView{{Role: "lead", Label: "the lead", Holders: []string{"bob"}}},
			res.Session.WaitingFor)
		require.False(t, res.Session.WaitingOnMe, "alice handed the role to bob")
		require.True(t, h.session(t, user("bob"), waitCase).WaitingOnMe)
	})
	t.Run("failed re-read", func(t *testing.T) {
		t.Parallel()
		h := newHandoffHarness(t)
		reread := h.anchors.calls.Load() + 2
		h.anchors.fail = func(n int64) error {
			if n == reread {
				return errors.New("store down")
			}
			return nil
		}
		res := h.postTo(t, user("alice"), waitCase, "handoff", props("to", "bob"))
		require.Equal(t, "handed", res.Session.State)
		require.Equal(t, []SessionWaitingView{{Role: "lead", Label: "the lead"}}, res.Session.WaitingFor,
			"no holders rather than the ones from before the write")
		require.False(t, res.Session.WaitingOnMe, "not the stale answer from before the write")
		require.True(t, h.session(t, user("bob"), waitCase).WaitingOnMe)
	})
	t.Run("no handoff", func(t *testing.T) {
		t.Parallel()
		h := newHandoffHarness(t)
		before := h.anchors.calls.Load()
		res := h.postTo(t, user("alice"), waitCase, "note", props("body", "x"))
		require.EqualValues(t, 1, h.anchors.calls.Load()-before)
		require.Equal(t, "noted", res.Session.State)
		require.Equal(t, "completed", res.Session.Status)
	})
}

// TestHandoff_SessionTimerSurvives pins the since: session edge case: a
// handoff to another state and back keeps the session deadline, and the
// new holder is waited on throughout.
func TestHandoff_SessionTimerSurvives(t *testing.T) {
	t.Parallel()
	h := newHandoffHarness(t)
	h.postTo(t, user("alice"), waitCase, "handoff", props("to", "bob"))
	first := h.session(t, user("bob"), waitCase)
	require.Equal(t, "handed", first.State)
	require.True(t, first.WaitingOnMe)
	require.NotNil(t, first.DueAt)
	h.clock.Add(time.Hour)
	h.postTo(t, user("bob"), waitCase, "handoff", props("to", "alice"))
	v := h.session(t, user("alice"), waitCase)
	require.Equal(t, "waiting", v.State)
	require.NotNil(t, v.DueAt)
	require.True(t, v.DueAt.Equal(*first.DueAt), "due %v; want the session deadline %v", v.DueAt, first.DueAt)
	require.True(t, v.WaitingOnMe)
	require.False(t, h.session(t, user("bob"), waitCase).WaitingOnMe)
}

// TestDescribe_SessionStatus pins AC 21: every state carries its status,
// and a waiting state its role names.
func TestDescribe_SessionStatus(t *testing.T) {
	t.Parallel()
	h := newWaitHarness(t)
	var got *SessionDescription
	for _, ct := range h.svc.Describe() {
		if ct.Name == "case" {
			got = ct.Session
		}
	}
	require.NotNil(t, got)
	require.Equal(t, []SessionStateDescription{
		{Name: "waiting", Label: "Waiting", Status: "input_required", WaitingFor: []string{"lead", "watcher"}},
		{Name: "working", Label: "Working", Status: "working"},
		{Name: "done", Label: "done", Final: true, Status: "completed"},
	}, got.States)
}

// TestSession_NoSession pins the no_session answer: decided from the
// configuration before the anchor is read, the same for a missing and an
// existing anchor.
func TestSession_NoSession(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	for _, anchor := range []string{"TKT-1", "TKT-404"} {
		before := h.anchors.calls.Load()
		_, err := h.svc.Session(as(user("alice")), RawAddress{EntityType: "ticket", Anchor: anchor, Type: "room"})
		requireDenial(t, err, 404, TypeNoSession)
		require.Equal(t, before, h.anchors.calls.Load(), "no anchor read")
	}
	_, err := h.svc.Session(as(&fakeAccess{p: principal.Principal{User: "x"}}), RawAddress{
		EntityType: "ticket", Anchor: "TKT-1", Type: "topics", Name: "n"})
	requireDenial(t, err, 404, TypeNoSession)
}
