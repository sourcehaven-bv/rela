package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/testutil/logtest"
)

// sessionStates is the session of the incident type: the breach-response
// example of the docs, plus a re-entry on checked and a guard on the time
// the session entered its state, read through stamp.
const sessionStates = `
        detected:
          label: Detected
          on:
            contained: { to: contained, roles: [lead] }
            dismiss:   { to: dismissed }
            escalated: { to: contained, by: trigger }
            checked:   { to: detected, when: "message.score > 2" }
            stamp:     { to: contained, when: "session.entered_at == message.at" }
            probe:     { to: contained, when: "regex(message.text, message.pattern)" }
        contained:
          label: Contained
          on:
            assessed: { to: assessed, when: "message.risk ~= 'none'" }
        assessed:
          label: Risk assessed
          on:
            notified: { to: notified }
        notified:  { label: AP notified, final: true }
        dismissed: { label: Dismissed, final: true }
`

// sessionRealtime declares incident, a cardinality-one session type, and
// cases, a cardinality-many one. alice owns TKT-1, so she holds lead.
func sessionRealtime(initial, states string) string {
	return `
version: 1
channel_types:
  incident:
    label: Incident
    attach_to: [ticket]
    read: { from: attached }
    roles:
      lead:   { from: entity.owner }
      member: { from: attached }
      dpo:    { permission: privacy.officer }
      mod:    { permission: chat.moderate }
    message_types:
      text:      { properties: { body: { type: markdown, required: true } }, post: [member] }
      contained: { properties: { note: { type: markdown } }, post: [lead, member] }
      assessed:  { properties: { risk: { type: risk-level, required: true } }, post: [lead, dpo] }
      notified:  { properties: { reference: { type: string, required: true } }, post: [dpo] }
      dismiss:   { properties: { reason: { type: string, required: true } }, post: [lead, member] }
      escalated: { properties: { note: { type: string } }, post: [lead] }
      checked:   { properties: { score: { type: integer } }, post: [member] }
      stamp:     { properties: { at: { type: datetime, required: true } }, post: [member] }
      probe:     { properties: { text: { type: string }, pattern: { type: string } }, post: [member] }
    message_ops:
      edit_own:   { roles: [member], within: 15m }
      delete_own: { roles: [member] }
      remove:     { roles: [mod] }
    lifecycle:
      close:  { roles: [member] }
      reopen: { roles: [member] }
    session:
      label: Breach response
      initial: ` + initial + `
      states:` + states + `
  cases:
    attach_to: [ticket]
    cardinality: many
    read: { from: attached }
    create: { roles: [member] }
    roles:
      member: { from: attached }
    message_types:
      done: { properties: { note: { type: string } }, post: [member] }
    session:
      initial: open-case
      states:
        open-case: { on: { done: { to: finished } } }
        finished:  { final: true }
`
}

var incident = RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "incident"}

func incidentKey() channels.Key {
	return channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "incident"}
}

func newSessionHarness(t *testing.T) *harness {
	t.Helper()
	return newHarness(t, withYAML(sessionRealtime("detected", sessionStates)))
}

func props(kv ...any) map[string]any {
	out := map[string]any{}
	for i := 0; i < len(kv); i += 2 {
		out[kv[i].(string)] = kv[i+1]
	}
	return out
}

// postTo posts typ with props to addr as a.
func (h *harness) postTo(t *testing.T, a *fakeAccess, addr RawAddress, typ string, p map[string]any) PostResult {
	t.Helper()
	res, err := h.svc.Post(as(a), addr, PostInput{Type: typ, Props: p})
	require.NoError(t, err)
	require.NotNil(t, res.Message)
	return res
}

func (h *harness) readAll(t *testing.T, a *fakeAccess, addr RawAddress) ReadView {
	t.Helper()
	v, err := h.svc.Read(as(a), addr, ReadRequest{})
	require.NoError(t, err)
	return v
}

// sessionAudit returns the session records of TKT-1's audit trail.
func (h *harness) sessionAudit(t *testing.T) []channels.AuditRecord {
	t.Helper()
	recs, err := h.store.Audit(context.Background(), "TKT-1")
	require.NoError(t, err)
	var out []channels.AuditRecord
	for _, r := range recs {
		if r.Op == channels.AuditSession {
			out = append(out, r)
		}
	}
	return out
}

// requireSession asserts the session of the incident channel as alice
// reads it.
func (h *harness) requireSession(t *testing.T, state string, gen int64) *SessionView {
	t.Helper()
	s := h.readAll(t, user("alice"), incident).Channel.Session
	require.NotNil(t, s)
	require.Equal(t, state, s.State)
	require.Equal(t, gen, s.Generation)
	return s
}

func TestSessionPost_Transitions(t *testing.T) {
	t.Parallel()
	h := newSessionHarness(t)
	h.clock.Add(time.Minute)
	posted := h.clock.Now()
	res := h.postTo(t, user("alice"), incident, "contained", nil)
	require.Equal(t, "contained", res.Session.State)

	s := h.requireSession(t, "contained", 1)
	require.Equal(t, "Contained", s.Label)
	require.True(t, s.EnteredAt.Equal(posted), "entered_at %v", s.EnteredAt)
	require.Equal(t, []SessionTransitionView{{MessageType: "assessed", To: "assessed", Guarded: true}}, s.Transitions)

	msgs := h.readAll(t, user("bob"), incident).Messages
	require.Len(t, msgs, 2)
	require.Equal(t, "contained", msgs[0].Type)
	notice := msgs[1]
	require.Equal(t, realtimecfg.SessionNoticeType, notice.Type)
	require.True(t, notice.Author.System)
	require.Equal(t, "session:incident", notice.TriggeredBy)
	require.Equal(t, "detected", notice.Props["from"])
	require.Equal(t, "contained", notice.Props["to"])
	require.EqualValues(t, 1, notice.Props["generation"])
	require.EqualValues(t, 1, notice.Props["cause"])

	audit := h.sessionAudit(t)
	require.Len(t, audit, 1)
	require.Equal(t, "alice", audit[0].By.User)
	require.Equal(t, "detected", audit[0].From)
	require.Equal(t, "contained", audit[0].To)
	require.Equal(t, int64(1), audit[0].Generation)
	require.Empty(t, audit[0].TriggeredBy)
}

