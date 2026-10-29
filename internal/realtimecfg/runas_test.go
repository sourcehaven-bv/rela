package realtimecfg

import (
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// runAsBase is a session with run_as and one timer that sets a literal.
const runAsBase = `version: 1
channel_types:
  br:
    attach_to: [datalek]
    roles:
      lead:   { from: entity.owner }
      member: { from: attached }
    message_types:
      text:      { properties: { body: { type: string } }, post: [member] }
      contained: { properties: { risk: { type: risk-level }, note: { type: markdown } }, post: [lead, member] }
    session:
      initial: open
      run_as: system:bot
      states:
        open:
          on:
            contained: { to: done, roles: [lead] }
          after:
            late:
              period: 1h
              to: done
              outputs:
                - set: { properties: { status: late } }
        done: { final: true }
`

// botPolicy grants system:bot what runAsBase and its variants write.
const botPolicy = `
roles:
  bot:
    read: [datalek, timeline-entry]
    update: [datalek]
    create: [timeline-entry]
assignments:
  system:bot: bot
`

func parsePolicy(t *testing.T, body string) *acl.Policy {
	t.Helper()
	p, err := acl.ParsePolicy([]byte(body), "acl.yaml")
	if err != nil {
		t.Fatalf("parse acl: %v", err)
	}
	return p
}

// editBase applies each replacement to runAsBase; each must match exactly
// once.
func editBase(t *testing.T, edits ...replace) string {
	t.Helper()
	src := runAsBase
	for _, e := range edits {
		if strings.Count(src, e.old) != 1 {
			t.Fatalf("edit %q matches %d times; want exactly one", e.old, strings.Count(src, e.old))
		}
		src = strings.Replace(src, e.old, e.new, 1)
	}
	return src
}

const runAsLine = "      run_as: system:bot\n"

// TestSessionRunAs_Validation pins AC 1: a job identity loads, and every
// other name is refused with one problem at session.run_as.
func TestSessionRunAs_Validation(t *testing.T) {
	const key = "channel_types.br.session.run_as"
	cases := []struct {
		name, value string
		ok          bool
	}{
		{"system name", "system:breach-bot", true},
		{"integration name", "integration:breach-bot", true},
		{"plain name", "alice", false},
		{"unknown", "unknown", false},
		{"empty", `""`, false},
		{"upper case and space", `"system:Breach Bot"`, false},
		{"empty segment", "system:a::b", false},
		{"realtime", "system:realtime", false},
		{"scheduler", "system:scheduler", false},
		{"automation", "system:automation", false},
		{"webhook", "system:webhook:x", false},
		{"subscription", "system:subscription:x", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := editBase(t, replace{runAsLine, "      run_as: " + tc.value + "\n"})
			c, err := Parse([]byte(src), testMeta(t), nil)
			if tc.ok {
				if err != nil {
					t.Fatalf("Parse: %v", err)
				}
				ct, _ := c.ChannelType("br")
				if got := ct.Session().RunAs(); got != tc.value {
					t.Errorf("RunAs = %q; want %q", got, tc.value)
				}
				return
			}
			assertProblem(t, err, key, "")
			var perr *Error
			if !errors.As(err, &perr) || len(perr.Problems) != 1 {
				t.Errorf("problems = %v; want 1", err)
			}
		})
	}
}

// TestSessionRunAs_UnusedWarns pins AC 3: run_as on a session without job
// outputs loads with one warning.
func TestSessionRunAs_UnusedWarns(t *testing.T) {
	src := editBase(t, replace{"              outputs:\n                - set: { properties: { status: late } }\n", ""})
	c, err := Parse([]byte(src), testMeta(t), parsePolicy(t, botPolicy))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	w := c.Warnings()
	if len(w) != 1 || w[0].Key != "channel_types.br.session.run_as" ||
		w[0].Reason != "session.run_as is not used: no timer or trigger move of this session has outputs" {

		t.Errorf("warnings = %+v", w)
	}
}

// jobVariant returns runAsBase with one job-move output whose status
// value is tpl. kind is timer, trigger (by: trigger) or reachable (no
// roles, and a trigger posts the message type).
func jobVariant(t *testing.T, kind, tpl string) string {
	t.Helper()
	timerOut := "              outputs:\n                - set: { properties: { status: late } }\n"
	set := `[ { set: { properties: { status: "` + tpl + `" } } } ]`
	switch kind {
	case "timer":
		return editBase(t, replace{timerOut,
			"              outputs:\n                - set: { properties: { status: \"" + tpl + "\" } }\n"})
	case "trigger":
		return editBase(t, replace{timerOut, ""},
			replace{"contained: { to: done, roles: [lead] }", "contained: { to: done, by: trigger, outputs: " + set + " }"})
	default:
		return editBase(t, replace{timerOut, ""},
			replace{"contained: { to: done, roles: [lead] }", "contained: { to: done, outputs: " + set + " }"},
			replace{"        done: { final: true }\n", "        done: { final: true }\n" + triggerT1})
	}
}

