package realtimecfg

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// pocTypes are the message types of the agent-runner session, in the order
// the fixture declares them.
var pocTypes = []string{
	"text", "go", "claim", "resume", "progress", "plan", "approve", "revise", "ask", "answer",
	"permission", "allow", "deny", "finished", "fail", "accept", "rework", "retry", "abandon",
}

// pocPosters are the roles that post each type.
var pocPosters = map[string]string{
	"text": "owner, runner", "go": "owner", "claim": "runner", "resume": "runner", "progress": "runner",
	"plan": "runner", "approve": "owner", "revise": "owner", "ask": "runner", "answer": "owner",
	"permission": "runner", "allow": "owner", "deny": "owner", "finished": "runner", "fail": "runner",
	"accept": "owner", "rework": "owner", "retry": "owner", "abandon": "owner",
}

const pocStates = `
        new:
          on: { go: { to: queued } }
        queued:
          on:
            claim:  { to: planning, roles: [runner] }
            resume: { to: working, roles: [runner] }
        planning:
          on:
            plan: { to: plan-review, roles: [runner] }
            ask:  { to: plan-asking, roles: [runner] }
            fail: { to: blocked, roles: [runner] }
        plan-asking:
          on: { answer: { to: planning, roles: [owner] } }
        plan-review:
          on:
            approve: { to: working, roles: [owner] }
            revise:  { to: planning, roles: [owner] }
        working:
          on:
            ask:        { to: asking, roles: [runner] }
            permission: { to: approving, roles: [runner] }
            finished:   { to: review, roles: [runner] }
            fail:       { to: blocked, roles: [runner] }
        asking:
          on: { answer: { to: working, roles: [owner] } }
        approving:
          on:
            allow: { to: working, roles: [owner] }
            deny:  { to: working, roles: [owner] }
        review:
          on:
            accept: { to: done, roles: [owner] }
            rework: { to: queued, roles: [owner] }
        blocked:
          on:
            retry:   { to: queued, roles: [owner] }
            abandon: { to: abandoned, roles: [owner] }
        done:      { final: true }
        abandoned: { final: true }`

// pocRealtime is the agent-runner session of the PoC on datalek, with
// in[type] as the `in:` value of each type that has one (written as is,
// so "~" and "" test the null forms) and session holding extra session
// keys.
func pocRealtime(in map[string]string, session string) string {
	var b strings.Builder
	b.WriteString(`version: 1
channel_types:
  run:
    attach_to: [datalek]
    roles:
      owner:  { from: entity.owner }
      runner: { permission: incident.decide }
    message_types:
`)
	for _, mt := range pocTypes {
		fmt.Fprintf(&b, "      %s:\n        properties: { body: { type: string } }\n        post: [%s]\n",
			mt, pocPosters[mt])
		if v, ok := in[mt]; ok {
			fmt.Fprintf(&b, "        in: %s\n", v)
		}
	}
	b.WriteString("    session:\n      initial: new\n" + session + "      states:" + pocStates + "\n")
	return b.String()
}

// pocIn is `in:` on every transition-keyed type of the PoC, each listing
// exactly the states whose on: keys it, and progress limited to planning
// and working. text has none.
var pocIn = map[string]string{
	"go": "[new]", "claim": "[queued]", "resume": "[queued]", "plan": "[planning]",
	"ask": "[planning, working]", "answer": "[plan-asking, asking]", "approve": "[plan-review]",
	"revise": "[plan-review]", "permission": "[working]", "finished": "[working]", "fail": "[planning, working]",
	"allow": "[approving]", "deny": "[approving]", "accept": "[review]", "rework": "[review]",
	"retry": "[blocked]", "abandon": "[blocked]", "progress": "[planning, working]",
}

// TestParse_MessageIn pins AC 1: `in:` compiles to MessageType.In in file
// order, PostableIn follows it, and a type without `in:` is postable in
// every state.
func TestParse_MessageIn(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]string
		want map[string][]string
	}{
		{"approve only", map[string]string{"approve": "[plan-review]"},
			map[string][]string{"approve": {"plan-review"}}},
		{"every transition type", pocIn, map[string][]string{
			"approve": {"plan-review"}, "ask": {"planning", "working"}, "progress": {"planning", "working"},
			"answer": {"plan-asking", "asking"},
		}},
		{"file order kept", map[string]string{"progress": "[working, planning]"},
			map[string][]string{"progress": {"working", "planning"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := Parse([]byte(pocRealtime(tc.in, "")), testMeta(t), testPolicy(t))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			ct, _ := c.ChannelType("run")
			for mt, want := range tc.want {
				if got := ct.MessageTypes[mt].In; !slices.Equal(got, want) {
					t.Errorf("%s.In = %v; want %v", mt, got, want)
				}
			}
			text := ct.MessageTypes["text"]
			if text.In != nil {
				t.Errorf("text.In = %v; want nil", text.In)
			}
			for _, st := range ct.Session().States() {
				if !text.PostableIn(st.Name) {
					t.Errorf("text is not postable in %s", st.Name)
				}
			}
			approve := ct.MessageTypes["approve"]
			if _, limited := tc.in["approve"]; limited && (!approve.PostableIn("plan-review") ||
				approve.PostableIn("working")) {

				t.Error("PostableIn does not follow in:")
			}
		})
	}
}

