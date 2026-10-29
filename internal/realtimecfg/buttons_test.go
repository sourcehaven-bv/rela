package realtimecfg

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// buttonsBase is the guide's plan review with buttons, plus a lead role,
// more property types and a reminder, so the cases below can edit it.
// %s is the list of entries of on.decide out of review.
const buttonsBase = `version: 1
channel_types:
  plan-review:
    label: Plan review
    attach_to: [datalek]
    roles:
      owner:  { label: the owner, from: entity.owner }
      member: { from: attached }
      lead:   { permission: incident.decide }
    message_types:
      text: { properties: { body: { type: markdown, required: true } }, post: [owner, member] }
      plan: { properties: { plan: { type: markdown, required: true } }, post: [member], in: [drafting] }
      nudge: { properties: { body: { type: string, required: true } }, post: [member] }
      decide:
        label: Decide
        properties:
          decision: { type: plan-decision, required: true }
          note:     { type: markdown }
          about:    { type: ref, to: [datalek] }
          count:    { type: integer }
          urgent:   { type: boolean }
          due:      { type: date }
          short:    { type: string, max: 5 }
          tags:     { type: string, list: true, max: 10, max_items: 2 }
        post: [owner, member, lead]
        in: [review]
    session:
      initial: drafting
      reminders:
        ping: { in: [review], every: 1h, post: { type: nudge, props: { body: hi } } }
      states:
        drafting:
          on:
            plan: { to: review }
        review:
          on:
            decide:
%s
        approved: { final: true }
`

// docsEntries are the guide's two decide entries with their buttons.
const docsEntries = `              - to: approved
                when: "message.decision == 'approve'"
                button: { label: Approve, props: { decision: approve } }
              - to: drafting
                button: { label: Revise, props: { decision: revise } }`

const decideKey = "channel_types.plan-review.session.states.review.on.decide"

// buttonsSrc is buttonsBase with entries as the decide list.
func buttonsSrc(entries string) string { return fmt.Sprintf(buttonsBase, entries) }

// entry is one decide list entry: to, then extra keys of the mapping.
func entry(to, extra string) string {
	if extra == "" {
		return "              - { to: " + to + " }"
	}
	return "              - { to: " + to + ", " + extra + " }"
}

func entries(es ...string) string { return strings.Join(es, "\n") }