// TestJobOutputs_Templates pins AC 4: per job-move output kind, the
// references a template may and may not read.
func TestJobOutputs_Templates(t *testing.T) {
	keys := map[string]string{
		"timer":     "channel_types.br.session.states.open.after.late.outputs[0].set.properties.status",
		"trigger":   "channel_types.br.session.states.open.on.contained.outputs[0].set.properties.status",
		"reachable": "channel_types.br.session.states.open.on.contained.outputs[0].set.properties.status",
	}
	cases := []struct {
		ref string
		// refused maps an output kind to the reason it is refused with.
		refused map[string]string
	}{
		{"{{message.author}}", map[string]string{"timer": "a timer move has no message",
			"trigger": "cannot read message.author", "reachable": "cannot read message.author"}},
		{"{{channel.title}}", map[string]string{"timer": "channel.title is free text",
			"trigger": "channel.title is free text", "reachable": "channel.title is free text"}},
		{"{{message.note}}", map[string]string{"timer": "a timer move has no message"}},
		{"x {{message.note}}", map[string]string{"timer": "a timer move has no message"}},
		{"{{session.from}}", nil},
		{"{{session.to}}", nil},
		{"{{session.generation}}", nil},
		{"{{channel.key}}", nil},
		{"{{channel.type}}", nil},
		{"{{now}}", nil},
		{"{{today}}", nil},
		{"literal", nil},
	}
	policy := parsePolicy(t, botPolicy)
	for _, tc := range cases {
		for _, kind := range []string{"timer", "trigger", "reachable"} {
			t.Run(kind+" "+tc.ref, func(t *testing.T) {
				_, err := Parse([]byte(jobVariant(t, kind, tc.ref)), testMeta(t), policy)
				reason, refused := tc.refused[kind]
				if !refused {
					if err != nil {
						t.Fatalf("Parse: %v", err)
					}
					return
				}
				assertProblem(t, err, keys[kind], reason)
			})
		}
	}
}

// TestTriggerOutputs_Compile pins AC 5: by: trigger and trigger-reachable
// transitions take outputs that read the trigger's message properties, and
// a transition with roles keeps the person rules.
func TestTriggerOutputs_Compile(t *testing.T) {
	policy := parsePolicy(t, botPolicy)
	for _, kind := range []string{"trigger", "reachable"} {
		c, err := Parse([]byte(jobVariant(t, kind, "{{message.note}}")), testMeta(t), policy)
		if err != nil {
			t.Fatalf("%s: Parse: %v", kind, err)
		}
		ct, _ := c.ChannelType("br")
		tr, _ := firstBranch(ct.Session(), "open", "contained")
		if len(tr.Outputs()) != 1 || !tr.job {
			t.Errorf("%s: outputs = %+v, job = %v", kind, tr.Outputs(), tr.job)
		}
	}
	person := editBase(t,
		replace{"contained: { to: done, roles: [lead] }",
			`contained: { to: done, roles: [lead], outputs: [ { set: { properties: { status: "{{message.author}} {{channel.title}}" } } } ] }`},
		replace{"        done: { final: true }\n", "        done: { final: true }\n" + triggerT1})
	c, err := Parse([]byte(person), testMeta(t), policy)
	if err != nil {
		t.Fatalf("person transition: %v", err)
	}
	ct, _ := c.ChannelType("br")
	if tr, _ := firstBranch(ct.Session(), "open", "contained"); tr.job || len(tr.Outputs()) != 1 {
		t.Errorf("a transition with roles is a person move: job = %v", tr.job)
	}
}