func TestSessionPost_GuardFails(t *testing.T) {
	t.Parallel()
	h := newSessionHarness(t)
	h.postTo(t, user("alice"), incident, "contained", nil)
	res := h.postTo(t, user("alice"), incident, "assessed", props("risk", "none"))
	require.Equal(t, "contained", res.Session.State)
	h.requireSession(t, "contained", 1)
	require.Len(t, h.readAll(t, user("alice"), incident).Messages, 3, "message, notice, assessed")
	require.Len(t, h.sessionAudit(t), 1)
}

func TestSessionPost_GuardAbsentProperty(t *testing.T) {
	t.Parallel()
	h := newSessionHarness(t)
	res, err := h.svc.Post(as(user("bob")), incident, PostInput{Type: "checked"})
	require.NoError(t, err)
	require.Equal(t, int64(0), res.Session.Generation)
	h.requireSession(t, "detected", 0)
	require.Empty(t, h.sessionAudit(t))

	// An absent optional property is a valid config: nothing to warn about.
	ct, _ := h.cfg.Load().Config.ChannelType("incident")
	failed := guardFailure{}
	decide := sessionDecider(context.Background(), ct.Session(), ct.MessageTypes["checked"], props(), time.Now(),
		poster{roles: map[string]bool{}}, &failed, nil)
	_, moved := decide(channels.Channel{State: channels.StateOpen})
	require.False(t, moved)
	require.NoError(t, failed.err, "an absent property is not logged")
}

func TestSessionPost_RoleNarrowing(t *testing.T) {
	t.Parallel()
	h := newSessionHarness(t)
	h.postTo(t, user("bob"), incident, "contained", nil)
	h.requireSession(t, "detected", 0)

	ceiling := user("alice")
	ceiling.ceiling = false
	_, err := h.svc.Post(as(ceiling), incident, PostInput{Type: "contained"})
	requireDenial(t, err, http.StatusForbidden, TypePostDenied)
	h.requireSession(t, "detected", 0)

	h.postTo(t, user("alice"), incident, "contained", nil)
	h.requireSession(t, "contained", 1)
}

func TestSessionPost_AnyPoster(t *testing.T) {
	t.Parallel()
	h := newSessionHarness(t)
	h.postTo(t, user("bob"), incident, "dismiss", props("reason", "false alarm"))
	s := h.requireSession(t, "dismissed", 1)
	require.True(t, s.Final)
	require.Empty(t, s.Transitions)
}

func TestSessionPost_TriggerOnly(t *testing.T) {
	t.Parallel()
	h := newSessionHarness(t)
	h.postTo(t, user("alice"), incident, "escalated", nil)
	h.requireSession(t, "detected", 0)
	require.Empty(t, h.sessionAudit(t))
}

func TestSessionPost_NoTransitionHere(t *testing.T) {
	t.Parallel()
	h := newSessionHarness(t)
	h.postTo(t, user("erin", "privacy.officer"), incident, "notified", props("reference", "AP-1"))
	h.requireSession(t, "detected", 0)
}

func TestSessionPost_FinalState(t *testing.T) {
	t.Parallel()
	h := newSessionHarness(t)
	erin := user("erin", "privacy.officer")
	h.postTo(t, user("alice"), incident, "contained", nil)
	h.postTo(t, user("alice"), incident, "assessed", props("risk", "low"))
	h.postTo(t, erin, incident, "notified", props("reference", "AP-1"))
	s := h.requireSession(t, "notified", 3)
	require.True(t, s.Final)
	require.Empty(t, s.Transitions)

	h.postTo(t, user("alice"), incident, "dismiss", props("reason", "late"))
	h.postTo(t, erin, incident, "notified", props("reference", "AP-2"))
	h.requireSession(t, "notified", 3)
}

func TestSessionPost_ReEntry(t *testing.T) {
	t.Parallel()
	h := newSessionHarness(t)
	h.postTo(t, user("bob"), incident, "text", props("body", "hi"))
	h.clock.Add(time.Hour)
	h.postTo(t, user("bob"), incident, "checked", props("score", 5))
	s := h.requireSession(t, "detected", 1)
	require.True(t, s.EnteredAt.Equal(h.clock.Now()))
	msgs := h.readAll(t, user("bob"), incident).Messages
	require.Equal(t, realtimecfg.SessionNoticeType, msgs[len(msgs)-1].Type)
	require.Equal(t, "detected", msgs[len(msgs)-1].Props["from"])
}

func TestSessionPost_Reply(t *testing.T) {
	t.Parallel()
	h := newSessionHarness(t)
	h.postTo(t, user("bob"), incident, "text", props("body", "what now?"))
	res, err := h.svc.Post(as(user("bob")), incident, PostInput{
		Type: "dismiss", Props: props("reason", "nothing leaked"), ReplyTo: 1,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), res.Message.ReplyTo)
	h.requireSession(t, "dismissed", 1)
}

func TestSessionPost_ManyChannels(t *testing.T) {
	t.Parallel()
	h := newSessionHarness(t)
	bob := user("bob")
	a, err := h.svc.Create(as(bob), "ticket", "TKT-1", "cases", CreateInput{Title: "a"})
	require.NoError(t, err)
	b, err := h.svc.Create(as(bob), "ticket", "TKT-1", "cases", CreateInput{Title: "b"})
	require.NoError(t, err)
	require.Equal(t, "open-case", a.Session.State)

	addrA := RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "cases", Name: a.Name}
	addrB := RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "cases", Name: b.Name}
	h.postTo(t, bob, addrA, "done", nil)
	require.Equal(t, "finished", h.readAll(t, bob, addrA).Channel.Session.State)
	sb := h.readAll(t, bob, addrB).Channel.Session
	require.Equal(t, "open-case", sb.State)
	require.Equal(t, int64(0), sb.Generation)

	list, err := h.svc.ListAnchor(as(bob), "ticket", "TKT-1", "")
	require.NoError(t, err)
	states := map[string]string{}
	for _, tc := range list.Types {
		for _, ch := range tc.Channels {
			if ch.Session != nil {
				states[ch.Name] = ch.Session.State
			}
		}
	}
	require.Equal(t, map[string]string{a.Name: "finished", b.Name: "open-case"}, states)
}

