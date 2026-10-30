package realtimecfg

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
)

// defaultInBase is the plan example of the realtime guide with
// session.default_in: keyed and no in: on any type. It is cardinality one,
// so an entry may be by: trigger.
const defaultInBase = `version: 1
channel_types:
  plan:
    attach_to: [datalek]
    cardinality: one
    roles:
      owner:  { from: entity.owner }
      member: { from: attached }
    message_types:
      text:    { properties: { body: { type: markdown, required: true } }, post: [owner, member] }
      plan:    { properties: { plan: { type: markdown, required: true } }, post: [member] }
      approve: { properties: { note: { type: string } }, post: [owner] }
      revise:  { properties: { feedback: { type: markdown, required: true } }, post: [owner] }
    session:
      initial: drafting
      default_in: keyed
      states:
        drafting:
          on:
            plan: { to: review }
        review:
          waiting_for: [owner]
          on:
            approve: { to: approved, roles: [owner] }
            revise:  { to: drafting, roles: [owner] }
        approved: { final: true }
`

// The key paths of default_in and of the message types in [defaultInBase].
const (
	defaultInKey = "channel_types.plan.session.default_in"
	planTypesKey = "channel_types.plan.message_types"
)

// allKeyedAuthored gives every keyed type of [defaultInBase] the in: that
// default_in would derive, so default_in: keyed has no effect.
var allKeyedAuthored = []replace{
	{"post: [member] }", "post: [member], in: [drafting] }"},
	{"post: [owner] }\n      revise", "post: [owner], in: [review] }\n      revise"},
	{"required: true } }, post: [owner] }", "required: true } }, post: [owner], in: [review] }"},
}

// noEffectWarning is the warning of default_in when every keyed type
// declares in:.
const noEffectWarning = "default_in: keyed has no effect, because every message type a transition is keyed by " +
	"declares in:; drop the in: lists that only repeat the on: keys, or drop default_in"

// defaultInSrc applies each old/new pair to [defaultInBase] once, and
// fails the test when one does not match exactly once.
func defaultInSrc(t *testing.T, edits ...replace) string {
	t.Helper()
	src := defaultInBase
	for _, e := range edits {
		if n := strings.Count(src, e.old); n != 1 {
			t.Fatalf("edit %q matches %d times; want exactly one", e.old, n)
		}
		src = strings.Replace(src, e.old, e.new, 1)
	}
	return src
}

// parseClean loads src and fails the test on a problem.
func parseClean(t *testing.T, src string) *Compiled {
	t.Helper()
	c, err := Parse([]byte(src), testMeta(t), testPolicy(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return c
}

// TestParse_SessionDefaultIn pins AC 1: default_in loads as keyed or
// absent, any other value is refused at its key, and the key misplaced
// under a state or on a message type is refused.
func TestParse_SessionDefaultIn(t *testing.T) {
	const on = "      default_in: keyed\n"
	t.Run("keyed", func(t *testing.T) {
		ct, _ := parseClean(t, defaultInBase).ChannelType("plan")
		if got := ct.MessageTypes["plan"].In; !slices.Equal(got, []string{"drafting"}) {
			t.Errorf("plan.In = %v; want [drafting]", got)
		}
	})
	for _, tc := range []struct{ name, value string }{{"absent", ""}, {"null", "      default_in: ~\n"}} {
		t.Run(tc.name, func(t *testing.T) {
			ct, _ := parseClean(t, defaultInSrc(t, replace{on, tc.value})).ChannelType("plan")
			for _, mt := range ct.MessageTypes {
				if mt.In != nil {
					t.Errorf("%s.In = %v; want nil", mt.Name, mt.In)
				}
			}
		})
	}
	cases := []struct {
		name, src, key, reason string
	}{
		{"all", defaultInSrc(t, replace{on, "      default_in: all\n"}), defaultInKey,
			"default_in may only be keyed"},
		{"Keyed", defaultInSrc(t, replace{on, "      default_in: Keyed\n"}), defaultInKey,
			"default_in may only be keyed"},
		{"a list", defaultInSrc(t, replace{on, "      default_in: [keyed]\n"}), defaultInKey, ""},
		{"under a state", defaultInSrc(t, replace{on, ""},
			replace{"        drafting:\n", "        drafting:\n          default_in: keyed\n"}),
			"channel_types.plan.session.states.drafting.default_in",
			"default_in belongs to the session: session.default_in: keyed"},
		{"on a message type", defaultInSrc(t, replace{"post: [member] }", "post: [member], default_in: keyed }"}),
			planTypesKey + ".plan.default_in", "unknown key"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.src), testMeta(t), testPolicy(t))
			assertProblem(t, err, tc.key, tc.reason)
		})
	}
}

