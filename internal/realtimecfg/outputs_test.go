package realtimecfg

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// TestParse_SessionOutputs pins AC 1: the outputs of the golden file's
// breach-response session compile in file order, with their kind, type,
// face, relate and property templates, and on_final is close.
func TestParse_SessionOutputs(t *testing.T) {
	c, err := Parse([]byte(golden(t)), testMeta(t), testPolicy(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ct, _ := c.ChannelType("breach-response")
	s := ct.Session()
	if s.OnFinal() != OnFinalClose {
		t.Errorf("OnFinal = %q", s.OnFinal())
	}
	tr, _ := s.Transition("detected", "contained")
	outs := tr.Outputs()
	if len(outs) != 1 {
		t.Fatalf("detected.contained outputs = %d; want 1", len(outs))
	}
	o := outs[0]
	if o.Kind != OutputCreate || o.Type != "timeline-entry" || o.Face != "" || o.Relate != "about" {
		t.Errorf("create = %+v", o)
	}
	if got := o.PropertyNames(); !slices.Equal(got, []string{"at", "title"}) {
		t.Errorf("create properties = %v", got)
	}
	at := time.Date(2026, 10, 26, 9, 30, 0, 0, time.FixedZone("CET", 3600))
	vars := OutputVars{Message: map[string]any{"note": "door locked"}, Author: "u-ann", At: at}
	if v, _ := o.Properties()["title"].Eval(vars); v != "Contained by u-ann: door locked" {
		t.Errorf("title = %v", v)
	}
	if v, _ := o.Properties()["at"].Eval(vars); v != "2026-10-26T08:30:00Z" {
		t.Errorf("at = %v", v)
	}

	set, _ := s.Transition("assessed", "notified")
	outs = set.Outputs()
	if len(outs) != 1 || outs[0].Kind != OutputSet || outs[0].Type != "" || outs[0].Face != "" {
		t.Fatalf("assessed.notified outputs = %+v", outs)
	}
	props := outs[0].Properties()
	if !props["status"].Literal() || props["reference"].Literal() {
		t.Error("status should be a literal and reference a reference")
	}
	if v, ok := props["reference"].Eval(OutputVars{Message: map[string]any{"reference": "AP-1"}}); !ok || v != "AP-1" {
		t.Errorf("reference = %v, %v", v, ok)
	}
	if none, _ := s.Transition("detected", "dismiss"); none.Outputs() != nil {
		t.Errorf("dismiss has outputs: %+v", none.Outputs())
	}

	// A family-anchored type over a faced and an unfaced type loads when
	// set names a face the faced type declares.
	const mixed = `version: 1
channel_types:
  mix:
    attach_to: [datalek, page]
    roles: { member: { from: attached } }
    message_types:
      text: { properties: { body: { type: string } }, post: [member] }
    session:
      initial: open
      states:
        open:
          on:
            text: { to: done, roles: [member], outputs: [ { set: { face: draft, properties: { title: "{{message.body}}" } } } ] }
        done: { final: true }
`
	c, err = Parse([]byte(mixed), testMeta(t), nil)
	if err != nil {
		t.Fatalf("mixed attach_to: %v", err)
	}
	ct, _ = c.ChannelType("mix")
	if tr, _ := ct.Session().Transition("open", "text"); tr.Outputs()[0].Face != "draft" {
		t.Errorf("mixed set face = %q", tr.Outputs()[0].Face)
	}
}

// outputsBase is the realtime.yaml TestParse_SessionOutputsInvalid edits.
const outputsBase = `version: 1
channel_types:
  br:
    attach_to: [datalek]
    roles:
      lead:   { from: entity.owner }
      member: { from: attached }
    message_types:
      text:      { properties: { body: { type: string } }, post: [member] }
      contained: { properties: { risk: { type: risk-level }, note: { type: markdown }, n: { type: integer } }, post: [lead, member] }
    session:
      initial: open
      on_final: close
      states:
        open:
          on:
            contained:
              to: done
              roles: [lead]
              outputs:
                - set: { properties: { status: done } }
                - create: { type: timeline-entry, properties: { title: "x {{message.note}}" }, relate: { type: about } }
        done: { final: true }
`

// TestParse_SessionOutputsInvalid pins AC 2 and 3: each invalid output
// reports its key path and reason.
func TestParse_SessionOutputsInvalid(t *testing.T) {
	const k = "channel_types.br.session"
	const o = k + ".states.open.on.contained.outputs"
	const set = "- set: { properties: { status: done } }"
	const create = `- create: { type: timeline-entry, properties: { title: "x {{message.note}}" }, relate: { type: about } }`
	setTo := func(props string) []replace { return []replace{{set, "- set: { properties: { " + props + " } }"}} }
	createAs := func(body string) []replace { return []replace{{create, "- create: { " + body + " }"}} }
	more := "\n                " + create
	cases := []struct {
		name   string
		edits  []replace
		key    string
		reason string
	}{
		{"too many outputs", []replace{{create, create + more + more + more}}, o, "at most 4 outputs per transition"},
		{"two sets", []replace{{create, create + "\n                " + set}}, o + "[2]", "at most one set per transition"},
		{"set and create", []replace{{create, create + "\n                - { set: { properties: { status: x } }, " +
			"create: { type: timeline-entry, properties: { title: x } } }"}}, o + "[2]", "either set or create, not both"},
		{"neither", []replace{{create, create + "\n                - {}"}}, o + "[2]", "an output names set or create"},
		{"empty properties", []replace{{set, "- set: { properties: {} }"}}, o + "[0].set.properties",
			"name at least one property"},
		{"undeclared property", setTo("nope: x"), o + "[0].set.properties.nope", `datalek has no property "nope"`},
		{"computed property", createAs("type: memo, face: draft, properties: { title: x, double: 2 }"),
			o + "[1].create.properties.double", "memo.double is computed"},
		{"file property", setTo("evidence: x"), o + "[0].set.properties.evidence", "is a file property"},
		{"external_ref property", setTo("ticket: x"), o + "[0].set.properties.ticket", "is an external_ref property"},
		{"literal of the wrong type", setTo("status: 3"), o + "[0].set.properties.status", "cannot fill datalek.status"},
		{"enum literal not in the enum", setTo("severity: extreme"), o + "[0].set.properties.severity",
			"cannot fill datalek.severity"},
		{"unknown reference", setTo(`status: "{{foo}}"`), o + "[0].set.properties.status", "unknown reference foo"},
		{"entity reference", setTo(`status: "{{entity.title}}"`), o + "[0].set.properties.status",
			"outputs cannot read the entity"},
		{"undeclared message property", setTo(`status: "{{message.x}}"`), o + "[0].set.properties.status",
			`message type contained has no property "x"`},
		{"whole reference cannot fill", setTo(`count: "{{message.risk}}"`), o + "[0].set.properties.count",
			"message property risk (enum) cannot fill datalek.count"},
		{"mixed template in a non-string property", setTo(`count: "n {{message.note}}"`),
			o + "[0].set.properties.count", "text around a reference fills only a string property"},
		{"mixed template with a non-text reference", setTo(`status: "n {{message.n}}"`),
			o + "[0].set.properties.status", "message property n (integer) is not text"},
		{"unterminated reference", setTo(`status: "{{message.note"`), o + "[0].set.properties.status",
			"unterminated {{"},
		{"template too long", setTo("status: " + strings.Repeat("x", 1100)), o + "[0].set.properties.status",
			"a template is at most 1024 bytes"},
		{"create type unknown", createAs("type: nope, properties: { title: x }"), o + "[1].create.type",
			`unknown entity type "nope"`},
		{"create type alias", createAs("type: breach, properties: { title: x }"), o + "[1].create.type",
			`"breach" is an alias of "datalek"`},
		{"create manual id", createAs("type: handbook, properties: { title: x }"), o + "[1].create.type",
			"id_type: manual"},
		{"create missing required", createAs("type: memo, face: draft, properties: { level: 1 }"),
			o + "[1].create.properties", `memo requires property "title"`},
		{"create required from optional", createAs(`type: memo, face: draft, properties: { title: "{{message.note}}" }`),
			o + "[1].create.properties.title", "message property note is optional"},
		{"create without face", createAs("type: memo, properties: { title: x }"), o + "[1].create.face",
			"memo declares faces; name the face to create"},
		{"create relate cannot link", createAs("type: decision, properties: { title: x }, relate: { type: about }"),
			o + "[1].create.relate.type", "cannot link decision to datalek"},
		{"set face on a face-anchored type", []replace{
			{"    attach_to: [datalek]\n", "    attach_to: [page]\n    anchor: face\n"},
			{set, "- set: { face: draft, properties: { title: x } }"},
		}, o + "[0].set.face", "the channel is anchored to one face; set writes that face"},
		{"set without face on a faced family", []replace{
			{"    attach_to: [datalek]\n", "    attach_to: [page]\n"},
			{set, "- set: { properties: { title: x } }"},
		}, o + "[0].set.face", "page declares faces and the channel is anchored to the whole entity"},
		{"set with an unknown face", []replace{
			{"    attach_to: [datalek]\n", "    attach_to: [page]\n"},
			{set, "- set: { face: nope, properties: { title: x } }"},
		}, o + "[0].set.face", `page has no face "nope"`},
		{"set property of two types", []replace{
			{"    attach_to: [datalek]\n", "    attach_to: [datalek, incident]\n"},
			{set, `- set: { properties: { due: "{{today}}" } }`},
		}, o + "[0].set.properties.due", "datalek.due is date and incident.due is datetime"},
		{"set face on an unfaced type", []replace{{set, "- set: { face: draft, properties: { status: x } }"}},
			o + "[0].set.face", "datalek declares no faces"},
		{"on_final other than close", []replace{{"on_final: close", "on_final: archive"}}, k + ".on_final",
			"on_final may only be close"},
		{"action named outputs", []replace{{"text:      { properties: { body: { type: string } }, post: [member] }",
			"text:      { properties: { body: { type: string } }, post: [member], actions: { outputs: { roles: [member] } } }"}},
			"channel_types.br.message_types.text.actions.outputs", "reserved for the session's output marks"},
		{"by trigger", []replace{{"              roles: [lead]\n", "              by: trigger\n"}}, o,
			"outputs on a by: trigger transition need a run_as identity, which arrives in a later release"},
		{"trigger reachable", []replace{
			{"              roles: [lead]\n", ""},
			{"        done: { final: true }\n", "        done: { final: true }\ntriggers:\n  t1:\n    entity_type: datalek\n" +
				"    on: enter\n    when: \"entity.status == 'x'\"\n    channel: br\n    post: { type: contained }\n"},
		}, o, "trigger t1 posts contained into this channel type"},
		{"timer outputs", []replace{{"        done: { final: true }\n",
			"          after:\n            late: { period: 1h, to: done, outputs: [] }\n        done: { final: true }\n"}},
			k + ".states.open.after.late.outputs", "outputs on a timer need a run_as identity"},
		{"session run_as", []replace{{"      on_final: close\n", "      on_final: close\n      run_as: system:bot\n"}},
			k + ".run_as", "session.run_as arrives in a later release"},
	}
	meta, policy := testMeta(t), testPolicy(t)
	if _, err := Parse([]byte(outputsBase), meta, policy); err != nil {
		t.Fatalf("the base file does not load: %v", err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := outputsBase
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
