package realtimecfg

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// TestParse_Session pins AC 1, 3 and 5: the breach-response session of the
// golden file compiles with its states in declaration order, a state named
// like a Windows device loads, and states given through an alias keep the
// alias's key order.
func TestParse_Session(t *testing.T) {
	c, err := Parse([]byte(golden(t)), testMeta(t), testPolicy(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	room, _ := c.ChannelType("incident-room")
	if room.Session() != nil {
		t.Error("incident-room has a session it does not declare")
	}
	ct, ok := c.ChannelType("breach-response")
	if !ok || ct.Session() == nil {
		t.Fatal("breach-response has no session")
	}
	s := ct.Session()
	if s.Label() != "GDPR breach response" || s.Initial() != "detected" {
		t.Errorf("label, initial = %q, %q", s.Label(), s.Initial())
	}
	var names []string
	var final []string
	for _, st := range s.States() {
		names = append(names, st.Name)
		if st.Final {
			final = append(final, st.Name)
		}
	}
	if want := []string{"detected", "contained", "assessed", "overdue", "notified", "dismissed"}; !slices.Equal(names, want) {
		t.Errorf("states = %v; want declaration order %v", names, want)
	}
	if !slices.Equal(final, []string{"notified", "dismissed"}) {
		t.Errorf("final states = %v", final)
	}
	detected, _ := s.State("detected")
	if detected.Label != "Detected" {
		t.Errorf("detected label = %q", detected.Label)
	}
	var keyed []string
	for _, tr := range detected.Transitions() {
		keyed = append(keyed, tr.MessageType+">"+tr.To)
	}
	if want := []string{"contained>contained", "dismiss>dismissed", "escalated>contained"}; !slices.Equal(keyed, want) {
		t.Errorf("detected transitions = %v; want %v", keyed, want)
	}
	tr, ok := s.Transition("detected", "contained")
	if !ok || !slices.Equal(tr.Roles, []string{"lead"}) || tr.By != TransitionByAny || tr.When != nil {
		t.Errorf("detected.contained = %+v", tr)
	}
	if tr, _ := s.Transition("detected", "escalated"); tr.By != TransitionByTrigger || len(tr.Roles) != 0 {
		t.Errorf("detected.escalated = %+v", tr)
	}
	if tr, _ := s.Transition("contained", "assessed"); tr.When == nil {
		t.Error("contained.assessed lost its guard")
	}
	if _, ok := s.Transition("notified", "text"); ok {
		t.Error("a final state has a transition")
	}
	if !s.Drives("dismiss") || s.Drives("text") {
		t.Error("Drives does not follow the on: entries")
	}

	const aliased = `version: 1
channel_types:
  first:
    attach_to: [datalek]
    roles: { member: { from: attached } }
    message_types:
      text: { post: [member] }
    session:
      initial: zeta
      states: &base
        zeta: { on: { text: { to: aux } } }
        aux:  { final: true }
  second:
    attach_to: [datalek]
    roles: { member: { from: attached } }
    message_types:
      text: { post: [member] }
    session:
      initial: zeta
      states: *base
`
	c, err = Parse([]byte(aliased), testMeta(t), nil)
	if err != nil {
		t.Fatalf("Parse with an aliased states map: %v", err)
	}
	for _, name := range []string{"first", "second"} {
		ct, _ := c.ChannelType(name)
		var got []string
		for _, st := range ct.Session().States() {
			got = append(got, st.Name)
		}
		if !slices.Equal(got, []string{"zeta", "aux"}) {
			t.Errorf("%s states = %v; want the file order zeta, aux", name, got)
		}
		if tr, ok := ct.Session().Transition("zeta", "text"); !ok || tr.To != "aux" {
			t.Errorf("%s cannot move into aux: %+v", name, tr)
		}
	}
}

// sessionBase is the realtime.yaml TestParse_SessionInvalid edits.
const sessionBase = `version: 1
channel_types:
  br:
    attach_to: [datalek]
    roles:
      lead:   { from: entity.owner }
      member: { from: attached }
    message_types:
      text:      { properties: { body: { type: string } }, post: [member] }
      contained: { properties: { risk: { type: risk-level } }, post: [lead, member] }
      typing:    { persist: ephemeral, post: [member] }
    session:
      initial: open
      states:
        open:
          on:
            contained: { to: done }
        done: { final: true }
`

// TestParse_SessionInvalid pins AC 2: each invalid session reports its key
// path and reason.
func TestParse_SessionInvalid(t *testing.T) {
	const k = "channel_types.br.session"
	const on = k + ".states.open.on.contained"
	var many strings.Builder
	many.WriteString("      states:\n        open:\n          on:\n            contained: { to: done }\n" +
		"        done: { final: true }\n")
	for i := range 63 {
		fmt.Fprintf(&many, "        s%d: { final: true }\n", i)
	}
	states := "      states:\n        open:\n          on:\n            contained: { to: done }\n        done: { final: true }\n"
	cases := []struct {
		name   string
		edits  []replace
		key    string
		reason string
	}{
		{"empty states", []replace{{states, "      states: {}\n"}}, k + ".states", "a session needs at least one state"},
		{"too many states", []replace{{states, many.String()}}, k + ".states", "at most 64 states"},
		{"bad state name", []replace{{"done: { final: true }", "Done: { final: true }"}},
			k + ".states.Done", "state name must match ^[a-z][a-z0-9-]{0,62}$"},
		{"missing initial", []replace{{"      initial: open\n", ""}}, k + ".initial", "name the state a new session starts in"},
		{"undeclared initial", []replace{{"initial: open", "initial: nope"}},
			k + ".initial", `unknown state "nope"; declare it under states`},
		{"initial final", []replace{{"initial: open", "initial: done"}}, k + ".initial", "the initial state cannot be final"},
		{"final with on", []replace{{"done: { final: true }", "done: { final: true, on: { text: { to: open } } }"}},
			k + ".states.done.on", "a final state has no transitions; drop on or remove final: true"},
		{"non-final without on", []replace{{"done: { final: true }", "done: { label: Done }"}},
			k + ".states.done", "a state that is not final needs a transition under on or a timer under after"},
		{"on key not a message type", []replace{{"contained: { to: done }", "nope: { to: done }"}},
			k + ".states.open.on.nope", `unknown message type "nope"; a transition is keyed by a message type`},
		{"ephemeral message type", []replace{{"contained: { to: done }", "typing: { to: done }"}},
			k + ".states.open.on.typing", "an ephemeral message is never stored, so it cannot move a session"},
		{"missing to", []replace{{"contained: { to: done }", "contained: { roles: [lead] }"}},
			on + ".to", "name the state this transition moves to"},
		{"undeclared to", []replace{{"contained: { to: done }", "contained: { to: gone }"}},
			on + ".to", `unknown state "gone"`},
		{"undeclared role", []replace{{"contained: { to: done }", "contained: { to: done, roles: [boss] }"}},
			on + ".roles[0]", `unknown role "boss"`},
		{"role not in post", []replace{
			{"member: { from: attached }", "member: { from: attached }\n      dpo:    { permission: privacy.officer }"},
			{"contained: { to: done }", "contained: { to: done, roles: [dpo] }"},
		}, on + ".roles[0]", `role "dpo" may not post contained, so it could never fire this transition`},
		{"by not trigger", []replace{{"contained: { to: done }", "contained: { to: done, by: agent }"}},
			on + ".by", "by must be trigger"},
		{"by with roles", []replace{{"contained: { to: done }", "contained: { to: done, by: trigger, roles: [lead] }"}},
			on + ".by", "name roles or by: trigger, not both"},
		{"when does not compile", []replace{{"contained: { to: done }", `contained: { to: done, when: "message.nope == 1" }`}},
			on + ".when", "does not compile"},
		{"when names entity", []replace{{"contained: { to: done }", `contained: { to: done, when: "entity.status == 'open'" }`}},
			on + ".when", "transition guards cannot read the entity yet; use a trigger that posts a message"},
		{"when names an unknown entity property", []replace{
			{"contained: { to: done }", `contained: { to: done, when: "entity.nope == 'open'" }`},
		}, on + ".when", "transition guards cannot read the entity yet"},
		{"message type named session", []replace{
			{"      typing:    { persist: ephemeral, post: [member] }\n",
				"      typing:    { persist: ephemeral, post: [member] }\n      session:   { post: [member] }\n"},
		}, "channel_types.br.message_types.session", `"session" is reserved in a channel type with a session`},
		{"by trigger on a many channel type", []replace{
			{"    attach_to: [datalek]\n", "    attach_to: [datalek]\n    cardinality: many\n    create: { roles: [member] }\n"},
			{"contained: { to: done }", "contained: { to: done, by: trigger }"},
		}, on + ".by", "a trigger posts only into cardinality one channel types, so by: trigger never fires here"},
		{"guard on reminders of a session without any", []replace{
			{"contained: { to: done }", `contained: { to: done, when: "session.reminders['x'] >= 1" }`},
		}, on + ".when", `unknown attribute "x"`},
		{"session-level final", []replace{{"      initial: open\n", "      initial: open\n      final: [done]\n"}},
			k + ".final", "mark each final state with final: true"},
		{"merge key", []replace{{"        done: { final: true }", "        <<: { done: { final: true } }"}},
			k + ".states", "merge keys (<<) are not supported in realtime.yaml"},
	}
	meta, policy := testMeta(t), testPolicy(t)
	if _, err := Parse([]byte(sessionBase), meta, policy); err != nil {
		t.Fatalf("the base file does not load: %v", err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := sessionBase
			for _, e := range tc.edits {
				if strings.Count(src, e.old) != 1 {
					t.Fatalf("edit %q matches %d times; want exactly one", e.old, strings.Count(src, e.old))
				}
				src = strings.Replace(src, e.old, e.new, 1)
			}
			_, err := Parse([]byte(src), meta, policy)
			assertProblem(t, err, tc.key, tc.reason)
		})
	}
}

// TestCheckReload_SessionChangesAllowed pins AC 6: sessions do not decide
// how channels are keyed, so no session change refuses a reload.
func TestCheckReload_SessionChangesAllowed(t *testing.T) {
	meta := testMeta(t)
	prev, err := Parse([]byte(golden(t)), meta, nil)
	if err != nil {
		t.Fatal(err)
	}
	block := golden(t)[strings.Index(golden(t), "    session:\n"):strings.Index(golden(t), "  incident-room:\n")]
	tests := map[string][]replace{
		"state removed": {
			{"            dismiss:   { to: dismissed }\n", ""},
			{"        dismissed: { label: Dismissed, final: true }\n", ""},
		},
		"initial renamed": {{"initial: detected", "initial: contained"}},
		"session removed": {{block, ""}},
		"timer added": {{"            notified: { to: notified }\n        notified:",
			"            notified: { to: notified }\n          after:\n" +
				"            late: { period: 1h, to: notified }\n        notified:"}},
		"timer removed": {{"          after:\n            no-containment: { period: 24h, to: overdue, " +
			"label: Not contained within a day }\n", ""}},
		"timer renamed":  {{"no-containment:", "not-contained:"}},
		"period changed": {{"no-containment: { period: 24h", "no-containment: { period: 1h"}},
		"reminder added": {{"what changed since the last one?\" }\n", "what changed since the last one?\" }\n" +
			"        other:\n          in: [overdue]\n          every: 1h\n" +
			"          post: { type: reminder, props: { body: x } }\n"}},
		"reminder removed": {
			{"escalate-stalled: { to: overdue, when: \"session.reminders['status-update'] >= 3\" }", "escalate-stalled: { to: overdue }"},
			{golden(t)[strings.Index(golden(t), "      reminders:\n"):strings.Index(golden(t), "  incident-room:\n")], ""},
		},
		"reminder renamed": {
			{"session.reminders['status-update']", "session.reminders['status']"},
			{"        status-update:\n          label: Status update", "        status:\n          label: Status update"},
		},
		"reminder in changed":    {{"in: [detected, contained, assessed, overdue]", "in: [contained]"}},
		"reminder every changed": {{"every: 30m", "every: 1h"}},
		"reminder first changed": {{"first: 10m", "first: 1h"}},
		"reminder max changed":   {{"max: 48", "max: 5"}},
	}
	for name, edits := range tests {
		t.Run(name, func(t *testing.T) {
			src := golden(t)
			for _, e := range edits {
				if strings.Count(src, e.old) != 1 {
					t.Fatalf("edit %q matches %d times", e.old, strings.Count(src, e.old))
				}
				src = strings.Replace(src, e.old, e.new, 1)
			}
			next, err := Parse([]byte(src), meta, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := CheckReload(prev, next); err != nil {
				t.Errorf("refused: %v", err)
			}
		})
	}
}