// parseButtons loads src and fails the test on a problem.
func parseButtons(t *testing.T, src string) *Compiled {
	t.Helper()
	c, err := Parse([]byte(src), testMeta(t), testPolicy(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return c
}

// decideEntries returns the decide entries out of review.
func decideEntries(t *testing.T, c *Compiled) []Transition {
	t.Helper()
	ct, _ := c.ChannelType("plan-review")
	return ct.Session().Branches("review", "decide")
}

// TestButtons_Load pins AC 1: the key loads on a list entry and on a
// mapping entry; null is absent; a missing or empty props posts nothing;
// a null value is absent; the props are in stored form and a copy.
func TestButtons_Load(t *testing.T) {
	t.Run("list entries", func(t *testing.T) {
		c := parseButtons(t, buttonsSrc(docsEntries))
		if w := c.Warnings(); len(w) != 0 {
			t.Errorf("warnings = %v", w)
		}
		got := decideEntries(t, c)
		for i, want := range []string{"Approve", "Revise"} {
			b, ok := got[i].Button()
			if !ok || b.Label != want || got[i].Index() != i {
				t.Errorf("entry %d: button %+v %v index %d; want %s at %d", i, b, ok, got[i].Index(), want, i)
			}
		}
		b, _ := got[1].Button()
		if d := b.Props()["decision"]; d != "revise" {
			t.Errorf("Revise props = %v", b.Props())
		}
	})
	t.Run("label in NFC", func(t *testing.T) {
		// 100 decomposed é are 200 runes; their NFC form fits the limit.
		label := strings.Repeat("e\u0301", 100)
		src := buttonsSrc(entry("drafting", `button: { label: "`+label+`", props: { decision: revise } }`))
		b, _ := decideEntries(t, parseButtons(t, src))[0].Button()
		if want := strings.Repeat("\u00e9", 100); b.Label != want {
			t.Errorf("label = %q; want the NFC form", b.Label)
		}
	})
	t.Run("mapping entry", func(t *testing.T) {
		src := strings.Replace(buttonsSrc(entry("drafting", "")), "            decide:\n              - { to: drafting }",
			"            decide: { to: approved, button: { label: Approve, props: { decision: approve } } }", 1)
		got := decideEntries(t, parseButtons(t, src))
		b, ok := got[0].Button()
		if !ok || b.Label != "Approve" || got[0].Index() != 0 {
			t.Errorf("mapping button = %+v %v", b, ok)
		}
	})
	t.Run("null button", func(t *testing.T) {
		got := decideEntries(t, parseButtons(t, buttonsSrc(entry("drafting", "button: null"))))
		if _, ok := got[0].Button(); ok {
			t.Error("button: null reads as a button")
		}
	})
	t.Run("no button", func(t *testing.T) {
		got := decideEntries(t, parseButtons(t, buttonsSrc(entry("drafting", ""))))
		if _, ok := got[0].Button(); ok {
			t.Error("an entry without button reports one")
		}
	})
	optional := func(src string) string {
		return strings.Replace(src, "decision: { type: plan-decision, required: true }", "decision: { type: plan-decision }", 1)
	}
	for _, props := range []string{"", ", props: {}", ", props: null"} {
		t.Run("no props"+props, func(t *testing.T) {
			src := optional(buttonsSrc(entry("drafting", "button: { label: Go"+props+" }")))
			b, ok := decideEntries(t, parseButtons(t, src))[0].Button()
			if !ok || len(b.Props()) != 0 {
				t.Errorf("button = %+v %v; want one with no props", b, ok)
			}
		})
	}
	t.Run("null value and stored form", func(t *testing.T) {
		src := buttonsSrc(entry("drafting", "button: { label: Go, props: { decision: revise, short: null, "+
			"count: 3, urgent: true, due: 2026-11-01, tags: [ ' a ', b ] } }"))
		b, _ := decideEntries(t, parseButtons(t, src))[0].Button()
		p := b.Props()
		if _, set := p["short"]; set {
			t.Error("a null preset value is set")
		}
		if p["count"] != int64(3) || p["urgent"] != true || p["due"] != "2026-11-01" {
			t.Errorf("props = %#v; want stored form", p)
		}
		tags, _ := p["tags"].([]any)
		if len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
			t.Errorf("tags = %#v; want the normalized list", p["tags"])
		}
		tags[0] = "changed"
		p["count"] = int64(9)
		if again := b.Props(); again["tags"].([]any)[0] != "a" || again["count"] != int64(3) {
			t.Error("Props shares its map or a list with the button")
		}
	})
}

// buttonCase is one refused or warned config: its key and reason.
type buttonCase struct {
	name, entries, key, reason string
}

func runButtonProblems(t *testing.T, cases []buttonCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(buttonsSrc(tc.entries)), testMeta(t), testPolicy(t))
			assertProblem(t, err, tc.key, tc.reason)
		})
	}
}

// TestButtons_Problems pins AC 2: the shape of button.
func TestButtons_Problems(t *testing.T) {
	k := decideKey + "[0].button"
	revise := "props: { decision: revise }"
	runButtonProblems(t, []buttonCase{
		{"no label", entry("drafting", "button: { "+revise+" }"), k + ".label", "name the text of the button"},
		{"blank label", entry("drafting", "button: { label: '  ', "+revise+" }"), k + ".label",
			"name the text of the button"},
		{"long label", entry("drafting", "button: { label: "+strings.Repeat("x", 101)+", "+revise+" }"),
			k + ".label", "must be at most 100 characters"},
		{"hazard label", entry("drafting", "button: { label: \"Go\\u202e\", "+revise+" }"), k + ".label",
			"U+202E, which displays as something other than what it is"},
		{"unknown key", entry("drafting", "button: { label: Go, style: danger, "+revise+" }"), k + ".style",
			"unknown key"},
		{"scalar button", entry("drafting", "button: Approve"), k, "must be a mapping"},
		{"mapping value", entry("drafting", "button: { label: Go, props: { decision: { a: b } } }"),
			k + ".props.decision", "must be a single value or a list"},
		{"mapping in a list", entry("drafting", "button: { label: Go, props: { tags: [ { a: b } ] } }"),
			k + ".props.tags", "must be a single value or a list"},
		{"trigger entry", entry("drafting", "by: trigger, button: { label: Go, "+revise+" }"), k,
			"only a trigger fires this entry"},
	})
	t.Run("no person class", func(t *testing.T) {
		src := strings.Replace(buttonsSrc(entry("drafting", "button: { label: Go, "+revise+" }")),
			"        post: [owner, member, lead]\n", "        post: []\n", 1)
		_, err := Parse([]byte(src), testMeta(t), testPolicy(t))
		assertProblem(t, err, k, "no role may post decide, so no person could click this button")
	})
}

