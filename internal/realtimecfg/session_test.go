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
	if want := []string{
		"detected", "contained", "handed-over", "assessed", "overdue", "notified", "dismissed",
	}; !slices.Equal(names, want) {
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
	tr, ok := firstBranch(s, "detected", "contained")
	if !ok || !slices.Equal(tr.Roles, []string{"lead"}) || tr.By != TransitionByAny || tr.When != nil {
		t.Errorf("detected.contained = %+v", tr)
	}
	if tr, _ := firstBranch(s, "detected", "escalated"); tr.By != TransitionByTrigger || len(tr.Roles) != 0 {
		t.Errorf("detected.escalated = %+v", tr)
	}
	if tr, _ := firstBranch(s, "contained", "assessed"); tr.When == nil {
		t.Error("contained.assessed lost its guard")
	}
	if _, ok := firstBranch(s, "notified", "text"); ok {
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
		if tr, ok := firstBranch(ct.Session(), "zeta", "text"); !ok || tr.To != "aux" {
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
		{"awaits", []replace{{"        open:\n", "        open:\n          awaits: [lead]\n"}},
			k + ".states.open.awaits", "use waiting_for"},
		{"session-level status", []replace{{"      initial: open\n", "      initial: open\n      status: working\n"}},
			k + ".status", "status is set per state"},
		{"session-level waiting_for", []replace{{"      initial: open\n", "      initial: open\n      waiting_for: [lead]\n"}},
			k + ".waiting_for", "waiting_for is set per state"},
		{"transition waiting_for", []replace{{"contained: { to: done }", "contained: { to: done, waiting_for: [lead] }"}},
			on + ".waiting_for", "waiting_for belongs to the state the session enters"},
		{"timer waiting_for", []replace{{"contained: { to: done }\n",
			"contained: { to: done }\n          after:\n            late: { period: 1h, to: done, waiting_for: [lead] }\n"}},
			k + ".states.open.after.late.waiting_for", "waiting_for belongs to the state the session enters"},
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
			{"        dismissed: { label: Dismissed, final: true, status: cancelled }\n", ""},
		},
		"initial renamed": {{"initial: detected", "initial: contained"}},
		"session removed": {{block, ""}},
		"timer added": {{"{{message.reference}}\" } }\n        notified:",
			"{{message.reference}}\" } }\n          after:\n" +
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
		"reminder in changed":    {{"in: [detected, contained, handed-over, assessed, overdue]", "in: [contained, handed-over]"}},
		"reminder every changed": {{"every: 30m", "every: 1h"}},
		"reminder first changed": {{"first: 10m", "first: 1h"}},
		"reminder max changed":   {{"max: 48", "max: 5"}},
		"outputs added": {{"escalate-stalled: { to: overdue, when:",
			"escalate-stalled: { to: overdue, roles: [dpo], outputs: [ { set: { properties: { status: late } } } ], when:"}},
		"outputs removed":  {{"              outputs:\n                - set: { properties: { risk: \"{{message.risk}}\" } }\n", ""}},
		"outputs changed":  {{"risk: \"{{message.risk}}\"", "notes: \"{{message.risk}}\""}},
		"on_final removed": {{"      on_final: close\n", ""}},
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

// firstBranch is the first entry of on.<msgType> out of state, as the
// mapping form compiles it.
func firstBranch(s *Session, state, msgType string) (Transition, bool) {
	b := s.Branches(state, msgType)
	if len(b) == 0 {
		return Transition{}, false
	}
	return b[0], true
}

// branchBase is a session whose review state moves on decide; ON is
// replaced by the value of on.decide.
const branchBase = `version: 1
channel_types:
  br:
    attach_to: [datalek]
    roles:
      lead:   { from: entity.owner }
      member: { from: attached }
    message_types:
      text:   { properties: { body: { type: string } }, post: [member] }
      decide: { properties: { risk: { type: risk-level } }, post: [lead, member] }
    session:
      initial: review
      states:
        review:
          on:
            decide: ON
        working:  { on: { text: { to: done } } }
        planning: { on: { text: { to: review } } }
        done: { final: true }
`

// branchKey is the key of review's on.decide in branchBase.
const branchKey = "channel_types.br.session.states.review.on.decide"

// branchSrc returns branchBase with on.decide set to on.
func branchSrc(on string) string { return strings.Replace(branchBase, "ON", on, 1) }

// TestSession_BranchList pins AC 1 of TKT-FPXTQJ: on.<type> takes a
// mapping or a list of entries; a one-entry list compiles to the same
// transition as the mapping, and the entries keep file order.
func TestSession_BranchList(t *testing.T) {
	type entry struct {
		to, key string
		roles   []string
		guarded bool
	}
	cases := []struct {
		name, on string
		want     []entry
	}{
		{"mapping", "{ to: working, roles: [lead] }", []entry{{"working", branchKey, []string{"lead"}, false}}},
		{"one-entry list", "[ { to: working, roles: [lead] } ]",
			[]entry{{"working", branchKey + "[0]", []string{"lead"}, false}}},
		{"two-entry list", `[ { to: working, when: "message.risk == 'high'" }, { to: planning } ]`, []entry{
			{"working", branchKey + "[0]", nil, true}, {"planning", branchKey + "[1]", nil, false},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := Parse([]byte(branchSrc(tc.on)), testMeta(t), nil)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			ct, _ := c.ChannelType("br")
			s := ct.Session()
			var got []entry
			for _, tr := range s.Branches("review", "decide") {
				if tr.MessageType != "decide" || tr.By != TransitionByAny {
					t.Errorf("entry %+v", tr)
				}
				got = append(got, entry{tr.To, tr.Key(), tr.Roles, tr.When != nil})
			}
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Errorf("Branches = %v; want %v", got, tc.want)
			}
			st, _ := s.State("review")
			if n := len(st.Transitions()); n != len(tc.want) {
				t.Errorf("Transitions has %d rows; want one per entry (%d)", n, len(tc.want))
			}
			if tr, ok := s.TransitionTo("review", "decide", tc.want[len(tc.want)-1].to); !ok ||
				tr.Key() != tc.want[len(tc.want)-1].key {

				t.Errorf("TransitionTo = %+v, %v", tr, ok)
			}
			if _, ok := s.TransitionTo("review", "decide", "done"); ok {
				t.Error("TransitionTo found a target no entry names")
			}
		})
	}
}

// TestSession_BranchListWarnings pins AC 4 of TKT-FPXTQJ: a list in which
// every entry has a when loads with a warning at on.<type>; a list ending
// in an unguarded entry and the mapping form do not warn.
func TestSession_BranchListWarnings(t *testing.T) {
	cases := []struct {
		name, on string
		warn     bool
	}{
		{"no catch-all", `[ { to: working, when: "message.risk == 'high'" }, ` +
			`{ to: planning, when: "message.risk == 'low'" } ]`, true},
		{"unguarded last entry", `[ { to: working, when: "message.risk == 'high'" }, { to: planning } ]`, false},
		{"guarded mapping", `{ to: working, when: "message.risk == 'high'" }`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := Parse([]byte(branchSrc(tc.on)), testMeta(t), nil)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			w := c.Warnings()
			if !tc.warn {
				if len(w) != 0 {
					t.Errorf("warnings = %v; want none", w)
				}
				return
			}
			if len(w) != 1 || w[0].Key != branchKey || !strings.Contains(w[0].Reason,
				"every entry has a when; a post that matches none is stored and does not move the session") {

				t.Errorf("warnings = %v; want one at %s", w, branchKey)
			}
		})
	}
}

// TestSession_BranchListProblems pins AC 6 of TKT-FPXTQJ: the load refuses
// a bad list with the key of the entry, and keeps the mapping form's keys.
func TestSession_BranchListProblems(t *testing.T) {
	const set = `outputs: [ { set: { properties: { status: x } } } ]`
	nine := "[" + strings.Repeat("{ to: working, roles: [lead] }, ", 8) + "{ to: planning } ]"
	cases := []struct {
		name, on    string
		edits       []replace
		key, reason string
	}{
		{"empty list", "[]", nil, branchKey, "name at least one entry, or drop on.decide"},
		{"nine entries", nine, nil, branchKey, "at most 8 entries per message type"},
		{"covered by an entry without a rule", "[ { to: working }, { to: planning } ]", nil, branchKey + "[1]",
			"earlier entries without when admit every poster this entry admits, so it can never fire"},
		{"covered by the same role", "[ { to: working, roles: [lead] }, { to: planning, roles: [lead] } ]", nil,
			branchKey + "[1]", "so it can never fire"},
		{"covered by two earlier entries", "[ { to: working, roles: [lead, member] }, { to: planning, by: trigger }, " +
			"{ to: done } ]", nil, branchKey + "[2]", "so it can never fire"},
		{"duplicate target with outputs", "[ { to: working, roles: [lead], " + set + " }, { to: working } ]", nil,
			branchKey + "[1].to", "entry 0 already leads to working and one of the two has outputs"},
		{"duplicate target with outputs on the later entry", "[ { to: working, roles: [lead] }, { to: working, " +
			set + " } ]", nil, branchKey + "[1].to", "entry 0 already leads to working"},
		{"unknown target in an entry", "[ { to: working, roles: [lead] }, { to: gone } ]", nil,
			branchKey + "[1].to", `unknown state "gone"`},
		{"missing target in an entry", "[ { to: working, roles: [lead] }, { when: \"message.risk == 'high'\" } ]", nil,
			branchKey + "[1].to", "name the state this transition moves to"},
		{"entity guard in an entry", `[ { to: working, roles: [lead] }, { to: planning, when: "entity.status == 'x'" } ]`,
			nil, branchKey + "[1].when", "transition guards cannot read the entity yet"},
		{"unknown role in an entry", "[ { to: working, roles: [lead] }, { to: planning, roles: [boss] } ]", nil,
			branchKey + "[1].roles[0]", `unknown role "boss"`},
		{"role that may not post in an entry", "[ { to: working, roles: [lead] }, { to: planning, roles: [dpo] } ]",
			[]replace{{"      member: { from: attached }\n",
				"      member: { from: attached }\n      dpo:    { permission: privacy.officer }\n"}},
			branchKey + "[1].roles[0]", `role "dpo" may not post decide`},
		{"state not in the type's in", "[ { to: working, roles: [lead] }, { to: planning } ]",
			[]replace{{"post: [lead, member] }", "post: [lead, member], in: [working] }"}},
			branchKey, "decide may be posted only in working, so this transition could never fire"},
		{"null mapping", "~", nil, branchKey + ".to", "name the state this transition moves to"},
		{"broken guard counts as guarded", `[ { to: working, when: "message.nope == 1" }, { to: planning } ]`, nil,
			branchKey + "[0].when", "does not compile"},
	}
	meta := testMeta(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := branchSrc(tc.on)
			for _, e := range tc.edits {
				src = strings.Replace(src, e.old, e.new, 1)
			}
			_, err := Parse([]byte(src), meta, nil)
			assertOneProblem(t, err, tc.key, tc.reason)
		})
	}
	// An unusable message type is reported once at on.<type>, and the
	// entries' own problems show in the same load.
	t.Run("unknown type reported at the key with entry problems", func(t *testing.T) {
		src := strings.Replace(branchSrc("[ { to: working }, { to: gone } ]"), "decide: [", "nope: [", 1)
		_, err := Parse([]byte(src), meta, nil)
		nope := strings.TrimSuffix(branchKey, "decide") + "nope"
		assertProblem(t, err, nope, `unknown message type "nope"`)
		assertProblem(t, err, nope+"[1].to", `unknown state "gone"`)
	})
	for name, on := range map[string]string{
		"duplicate target without outputs":  `[ { to: working, roles: [lead] }, { to: working, when: "message.urgent" } ]`,
		"entry after a role-narrowed entry": "[ { to: working, roles: [lead] }, { to: planning } ]",
		"entry after a guarded entry":       `[ { to: working, when: "message.risk == 'high'" }, { to: planning } ]`,
		"other roles after a covering role": "[ { to: working, roles: [lead] }, { to: planning, roles: [member] } ]",
	} {
		t.Run(name, func(t *testing.T) {
			src := branchSrc(on)
			if name == "duplicate target without outputs" {
				src = strings.Replace(src, "risk: { type: risk-level } }", "risk: { type: risk-level }, "+
					"urgent: { type: boolean } }", 1)
			}
			if _, err := Parse([]byte(src), meta, nil); err != nil {
				t.Errorf("Parse: %v", err)
			}
		})
	}
}

