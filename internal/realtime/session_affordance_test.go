package realtime

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// exampleStates is the session of the breach-response example of the docs
// plus flag, a transition only dpo fires on a type every member may post.
const exampleStates = `
        detected:
          label: Detected
          on:
            contained: { to: contained, roles: [lead] }
            dismiss:   { to: dismissed }
            escalated: { to: contained, by: trigger }
            flag:      { to: contained, roles: [dpo] }
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

// exampleRealtime declares breach, the breach-response example of the docs
// on tickets, and chat, a type without a session. alice owns TKT-1, so she
// holds lead.
func exampleRealtime(states string) string {
	return `
version: 1
channel_types:
  breach:
    label: Breach response
    attach_to: [ticket]
    read: { from: attached }
    roles:
      lead:   { from: entity.owner }
      member: { from: attached }
      dpo:    { permission: privacy.officer }
    message_types:
      text:      { properties: { body: { type: markdown, required: true } }, post: [member] }
      contained: { properties: { note: { type: markdown } }, post: [lead, member] }
      assessed:  { properties: { risk: { type: risk-level, required: true } }, post: [lead, dpo] }
      notified:  { properties: { reference: { type: string, required: true } }, post: [dpo] }
      dismiss:   { properties: { reason: { type: string, required: true } }, post: [lead] }
      escalated: { properties: { note: { type: string } }, post: [lead] }
      flag:      { properties: { note: { type: string } }, post: [member, dpo] }
    lifecycle:
      close:  { roles: [member] }
      reopen: { roles: [member] }
    session:
      label: GDPR breach response
      initial: detected
      states:` + states + `
  chat:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      text: { properties: { body: { type: string } }, post: [member] }