func TestSessionPost_Concurrent(t *testing.T) {
	t.Parallel()
	h := newSessionHarness(t)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Go(func() {
			_, errs[i] = h.svc.Post(as(user("alice")), incident, PostInput{Type: "contained"})
		})
	}
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}
	h.requireSession(t, "contained", 1)
	msgs := h.readAll(t, user("alice"), incident).Messages
	require.Len(t, msgs, 3)
	var notices []MessageView
	for _, m := range msgs {
		if m.Type == realtimecfg.SessionNoticeType {
			notices = append(notices, m)
		}
	}
	require.Len(t, notices, 1)
	require.Equal(t, int64(2), notices[0].Seq, "the notice follows the lower seq")
	require.EqualValues(t, 1, notices[0].Props["cause"])
}

func TestSessionPost_Degraded(t *testing.T) {
	t.Parallel()
	h := newSessionHarness(t)
	snap := h.cfg.Load()
	snap.Degraded = true
	h.cfg.set(snap)
	res := h.postTo(t, user("alice"), incident, "contained", nil)
	require.NotNil(t, res.Session)
	require.Equal(t, "detected", res.Session.State)
	require.Equal(t, int64(0), res.Session.Generation)
	h.requireSession(t, "detected", 0)
}

func TestSessionStranded(t *testing.T) {
	t.Parallel()
	h := newSessionHarness(t)
	alice := user("alice")
	h.postTo(t, alice, incident, "contained", nil)

	without := `
        detected:
          on:
            dismiss: { to: dismissed }
        dismissed: { final: true }
`
	h.cfg.set(Snapshot{Config: testCompiled(t, sessionRealtime("detected", without))})
	s := h.requireSession(t, "contained", 1)
	require.True(t, s.Stranded)
	require.Equal(t, "contained", s.Label)
	require.Empty(t, s.Transitions)

	h.postTo(t, alice, incident, "dismiss", props("reason", "stuck"))
	h.requireSession(t, "contained", 1)

	_, err := h.svc.Lifecycle(as(alice), incident, realtimecfg.LifecycleClose)
	require.NoError(t, err)
	v, err := h.svc.Lifecycle(as(alice), incident, realtimecfg.LifecycleReopen)
	require.NoError(t, err)
	require.Equal(t, channels.StateOpen, v.State)
	require.True(t, v.Session.Stranded)
}

func TestSessionInitialChange(t *testing.T) {
	t.Parallel()
	h := newSessionHarness(t)
	h.postTo(t, user("bob"), incident, "text", props("body", "hi"))
	h.requireSession(t, "detected", 0)
	h.cfg.set(Snapshot{Config: testCompiled(t, sessionRealtime("contained", sessionStates))})
	h.requireSession(t, "contained", 0)
}

func TestSessionEnteredAt(t *testing.T) {
	t.Parallel()
	t.Run("not stored", func(t *testing.T) {
		t.Parallel()
		h := newSessionHarness(t)
		now := h.clock.Now()
		s := h.requireSession(t, "detected", 0)
		require.True(t, s.EnteredAt.IsZero(), "an unstored channel has no entered_at: %v", s.EnteredAt)
		// The guard still reads the unstored session as entered now: it
		// compares session.entered_at with the posted time.
		h.postTo(t, user("bob"), incident, "stamp", props("at", now.Format(time.RFC3339)))
		h.requireSession(t, "contained", 1)
	})
	t.Run("stored", func(t *testing.T) {
		t.Parallel()
		h := newSessionHarness(t)
		created := h.clock.Now()
		h.postTo(t, user("bob"), incident, "text", props("body", "hi"))
		h.clock.Add(time.Hour)
		s := h.requireSession(t, "detected", 0)
		require.True(t, s.EnteredAt.Equal(created), "entered_at %v", s.EnteredAt)
		h.postTo(t, user("bob"), incident, "stamp", props("at", h.clock.Now().Format(time.RFC3339)))
		h.requireSession(t, "detected", 0)
		h.postTo(t, user("bob"), incident, "stamp", props("at", created.Format(time.RFC3339)))
		h.requireSession(t, "contained", 1)
	})
}

func TestSessionLifecycleAndEdits(t *testing.T) {
	t.Parallel()
	h := newSessionHarness(t)
	alice := user("alice")
	h.postTo(t, alice, incident, "contained", props("note", "first"))

	_, err := h.svc.Lifecycle(as(alice), incident, realtimecfg.LifecycleClose)
	require.NoError(t, err)
	h.requireSession(t, "contained", 1)
	_, err = h.svc.Post(as(alice), incident, PostInput{Type: "assessed", Props: props("risk", "high")})
	requireDenial(t, err, http.StatusConflict, TypeChannelNotOpen)

	// A post that reaches the store of a closed channel never moves it.
	ct, _ := h.cfg.Load().Config.ChannelType("incident")
	mt := ct.MessageTypes["assessed"]
	decide := sessionDecider(context.Background(), ct.Session(), mt, props("risk", "high"), h.clock.Now(),
		poster{roles: map[string]bool{}}, &guardFailure{}, nil)
	_, moved := decide(channels.Channel{
		State: channels.StateClosed, Session: channels.Session{State: "contained", Generation: 1},
	})
	require.False(t, moved)

	_, err = h.svc.Lifecycle(as(alice), incident, realtimecfg.LifecycleReopen)
	require.NoError(t, err)
	h.requireSession(t, "contained", 1)

	_, err = h.svc.Edit(as(alice), incident, 1, props("note", "edited"))
	requireDenial(t, err, http.StatusConflict, TypeSessionMessage)
	h.requireSession(t, "contained", 1)
	require.Len(t, h.sessionAudit(t), 1)
}

