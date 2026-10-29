package realtimecfg

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// waitBase is a session whose open state waits for lead, with an answer
// (answer) and a re-entry (poke). It loads without a policy.
const waitBase = `version: 1
channel_types:
  br:
    attach_to: [datalek]
    roles:
      lead:   { from: entity.owner, label: the incident lead }
      member: { from: attached }
      dpo:    { permission: privacy.officer }
      agent:  { principal_type: agent }
    message_types:
      text:   { properties: { body: { type: string } }, post: [member] }
      answer: { properties: { note: { type: string } }, post: [lead, member, dpo] }
      poke:   { properties: { note: { type: string } }, post: [lead, member] }
    session:
      initial: open
      states:
        open:
          waiting_for: [lead]
          on:
            answer: { to: done }
            poke:   { to: open }
        done: { final: true }
`

const (
	openKey    = "channel_types.br.session.states.open"
	doneKey    = "channel_types.br.session.states.done"
	waitLine   = "          waiting_for: [lead]\n"
	doneLine   = "        done: { final: true }\n"
	answerLine = "            answer: { to: done }\n"
	pokeLine   = "            poke:   { to: open }\n"
)

// waitEdit applies each replacement to waitBase; each must match once.
func waitEdit(t *testing.T, edits ...replace) string {
	t.Helper()
	src := waitBase
	for _, e := range edits {
		if strings.Count(src, e.old) != 1 {
			t.Fatalf("edit %q matches %d times; want exactly one", e.old, strings.Count(src, e.old))
		}
		src = strings.Replace(src, e.old, e.new, 1)
	}
	return src
}

// assertOneProblem asserts err holds exactly one problem, at key, whose
// reason contains reason.
func assertOneProblem(t *testing.T, err error, key, reason string) {
	t.Helper()
	assertProblem(t, err, key, reason)
	var perr *Error
	if errors.As(err, &perr) && len(perr.Problems) != 1 {
		t.Errorf("problems = %v; want exactly one", perr.Problems)
	}
}

// waitState parses src without a policy and returns the state name of br.
func waitState(t *testing.T, src, name string) SessionState {
	t.Helper()
	c, err := Parse([]byte(src), testMeta(t), nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ct, _ := c.ChannelType("br")
	st, ok := ct.Session().State(name)
	if !ok {
		t.Fatalf("no state %s", name)
	}
	return st
}

// TestSessionStatus_Values pins AC 1: status accepts the five MCP tasks
// values only, and an explicit empty string is not the default.
func TestSessionStatus_Values(t *testing.T) {
	cases := []struct {
		value string
		ok    bool
	}{
		{"completed", true},
		{"cancelled", true},
		{"failed", true},
		{"working", false},
		{"input_required", false},
		{"submitted", false},
		{"rejected", false},
		{"auth_required", false},
		{"done", false},
		{`""`, false},
	}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			// The final state takes the terminal values; the others are
			// refused there for the final rule or as unknown values.
			src := waitEdit(t, replace{doneLine, "        done: { final: true, status: " + tc.value + " }\n"})
			_, err := Parse([]byte(src), testMeta(t), nil)
			if tc.ok {
				if err != nil {
					t.Fatalf("Parse: %v", err)
				}
				return
			}
			reason := "status must be one of working, input_required, completed, cancelled, failed"
			if tc.value == "working" || tc.value == "input_required" {
				reason = finalNoAnswer
			}
			assertOneProblem(t, err, doneKey+".status", reason)
		})
	}
	t.Run("working on a state that is not final", func(t *testing.T) {
		src := waitEdit(t, replace{waitLine, "          status: working\n"})
		if st := waitState(t, src, "open"); st.Status != StatusWorking {
			t.Errorf("status = %s", st.Status)
		}
	})
}