// TestButtons_PropsProblems pins AC 3: the preset follows the rules of a
// post.
func TestButtons_PropsProblems(t *testing.T) {
	k := decideKey + "[0].button.props"
	btn := func(props string) string { return entry("drafting", "button: { label: Go, props: { "+props+" } }") }
	var many strings.Builder
	for i := range 33 {
		fmt.Fprintf(&many, "p%d: x, ", i)
	}
	runButtonProblems(t, []buttonCase{
		{"undeclared", btn("decision: revise, nope: x"), k, `message type decide has no property "nope"`},
		{"string for integer", btn("decision: revise, count: 'three'"), k, `property "count": must be a whole number`},
		{"quoted boolean", btn("decision: revise, urgent: 'true'"), k, `property "urgent": must be a boolean`},
		{"yes boolean", btn("decision: revise, urgent: yes"), k, `property "urgent": must be a boolean`},
		{"outside enum", btn("decision: maybe"), k, `property "decision": must be one of [approve revise]`},
		{"list for enum", btn("decision: [approve]"), k, `property "decision": must be a string`},
		{"over max", btn("decision: revise, short: toolong"), k, `property "short": must be at most 5 characters`},
		{"empty item", btn("decision: revise, tags: ['  ']"), k, `property "tags": item 1 is empty`},
		{"long item", btn("decision: revise, tags: [abcdefghijk]"), k, `item 1 must be at most 10 characters`},
		{"repeated item", btn("decision: revise, tags: [a, ' a']"), k, `item 2 repeats an earlier item`},
		{"hazard item", btn("decision: revise, tags: [\"a\\u202e\"]"), k, `item 1 holds U+202E`},
		{"too many items", btn("decision: revise, tags: [a, b, c]"), k, `must have at most 2 items`},
		{"required left out", btn("short: ab"), k, `property "decision" is required`},
		{"required null", btn("decision: null"), k, `property "decision" is required`},
		{"too many properties", btn(many.String() + "decision: revise"), k, "at most 32 properties"},
		{"markdown", btn("decision: revise, note: hi"), k + ".note", "a preset cannot set a markdown property"},
		{"ref", btn("decision: revise, about: DL-1"), k + ".about",
			"a preset cannot set a ref property; a click cannot check the entity it names"},
		{"markdown reason", btn("decision: revise, note: hi"), k + ".note", "a click posts text the clicker never sees"},
		{"integer over 2^53", btn("decision: revise, count: 9007199254740993"), k + ".count",
			"a browser cannot carry a larger integer exactly"},
		{"integer under -2^53", btn("decision: revise, count: -9007199254740993"), k + ".count",
			"must be between -9007199254740992 and 9007199254740992"},
	})
	t.Run("integer at 2^53", func(t *testing.T) {
		b, _ := decideEntries(t, parseButtons(t, buttonsSrc(btn("decision: revise, count: 9007199254740992"))))[0].Button()
		if b.Props()["count"] != int64(1<<53) {
			t.Errorf("count = %v; want 2^53", b.Props()["count"])
		}
	})
	t.Run("required empty list", func(t *testing.T) {
		src := strings.Replace(buttonsSrc(btn("decision: revise, tags: []")),
			"tags:     { type: string, list: true, max: 10, max_items: 2 }",
			"tags:     { type: string, list: true, max: 10, max_items: 2, required: true }", 1)
		_, err := Parse([]byte(src), testMeta(t), testPolicy(t))
		assertProblem(t, err, k, `property "tags" is required`)
	})
	t.Run("over message_bytes", func(t *testing.T) {
		src := strings.Replace(buttonsSrc(btn("decision: revise")), "version: 1\n",
			"version: 1\nlimits: { message_bytes: 10 }\n", 1)
		_, err := Parse([]byte(src), testMeta(t), testPolicy(t))
		assertProblem(t, err, k, "the preset is over message_bytes (10 bytes)")
	})
	when := func(cond string) string {
		return strings.Replace(buttonsSrc(btn("decision: revise")), "        post: [owner, member, lead]\n",
			"        post: [owner, member, lead]\n        when: \""+cond+"\"\n", 1)
	}
	t.Run("fails the type's when", func(t *testing.T) {
		_, err := Parse([]byte(when("message.decision == 'approve'")), testMeta(t), testPolicy(t))
		assertProblem(t, err, k, "the preset does not meet the when: condition of message type decide")
	})
	t.Run("type's when calls today", func(t *testing.T) {
		_, err := Parse([]byte(when("message.due == nil or message.due > today()")), testMeta(t), testPolicy(t))
		assertProblem(t, err, k, "calls today(), so whether this preset meets it would depend on the date")
	})
	t.Run("type's when holds", func(t *testing.T) {
		parseButtons(t, when("message.decision == 'approve' or message.decision == 'revise'"))
	})
}