// TestParse_MessageInClone pins that a copy of the channel type does not
// share its in: list.
func TestParse_MessageInClone(t *testing.T) {
	c, err := Parse([]byte(pocRealtime(map[string]string{"approve": "[plan-review]"}, "")), testMeta(t),
		testPolicy(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	a, _ := c.ChannelType("run")
	a.MessageTypes["approve"].In[0] = "changed"
	b, _ := c.ChannelType("run")
	if got := b.MessageTypes["approve"].In; !slices.Equal(got, []string{"plan-review"}) {
		t.Errorf("a copy shares in: %v", got)
	}
}

// TestParse_MessageInInvalid pins AC 2: each invalid `in:` refuses the
// file with a problem at its key.
func TestParse_MessageInInvalid(t *testing.T) {
	const mk = "channel_types.run.message_types"
	const sk = "channel_types.run.session.states"
	cases := []struct {
		name, src, key, reason string
	}{
		{"null", pocRealtime(map[string]string{"approve": "~"}, ""), mk + ".approve.in",
			"name the states, or drop in to allow every state"},
		{"no value", pocRealtime(map[string]string{"approve": ""}, ""), mk + ".approve.in",
			"name the states, or drop in to allow every state"},
		{"empty list", pocRealtime(map[string]string{"progress": "[]"}, ""), mk + ".progress.in",
			"in names no state, so the type could never be posted"},
		{"scalar", pocRealtime(map[string]string{"progress": "planning"}, ""), mk + ".progress.in",
			"must be a list"},
		{"unknown state", pocRealtime(map[string]string{"progress": "[nope]"}, ""), mk + ".progress.in[0]",
			`unknown state "nope"; declare it under states`},
		{"listed twice", pocRealtime(map[string]string{"progress": "[working, working]"}, ""),
			mk + ".progress.in[1]", "state working is listed twice"},
		{"transition outside in", pocRealtime(map[string]string{"ask": "[working]"}, ""),
			sk + ".planning.on.ask", "ask may be posted only in working, so this transition could never fire; " +
				"add planning to message_types.ask.in"},
		{"only final states with on_final close", pocRealtime(map[string]string{"text": "[done, abandoned]"},
			"      on_final: close\n"), mk + ".text.in",
			"the channel closes when the session enters these states, so the type would be offered at most " +
				"until the close runs"},
		{"states key", strings.Replace(pocRealtime(nil, ""), "        post: [owner, runner]\n",
			"        post: [owner, runner]\n        states: [working]\n", 1), mk + ".text.states",
			"use in: the session states in which the type may be posted"},
		{"from key", strings.Replace(pocRealtime(nil, ""), "        post: [owner, runner]\n",
			"        post: [owner, runner]\n        from: [working]\n", 1), mk + ".text.from",
			"use in: the session states in which the type may be posted"},
		{"no session", `version: 1
channel_types:
  chat:
    attach_to: [datalek]
    roles:
      member: { from: attached }
    message_types:
      text: { properties: { body: { type: string } }, post: [member], in: [open] }
`, "channel_types.chat.message_types.text.in", "in names session states; this channel type has no session"},
		{"no session, null", `version: 1
channel_types:
  chat:
    attach_to: [datalek]
    roles:
      member: { from: attached }
    message_types:
      text: { properties: { body: { type: string } }, post: [member], in: ~ }
`, "channel_types.chat.message_types.text.in", "in names session states; this channel type has no session"},
		{"reminder outside in", reminderBase(reminderStates, `
        r: { in: [open, mid], every: 1h, post: { type: nudge, props: { body: hi } } }`),
			"channel_types.br.session.reminders.r.in[1]", "the reminder posts nudge, which may be posted only in open"},
	}
	// The reminder case needs in: on nudge.
	cases[len(cases)-1].src = strings.Replace(cases[len(cases)-1].src,
		"      nudge: { properties: { body: { type: string, required: true } }, post: [member] }",
		"      nudge: { properties: { body: { type: string, required: true } }, post: [member], in: [open] }", 1)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.src), testMeta(t), testPolicy(t))
			assertProblem(t, err, tc.key, tc.reason)
		})
	}
}

