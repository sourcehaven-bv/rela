package realtimecfg

import (
	"strings"
	"testing"
)

// choicesBase is the PoC shape: the runner asks with options, the owner
// answers, and ask moves the session to asking, where answer is postable.
const choicesBase = `version: 1
channel_types:
  run:
    attach_to: [datalek]
    roles:
      owner:  { from: entity.owner }
      runner: { permission: incident.decide }
    message_types:
      text:
        properties: { body: { type: string } }
        post: [owner, runner]
      ask:
        properties:
          question: { type: markdown, required: true }
          options:  { type: string, list: true, max: 80, max_items: 4 }
        post: [runner]
        choices: { from: options, answer: answer, into: body, other: true }
      answer:
        properties: { body: { type: string, required: true } }
        post: [owner]
        in: [asking]
      go:
        properties: { body: { type: string } }
        post: [owner]
      signal:
        persist: ephemeral
        properties: { body: { type: string } }
        post: [owner]
    session:
      initial: working
      states:
        working:
          on:
            ask: { to: asking, roles: [runner] }
            go:  { to: review }
        asking:
          on: { answer: { to: working, roles: [owner] } }
        review:
          on: { go: { to: done } }
        done: { final: true }
`

const choicesKey = "channel_types.run.message_types.ask.choices"

// choicesSrc applies each old/new pair to choicesBase once, and fails the
// test when one does not apply.
func choicesSrc(t *testing.T, edits ...replace) string {
	t.Helper()
	src := choicesBase
	for _, e := range edits {
		if !strings.Contains(src, e.old) {
			t.Fatalf("edit %q does not apply", e.old)
		}
		src = strings.Replace(src, e.old, e.new, 1)
	}
	return src
}

// TestChoices_Load pins AC 4: the PoC shape loads, and MessageType.Choices
// holds the four values. A type without choices has none.
func TestChoices_Load(t *testing.T) {
	c, err := Parse([]byte(choicesBase), testMeta(t), testPolicy(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if w := c.Warnings(); len(w) != 0 {
		t.Errorf("warnings = %v; want none for the PoC shape", w)
	}
	ct, _ := c.ChannelType("run")
	got := ct.MessageTypes["ask"].Choices
	want := Choices{From: "options", Answer: "answer", Into: "body", Other: true}
	if got == nil || *got != want {
		t.Fatalf("ask.Choices = %+v; want %+v", got, want)
	}
	if ct.MessageTypes["answer"].Choices != nil {
		t.Error("answer has choices; it declares none")
	}
	// A copy does not share the choices.
	got.Other = false
	again, _ := c.ChannelType("run")
	if !again.MessageTypes["ask"].Choices.Other {
		t.Error("a copy shares its Choices with the compiled config")
	}
}

// TestChoices_Problems pins AC 4: each refusal has its key and reason.
func TestChoices_Problems(t *testing.T) {
	cases := []struct {
		name   string
		edits  []replace
		key    string
		reason string
	}{
		{"from names no property", []replace{{"from: options", "from: nope"}},
			choicesKey + ".from", `message type ask has no property "nope"`},
		{"from is not a list", []replace{{"from: options", "from: question"}},
			choicesKey + ".from", "property question is a markdown property; the options are a list"},
		{"from is empty", []replace{{"from: options, ", ""}},
			choicesKey + ".from", "name the list property that holds the options"},
		{"answer names no type", []replace{{"answer: answer", "answer: reply"}},
			choicesKey + ".answer", `channel type run has no message type "reply"`},
		{"answer is the question", []replace{{"answer: answer", "answer: ask"}},
			choicesKey + ".answer", "a question cannot answer itself"},
		{"answer is ephemeral", []replace{{"answer: answer", "answer: signal"}},
			choicesKey + ".answer", "message type signal is ephemeral"},
		{"answer is empty", []replace{{"answer: answer, ", ""}},
			choicesKey + ".answer", "name the message type a click posts"},
		{"choices on an ephemeral type", []replace{{
			"        persist: ephemeral\n        properties: { body: { type: string } }\n",
			"        persist: ephemeral\n        properties: { body: { type: string } }\n" +
				"        choices: { from: body, answer: answer, into: body }\n"}},
			"channel_types.run.message_types.signal.choices", "an ephemeral message is never stored"},
		{"into names no property", []replace{{"into: body", "into: text"}},
			choicesKey + ".into", `message type answer has no property "text"`},
		{"into is empty", []replace{{"into: body, ", ""}},
			choicesKey + ".into", "name the property of the answer type that receives the option"},
		{"into is markdown", []replace{{"properties: { body: { type: string, required: true } }",
			"properties: { body: { type: markdown, required: true } }"}},
			choicesKey + ".into", "property body is a markdown property; an option goes into a string property"},
		{"into is an integer", []replace{{"properties: { body: { type: string, required: true } }",
			"properties: { body: { type: integer, required: true } }"}},
			choicesKey + ".into", "property body is a integer property"},
		{"into is a list", []replace{{"properties: { body: { type: string, required: true } }",
			"properties: { body: { type: string, list: true, required: true } }"}},
			choicesKey + ".into", "property body is a list of string property"},
		{"into is too short", []replace{{"properties: { body: { type: string, required: true } }",
			"properties: { body: { type: string, max: 40, required: true } }"}},
			choicesKey + ".into", "an option of up to 80 characters does not fit into body, which holds at most 40"},
		{"answer requires another property", []replace{{"properties: { body: { type: string, required: true } }",
			"properties: { body: { type: string }, why: { type: string, required: true } }"}},
			choicesKey + ".answer", `message type answer requires property "why"; a click fills only "body"`},
		{"unknown key", []replace{{"other: true }", "other: true, label: Pick }"}},
			choicesKey + ".label", "unknown key"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(choicesSrc(t, tc.edits...)), testMeta(t), testPolicy(t))
			assertProblem(t, err, tc.key, tc.reason)
		})
	}
}