// TestButtons_Guards pins AC 4.
func TestButtons_Guards(t *testing.T) {
	approve := "button: { label: Approve, props: { decision: approve } }"
	approve2 := "button: { label: Approve again, props: { decision: approve } }"
	revise := "button: { label: Revise, props: { decision: revise } }"
	isApprove := `when: "message.decision == 'approve'"`
	k0, k1, k2 := decideKey+"[0]", decideKey+"[1]", decideKey+"[2]"
	reminded := `when: "session.reminders.ping > 2"`

	loads := []struct {
		name, entries string
		// shadowing are the entries recorded as taking the preset first;
		// hidden adds the entries the load cannot judge.
		shadowing, hidden []int
	}{
		{"own guard holds", entries(entry("approved", isApprove+", "+approve), entry("drafting", "")), nil, nil},
		{"own guard reads session.state", entries(entry("approved", `when: "session.state == 'review'", `+approve),
			entry("drafting", "")), nil, nil},
		{"earlier session guard for a disjoint role", entries(entry("drafting", "roles: [lead], "+reminded),
			entry("approved", "roles: [member], "+approve)), nil, []int{0}},
		{"earlier trigger entry with a session guard", entries(entry("drafting", "by: trigger, "+reminded),
			entry("approved", approve)), nil, nil},
		{"shadowed for a lead, reachable for others", entries(entry("drafting", "roles: [lead]"),
			entry("approved", approve)), []int{0}, []int{0}},
		{"earlier guard that does not hold", entries(entry("approved", isApprove), entry("drafting", revise)),
			nil, nil},
		{"earlier guard holds for a disjoint role", entries(entry("drafting", "roles: [lead], "+isApprove),
			entry("approved", "roles: [member], "+approve)), []int{0}, []int{0}},
		{"earlier guard errors for a disjoint role", entries(entry("drafting", `roles: [lead], when: "regex(message.short, '[')"`),
			entry("approved", "roles: [member], button: { label: Go, props: { decision: approve, short: a } }")),
			nil, []int{0}},
	}
	for _, tc := range loads {
		t.Run(tc.name, func(t *testing.T) {
			src := buttonsSrc(tc.entries)
			c := parseButtons(t, src)
			var b TransitionButton
			ok := false
			for _, e := range decideEntries(t, c) {
				if eb, has := e.Button(); has {
					b, ok = eb, true
				}
			}
			if !ok {
				t.Fatal("the button entry lost its button")
			}
			if !slices.Equal(b.shadowing, tc.shadowing) {
				t.Errorf("shadowing %v; want %v", b.shadowing, tc.shadowing)
			}
			if !slices.Equal(b.hiddenBy(), tc.hidden) {
				t.Errorf("hidden by %v; want %v", b.hiddenBy(), tc.hidden)
			}
		})
	}

	runButtonProblems(t, []buttonCase{
		{"own guard does not hold", entries(entry("approved", isApprove+", "+revise), entry("drafting", "")),
			k0 + ".button", "the entry's when: does not hold for the preset"},
		{"own guard reads session.reminders", entries(entry("approved", reminded+", "+approve), entry("drafting", "")),
			k0 + ".button", "the entry's when: reads session.reminders"},
		{"own guard calls today", entries(entry("approved", `when: "message.due == nil or message.due < today()", `+
			approve), entry("drafting", "")), k0 + ".button", "the entry's when: calls today()"},
		{"own guard errors", entries(entry("approved", `when: "regex(message.short, '[')", `+
			"button: { label: Go, props: { decision: approve, short: a } }"), entry("drafting", "")),
			k0 + ".button", "the entry's when: fails to evaluate for the preset"},
		{"earlier guard errors", entries(entry("approved", `when: "regex(message.short, '[')"`),
			entry("drafting", "button: { label: Go, props: { decision: revise, short: a } }")),
			k1 + ".button", "the when: of entry 0 fails to evaluate for the preset"},
		{"earlier session guard for a shared role", entries(entry("drafting", reminded),
			entry("approved", approve)), k1 + ".button", "no one would ever see the button"},
		{"earlier unguarded entry for the same role", entries(entry("drafting", "roles: [lead]"),
			entry("approved", "roles: [lead], "+approve)), k1, "can never fire"},
		{"earlier guard holds for the preset", entries(entry("approved", isApprove),
			entry("drafting", approve)), k1 + ".button", "no one would ever see the button"},
		{"two buttons with the same preset", entries(entry("approved", isApprove+", "+approve),
			entry("drafting", approve2), entry("approved", "")), k1 + ".button", "no one would ever see the button"},
	})

	t.Run("refused by unreachable reports once", func(t *testing.T) {
		_, err := Parse([]byte(buttonsSrc(entries(entry("drafting", ""), entry("approved", approve)))),
			testMeta(t), testPolicy(t))
		assertProblem(t, err, k1, "can never fire")
		assertNoProblemAt(t, err, k1+".button")
	})
	t.Run("broken guard reports once", func(t *testing.T) {
		_, err := Parse([]byte(buttonsSrc(entries(entry("approved", `when: "message.nope == 1", `+approve),
			entry("drafting", "")))), testMeta(t), testPolicy(t))
		assertProblem(t, err, k0+".when", "does not compile")
		assertNoProblemAt(t, err, k0+".button")
	})
	t.Run("earlier broken guard reports once", func(t *testing.T) {
		_, err := Parse([]byte(buttonsSrc(entries(entry("approved", `when: "message.nope == 1"`),
			entry("drafting", revise), entry("approved", "roles: [lead]")))), testMeta(t), testPolicy(t))
		assertProblem(t, err, k0+".when", "does not compile")
		assertNoProblemAt(t, err, k1+".button")
		assertNoProblemAt(t, err, k2+".button")
	})
}