`
}

var (
	breach  = RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "breach"}
	plainCh = RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "chat"}
)

// exampleProps are props for each message type of breach that satisfy
// every guard.
var exampleProps = map[string]map[string]any{
	"text":      {"body": "hi"},
	"contained": {"note": "done"},
	"assessed":  {"risk": "low"},
	"notified":  {"reference": "AP-1"},
	"dismiss":   {"reason": "false alarm"},
	"escalated": {"note": "up"},
	"flag":      {"note": "look"},
}

// exampleStateNames are the states a test drives the breach channel to;
// "unstored" is the cardinality-one channel before its first message.
var exampleStateNames = []string{"unstored", "detected", "contained", "assessed", "notified", "dismissed"}

func newExampleHarness(t *testing.T) *harness {
	t.Helper()
	return newHarness(t, withYAML(exampleRealtime(exampleStates)))
}

// driveTo moves the breach channel to state by posts of alice and erin.
func (h *harness) driveTo(t *testing.T, state string) {
	t.Helper()
	alice, erin := user("alice"), user("erin", "privacy.officer")
	steps := map[string][][2]any{
		"unstored":  nil,
		"detected":  {{user("bob"), "text"}},
		"contained": {{alice, "contained"}},
		"assessed":  {{alice, "contained"}, {alice, "assessed"}},
		"notified":  {{alice, "contained"}, {alice, "assessed"}, {erin, "notified"}},
		"dismissed": {{alice, "dismiss"}},
	}
	for _, s := range steps[state] {
		typ := s[1].(string)
		h.postTo(t, s[0].(*fakeAccess), breach, typ, exampleProps[typ])
	}
	if state != "unstored" {
		v, err := h.svc.Head(as(alice), breach)
		require.NoError(t, err)
		want := state
		require.Equal(t, want, v.Channel.Session.State)
	}
}

func (h *harness) headActions(t *testing.T, a *fakeAccess, addr RawAddress) Affordances {
	t.Helper()
	v, err := h.svc.Head(as(a), addr)
	require.NoError(t, err)
	return v.Actions
}

// examplePrincipals are the callers of the breach fixture: the lead, a
// member, the privacy officer, and the privacy officer under a client
// ceiling that denies privacy.officer.
func examplePrincipals() map[string]*fakeAccess {
	ceiled := user("erin", "privacy.officer")
	ceiled.deniedPerms = map[string]bool{"privacy.officer": true}
	return map[string]*fakeAccess{
		"lead": user("alice"), "member": user("bob"), "dpo": user("erin", "privacy.officer"), "dpo-ceiled": ceiled,
	}
}

// TestDescribe_Session pins AC 1: a session type describes its label,
// initial state and states in declaration order, and no transitions; a
// type without a session has none.
func TestDescribe_Session(t *testing.T) {
	t.Parallel()
	h := newExampleHarness(t)
	types := map[string]ChannelTypeView{}
	for _, ct := range h.svc.Describe() {
		types[ct.Name] = ct
	}
	tests := []struct {
		name string
		want *SessionDescription
	}{
		{name: "breach", want: &SessionDescription{
			Label: "GDPR breach response", Initial: "detected",
			States: []SessionStateDescription{
				{Name: "detected", Label: "Detected", Status: "working"},
				{Name: "contained", Label: "Contained", Status: "working"},
				{Name: "assessed", Label: "Risk assessed", Status: "working"},
				{Name: "notified", Label: "AP notified", Final: true, Status: "completed"},
				{Name: "dismissed", Label: "Dismissed", Final: true, Status: "completed"},
			},
		}},
		{name: "chat"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, types[tc.name].Session)
		})
	}

	t.Run("labels default to names", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, withYAML(sessionRealtime("detected", sessionStates)))
		for _, ct := range h.svc.Describe() {
			if ct.Name != "cases" {
				continue
			}
			require.Equal(t, &SessionDescription{
				Label: "cases", Initial: "open-case",
				States: []SessionStateDescription{
					{Name: "open-case", Label: "open-case", Status: "working"},
					{Name: "finished", Label: "finished", Final: true, Status: "completed"},
				},
			}, ct.Session)
		}
	})
}

// TestAffordances_SessionTransitions pins AC 3: the fireable transitions
// per caller and channel state.
func TestAffordances_SessionTransitions(t *testing.T) {
	t.Parallel()
	ps := examplePrincipals()
	tests := []struct {
		name   string
		who    string
		state  string
		addr   RawAddress
		setup  func(t *testing.T, h *harness)
		post   []string // must be in post; nil skips the check
		expect []string
	}{
		{name: "lead in detected", who: "lead", state: "detected", expect: []string{"contained", "dismiss"}},
		{name: "member in detected", who: "member", state: "detected", expect: []string{}},
		{name: "dpo in contained lists a guarded transition", who: "dpo", state: "contained",
			expect: []string{"assessed"}},
		{name: "final state", who: "dpo", state: "notified", expect: []string{}},
		{name: "unstored reads as initial", who: "lead", state: "unstored", expect: []string{"contained", "dismiss"}},
		{name: "dpo holds flag", who: "dpo", state: "detected", expect: []string{"flag"}},
		{name: "ceiling denies the transition role, not the post role", who: "dpo-ceiled", state: "detected",
			post: []string{"flag"}, expect: []string{}},
		{name: "closed channel", who: "lead", state: "detected", expect: []string{},
			setup: func(t *testing.T, h *harness) {
				t.Helper()
				_, err := h.svc.Lifecycle(as(user("alice")), breach, realtimecfg.LifecycleClose)
				require.NoError(t, err)
			}},
		{name: "stranded state", who: "dpo", state: "contained", expect: []string{},
			setup: func(t *testing.T, h *harness) {
				t.Helper()
				without := `
        detected:
          on:
            dismiss: { to: dismissed }
        dismissed: { final: true }