// TestDefaultIn_Derive pins AC 2 and 3: with default_in: keyed, a type
// without in: gets its keyed states, in the order of session.states,
// whatever form its entries take; an authored in: is kept and checked as
// before; a type without a keyed state stays free.
func TestDefaultIn_Derive(t *testing.T) {
	cases := []struct {
		name  string
		edits []replace
		want  map[string][]string
		// authored names the types whose in: the file writes.
		authored []string
	}{
		{"single entry", nil, map[string][]string{
			"plan": {"drafting"}, "approve": {"review"}, "revise": {"review"},
		}, nil},
		{"list of entries keys its state once", []replace{{"approve: { to: approved, roles: [owner] }",
			`approve: [ { to: approved, roles: [owner], when: "message.note == 'ok'" }, { to: drafting, roles: [owner] } ]`}},
			map[string][]string{"approve": {"review"}}, nil},
		{"type keyed in two states lists both in declaration order", []replace{
			{"approve: { to: approved, roles: [owner] }", "approve: { to: a-check, roles: [owner] }"},
			{"        approved: { final: true }\n",
				"        a-check:\n          on: { approve: { to: approved } }\n        approved: { final: true }\n"},
		}, map[string][]string{"approve": {"review", "a-check"}}, nil},
		{"by: trigger entry keys its state", []replace{{"plan: { to: review }", "plan: { to: review, by: trigger }"}},
			map[string][]string{"plan": {"drafting"}}, nil},
		{"shadowed entry keys its state", []replace{{"revise:  { to: drafting, roles: [owner] }",
			"revise:  [ { to: drafting, roles: [owner] }, { to: approved } ]"}},
			map[string][]string{"revise": {"review"}}, nil},
		{"entry limited to another role keys its state", []replace{
			{"properties: { note: { type: string } }, post: [owner] }", "properties: { note: { type: string } }, post: [owner, member] }"},
			{"approve: { to: approved, roles: [owner] }", "approve: { to: approved, roles: [member] }"},
		},
			map[string][]string{"approve": {"review"}}, nil},
		{"authored in widens", []replace{{"post: [owner] }\n      revise", "post: [owner], in: [drafting, review] }\n      revise"}},
			map[string][]string{"approve": {"drafting", "review"}, "revise": {"review"}}, []string{"approve"}},
		{"unkeyed type stays nil", nil, map[string][]string{"text": nil}, nil},
		{"reminder post type stays nil", []replace{
			{"      revise:  { properties", "      nudge:   { properties: { body: { type: string } }, post: [member] }\n      revise:  { properties"},
			{"      states:\n", "      reminders:\n        poke: { in: [review], every: 1h, post: { type: nudge, props: { body: hi } } }\n" +
				"      states:\n"},
		}, map[string][]string{"nudge": nil, "approve": {"review"}}, nil},
		{"unkeyed choices answer stays nil", []replace{
			{"      plan:    { properties: { plan: { type: markdown, required: true } }, post: [member] }\n",
				"      plan:\n        properties:\n          plan: { type: markdown, required: true }\n" +
					"          options: { type: string, list: true, max: 80, max_items: 4 }\n" +
					"        post: [member]\n        choices: { from: options, answer: pick, into: body }\n" +
					"      pick:    { properties: { body: { type: string } }, post: [owner] }\n"},
		}, map[string][]string{"pick": nil, "plan": {"drafting"}}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ct, _ := parseClean(t, defaultInSrc(t, tc.edits...)).ChannelType("plan")
			for mn, want := range tc.want {
				mt := ct.MessageTypes[mn]
				if mt == nil {
					t.Fatalf("no message type %s", mn)
				}
				if !slices.Equal(mt.In, want) {
					t.Errorf("%s.In = %v; want %v", mn, mt.In, want)
				}
				wantKey := defaultInKey
				if want == nil || slices.Contains(tc.authored, mn) {
					wantKey = planTypesKey + "." + mn + ".in"
				}
				if got := ct.InKey(mt); got != wantKey {
					t.Errorf("InKey(%s) = %q; want %q", mn, got, wantKey)
				}
			}
		})
	}
	t.Run("authored in missing a keyed state is refused and suggests dropping in", func(t *testing.T) {
		src := defaultInSrc(t, replace{"post: [owner] }\n      revise", "post: [owner], in: [drafting] }\n      revise"})
		_, err := Parse([]byte(src), testMeta(t), testPolicy(t))
		assertProblem(t, err, "channel_types.plan.session.states.review.on.approve",
			"approve may be posted only in drafting, so this transition could never fire; "+
				"add review to message_types.approve.in, or drop in to take the keyed states from session.default_in")
	})
}