// assertNoProblemAt fails when err has a problem whose key is key.
func assertNoProblemAt(t *testing.T, err error, key string) {
	t.Helper()
	var cfgErr *Error
	if !errors.As(err, &cfgErr) {
		return
	}
	for _, p := range cfgErr.Problems {
		if p.Key == key {
			t.Errorf("unexpected problem at %s: %s", key, p.Reason)
		}
	}
}

// TestButtons_StateChecks pins AC 5: the labels of a state together.
func TestButtons_StateChecks(t *testing.T) {
	isApprove := `when: "message.decision == 'approve'"`
	k1 := decideKey + "[1].button.label"
	runButtonProblems(t, []buttonCase{
		{"case folded duplicate", entries(entry("approved", isApprove+", button: { label: Go, props: { decision: approve } }"),
			entry("drafting", "button: { label: GO, props: { decision: revise } }")),
			k1, "the button of " + decideKey + "[0] in state review already reads \"Go\""},
		{"duplicate that differs by a style selector", entries(
			entry("approved", isApprove+", button: { label: \"Go✅\", props: { decision: approve } }"),
			entry("drafting", "button: { label: \"Go✅\uFE0F\", props: { decision: revise } }")),
			k1, "already reads"},
		// A caller with both roles could see both: the lead entry's guard
		// does not hold for the member entry's preset.
		{"same label on disjoint role entries", entries(
			entry("approved", "roles: [lead], "+isApprove+", button: { label: Go, props: { decision: approve } }"),
			entry("drafting", "roles: [member], button: { label: Go, props: { decision: revise } }")),
			k1, "already reads"},
	})
	t.Run("duplicate across types", func(t *testing.T) {
		src := strings.Replace(buttonsSrc(entries(entry("approved", isApprove+", button: { label: Go, props: { decision: approve } }"),
			entry("drafting", ""))),
			"            decide:\n", "            nudge: { to: drafting, button: { label: go, props: { body: x } } }\n            decide:\n", 1)
		src = strings.Replace(src, "nudge: { properties: { body: { type: string, required: true } }, post: [member] }",
			"nudge: { properties: { body: { type: string, required: true } }, post: [member], in: [review] }", 1)
		_, err := Parse([]byte(src), testMeta(t), testPolicy(t))
		assertProblem(t, err, "channel_types.plan-review.session.states.review.on.nudge.button.label", "already reads")
	})
	t.Run("lead entry that shadows a later entry", func(t *testing.T) {
		c := parseButtons(t, buttonsSrc(entries(
			entry("drafting", "roles: [lead], button: { label: Go, props: { decision: approve } }"),
			entry("approved", "button: { label: Go, props: { decision: approve } }"))))
		if w := c.Warnings(); len(w) != 0 {
			t.Errorf("warnings = %v", w)
		}
	})
	t.Run("eleven buttons warn", func(t *testing.T) {
		// A list holds at most 8 entries, so the buttons are split over
		// decide and a second type.
		button := func(i int, props string) string {
			return entry("drafting", fmt.Sprintf("when: \"message.count == %d\", button: { label: B%d, props: { %s } }",
				i, i, props))
		}
		var d1, d2 []string
		for i := range 7 {
			d1 = append(d1, button(i, fmt.Sprintf("decision: revise, count: %d", i)))
		}
		for i := 7; i < 11; i++ {
			d2 = append(d2, button(i, fmt.Sprintf("count: %d", i)))
		}
		src := strings.Replace(buttonsSrc(entries(d1...)), "        in: [review]\n",
			"        in: [review]\n      tally:\n        properties: { count: { type: integer } }\n"+
				"        post: [member]\n", 1)
		src = strings.Replace(src, "            decide:\n", "            tally:\n"+entries(d2...)+"\n            decide:\n", 1)
		c := parseButtons(t, src)
		if !hasWarning(c, "channel_types.plan-review.session.states.review.on", "11 preset buttons") {
			t.Errorf("warnings = %v; want the crowding warning", c.Warnings())
		}
	})
	t.Run("label equals a type label", func(t *testing.T) {
		c := parseButtons(t, buttonsSrc(entry("drafting", "button: { label: decide, props: { decision: revise } }")))
		if !hasWarning(c, decideKey+"[0].button.label", "equals the label of message type decide") {
			t.Errorf("warnings = %v; want the type label warning", c.Warnings())
		}
	})
	t.Run("label equals a trigger-only type label", func(t *testing.T) {
		src := strings.Replace(buttonsSrc(entry("drafting", "button: { label: Flag, props: { decision: revise } }")),
			"            decide:\n", "            flag: { to: drafting, by: trigger }\n            decide:\n", 1)
		src = strings.Replace(src, "      decide:\n",
			"      flag: { label: Flag, properties: { body: { type: string } }, post: [member] }\n      decide:\n", 1)
		c := parseButtons(t, src)
		if hasWarning(c, decideKey+"[0].button.label", "equals the label of message type") {
			t.Errorf("warnings = %v; a trigger-only type has no button to clash with", c.Warnings())
		}
		// The same config with a person-fired flag entry warns, so the case
		// above is not passing for another reason.
		c = parseButtons(t, strings.Replace(src, "flag: { to: drafting, by: trigger }", "flag: { to: drafting }", 1))
		if !hasWarning(c, decideKey+"[0].button.label", "equals the label of message type flag") {
			t.Errorf("warnings = %v; want the type label warning", c.Warnings())
		}
	})
	t.Run("docs example has no problems", func(t *testing.T) {
		if w := parseButtons(t, buttonsSrc(docsEntries)).Warnings(); len(w) != 0 {
			t.Errorf("warnings = %v", w)
		}
	})
}