// TestTransition_Admits pins the poster rule the decider and the load
// checks share (TKT-FPXTQJ): roles exclude a trigger, by: trigger admits
// only a trigger, and an entry with neither admits everyone. A nil holds
// is a trigger's post.
func TestTransition_Admits(t *testing.T) {
	t.Parallel()
	lead := func(r string) bool { return r == "lead" }
	none := func(string) bool { return false }
	cases := []struct {
		name  string
		t     Transition
		holds func(string) bool
		who   posterClass
		want  bool
	}{
		{"roles, holder", Transition{Roles: []string{"lead"}}, lead, posterClass{role: "lead"}, true},
		{"roles, non-holder", Transition{Roles: []string{"lead"}}, none, posterClass{role: "member"}, false},
		{"roles, trigger", Transition{Roles: []string{"lead"}}, nil, triggerPoster, false},
		{"by trigger, trigger", Transition{By: TransitionByTrigger}, nil, triggerPoster, true},
		{"by trigger, person", Transition{By: TransitionByTrigger}, lead, posterClass{role: "lead"}, false},
		{"no rule, person", Transition{}, none, posterClass{role: "member"}, true},
		{"no rule, trigger", Transition{}, nil, triggerPoster, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.t.Admits(tc.holds); got != tc.want {
				t.Errorf("Admits = %v; want %v", got, tc.want)
			}
			if got := entryAdmits(tc.t, tc.who); got != tc.want {
				t.Errorf("entryAdmits = %v; want %v", got, tc.want)
			}
		})
	}
}