// TestDefaultIn_AbsentKeepsToday pins AC 4: without default_in a keyed
// type stays free, and InKey is the type's in: path, also for a
// MessageType built by hand.
func TestDefaultIn_AbsentKeepsToday(t *testing.T) {
	src := defaultInSrc(t, replace{"      default_in: keyed\n", ""},
		replace{"post: [owner] }\n      revise", "post: [owner], in: [review] }\n      revise"})
	c := parseClean(t, src)
	ct, _ := c.ChannelType("plan")
	want := map[string][]string{"text": nil, "plan": nil, "approve": {"review"}, "revise": nil}
	for mn, w := range want {
		mt := ct.MessageTypes[mn]
		if !slices.Equal(mt.In, w) {
			t.Errorf("%s.In = %v; want %v", mn, mt.In, w)
		}
		if got := ct.InKey(mt); got != planTypesKey+"."+mn+".in" {
			t.Errorf("InKey(%s) = %q", mn, got)
		}
	}
	if w := c.Warnings(); len(w) != 0 {
		t.Errorf("warnings = %v", w)
	}
	hand := &MessageType{Name: "approve", In: []string{"review"}}
	if got := ct.InKey(hand); got != planTypesKey+".approve.in" {
		t.Errorf("InKey(hand-built) = %q", got)
	}
}

// TestDefaultIn_PoCEquivalent pins AC 5: the PoC fixture without the 17
// in: lines that repeat on: keys, and with default_in: keyed, compiles to
// the same In for each type as the fixture with them.
func TestDefaultIn_PoCEquivalent(t *testing.T) {
	trimmed := map[string]string{"progress": pocIn["progress"]}
	if len(pocIn)-len(trimmed) != 17 {
		t.Fatalf("pocIn has %d keyed types; want 17", len(pocIn)-len(trimmed))
	}
	authored := parseClean(t, pocRealtime(pocIn, ""))
	derived := parseClean(t, pocRealtime(trimmed, "      default_in: keyed\n"))
	if w := derived.Warnings(); len(w) != 0 {
		t.Errorf("warnings = %v", w)
	}
	a, _ := authored.ChannelType("run")
	d, _ := derived.ChannelType("run")
	for _, mn := range pocTypes {
		am, dm := a.MessageTypes[mn], d.MessageTypes[mn]
		if !slices.Equal(am.In, dm.In) {
			t.Errorf("%s.In = %v; want %v", mn, dm.In, am.In)
		}
		wantKey := "channel_types.run.session.default_in"
		if _, kept := trimmed[mn]; kept || mn == "text" {
			wantKey = "channel_types.run.message_types." + mn + ".in"
		}
		if got := d.InKey(dm); got != wantKey {
			t.Errorf("InKey(%s) = %q; want %q", mn, got, wantKey)
		}
	}
	if d.MessageTypes["text"].In != nil {
		t.Errorf("text.In = %v; want nil", d.MessageTypes["text"].In)
	}
}