func hasWarning(c *Compiled, key, reason string) bool {
	for _, w := range c.Warnings() {
		if w.Key == key && strings.Contains(w.Reason, reason) {
			return true
		}
	}
	return false
}

// TestButtons_Offers pins the offer rule: a button is offered to a caller
// its entry admits, unless an earlier unguarded entry, or an entry the
// load recorded on the button, admits them too. A caller with two roles
// is judged on both.
func TestButtons_Offers(t *testing.T) {
	src := buttonsSrc(entries(
		entry("drafting", "roles: [lead]"),
		entry("drafting", `roles: [owner], when: "session.reminders.ping > 2"`),
		entry("approved", "button: { label: Approve, props: { decision: approve } }"),
	))
	ct, _ := parseButtons(t, src).ChannelType("plan-review")
	st, _ := ct.Session().State("review")
	es := decideEntries(t, parseButtons(t, src))
	holds := func(roles ...string) func(string) bool {
		return func(r string) bool { return strings.Contains(" "+strings.Join(roles, " ")+" ", " "+r+" ") }
	}
	cases := []struct {
		name  string
		holds func(string) bool
		entry int
		want  bool
	}{
		{"lead takes the lead entry", holds("lead"), 0, true},
		{"lead loses the button", holds("lead"), 2, false},
		{"member sees the button", holds("member"), 2, true},
		{"owner loses the button to an undecidable entry", holds("owner"), 2, false},
		{"owner is offered the guarded entry", holds("owner"), 1, true},
		{"member and lead loses the button", holds("member", "lead"), 2, false},
		{"member and owner loses the button", holds("member", "owner"), 2, false},
		{"member is not offered the lead entry", holds("member"), 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := st.Offers(es[tc.entry], tc.holds); got != tc.want {
				t.Errorf("Offers = %v; want %v", got, tc.want)
			}
		})
	}
}