// workingType is a channel type on datalek whose session starts in initial
// and has the states given, each a line under `states:`.
func workingType(initial string, states ...string) string {
	return `  run:
    attach_to: [datalek]
    roles:
      owner:  { from: entity.owner }
      member: { from: attached }
    message_types:
      go:   { properties: { note: { type: string } }, post: [owner, member] }
      done: { properties: { note: { type: string } }, post: [owner, member] }
    session:
      initial: ` + initial + `
      states:
` + "        " + strings.Join(states, "\n        ") + "\n"
}

// TestSession_WorkingStates pins AC 1 of TKT-21G6JS: the working states
// come from each state's status, declared or defaulted, and "" is listed
// when the initial state is working.
func TestSession_WorkingStates(t *testing.T) {
	cases := []struct {
		name  string
		types string
		want  []string
	}{
		{
			name: "PoC shape",
			types: workingType("queued",
				"queued:   { waiting_for: [owner], on: { go: { to: planning } } }",
				"planning: { status: working, on: { go: { to: review } } }",
				"review:   { waiting_for: [owner], on: { go: { to: working } } }",
				"working:  { status: working, on: { done: { to: finished } } }",
				"finished: { final: true }",
			),
			want: []string{"planning", "working"},
		},
		{
			name: "working initial",
			types: workingType("planning",
				"planning: { status: working, on: { go: { to: review } } }",
				"review:   { waiting_for: [owner], on: { done: { to: finished } } }",
				"finished: { final: true }",
			),
			want: []string{"planning", ""},
		},
		{
			name: "default status",
			types: workingType("queued",
				"queued:   { waiting_for: [owner], on: { go: { to: busy } } }",
				"busy:     { on: { done: { to: finished } } }",
				"finished: { final: true }",
			),
			want: []string{"busy"},
		},
		{
			name: "all final or waiting",
			types: workingType("queued",
				"queued:    { waiting_for: [owner], on: { go: { to: finished }, done: { to: failed } } }",
				"finished:  { final: true }",
				"failed:    { final: true, status: failed }",
				"cancelled: { final: true, status: cancelled }",
			),
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := Parse([]byte("version: 1\nchannel_types:\n"+tc.types), testMeta(t), nil)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			ct, _ := c.ChannelType("run")
			if got := ct.Session().WorkingStates(); !slices.Equal(got, tc.want) {
				t.Errorf("WorkingStates() = %q; want %q", got, tc.want)
			}
		})
	}
	t.Run("status declared", func(t *testing.T) {
		c, err := Parse([]byte("version: 1\nchannel_types:\n"+workingType("busy",
			"busy:     { on: { go: { to: planning } } }",
			"planning: { status: working, on: { done: { to: finished } } }",
			"finished: { final: true }",
		)), testMeta(t), nil)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		ct, _ := c.ChannelType("run")
		busy, _ := ct.Session().State("busy")
		planning, _ := ct.Session().State("planning")
		if busy.StatusDeclared() || !planning.StatusDeclared() {
			t.Errorf("StatusDeclared busy, planning = %v, %v; want false, true",
				busy.StatusDeclared(), planning.StatusDeclared())
		}
	})
}