// TestChoices_ButtonWarning pins AC 4: a list too long, or with items too
// long, for buttons loads with a warning at .from.
func TestChoices_ButtonWarning(t *testing.T) {
	cases := []struct {
		name, options string
		warn          bool
	}{
		{"short list", "max: 80, max_items: 4", false},
		{"ten items of 100", "max: 100, max_items: 10", false},
		{"defaults", "max: 80", true},
		{"eleven items", "max: 80, max_items: 11", true},
		{"long items", "max: 101, max_items: 4", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := choicesSrc(t, replace{"max: 80, max_items: 4", tc.options},
				replace{"type: string, required: true }", "type: string, max: 500, required: true }"})
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
			if len(w) != 1 || w[0].Key != choicesKey+".from" || !strings.Contains(w[0].Reason,
				"too many or too long for buttons") {

				t.Errorf("warnings = %v; want one at %s.from", w, choicesKey)
			}
		})
	}
}

// TestChoices_SessionWarning pins AC 5: in a session channel the load
// warns when the question type never leads to a state where the answer
// type is postable, and not for the PoC.
func TestChoices_SessionWarning(t *testing.T) {
	cases := []struct {
		name  string
		edits []replace
		warn  bool
	}{
		{"PoC", nil, false},
		{"answer only in review", []replace{
			{"        in: [asking]\n", "        in: [review]\n"},
			{"          on: { answer: { to: working, roles: [owner] } }",
				"          on: { text: { to: working } }"},
		}, true},
		{"ask has no transition", []replace{
			{"            ask: { to: asking, roles: [runner] }\n", ""},
		}, true},
		{"ask leads only to working", []replace{
			{"            ask: { to: asking, roles: [runner] }\n", ""},
			{"          on: { go: { to: done } }", "          on: { go: { to: done }, ask: { to: working } }"},
			{"        post: [runner]\n", "        post: [runner]\n        in: [review]\n"},
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := Parse([]byte(choicesSrc(t, tc.edits...)), testMeta(t), testPolicy(t))
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
			if len(w) != 1 || w[0].Key != choicesKey+".answer" || !strings.Contains(w[0].Reason,
				"the buttons never show") {

				t.Errorf("warnings = %v; want one at %s.answer", w, choicesKey)
			}
		})
	}
}

// TestChoicesReachable pins the over-approximation: a guarded entry counts.
func TestChoicesReachable(t *testing.T) {
	src := choicesSrc(t, replace{"ask: { to: asking, roles: [runner] }",
		`ask: [ { to: review, when: "message.question == 'x'" }, { to: asking } ]`})
	c, err := Parse([]byte(src), testMeta(t), testPolicy(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ct, _ := c.ChannelType("run")
	if !choicesReachable(ct.Session(), ct.MessageTypes["ask"], ct.MessageTypes["answer"]) {
		t.Error("choicesReachable = false; the second entry leads to asking")
	}
	// go has no in:, so every state admits it.
	if !choicesReachable(ct.Session(), ct.MessageTypes["ask"], ct.MessageTypes["go"]) {
		t.Error("choicesReachable = false for a type postable everywhere")
	}
}