// TestSessionGuardFailure pins that a guard that fails to evaluate, here
// on an invalid pattern, moves nothing, is recorded for the caller to log,
// and is reported once per loaded configuration.
func TestSessionGuardFailure(t *testing.T) {
	t.Parallel()
	ct, _ := testCompiled(t, sessionRealtime("detected", sessionStates)).ChannelType("incident")
	mt := ct.MessageTypes["probe"]
	var failed guardFailure
	decide := sessionDecider(context.Background(), ct.Session(), mt, props("text", "a", "pattern", "("), time.Now(),
		poster{roles: map[string]bool{}}, &failed, nil)
	_, moved := decide(channels.Channel{State: channels.StateOpen})
	require.False(t, moved)
	require.Error(t, failed.err)
	require.Equal(t, "detected", failed.state)
	require.True(t, failed.t.WarnGuardOnce())
	require.False(t, failed.t.WarnGuardOnce(), "a second failure of the same guard is not reported")

	_, moved = decide(channels.Channel{State: channels.StateClosed})
	require.False(t, moved)
	require.NoError(t, failed.err, "each decide starts clean")
}

// TestSessionMessageProtected pins that the author can neither edit nor
// delete a message that moved the session, and is not offered either,
// while a moderator may still remove it. A message of the same type that
// moved nothing stays the author's to change.
func TestSessionMessageProtected(t *testing.T) {
	t.Parallel()
	h := newSessionHarness(t)
	alice, dave := user("alice"), user("dave", "chat.moderate")
	h.postTo(t, alice, incident, "contained", props("note", "first"))
	res, err := h.svc.Post(as(alice), incident, PostInput{Type: "contained", Props: props("note", "again")})
	require.NoError(t, err)
	require.Equal(t, int64(3), res.Message.Seq, "contained in contained moves nothing, so no notice")
	require.Equal(t, "contained", res.Session.State, "the post answers with the session it left in place")

	msgs := h.readAll(t, alice, incident).Messages
	require.Len(t, msgs, 3)
	require.NotContains(t, msgs[0].Actions, OpEdit)
	require.NotContains(t, msgs[0].Actions, OpDelete)
	require.Contains(t, msgs[2].Actions, OpEdit)
	require.Contains(t, msgs[2].Actions, OpDelete)

	_, err = h.svc.Edit(as(alice), incident, 1, props("note", "edited"))
	requireDenial(t, err, http.StatusConflict, TypeSessionMessage)
	_, err = h.svc.Delete(as(alice), incident, 1, "")
	requireDenial(t, err, http.StatusConflict, TypeSessionMessage)
	_, err = h.svc.Edit(as(alice), incident, 3, props("note", "edited"))
	require.NoError(t, err)

	require.Contains(t, h.readAll(t, dave, incident).Messages[0].Actions, OpRemove)
	removed, err := h.svc.Delete(as(dave), incident, 1, "spam")
	require.NoError(t, err)
	require.NotNil(t, removed.Removed)
	h.requireSession(t, "contained", 1)
	require.Len(t, h.sessionAudit(t), 1, "the removal keeps the move in the audit trail")
	require.True(t, h.readAll(t, alice, incident).Messages[1].IsSessionNotice(), "the notice stays")
}

func TestSessionNoticeProtected(t *testing.T) {
	t.Parallel()
	h := newSessionHarness(t)
	alice := user("alice")
	h.postTo(t, alice, incident, "contained", nil)
	msgs := h.readAll(t, user("dave", "chat.moderate"), incident).Messages
	require.Len(t, msgs, 2)
	require.Nil(t, msgs[1].Actions)
	require.Contains(t, msgs[0].Actions, OpRemove, "a plain message still offers remove to a moderator")

	_, err := h.svc.Edit(as(alice), incident, 2, props("from", "x"))
	requireDenial(t, err, http.StatusConflict, TypeSessionNotice)
	_, err = h.svc.Delete(as(user("dave", "chat.moderate")), incident, 2, "spam")
	requireDenial(t, err, http.StatusConflict, TypeSessionNotice)
	_, err = h.svc.Delete(as(alice), incident, 2, "")
	requireDenial(t, err, http.StatusConflict, TypeSessionNotice)
}

// TestSessionNotice_Marker pins that a notice is recognized by both its
// type and its marker: a message typed session without the marker is not
// one.
func TestSessionNotice_Marker(t *testing.T) {
	t.Parallel()
	require.True(t, isNotice("session", "session:incident"))
	require.False(t, isNotice("session", "trigger:x"))
	require.False(t, isNotice("text", "session:incident"))
}

// postBucket returns a copy of a's per-channel bucket for posts of typ on
// the PoC run channel, or nil when there is none. A post that takes a
// token creates or changes it.
func (h *harness) postBucket(a *fakeAccess, typ string) *bucket {
	h.local.limiter.mu.Lock()
	defer h.local.limiter.mu.Unlock()
	ib := h.local.limiter.ids[bucketIdentity(a.p)]
	if ib == nil {
		return nil
	}
	b := ib.perOp[channelOp{key: pocKey(), op: opPostPfx + typ}]
	if b == nil {
		return nil
	}
	c := *b
	return &c
}

// requireNothingAppended fails unless the PoC run channel still ends at
// lastSeq.
func (h *harness) requireNothingAppended(t *testing.T, lastSeq int64) {
	t.Helper()
	head, err := h.store.Store.Head(context.Background(), pocKey())
	require.NoError(t, err)
	require.Equal(t, lastSeq, head.LastSeq, "a refused post stored something")
}