// TestButtons_DisjointRoleRecorded pins the two-role rule for an earlier
// entry whose roles share none with the button's: a caller who holds a
// role of each reaches the earlier entry first, so the load records it.
func TestButtons_DisjointRoleRecorded(t *testing.T) {
	src := buttonsSrc(entries(
		entry("drafting", `roles: [lead], when: "message.decision == 'approve'"`),
		entry("approved", "roles: [member], button: { label: Approve, props: { decision: approve } }"),
	))
	c := parseButtons(t, src)
	ct, _ := c.ChannelType("plan-review")
	st, _ := ct.Session().State("review")
	es := decideEntries(t, c)
	if !st.Offers(es[1], func(r string) bool { return r == "member" }) {
		t.Error("a member is not offered the button")
	}
	if st.Offers(es[1], func(r string) bool { return r == "member" || r == "lead" }) {
		t.Error("a member and lead is offered a button whose click the lead entry takes")
	}
}

// TestButtons_ErroringDisjointGuardHides pins the error branch for an
// earlier entry that admits none of the button's posters: the button
// loads, and a caller who also holds that entry's role does not see it,
// because the decision would stop at the failing guard.
func TestButtons_ErroringDisjointGuardHides(t *testing.T) {
	src := buttonsSrc(entries(
		entry("drafting", `roles: [lead], when: "regex(message.short, '[')"`),
		entry("approved", "roles: [member], button: { label: Go, props: { decision: approve, short: a } }"),
	))
	c := parseButtons(t, src)
	ct, _ := c.ChannelType("plan-review")
	st, _ := ct.Session().State("review")
	es := decideEntries(t, c)
	if !st.Offers(es[1], func(r string) bool { return r == "member" }) {
		t.Error("a member is not offered the button")
	}
	if st.Offers(es[1], func(r string) bool { return r == "member" || r == "lead" }) {
		t.Error("a member and lead is offered a button whose click stops at the failing guard")
	}
}

// TestButtons_OffersForeignEntry pins that Offers refuses an entry of
// another state.
func TestButtons_OffersForeignEntry(t *testing.T) {
	ct, _ := parseButtons(t, buttonsSrc(docsEntries)).ChannelType("plan-review")
	review, _ := ct.Session().State("review")
	drafting, _ := ct.Session().State("drafting")
	all := func(string) bool { return true }
	if !review.Offers(review.Transitions()[1], all) {
		t.Fatal("review does not offer its own Revise entry")
	}
	if drafting.Offers(review.Transitions()[1], all) {
		t.Error("drafting offers an entry of review")
	}
}