// TestTriggerOutputs_Branches pins TKT-FPXTQJ AC 7: a list entry a trigger
// can reach is a job move whose outputs need session.run_as, unless an
// earlier entry without when already admits triggers.
func TestTriggerOutputs_Branches(t *testing.T) {
	const out = "outputs: [ { set: { properties: { status: x } } } ]"
	base := func(on string) string {
		return editBase(t, replace{runAsLine, ""},
			replace{"              outputs:\n                - set: { properties: { status: late } }\n", ""},
			replace{"contained: { to: done, roles: [lead] }", "contained: " + on},
			replace{"        done: { final: true }\n", "        done: { final: true }\n" + triggerT1})
	}
	_, err := Parse([]byte(base("[ { to: open, roles: [lead] }, { to: done, "+out+" } ]")), testMeta(t), nil)
	assertOneProblem(t, err, "channel_types.br.session.states.open.on.contained[1].outputs[0].set",
		"set session.run_as to a job identity")

	c, err := Parse([]byte(base("[ { to: open, by: trigger }, { to: done, "+out+" } ]")), testMeta(t), nil)
	if err != nil {
		t.Fatalf("an entry no trigger reaches needs no run_as: %v", err)
	}
	ct, _ := c.ChannelType("br")
	if tr, ok := ct.Session().TransitionTo("open", "contained", "done"); !ok || tr.Job() {
		t.Errorf("shadowed entry: %+v, job = %v", tr, tr.Job())
	}
}

// TestRunAsGrantCheck pins AC 6: load refuses a run_as that lacks a grant
// its job outputs need, one problem per missing grant at the output key.
func TestRunAsGrantCheck(t *testing.T) {
	const timer = "channel_types.br.session.states.open.after.late.outputs[0]"
	timerOut := "                - set: { properties: { status: late } }\n"
	setOut := func(body string) replace { return replace{timerOut, "                - " + body + "\n"} }
	pageType := []replace{
		{"    attach_to: [datalek]\n", "    attach_to: [page]\n"},
		{"      lead:   { from: entity.owner }\n", ""},
		{"contained: { to: done, roles: [lead] }", "contained: { to: done, roles: [member] }"},
		{"post: [lead, member]", "post: [member]"},
	}
	faceAnchored := append([]replace{{"    attach_to: [datalek]\n", "    attach_to: [page]\n    anchor: face\n"}},
		pageType[1:]...)
	cases := []struct {
		name   string
		edits  []replace
		policy string
		// key and reason name the expected problem; an empty key means the
		// file loads.
		key, reason string
	}{
		{"unfaced type granted", nil, botPolicy, "", ""},
		{"unfaced type without update", nil, `
roles:
  bot: { read: [datalek] }
assignments: { system:bot: bot }
`, timer + ".set", "system:bot cannot update datalek: no role assigned to it or to everyone grants it"},
		{"face-anchored faced type needs every face", append(faceAnchored, setOut("set: { properties: { title: x } }")), `
roles:
  bot: { read: [page], update: ["page@draft"] }
assignments: { system:bot: bot }
`, timer + ".set", "system:bot cannot update page@published"},
		{"face-anchored faced type with every face", append(faceAnchored, setOut("set: { properties: { title: x } }")), `
roles:
  bot: { read: [page], update: ["page@draft", "page@published"] }
assignments: { system:bot: bot }
`, "", ""},
		{"family-anchored faced set", append(pageType, setOut("set: { face: draft, properties: { title: x } }")), `
roles:
  bot: { read: [page], update: ["page@published"] }
assignments: { system:bot: bot }
`, timer + ".set", "system:bot cannot update page@draft"},
		{"family-anchored set over faced and unfaced types", append([]replace{
			{"    attach_to: [datalek]\n", "    attach_to: [page, datalek]\n"}}, append(pageType[1:],
			setOut("set: { face: draft, properties: { title: x } }"))...), `
roles:
  bot: { read: [page, datalek], update: ["page@draft", datalek] }
assignments: { system:bot: bot }
`, "", ""},
		{"faced create", []replace{setOut("create: { type: memo, face: draft, properties: { title: x, kind: y } }")}, `
roles:
  bot: { read: [memo], create: ["memo@published"] }
assignments: { system:bot: bot }
`, timer + ".create", "system:bot cannot create memo@draft"},
		{"create field outside the block",
			[]replace{setOut("create: { type: memo, face: draft, properties: { title: x, kind: y } }")}, `
roles:
  bot:
    read: [memo]
    create: ["memo@draft"]
    fields: { memo: [{field: title}] }
assignments: { system:bot: bot }
`, timer + ".create.properties.kind", "system:bot cannot create memo.kind on memo@draft"},
		{"fields-blocked role", nil, `
roles:
  bot:
    read: [datalek]
    update: [datalek]
    fields: { datalek: [{field: title}] }
assignments: { system:bot: bot }
`, timer + ".set.properties.status", "system:bot cannot set datalek.status on datalek"},
		{"blocked role beside a blockless everyone", nil, `
roles:
  bot:
    read: [datalek]
    update: [datalek]
    fields: { datalek: [{field: title}] }
  everyone: { read: [datalek], update: [datalek] }
assignments: { system:bot: bot }
`, "", ""},
		{"everyone only", nil, `
roles:
  everyone: { read: [datalek], update: [datalek] }
`, "", ""},
		{"local role only", nil, `
roles:
  owner: { read: [datalek], update: [datalek] }
role_relations:
  about: { confers: owner }
`, timer + ".set", "system:bot cannot update datalek"},
		{"asserted role only", nil, `
roles:
  bot: { read: [datalek], update: [datalek] }
asserted_role_assignments:
  system:bot: [bot]
`, timer + ".set", "system:bot cannot update datalek"},
		{"enum value outside the option grant", []replace{setOut("set: { properties: { severity: high } }")}, `
roles:
  bot:
    read: [datalek]
    update: [datalek]
    options: { datalek: [{field: severity, option: low}] }
assignments: { system:bot: bot }
`, timer + ".set.properties.severity", `system:bot cannot set datalek.severity to "high"`},
		{"create related to the anchor", []replace{
			setOut("create: { type: timeline-entry, properties: { title: x }, relate: { type: about } }"),
		}, botPolicy, "", ""},
	}
	meta := testMeta(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := editBase(t, tc.edits...)
			_, err := Parse([]byte(src), meta, parsePolicy(t, tc.policy))
			if tc.key == "" {
				if err != nil {
					t.Fatalf("Parse: %v", err)
				}
				return
			}
			assertProblem(t, err, tc.key, tc.reason)
		})
	}

	t.Run("nil policy skips the check", func(t *testing.T) {
		if _, err := Parse([]byte(runAsBase), meta, nil); err != nil {
			t.Fatalf("Parse: %v", err)
		}
	})
	t.Run("trigger literal enum value", func(t *testing.T) {
		src := editBase(t,
			replace{"              outputs:\n" + timerOut, ""},
			replace{"contained: { to: done, roles: [lead] }",
				`contained: { to: done, by: trigger, outputs: [ { set: { properties: { risk: "{{message.risk}}" } } } ] }`},
			replace{"        done: { final: true }\n", "        done: { final: true }\n" +
				strings.Replace(triggerT1, "post: { type: contained }", "post: { type: contained, props: { risk: high } }", 1)})
		_, err := Parse([]byte(src), meta, parsePolicy(t, `
roles:
  bot:
    read: [datalek]
    update: [datalek]
    options: { datalek: [{field: risk, option: low}] }
assignments: { system:bot: bot }
`))
		assertProblem(t, err, "channel_types.br.session.states.open.on.contained.outputs[0].set.properties.risk",
			`system:bot cannot set datalek.risk to "high"`)
	})
}