// TestSessionStatus_Defaults pins AC 2.
func TestSessionStatus_Defaults(t *testing.T) {
	cases := []struct {
		name, src, state string
		want             SessionStatus
	}{
		{"final", waitBase, "done", StatusCompleted},
		{"waiting_for", waitBase, "open", StatusInputRequired},
		{"neither", strings.Replace(waitBase, waitLine, "", 1), "open", StatusWorking},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := waitState(t, tc.src, tc.state).Status; got != tc.want {
				t.Errorf("Status = %s; want %s", got, tc.want)
			}
		})
	}
	if got := waitState(t, waitBase, "open").WaitingFor(); !slices.Equal(got, []string{"lead"}) {
		t.Errorf("WaitingFor = %v", got)
	}
}

// TestSessionStatus_FinalRules pins AC 3.
func TestSessionStatus_FinalRules(t *testing.T) {
	cases := []struct {
		name   string
		edits  []replace
		key    string
		reason string
	}{
		{"final working", []replace{{doneLine, "        done: { final: true, status: working }\n"}},
			doneKey + ".status", finalNoAnswer},
		{"final input_required", []replace{{doneLine, "        done: { final: true, status: input_required }\n"}},
			doneKey + ".status", finalNoAnswer},
		{"final waiting_for", []replace{{doneLine, "        done: { final: true, waiting_for: [lead] }\n"}},
			doneKey + ".waiting_for", finalNoAnswer},
		{"not final completed", []replace{{waitLine, "          status: completed\n"}},
			openKey + ".status", "mark the state final: true"},
		{"not final cancelled", []replace{{waitLine, "          status: cancelled\n"}},
			openKey + ".status", "mark the state final: true"},
		{"not final failed", []replace{{waitLine, "          status: failed\n"}},
			openKey + ".status", "mark the state final: true"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(waitEdit(t, tc.edits...)), testMeta(t), nil)
			assertOneProblem(t, err, tc.key, tc.reason)
		})
	}
}

// TestSessionWaitingFor_Refs pins AC 4 and AC 5: waiting_for names
// declared, bounded roles once each.
func TestSessionWaitingFor_Refs(t *testing.T) {
	const wk = openKey + ".waiting_for"
	many := replace{"    attach_to: [datalek]\n",
		"    attach_to: [datalek]\n    cardinality: many\n    create: { roles: [member] }\n"}
	starter := replace{"      agent:  { principal_type: agent }\n",
		"      agent:  { principal_type: agent }\n      starter: { creator: true }\n"}
	cases := []struct {
		name   string
		edits  []replace
		key    string
		reason string
	}{
		{"unknown role", []replace{{waitLine, "          waiting_for: [boss]\n"}},
			wk + "[0]", `unknown role "boss"; declare it under roles`},
		{"read role", []replace{{waitLine, "          waiting_for: [read]\n"}},
			wk + "[0]", "the read role is not under roles"},
		{"duplicate", []replace{{waitLine, "          waiting_for: [lead, lead]\n"}},
			wk + "[1]", `role "lead" is listed twice`},
		{"empty list", []replace{{waitLine, "          waiting_for: []\n"}},
			wk, "name at least one role, or drop waiting_for"},
		{"attached role", []replace{{waitLine, "          waiting_for: [member]\n"}},
			wk + "[0]", "role member is held by everyone who can read the entity; " + boundedRule},
		{"permission role", []replace{{waitLine, "          waiting_for: [dpo]\n"}},
			wk + "[0]", "role dpo is held by everyone with permission privacy.officer; " + boundedRule},
		{"principal type role", []replace{
			{waitLine, "          waiting_for: [agent]\n"},
			{"post: [lead, member, dpo] }", "post: [lead, member, dpo, agent] }"},
		}, wk + "[0]", "role agent is held by every principal of type agent; " + boundedRule},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(waitEdit(t, tc.edits...)), testMeta(t), nil)
			assertOneProblem(t, err, tc.key, tc.reason)
		})
	}
	t.Run("creator role on a many type", func(t *testing.T) {
		src := waitEdit(t, many, starter, replace{waitLine, "          waiting_for: [lead, starter]\n"},
			replace{"post: [lead, member, dpo] }", "post: [lead, member, dpo, starter] }"})
		if got := waitState(t, src, "open").WaitingFor(); !slices.Equal(got, []string{"lead", "starter"}) {
			t.Errorf("WaitingFor = %v", got)
		}
	})
}