// TestPost_MessageIn pins AC 4 and AC 7: a post of a type outside its in:
// states is refused 409 session_wrong_state with the allowed states and
// the state it saw, stores nothing and spends no rate token; a stranded
// session admits only types without in:, a final state the types that list
// it, and a closed channel answers channel_not_open first.
func TestPost_MessageIn(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		who   *fakeAccess
		state string
		setup func(t *testing.T, h *harness)
		in    PostInput
		typ   string // empty: the post lands
		title string
		rule  string
	}{
		{name: "approve in working", who: pocOwner(), state: "working",
			in: PostInput{Type: "approve", Props: map[string]any{"note": "ok"}}, typ: TypeSessionWrongState,
			title: "approve may be posted only while the session is in plan-review; it is in working",
			rule:  "channel_types.run.message_types.approve.in"},
		{name: "two states listed", who: pocRunner(), state: "review",
			in: PostInput{Type: "progress", Props: map[string]any{"note": "n"}}, typ: TypeSessionWrongState,
			title: "progress may be posted only while the session is in planning, working; it is in review",
			rule:  "channel_types.run.message_types.progress.in"},
		{name: "unstored reads as initial", who: pocOwner(), state: "unstored",
			in: PostInput{Type: "approve", Props: map[string]any{"note": "ok"}}, typ: TypeSessionWrongState,
			title: "approve may be posted only while the session is in plan-review; it is in new",
			rule:  "channel_types.run.message_types.approve.in"},
		{name: "a type without in posts in every state", who: pocOwner(), state: "working",
			in: PostInput{Type: "text", Props: map[string]any{"body": "hi"}}},
		{name: "plain message where it moves nothing", who: pocOwner(), state: "plan-review",
			in: PostInput{Type: "approve", Props: map[string]any{"note": "ok"}}},
		{name: "final state admits a type that lists it", who: pocOwner(), state: "done",
			in: PostInput{Type: "thanks", Props: map[string]any{"note": "ty"}}},
		{name: "a type that lists only a final state", who: pocOwner(), state: "review",
			in: PostInput{Type: "thanks", Props: map[string]any{"note": "ty"}}, typ: TypeSessionWrongState,
			title: "thanks may be posted only while the session is in done; it is in review",
			rule:  "channel_types.run.message_types.thanks.in"},
		{name: "stranded refuses a type with in", who: pocRunner(), state: "working",
			setup: strandWorking, in: PostInput{Type: "progress", Props: map[string]any{"note": "n"}},
			typ: TypeSessionWrongState,
			title: "progress may be posted only while the session is in planning, busy; " +
				"the session is stranded in working",
			rule: "channel_types.run.message_types.progress.in"},
		{name: "stranded admits a type without in", who: pocRunner(), state: "working", setup: strandWorking,
			in: PostInput{Type: "text", Props: map[string]any{"body": "hi"}}},
		{name: "closed channel answers channel_not_open first", who: pocOwner(), state: "plan-review",
			setup: func(t *testing.T, h *harness) {
				t.Helper()
				_, err := h.svc.Lifecycle(as(pocOwner()), pocRun, realtimecfg.LifecycleClose)
				require.NoError(t, err)
			},
			in: PostInput{Type: "go", Props: map[string]any{"note": "n"}}, typ: TypeChannelNotOpen},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newPocHarness(t)
			h.pocDriveTo(t, tc.state)
			if tc.setup != nil {
				tc.setup(t, h)
			}
			var lastSeq int64
			if tc.state != "unstored" {
				head, err := h.store.Store.Head(context.Background(), pocKey())
				require.NoError(t, err)
				lastSeq = head.LastSeq
			}
			bucketBefore := h.postBucket(tc.who, tc.in.Type)
			res, err := h.svc.Post(as(tc.who), pocRun, tc.in)
			if tc.typ == "" {
				require.NoError(t, err)
				require.NotNil(t, res.Message)
				return
			}
			d := requireDenial(t, err, http.StatusConflict, tc.typ)
			if tc.title != "" {
				require.Equal(t, tc.title, d.Title)
				require.Equal(t, tc.rule, d.Rule)
			}
			if tc.state == "unstored" {
				_, err := h.store.Store.Head(context.Background(), pocKey())
				require.ErrorIs(t, err, channels.ErrNotFound, "a refused post created the channel")
			} else {
				h.requireNothingAppended(t, lastSeq)
			}
			require.Equal(t, bucketBefore, h.postBucket(tc.who, tc.in.Type), "a refused pre-check spent a rate token")
		})
	}
}

// strandWorking renames the PoC's working state, so a channel in working
// is stranded.
func strandWorking(t *testing.T, h *harness) {
	t.Helper()
	y := strings.NewReplacer("        working:\n", "        busy:\n", "to: working", "to: busy",
		"in: [planning, working]", "in: [planning, busy]", "in: [working]", "in: [busy]").
		Replace(pocRealtime(""))
	h.cfg.set(Snapshot{Config: testCompiled(t, y)})
}

// TestPost_MessageInStoreRefuses pins AC 5 for one post: when the session
// moves between the post's read and its append, the store refuses the
// post with the state it saw. The title names that state, not the head
// the call read; nothing is stored, and the token is spent like any failed
// append.
func TestPost_MessageInStoreRefuses(t *testing.T) {
	t.Parallel()
	h := newPocHarness(t)
	h.pocDriveTo(t, "plan-review")
	o := pocOwner()
	var lastSeq int64
	h.store.beforeAppend = func() {
		h.postTo(t, o, pocRun, "approve", map[string]any{"note": "go"})
		head, err := h.store.Store.Head(context.Background(), pocKey())
		require.NoError(t, err)
		lastSeq = head.LastSeq
	}
	require.Nil(t, h.postBucket(o, "revise"))
	_, err := h.svc.Post(as(o), pocRun, PostInput{Type: "revise", Props: map[string]any{"note": "again"}})
	d := requireDenial(t, err, http.StatusConflict, TypeSessionWrongState)
	require.Equal(t, "revise may be posted only while the session is in plan-review; it is in working", d.Title)
	require.Equal(t, "channel_types.run.message_types.revise.in", d.Rule)
	h.requireNothingAppended(t, lastSeq)
	require.NotNil(t, h.postBucket(o, "revise"), "a refusal by the store spends the token")
	v, err := h.svc.Head(as(o), pocRun)
	require.NoError(t, err)
	require.Equal(t, "working", v.Channel.Session.State)
}