// TestParse_MessageInAllowed pins the shapes AC 2 does not refuse: a type
// listed in a state where it moves nothing, a final state next to another,
// a final state without on_final: close, and a reminder whose states lie
// inside its type's in:.
func TestParse_MessageInAllowed(t *testing.T) {
	cases := []struct {
		name, src string
	}{
		{"plain message where it moves nothing", pocRealtime(map[string]string{"approve": "[plan-review, working]"}, "")},
		{"final and open state with close", pocRealtime(map[string]string{"text": "[review, done]"},
			"      on_final: close\n")},
		{"only final without close", pocRealtime(map[string]string{"text": "[done]"}, "")},
		{"reminder inside in", strings.Replace(reminderBase(reminderStates, `
        r: { in: [open], every: 1h, post: { type: nudge, props: { body: hi } } }`),
			"      nudge: { properties: { body: { type: string, required: true } }, post: [member] }",
			"      nudge: { properties: { body: { type: string, required: true } }, post: [member], in: [open, mid] }",
			1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse([]byte(tc.src), testMeta(t), testPolicy(t)); err != nil {
				t.Fatalf("Parse: %v", err)
			}
		})
	}
}

// TestParse_MessageInBrokenType pins that a message type with a problem of
// its own still has its in: checked: both problems are reported, each at
// its own key.
func TestParse_MessageInBrokenType(t *testing.T) {
	src := strings.Replace(pocRealtime(map[string]string{"progress": "[planning, nope]"}, ""),
		"      progress:\n        properties: { body: { type: string } }\n        post: [runner]\n",
		"      progress:\n        properties: { body: { type: string } }\n        post: [nobody]\n", 1)
	_, err := Parse([]byte(src), testMeta(t), testPolicy(t))
	if err == nil {
		t.Fatal("Parse accepted an unknown post role")
	}
	assertProblem(t, err, "channel_types.run.message_types.progress.post[0]", `unknown role "nobody"`)
	assertProblem(t, err, "channel_types.run.message_types.progress.in[1]", unknownState("nope"))
}

// TestMessageInCheck_SkipsMissingType pins the guard in
// [messageInCheck.withSession]: a type in the raw file that the compiled
// channel type lacks is skipped, with no problem and no panic. Without the
// guard the check would set in: on a nil type.
func TestMessageInCheck_SkipsMissingType(t *testing.T) {
	c, err := Parse([]byte(pocRealtime(nil, "")), testMeta(t), testPolicy(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	run, _ := c.ChannelType("run")
	var root yaml.Node
	if err := yaml.Unmarshal([]byte("channel_types: {run: {message_types: {ghost: {in: [planning]}}}}"), &root); err != nil {
		t.Fatal(err)
	}
	v := &validator{root: &root}
	ct := &ChannelType{Name: "run", MessageTypes: map[string]*MessageType{}}
	raw := &rawChannelType{MessageTypes: map[string]rawMessageType{"ghost": {In: []string{"planning"}}}}
	messageInCheck{v: v, raw: raw, ct: ct}.withSession(run.Session(), nil, "")
	if len(v.problems) != 0 || len(ct.MessageTypes) != 0 {
		t.Errorf("problems %v, types %v; want none", v.problems, ct.MessageTypes)
	}
}

// TestDocsExample_MessageIn pins AC 10: the `in:` example of the realtime
// guide loads, and its types carry the states it shows.
func TestDocsExample_MessageIn(t *testing.T) {
	data, err := os.ReadFile(realtimeGuide)
	if err != nil {
		t.Fatal(err)
	}
	const lead = "\n**Where a message type may be posted.**"
	at := bytes.Index(data, []byte(lead))
	if at < 0 {
		t.Fatalf("paragraph %q not found in the guide", lead)
	}
	example := "version: 1\n" + yamlBlockIn(t, string(data)[at:], lead, "channel_types:")
	c, err := Parse([]byte(example), testMeta(t), testPolicy(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ct, ok := c.ChannelType("plan")
	if !ok {
		t.Fatal("the example has no plan channel type")
	}
	for mt, want := range map[string][]string{"approve": {"review"}, "revise": {"review"}, "plan": {"drafting"}} {
		if got := ct.MessageTypes[mt].In; !slices.Equal(got, want) {
			t.Errorf("%s.In = %v; want %v", mt, got, want)
		}
	}
	if ct.MessageTypes["text"].In != nil {
		t.Error("text has in:; the example keeps it free")
	}
}