// TestSessionWaitingFor_Status pins AC 6.
func TestSessionWaitingFor_Status(t *testing.T) {
	cases := []struct {
		name  string
		edits []replace
		want  string
	}{
		{"input_required without waiting_for", []replace{{waitLine, "          status: input_required\n"}},
			"name whom the session waits for"},
		{"waiting_for with working", []replace{{waitLine, waitLine + "          status: working\n"}},
			"a state with waiting_for has status input_required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(waitEdit(t, tc.edits...)), testMeta(t), nil)
			assertOneProblem(t, err, openKey+".status", tc.want)
		})
	}
	src := waitEdit(t, replace{waitLine, waitLine + "          status: input_required\n"})
	if st := waitState(t, src, "open"); st.Status != StatusInputRequired {
		t.Errorf("explicit input_required = %s", st.Status)
	}
}

// TestSessionWaitingFor_CanAnswer pins AC 7: every waiting role needs a
// transition out of the state that it can fire.
func TestSessionWaitingFor_CanAnswer(t *testing.T) {
	const noAnswer = "role lead cannot post a message that moves the session out of open, so the session " +
		"would wait for it forever"
	cases := []struct {
		name  string
		edits []replace
		ok    bool
	}{
		{"role not in post", []replace{{"post: [lead, member, dpo] }", "post: [member, dpo] }"}}, false},
		{"narrowed out by roles", []replace{{answerLine, "            answer: { to: done, roles: [member] }\n"}},
			false},
		{"by trigger only", []replace{{answerLine, "            answer: { to: done, by: trigger }\n"}}, false},
		{"only re-entries", []replace{{answerLine, "            answer: { to: open }\n"}}, false},
		{"guarded answer", []replace{{answerLine,
			"            answer: { to: done, roles: [lead], when: \"message.note == 'ok'\" }\n"}}, true},
		// TKT-FPXTQJ AC 7: entries are judged per role.
		{"answered by the second entry of a list", []replace{{answerLine,
			"            answer: [ { to: open, roles: [member] }, { to: done } ]\n"}}, true},
		{"answer shadowed by an earlier re-entry for the role", []replace{{answerLine,
			"            answer: [ { to: open, roles: [lead] }, { to: done } ]\n"}}, false},
		{"answer after a guarded re-entry", []replace{{answerLine,
			"            answer: [ { to: open, roles: [lead], when: \"message.note == 'x'\" }, { to: done } ]\n"}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := Parse([]byte(waitEdit(t, tc.edits...)), testMeta(t), nil)
			if tc.ok {
				if err != nil {
					t.Fatalf("Parse: %v", err)
				}
				if len(c.Warnings()) != 0 {
					t.Errorf("warnings = %v", c.Warnings())
				}
				return
			}
			assertOneProblem(t, err, openKey+".waiting_for[0]", noAnswer)
		})
	}
}