// TestPost_MessageInEphemeral pins that an ephemeral type with in: is
// checked by the pre-check, the only check a signal meets.
func TestPost_MessageInEphemeral(t *testing.T) {
	t.Parallel()
	y := strings.Replace(pocRealtime(""), "    lifecycle:\n",
		"      nudge: { persist: ephemeral, ttl: 30s, post: [owner], in: [plan-review] }\n    lifecycle:\n", 1)
	h := newHarness(t, withYAML(y))
	h.pocDriveTo(t, "working")
	_, err := h.svc.Post(as(pocOwner()), pocRun, PostInput{Type: "nudge"})
	requireDenial(t, err, http.StatusConflict, TypeSessionWrongState)
}

// branchRealtime declares review, a session type whose message types lead
// to several states each (TKT-FPXTQJ). alice owns TKT-1, so she holds
// lead; plan-review waits for her.
const branchRealtime = `
version: 1
channel_types:
  review:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      lead:   { from: entity.owner }
      member: { from: attached }
    message_types:
      text:   { properties: { body: { type: markdown, required: true } }, post: [member] }
      decide: { properties: { decision: { type: plan-decision } }, post: [lead, member] }
      flag:   { properties: { note: { type: string } }, post: [lead, member] }
      ping:   { properties: { note: { type: string } }, post: [lead, member] }
      probe:  { properties: { pattern: { type: string } }, post: [member] }
    session:
      initial: plan-review
      states:
        plan-review:
          waiting_for: [lead]
          on:
            decide:
              - { to: working, when: "message.decision == 'approve'" }
              - { to: planning, when: "message.decision == 'revise'" }
              - { to: done, when: "message.decision == 'approve'" }
            flag:
              - { to: escalated, roles: [lead] }
              - { to: noted }
            ping:
              - { to: escalated, by: trigger }
              - { to: noted }
            probe:
              - { to: working, when: "regex('a', message.pattern)" }
              - { to: plan-review }
        working:   { final: true }
        planning:  { final: true }
        escalated: { final: true }
        noted:     { final: true }
        done:      { final: true }
`

var reviewCh = RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "review"}

func reviewKey() channels.Key {
	return channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "review"}
}

// reviewSession reads the review channel's session as alice.
func (h *harness) reviewSession(t *testing.T) *SessionView {
	t.Helper()
	s := h.readAll(t, user("alice"), reviewCh).Channel.Session
	require.NotNil(t, s)
	return s
}

// postAsTrigger posts typ into the review channel as a trigger would.
func (h *harness) postAsTrigger(t *testing.T, typ string, p map[string]any) {
	t.Helper()
	snap := h.cfg.Load()
	ct, ok := snap.Config.ChannelType("review")
	require.True(t, ok)
	w := &systemWriter{store: h.store, local: h.local, cfg: h.cfg, dueWake: h.wake}
	_, err := w.post(context.Background(), snap, h.clock.Now(), reviewKey(), ct, ct.MessageTypes[typ], p,
		"trigger:test", true)
	require.NoError(t, err)
}

// TestSessionBranches_FirstWins pins TKT-FPXTQJ AC 2: entries are tried in
// file order and the first that applies moves the session; of two that
// both hold, the first wins.
func TestSessionBranches_FirstWins(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ decision, to string }{{"revise", "planning"}, {"approve", "working"}} {
		t.Run(tc.decision, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t, withYAML(branchRealtime))
			res := h.postTo(t, user("bob"), reviewCh, "decide", props("decision", tc.decision))
			require.Equal(t, tc.to, res.Session.State)
			require.Equal(t, int64(1), res.Session.Generation)
			msgs := h.readAll(t, user("bob"), reviewCh).Messages
			require.Len(t, msgs, 2)
			require.True(t, msgs[1].IsSessionNotice())
			require.Equal(t, "plan-review", msgs[1].Props["from"])
			require.Equal(t, tc.to, msgs[1].Props["to"])
		})
	}
}

// TestSessionBranches_PosterRulePerEntry pins TKT-FPXTQJ AC 3: each entry
// has its own poster rule, and a by: trigger entry is skipped for a
// person's post.
func TestSessionBranches_PosterRulePerEntry(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, typ, to string
		post          func(t *testing.T, h *harness, typ string)
	}{
		{"lead takes the roles entry", "flag", "escalated", func(t *testing.T, h *harness, typ string) {
			t.Helper()
			h.postTo(t, user("alice"), reviewCh, typ, nil)
		}},
		{"member falls through", "flag", "noted", func(t *testing.T, h *harness, typ string) {
			t.Helper()
			h.postTo(t, user("bob"), reviewCh, typ, nil)
		}},
		{"trigger skips the roles entry", "flag", "noted", func(t *testing.T, h *harness, typ string) {
			t.Helper()
			h.postAsTrigger(t, typ, nil)
		}},
		{"person skips the by: trigger entry", "ping", "noted", func(t *testing.T, h *harness, typ string) {
			t.Helper()
			h.postTo(t, user("alice"), reviewCh, typ, nil)
		}},
		{"trigger takes the by: trigger entry", "ping", "escalated", func(t *testing.T, h *harness, typ string) {
			t.Helper()
			h.postAsTrigger(t, typ, nil)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t, withYAML(branchRealtime))
			tc.post(t, h, tc.typ)
			s := h.reviewSession(t)
			require.Equal(t, tc.to, s.State)
			require.Equal(t, int64(1), s.Generation)
		})
	}
}