// TestDefaultIn_Checks pins AC 8 and 9: the load checks that read in: see
// the derived value, and default_in warns when it gives no type an in:.
func TestDefaultIn_Checks(t *testing.T) {
	keyedChoices := []replace{
		{"    session:\n      initial: working\n", "    session:\n      initial: working\n      default_in: keyed\n"},
		{"        in: [asking]\n", ""},
	}
	t.Run("choices answer keyed in the question's target loads clean", func(t *testing.T) {
		c := parseClean(t, choicesSrc(t, keyedChoices...))
		if w := c.Warnings(); len(w) != 0 {
			t.Errorf("warnings = %v", w)
		}
		ct, _ := c.ChannelType("run")
		if got := ct.MessageTypes["answer"].In; !slices.Equal(got, []string{"asking"}) {
			t.Errorf("answer.In = %v; want [asking]", got)
		}
	})
	t.Run("choices answer keyed elsewhere warns", func(t *testing.T) {
		c := parseClean(t, choicesSrc(t, append(keyedChoices,
			replace{"          on: { answer: { to: working, roles: [owner] } }", "          on: { text: { to: working } }"},
			replace{"          on: { go: { to: done } }", "          on: { go: { to: done }, answer: { to: done } }"})...))
		if !hasWarning(c, choicesKey+".answer", "the buttons never show") {
			t.Errorf("warnings = %v; want the choices warning", c.Warnings())
		}
	})
	t.Run("decision_button on a derived type loads clean", func(t *testing.T) {
		c := parseClean(t, choicesSrc(t, append(keyedChoices,
			replace{"        post: [owner]\n      signal:", "        post: [owner]\n        decision_button: { style: primary }\n      signal:"})...))
		if w := c.Warnings(); len(w) != 0 {
			t.Errorf("warnings = %v", w)
		}
		ct, _ := c.ChannelType("run")
		if got := ct.MessageTypes["go"].In; !slices.Equal(got, []string{"working", "review"}) {
			t.Errorf("go.In = %v; want [working review]", got)
		}
	})
	t.Run("every keyed type has in: warns", func(t *testing.T) {
		c := parseClean(t, defaultInSrc(t, allKeyedAuthored...))
		if w := c.Warnings(); len(w) != 1 || w[0].Key != defaultInKey || w[0].Reason != noEffectWarning {
			t.Errorf("warnings = %v; want one at %s", w, defaultInKey)
		}
	})
	t.Run("timers only warns", func(t *testing.T) {
		c := parseClean(t, defaultInSrc(t,
			replace{"          on:\n            plan: { to: review }\n", "          after:\n            t: { period: 1h, to: review }\n"},
			replace{"          waiting_for: [owner]\n", ""},
			replace{"          on:\n            approve: { to: approved, roles: [owner] }\n" +
				"            revise:  { to: drafting, roles: [owner] }\n",
				"          after:\n            t: { period: 1h, to: approved }\n"}))
		const want = "default_in: keyed has no effect, because no transition is keyed by a message type; drop default_in"
		if w := c.Warnings(); len(w) != 1 || w[0].Key != defaultInKey || w[0].Reason != want {
			t.Errorf("warnings = %v; want one at %s", w, defaultInKey)
		}
	})
	t.Run("one derived type does not warn", func(t *testing.T) {
		c := parseClean(t, defaultInSrc(t,
			replace{"post: [member] }", "post: [member], in: [drafting] }"},
			replace{"post: [owner] }\n      revise", "post: [owner], in: [review] }\n      revise"}))
		if w := c.Warnings(); len(w) != 0 {
			t.Errorf("warnings = %v", w)
		}
	})
}