// TestSessionWaitingFor_DeadlineReset pins AC 8: a re-entry a waiting role
// can fire is refused where it would restart a since: state timer or a
// reminder.
func TestSessionWaitingFor_DeadlineReset(t *testing.T) {
	const stateTimer = "          after:\n            late: { period: 4h, to: done }\n"
	const sessionTimer = "          after:\n            late: { period: 4h, since: session, to: done }\n"
	reminder := func(in string) replace {
		return replace{doneLine, doneLine + "      reminders:\n        nag:\n          in: " + in +
			"\n          every: 1h\n          post: { type: text, props: { body: x } }\n"}
	}
	addTimer := func(timer string) replace { return replace{pokeLine, pokeLine + timer} }
	// away is a second state: poke leaves open for it, and back returns.
	away := func(backRoles string) []replace {
		return []replace{
			{pokeLine, "            poke:   { to: away }\n"},
			{doneLine, "        away:\n          on:\n            poke: { to: open" + backRoles + " }\n" +
				"            answer: { to: done }\n" + doneLine},
		}
	}
	cases := []struct {
		name  string
		edits []replace
		ok    bool
	}{
		{"since state timer", []replace{addTimer(stateTimer)}, false},
		{"since session timer only", []replace{addTimer(sessionTimer)}, true},
		{"re-entry narrowed to a role that is not waiting", []replace{addTimer(stateTimer),
			{pokeLine, "            poke:   { to: open, roles: [member] }\n"}}, true},
		{"re-entry by trigger", []replace{addTimer(stateTimer),
			{pokeLine, "            poke:   { to: open, by: trigger }\n"}}, true},
		{"no timers and no reminders", nil, true},
		// A reminder's span goes on across a re-entry: nothing restarts.
		{"reminder, direct re-entry", []replace{reminder("[open]")}, true},
		{"two-state cycle with a since state timer",
			append([]replace{addTimer(stateTimer)}, away("")...), false},
		{"two-state cycle back by a role that is not waiting",
			append([]replace{addTimer(stateTimer)}, away(", roles: [member]")...), true},
		{"two-state cycle with a since session timer",
			append([]replace{addTimer(sessionTimer)}, away("")...), true},
		{"two-state cycle leaving the reminder's states", append(away(""), reminder("[open]")), false},
		{"two-state cycle inside the reminder's states", append(away(""), reminder("[open, away]")), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(waitEdit(t, tc.edits...)), testMeta(t), nil)
			if tc.ok {
				if err != nil {
					t.Fatalf("Parse: %v", err)
				}
				return
			}
			assertOneProblem(t, err, openKey+".on.poke", deadlineReset)
		})
	}
}

// TestSessionWaitingFor_DeadlineResetBranches pins TKT-FPXTQJ AC 7: the
// deadline check walks list entries per role and reports at the entry.
func TestSessionWaitingFor_DeadlineResetBranches(t *testing.T) {
	timer := replace{pokeLine, "POKE          after:\n            late: { period: 4h, to: done }\n"}
	cases := []struct {
		name, poke string
		key        string
	}{
		{"re-entry in the second entry", "            poke: [ { to: done, when: \"message.note == 'x'\" }, { to: open } ]\n",
			openKey + ".on.poke[1]"},
		{"re-entry the waiting role never reaches", "            poke: [ { to: done, roles: [lead] }, { to: open } ]\n", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := strings.Replace(waitEdit(t, timer), "POKE", tc.poke, 1)
			_, err := Parse([]byte(src), testMeta(t), nil)
			if tc.key == "" {
				if err != nil {
					t.Fatalf("Parse: %v", err)
				}
				return
			}
			assertOneProblem(t, err, tc.key, deadlineReset)
		})
	}
}

// handoffBase hands open's lead role over with a set output whose owner
// value each case replaces. The handoff message carries a ref to the user
// type (risico in the testdata acl.yaml), a ref to another type and text.
const handoffBase = `version: 1
channel_types:
  br:
    attach_to: [datalek]
    read: { from: entity.reference }
    roles:
      lead:   { from: entity.owner }
      member: { from: attached }
    message_types:
      text:    { properties: { body: { type: string } }, post: [member] }
      handoff:
        properties:
          to:    { type: ref, to: [risico] }
          other: { type: ref, to: [maatregel] }
          any:   { type: ref, to: [risico, maatregel] }
          name:  { type: string }
        post: [lead]
      answer: { properties: { note: { type: string } }, post: [lead] }
    session:
      initial: open
      states:
        open:
          waiting_for: [lead]
          on:
            answer: { to: done }
            handoff:
              to: open
              roles: [lead]
              outputs:
                - set: { properties: { PROP: "VALUE" } }
        done: { final: true }
`