// TestJobLoops pins that a loop of a timer and a trigger-fired transition
// with outputs on it is refused at the timer, and that the same loop loads
// without outputs or when the transition names roles.
func TestJobLoops(t *testing.T) {
	const timerKey = "channel_types.br.session.states.open.after.late"
	loop := func(onStale, timerOutputs string) string {
		return editBase(t,
			replace{"              to: done\n              outputs:\n                - set: { properties: { status: late } }\n",
				"              to: stale\n" + timerOutputs},
			replace{"        done: { final: true }\n",
				"        stale:\n          on:\n            contained: " + onStale + "\n        done: { final: true }\n"})
	}
	withOutputs := "              outputs:\n                - set: { properties: { status: late } }\n"
	cases := []struct {
		name, src, key string
	}{
		{"timer outputs and a by: trigger move back", loop("{ to: open, by: trigger }", withOutputs), timerKey},
		{"outputs on the trigger move", loop(
			"{ to: open, by: trigger, outputs: [ { set: { properties: { status: late } } } ] }", ""), timerKey},
		{"no outputs on the loop", loop("{ to: open, by: trigger }", ""), ""},
		{"a person closes the loop", loop("{ to: open, roles: [lead] }", withOutputs), ""},
		// TKT-FPXTQJ AC 7: a list entry a trigger reaches is a job edge.
		{"by: trigger move back in a second entry", loop("[ { to: done, roles: [lead] }, { to: open, by: trigger } ]",
			withOutputs), timerKey},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.src), testMeta(t), parsePolicy(t, botPolicy))
			if tc.key == "" {
				if err != nil {
					t.Fatalf("Parse: %v", err)
				}
				return
			}
			assertProblem(t, err, tc.key, "timers and trigger-fired transitions form a loop through open, stale")
		})
	}
}