`
				h.cfg.set(Snapshot{Config: testCompiled(t, exampleRealtime(without))})
			}},
		{name: "type without a session", who: "lead", state: "unstored", addr: plainCh, post: []string{"text"},
			expect: []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newExampleHarness(t)
			h.driveTo(t, tc.state)
			if tc.setup != nil {
				tc.setup(t, h)
			}
			addr := breach
			if tc.addr != (RawAddress{}) {
				addr = tc.addr
			}
			a := h.headActions(t, ps[tc.who], addr)
			for _, mt := range tc.post {
				require.Contains(t, a.Post, mt)
			}
			require.Equal(t, tc.expect, a.Transitions)
		})
	}
}

// TestAffordances_TransitionsSubsetOfPost pins AC 4: transitions is a
// subset of post for every caller and state, and an empty JSON array, not
// null, for a caller without identity.
func TestAffordances_TransitionsSubsetOfPost(t *testing.T) {
	t.Parallel()
	for _, state := range exampleStateNames {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			h := newExampleHarness(t)
			h.driveTo(t, state)
			for who, a := range examplePrincipals() {
				aff := h.headActions(t, a, breach)
				require.NotNil(t, aff.Transitions, who)
				for _, mt := range aff.Transitions {
					require.Contains(t, aff.Post, mt, "%s in %s", who, state)
				}
			}
		})
	}
	t.Run("no identity", func(t *testing.T) {
		t.Parallel()
		h := newExampleHarness(t)
		anon := user("")
		anon.p = principal.Principal{}
		aff := h.headActions(t, anon, breach)
		b, err := json.Marshal(aff)
		require.NoError(t, err)
		require.Contains(t, string(b), `"transitions":[]`)
	})
}

// TestAffordances_TransitionsAgreeWithPost pins AC 5: for every caller,
// state and postable type, a post with guard-satisfying props moves the
// session exactly when the type is in transitions.
func TestAffordances_TransitionsAgreeWithPost(t *testing.T) {
	t.Parallel()
	for who, a := range examplePrincipals() {
		for _, state := range exampleStateNames {
			h := newExampleHarness(t)
			h.driveTo(t, state)
			aff := h.headActions(t, a, breach)
			for _, mt := range aff.Post {
				t.Run(who+"/"+state+"/"+mt, func(t *testing.T) {
					t.Parallel()
					h := newExampleHarness(t)
					h.driveTo(t, state)
					before, err := h.svc.Head(as(a), breach)
					require.NoError(t, err)
					res := h.postTo(t, a, breach, mt, exampleProps[mt])
					moved := res.Session.Generation > before.Channel.Session.Generation
					require.Equal(t, slices.Contains(aff.Transitions, mt), moved,
						"listed %v, moved %v", aff.Transitions, moved)
				})
			}
		}
	}
}

// fanRealtime is a session whose current state, s0, has n transitions,
// each by its own message type, that share one permission role (officer)
// and one entity-property role (lead). Every other state has a transition
// with roles of its own (officer-i, lead-i). Post roles start with member,
// which every caller holds, so a post check asks no other role.
func fanRealtime(n int) string {
	var types, fan, rest, roles strings.Builder
	for i := range n {
		fmt.Fprintf(&types, "      m%d: { properties: { body: { type: string } }, post: [member, officer, lead] }\n", i)
		fmt.Fprintf(&fan, "            m%d: { to: s1, roles: [officer, lead] }\n", i)
	}
	textPost := []string{"member"}
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&roles, "      officer-%d: { permission: perm.%d }\n      lead-%d: { from: entity.owner }\n", i, i, i)
		textPost = append(textPost, fmt.Sprintf("officer-%d", i), fmt.Sprintf("lead-%d", i))
		next := fmt.Sprintf("s%d", i+1)
		if i == n {
			next = "done"
		}
		fmt.Fprintf(&rest, "        s%d: { on: { text: { to: %s, roles: [officer-%d, lead-%d] } } }\n", i, next, i, i)
	}
	return `