// TestSessionBranches_NoMatchStoresWithoutMove pins TKT-FPXTQJ AC 4: a
// post that matches no entry is stored and the session stays, with no
// notice.
func TestSessionBranches_NoMatchStoresWithoutMove(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(branchRealtime))
	res := h.postTo(t, user("bob"), reviewCh, "decide", nil)
	require.Equal(t, "plan-review", res.Session.State)
	require.Equal(t, int64(0), res.Session.Generation)
	msgs := h.readAll(t, user("bob"), reviewCh).Messages
	require.Len(t, msgs, 1, "no notice")
	require.Equal(t, "decide", msgs[0].Type)
}

// TestSessionBranches_GuardErrorStops pins TKT-FPXTQJ AC 5: a guard that
// fails to evaluate stops the decision, so the message is stored and the
// session does not move, even though a later entry would apply. The
// failure is logged once per entry with its key. Not parallel: it captures
// the default logger.
func TestSessionBranches_GuardErrorStops(t *testing.T) {
	h := newHarness(t, withYAML(branchRealtime))
	logs := logtest.Capture(t, slog.LevelWarn)
	for range 2 {
		res := h.postTo(t, user("bob"), reviewCh, "probe", props("pattern", "("))
		require.Equal(t, "plan-review", res.Session.State)
		require.Equal(t, int64(0), res.Session.Generation, "the catch-all after the failed guard is not tried")
	}
	require.Len(t, h.readAll(t, user("bob"), reviewCh).Messages, 2, "both posts are stored, with no notice")
	out := logs()
	const key = "entry=channel_types.review.session.states.plan-review.on.probe[0]"
	require.Equal(t, 1, strings.Count(out, key), out)
	require.Contains(t, out, "the session did not move")
	require.Contains(t, out, "to=working")

	// A pattern the guard can evaluate still takes the entries in order.
	res := h.postTo(t, user("bob"), reviewCh, "probe", props("pattern", "b"))
	require.Equal(t, "plan-review", res.Session.State)
	require.Equal(t, int64(1), res.Session.Generation, "the catch-all re-enters")
}

// TestChoices_PoCFlow is the integration test of TKT-03FQPD: the runner
// asks with options, the owner's click posts the answer as a reply at the
// head generation and moves the session on, the runner gets a reply row,
// and the same click again is refused. In the PoC session the answer type
// has in: [plan-asking, asking], so the state check refuses the stale
// click with session_wrong_state. Without in:, the stale click reaches the
// generation check and is refused with session_moved.
func TestChoices_PoCFlow(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		answerIn  bool
		staleType string
	}{
		{"PoC session", true, TypeSessionWrongState},
		{"answer type without in", false, TypeSessionMoved},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			y := strings.Replace(pocRealtime(""),
				"ask:        { properties: { note: { type: string } }, post: [runner], in: [planning, working] }",
				"ask:        { properties: { note: { type: string }, options: { type: string, list: true, max: 80, "+
					"max_items: 4 } }, post: [runner], in: [planning, working], "+
					"choices: { from: options, answer: answer, into: note, other: true } }", 1)
			require.Contains(t, y, "choices:")
			if !tc.answerIn {
				const in = "post: [owner], in: [plan-asking, asking] }"
				require.Contains(t, y, in)
				y = strings.Replace(y, in, "post: [owner] }", 1)
			}
			h := newHarness(t, withYAML(y))
			h.pocDriveTo(t, "working")
			o, r := pocOwner(), pocRunner()

			q := h.postTo(t, r, pocRun, "ask", map[string]any{"note": "Which one?", "options": []any{"A", "B"}})
			require.Equal(t, "asking", q.Session.State)
			gen := q.Session.Generation
			require.Equal(t, []any{"A", "B"}, q.Message.Props["options"])

			click := PostInput{Type: "answer", Props: map[string]any{"note": "B"}, ReplyTo: q.Message.Seq,
				ExpectGeneration: &gen}
			res, err := h.svc.Post(as(o), pocRun, click)
			require.NoError(t, err)
			require.Equal(t, "working", res.Session.State)
			require.Equal(t, q.Message.Seq, res.Message.ReplyTo)

			inbox := h.inbox(t, r, InboxRequest{Kinds: []string{InboxKindReply}})
			require.Len(t, inbox.Mentions, 1)
			require.Equal(t, res.Message.Seq, inbox.Mentions[0].Seq)

			_, err = h.svc.Post(as(o), pocRun, click)
			requireDenial(t, err, http.StatusConflict, tc.staleType)
		})
	}
}

// buttonRealtime declares plan, whose decide entries carry one-click
// buttons (TKT-S9TLJJ). The roles come from permissions, so a caller holds
// exactly the roles a test gives them. Entry 0 is reviewer-only with a
// guard on the reminder count, which the load cannot decide; entry 1 is
// lead-only without a guard; entry 2 is auditor-only with a guard that
// holds for the Approve preset; entry 3 is the member-only Approve button
// and entry 4 the Revise button. Each entry leads to its own state, so the
// state a post reaches names the entry that fired.
func buttonRealtime() string {
	return `
version: 1
channel_types:
  plan:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      lead:     { permission: plan.lead }
      member:   { permission: plan.member }
      reviewer: { permission: plan.review }
      auditor:  { permission: plan.audit }
    message_types:
      text: { properties: { body: { type: string } }, post: [lead, member, reviewer] }
      ping: { properties: { body: { type: string } }, post: [lead] }
      decide:
        label: Decide
        properties:
          decision: { type: plan-decision, required: true }
          count:    { type: integer }
          tags:     { type: string, list: true, max: 20, max_items: 3 }
        post: [lead, member, reviewer, auditor]
    lifecycle:
      close: { roles: [member] }
    session:
      initial: plan-review
      states:
        plan-review:
          on:
            decide:
              - { to: nudged, roles: [reviewer], when: "session.reminders.nudge > 0" }
              - { to: escalated, roles: [lead] }
              - { to: audited, roles: [auditor], when: "message.decision == 'approve'" }
              - to: working
                roles: [member]
                when: "message.decision == 'approve'"
                button: { label: Approve, props: { decision: approve } }
                outputs: [ { set: { properties: { status: in-progress } } } ]
              - to: planning
                button: { label: Revise, props: { decision: revise, count: 2, tags: [ a, ' b ' ] } }
        nudged:    { final: true }
        escalated: { final: true }
        audited:   { final: true }
        working:   { final: true }
        planning:  { final: true }
      reminders:
        nudge: { in: [plan-review], every: 1h, post: { type: ping, props: { body: nudge } } }
`
}