// TestCheckReload_DefaultIn pins that CheckReload accepts a reload that
// adds or removes default_in, or that brings the AC 9 warning. CheckReload
// compares only the type shape, so these rows cannot show that the next
// post is judged by the new config; realtime TestReload_DefaultIn does.
// Each row checks the In of plan, or the warning, on the next config, so
// it fails when the reload carries the wrong file.
func TestCheckReload_DefaultIn(t *testing.T) {
	with := defaultInBase
	without := defaultInSrc(t, replace{"      default_in: keyed\n", ""})
	noEffect := defaultInSrc(t, allKeyedAuthored...)
	cases := []struct {
		name       string
		prev, next string
		planIn     []string
		warnings   int
	}{
		{"default_in added", without, with, []string{"drafting"}, 0},
		{"default_in removed", with, without, nil, 0},
		{"default_in with no effect", without, noEffect, []string{"drafting"}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prev, next := parseClean(t, tc.prev), parseClean(t, tc.next)
			ct, _ := next.ChannelType("plan")
			if got := ct.MessageTypes["plan"].In; !slices.Equal(got, tc.planIn) {
				t.Errorf("next plan.In = %v; want %v", got, tc.planIn)
			}
			w := next.Warnings()
			if len(w) != tc.warnings || (tc.warnings == 1 && w[0].Key != defaultInKey) {
				t.Fatalf("next warnings = %v; want %d at %s", w, tc.warnings, defaultInKey)
			}
			if err := CheckReload(prev, next); err != nil {
				t.Errorf("refused: %v", err)
			}
		})
	}
}

// TestDefaultIn_DerivedSkipsChecksThatCannotFire pins the comment on
// [messageInCheck.withSession]: a derived In is not checked against the
// reminders or against on_final: close, because each config that would
// fail those checks is refused at its own key. The load reports exactly
// that one problem, so the skipped check had nothing to add.
func TestDefaultIn_DerivedSkipsChecksThatCannotFire(t *testing.T) {
	cases := []struct {
		name, src, key, reason string
	}{
		{"a reminder posts a keyed type outside its keyed states", defaultInSrc(t, replace{"      states:\n",
			"      reminders:\n        poke: { in: [review], every: 1h, post: { type: plan, props: { plan: hi } } }\n" +
				"      states:\n"}),
			"channel_types.plan.session.reminders.poke.post.type", "a reminder never moves the session"},
		{"under on_final: close a type is keyed only in a final state", defaultInSrc(t,
			replace{"      default_in: keyed\n", "      default_in: keyed\n      on_final: close\n"},
			replace{"        approved: { final: true }\n",
				"        approved: { final: true, on: { text: { to: drafting } } }\n"}),
			"channel_types.plan.session.states.approved.on", "a final state has no transitions"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.src), testMeta(t), testPolicy(t))
			assertProblem(t, err, tc.key, tc.reason)
			var cfgErr *Error
			if errors.As(err, &cfgErr) && len(cfgErr.Problems) != 1 {
				t.Errorf("problems = %v; want only the one at %s", cfgErr.Problems, tc.key)
			}
		})
	}
}

// TestDocsExample_DefaultIn pins AC 10: the example after "Take in: from
// the transitions" in the realtime guide loads with no problems and no
// warnings, and its types get the in: the guide describes.
func TestDocsExample_DefaultIn(t *testing.T) {
	data, err := os.ReadFile(realtimeGuide)
	if err != nil {
		t.Fatal(err)
	}
	const para = "**Take `in:` from the transitions.**"
	doc := string(data)
	at := strings.Index(doc, para)
	if at < 0 {
		t.Fatalf("paragraph %q not found in the guide", para)
	}
	example := "version: 1\n" + yamlBlockIn(t, doc[at:], para, "channel_types:")
	if !strings.Contains(example, "default_in: keyed") {
		t.Fatal("the example lost its default_in")
	}
	c := parseClean(t, example)
	if w := c.Warnings(); len(w) != 0 {
		t.Errorf("warnings = %v", w)
	}
	ct, _ := c.ChannelType("plan")
	for mn, want := range map[string][]string{"plan": {"drafting"}, "approve": {"review"}, "revise": {"review"}} {
		mt := ct.MessageTypes[mn]
		if !slices.Equal(mt.In, want) {
			t.Errorf("%s.In = %v; want %v", mn, mt.In, want)
		}
		if got := ct.InKey(mt); got != defaultInKey {
			t.Errorf("InKey(%s) = %q; want %q", mn, got, defaultInKey)
		}
	}
	if in := ct.MessageTypes["text"].In; in != nil {
		t.Errorf("text.In = %v; want nil", in)
	}
}