version: 1
channel_types:
  fan:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      member:  { from: attached }
      officer: { permission: perm.0 }
      lead:    { from: entity.owner }
` + roles.String() + `    message_types:
      text: { properties: { body: { type: string } }, post: [` + strings.Join(textPost, ", ") + `] }
` + types.String() + `    session:
      initial: s0
      states:
        s0:
          on:
` + fan.String() + rest.String() + `        done: { final: true }
`
}

// TestAffordances_RolesAskedOnce pins that a head asks each distinct role
// of the current state's transitions once, and no role of another state:
// the permission and entity-property calls do not grow with the
// transitions out of the current state, nor with the session's size.
func TestAffordances_RolesAskedOnce(t *testing.T) {
	t.Parallel()
	t.Run("a role shared by a post and a transition", func(t *testing.T) {
		t.Parallel()
		h := newExampleHarness(t)
		erin := user("erin", "privacy.officer")
		h.headActions(t, erin, breach)
		require.Equal(t, int64(1), erin.permCalls.Load(), "dpo is asked once for post and transitions")
	})
	for _, n := range []int{1, 10} {
		t.Run(fmt.Sprintf("%d transitions out of the current state", n), func(t *testing.T) {
			t.Parallel()
			h := newHarness(t, withYAML(fanRealtime(n)))
			bob := user("bob")
			got := h.headActions(t, bob, RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "fan"})
			require.Empty(t, got.Transitions, "bob holds neither officer nor lead")
			require.Len(t, got.Post, n+1)
			require.Equal(t, int64(1), bob.permCalls.Load(), "officer is asked once; no officer-i is asked")
			require.Equal(t, int64(1), bob.identityCalls.Load(), "lead is asked once; no lead-i is asked")
		})
	}
}

// creatorRealtime is a many type whose transition needs the creator role:
// a creator role exists only on a many type, created before its first post.
const creatorRealtime = `
version: 1
channel_types:
  cases:
    attach_to: [ticket]
    cardinality: many
    read: { from: attached }
    create: { roles: [member] }
    roles:
      member: { from: attached }
      opener: { creator: true }
    message_types:
      text: { properties: { body: { type: string } }, post: [member, opener] }
    session:
      initial: triage
      states:
        triage:
          on:
            text: { to: open, roles: [opener] }
        open: { final: true }
`

// TestSession_FirstPostByCreatorMoves pins that the creator's first post
// moves the session through a creator role, and that a creator answer
// asked before the call holds a stored channel is not kept.
func TestSession_FirstPostByCreatorMoves(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		poster string
		moved  bool
	}{
		{name: "creator", poster: "alice", moved: true},
		{name: "not the creator", poster: "bob", moved: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t, withYAML(creatorRealtime))
			ch, err := h.svc.Create(as(user("alice")), "ticket", "TKT-1", "cases", CreateInput{})
			require.NoError(t, err)
			addr := RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "cases", Name: ch.Name}
			p := user(tc.poster)
			want := []string{}
			if tc.moved {
				want = []string{"text"}
			}
			require.Equal(t, want, h.headActions(t, p, addr).Transitions)
			res := h.postTo(t, p, addr, "text", map[string]any{"body": "first"})
			require.NotNil(t, res.Session)
			require.Equal(t, tc.moved, res.Session.State == "open", "state %s", res.Session.State)
		})
	}

	t.Run("a creator answer without a stored channel is not kept", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, withYAML(creatorRealtime))
		c, err := h.svc.begin(as(user("alice")))
		require.NoError(t, err)
		ct, ok := c.snap.Config.ChannelType("cases")
		require.True(t, ok)
		c.ct = ct
		require.False(t, c.heldRole("opener"))
		require.NotContains(t, c.ops.roles, "opener")
		c.head = &channels.Channel{CreatedBy: authorOf(c.p)}
		require.True(t, c.heldRole("opener"))
		require.True(t, c.ops.roles["opener"])
	})
}