// TestHandoffWarning pins AC 10.
func TestHandoffWarning(t *testing.T) {
	const pk = openKey + ".on.handoff.outputs[0].set.properties."
	cases := []struct {
		name, prop, value string
		warn              bool
	}{
		{"user ref", "owner", "{{message.to}}", false},
		{"author", "owner", "{{message.author}}", false},
		{"literal", "owner", "USR-9", false},
		{"free text", "owner", "{{message.name}}", true},
		{"ref to another type", "owner", "{{message.other}}", true},
		{"ref to the user type or another", "owner", "{{message.any}}", true},
		{"composite", "owner", "user {{message.author}}", true},
		{"channel key", "owner", "{{channel.key}}", true},
		{"read role property", "reference", "{{message.name}}", true},
		{"property no role reads", "status", "{{message.name}}", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := strings.NewReplacer("PROP", tc.prop, "VALUE", tc.value).Replace(handoffBase)
			c, err := Parse([]byte(src), testMeta(t), testPolicy(t))
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
			if len(w) != 1 || w[0].Key != pk+tc.prop || !strings.Contains(w[0].Reason, "need not name a user") {
				t.Errorf("warnings = %v; want one at %s", w, pk+tc.prop)
			}
		})
	}
	t.Run("second entry of a list", func(t *testing.T) {
		// TKT-FPXTQJ AC 7: the warning names the entry's own key.
		src := strings.Replace(handoffBase, "            handoff:\n              to: open\n              roles: [lead]\n"+
			"              outputs:\n                - set: { properties: { PROP: \"VALUE\" } }\n",
			"            handoff:\n              - { to: done, roles: [lead], when: \"message.name == 'x'\" }\n"+
				"              - to: open\n                roles: [lead]\n"+
				"                outputs:\n                  - set: { properties: { owner: \"{{message.name}}\" } }\n", 1)
		c, err := Parse([]byte(src), testMeta(t), testPolicy(t))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		const want = openKey + ".on.handoff[1].outputs[0].set.properties.owner"
		if w := c.Warnings(); len(w) != 1 || w[0].Key != want {
			t.Errorf("warnings = %v; want one at %s", w, want)
		}
	})
	t.Run("no user_entity_type", func(t *testing.T) {
		src := strings.NewReplacer("PROP", "owner", "VALUE", "{{message.name}}").Replace(handoffBase)
		p := parsePolicy(t, "roles:\n  handler:\n    read: [datalek]\n    update: [datalek]\n")
		c, err := Parse([]byte(src), testMeta(t), p)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if w := c.Warnings(); len(w) != 0 {
			t.Errorf("warnings = %v; want none", w)
		}
	})
}

// TestSessionWaitingFor_AnswerGrants pins AC 11: an answer whose output no
// role of acl.yaml can write loads with one warning at the output.
func TestSessionWaitingFor_AnswerGrants(t *testing.T) {
	src := waitEdit(t, replace{answerLine, "            answer:\n              to: done\n              outputs:\n" +
		"                - set: { properties: { status: answered } }\n"})
	const key = openKey + ".on.answer.outputs[0].set"
	cases := []struct {
		name, policy string
		warn         bool
	}{
		{"granted", "roles:\n  handler:\n    read: [datalek]\n    update: [datalek]\n", false},
		{"field closed", "roles:\n  handler:\n    read: [datalek]\n    update: [datalek]\n" +
			"    fields:\n      datalek: [{ field: title }]\n", true},
		{"no update", "roles:\n  handler:\n    read: [datalek]\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := parsePolicy(t, tc.policy+"  officer:\n    read: [\"*\"]\n    permissions: [privacy.officer]\n")
			c, err := Parse([]byte(src), testMeta(t), p)
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
			if len(w) != 1 || w[0].Key != key || !strings.Contains(w[0].Reason, "session_output_denied") {
				t.Errorf("warnings = %v; want one at %s", w, key)
			}
		})
	}
}