var planCh = RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "plan"}

// buttonPrincipals are the callers of the plan fixture, by the roles they
// hold.
func buttonPrincipals() map[string]*fakeAccess {
	return map[string]*fakeAccess{
		"lead":            user("lena", "plan.lead"),
		"member":          user("mo", "plan.member"),
		"reviewer":        user("rex", "plan.review"),
		"lead+member":     user("lm", "plan.lead", "plan.member"),
		"member+reviewer": user("mr", "plan.member", "plan.review"),
		"auditor":         user("aud", "plan.audit"),
		"auditor+member":  user("am", "plan.audit", "plan.member"),
	}
}

// TestSessionView_Button pins TKT-S9TLJJ AC 6: each row of
// session.transitions carries its entry, and a button entry's row its
// label and props in stored form. Both are config, the same for every
// reader.
func TestSessionView_Button(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(buttonRealtime()), withOutputs(&fakeOutputs{}))
	var rows []SessionTransitionView
	for who, a := range buttonPrincipals() {
		s := h.readAll(t, a, planCh).Channel.Session
		require.NotNil(t, s, who)
		if rows == nil {
			rows = s.Transitions
			continue
		}
		require.Equal(t, rows, s.Transitions, "%s reads other rows", who)
	}
	var decide []SessionTransitionView
	for _, r := range rows {
		if r.MessageType == "decide" {
			decide = append(decide, r)
		}
	}
	require.Equal(t, []SessionTransitionView{
		{MessageType: "decide", To: "nudged", Guarded: true, Entry: 0},
		{MessageType: "decide", To: "escalated", Entry: 1},
		{MessageType: "decide", To: "audited", Guarded: true, Entry: 2},
		{MessageType: "decide", To: "working", Guarded: true, Outputs: 1, Entry: 3,
			Button: &SessionButtonView{Label: "Approve", Props: map[string]any{"decision": "approve"}}},
		{MessageType: "decide", To: "planning", Entry: 4, Button: &SessionButtonView{Label: "Revise",
			Props: map[string]any{"decision": "revise", "count": int64(2), "tags": []any{"a", "b"}}}},
	}, decide)
	b, err := json.Marshal(decide[4])
	require.NoError(t, err)
	require.JSONEq(t, `{"message_type":"decide","to":"planning","entry":4,"button":{"label":"Revise",`+
		`"props":{"decision":"revise","count":2,"tags":["a","b"]}}}`, string(b))
}

// TestButtons_DeciderAgrees pins TKT-S9TLJJ AC 8 against the real
// decider: for every caller and button, on a fresh channel, a preset post
// with expect_generation moves the session along the button's entry
// exactly when actions.entries lists it. The cases cover each reason the
// load hides a button:
//   - an earlier unguarded entry (lead): the post takes it, also for a
//     caller who holds the button entry's role too (lead+member);
//   - an earlier entry the load cannot judge (reviewer): hidden even where
//     the post would pass it;
//   - an earlier guarded entry whose guard holds for the preset and whose
//     role the button's entry does not admit (auditor+member): the post
//     takes it, while the Revise preset, for which it does not hold, stays
//     listed.
func TestButtons_DeciderAgrees(t *testing.T) {
	t.Parallel()
	presets := map[int]map[string]any{
		3: {"decision": "approve"},
		4: {"decision": "revise", "count": 2, "tags": []any{"a", " b "}},
	}
	own := map[int]string{3: "working", 4: "planning"}
	tests := []struct {
		who string
		// listed and to are, by entry, whether actions.entries lists it and
		// where its preset post goes.
		listed map[int]bool
		to     map[int]string
	}{
		{who: "member", listed: map[int]bool{3: true, 4: true}, to: own},
		{who: "lead", to: map[int]string{3: "escalated", 4: "escalated"}},
		{who: "lead+member", to: map[int]string{3: "escalated", 4: "escalated"}},
		{who: "reviewer", to: map[int]string{3: "planning", 4: "planning"}},
		{who: "member+reviewer", to: own},
		{who: "auditor", listed: map[int]bool{4: true}, to: map[int]string{3: "audited", 4: "planning"}},
		{who: "auditor+member", listed: map[int]bool{4: true}, to: map[int]string{3: "audited", 4: "planning"}},
	}
	for _, tc := range tests {
		for entry, preset := range presets {
			t.Run(fmt.Sprintf("%s entry %d", tc.who, entry), func(t *testing.T) {
				t.Parallel()
				a := buttonPrincipals()[tc.who]
				h := newHarness(t, withYAML(buttonRealtime()), withOutputs(&fakeOutputs{}))
				head, err := h.svc.Head(as(a), planCh)
				require.NoError(t, err)
				listed := slices.Contains(head.Actions.Entries, EntryRef{MessageType: "decide", Entry: entry})
				require.Equal(t, tc.listed[entry], listed, "entries %v", head.Actions.Entries)
				res, err := h.svc.Post(as(a), planCh,
					PostInput{Type: "decide", Props: preset, ExpectGeneration: gen(0)})
				require.NoError(t, err)
				require.Equal(t, tc.to[entry], res.Session.State)
				if listed {
					require.Equal(t, own[entry], res.Session.State, "a listed button's post took another entry")
				}
			})
		}
	}
}
