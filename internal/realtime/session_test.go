package realtime

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
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
		require.True(t, s.EnteredAt.Equal(now))
		// The guard compares session.entered_at with the posted time.
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
