package realtimecfg

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// The key paths of the options in [optionsSrc].
const (
	presetKey = decideKey + "[0].button"
	allowKey  = "channel_types.plan-review.message_types.allow.decision_button"
	decideDB  = "channel_types.plan-review.message_types.decide.decision_button"
)

// optionsSrc is [buttonsSrc] with entries as the decide list, plus a log
// type allow without required properties that moves review to approved.
// decide is the value of decide's decision_button and allow that of
// allow's; each is left out when empty. A value may be a flow mapping or
// a block that starts on the next line.
func optionsSrc(entries, decide, allow string) string {
	src := buttonsSrc(entries)
	allowType := "      allow:\n        label: Allow\n        post: [member]\n        in: [review]\n"
	if allow != "" {
		allowType += "        decision_button: " + allow + "\n"
	}
	src = strings.Replace(src, "      decide:\n", allowType+"      decide:\n", 1)
	src = strings.Replace(src, "            decide:\n", "            allow: { to: approved }\n            decide:\n", 1)
	if decide != "" {
		src = strings.Replace(src, "        in: [review]\n    session:", "        in: [review]\n        decision_button: "+
			decide+"\n    session:", 1)
	}
	return src
}

// preset is a decide entry into drafting with a button whose extra keys
// are extra.
func preset(extra string) string {
	return entry("drafting", "button: { label: Go, props: { decision: revise }"+extra+" }")
}

// parseOptions loads src, fails the test on a problem, and returns the
// decide entries out of review and the allow and decide message types.
func parseOptions(t *testing.T, src string) (c *Compiled, decide []Transition, allow, decideMT *MessageType) {
	t.Helper()
	c = parseButtons(t, src)
	ct, _ := c.ChannelType("plan-review")
	return c, ct.Session().Branches("review", "decide"), ct.MessageTypes["allow"], ct.MessageTypes["decide"]
}

func presetOptions(t *testing.T, decide []Transition) ButtonOptions {
	t.Helper()
	b, ok := decide[0].Button()
	if !ok {
		t.Fatal("entry 0 has no button")
	}
	return b.Options
}

// TestButtonOptions_Load pins AC 1: both keys load on a preset button and
// on a message type, with null as absent.
func TestButtonOptions_Load(t *testing.T) {
	presetCases := []struct {
		name, extra string
		want        ButtonOptions
	}{
		{"true and primary", ", confirm: true, style: primary", ButtonOptions{Confirm: ConfirmAsk, Style: ButtonStylePrimary}},
		{"false", ", confirm: false", ButtonOptions{Confirm: ConfirmNever}},
		{"text", `, confirm: "This runs the tool on your machine."`,
			ButtonOptions{Confirm: ConfirmAsk, ConfirmText: "This runs the tool on your machine."}},
		{"quoted yes is text", `, confirm: "yes"`, ButtonOptions{Confirm: ConfirmAsk, ConfirmText: "yes"}},
		{"tagged str yes is text", ", confirm: !!str yes", ButtonOptions{Confirm: ConfirmAsk, ConfirmText: "yes"}},
		{"text folded", ", confirm: \"  Cafe\u0301\ufe0f \"", ButtonOptions{Confirm: ConfirmAsk, ConfirmText: "Caf\u00e9"}},
		{"null confirm", ", confirm: null", ButtonOptions{}},
		{"null style", ", style: null", ButtonOptions{}},
		{"absent keys", "", ButtonOptions{}},
	}
	for _, tc := range presetCases {
		t.Run("preset "+tc.name, func(t *testing.T) {
			c, decide, _, _ := parseOptions(t, optionsSrc(preset(tc.extra), "", ""))
			if got := presetOptions(t, decide); got != tc.want {
				t.Errorf("options = %+v; want %+v", got, tc.want)
			}
			if w := c.Warnings(); len(w) != 0 {
				t.Errorf("warnings = %v", w)
			}
		})
	}
	typeCases := []struct {
		name, value string
		want        *ButtonOptions
		warning     string
	}{
		{"both keys", "{ confirm: true, style: danger }", &ButtonOptions{Confirm: ConfirmAsk, Style: ButtonStyleDanger}, ""},
		{"block text keeps no newline", "\n          confirm: |\n            Runs the tool.\n",
			&ButtonOptions{Confirm: ConfirmAsk, ConfirmText: "Runs the tool."}, ""},
		{"null", "null", nil, ""},
		{"empty mapping", "{}", nil, "sets neither confirm nor style"},
		{"null keys", "{ confirm: null, style: null }", nil, "sets neither confirm nor style"},
	}
	for _, tc := range typeCases {
		t.Run("type "+tc.name, func(t *testing.T) {
			c, _, allow, _ := parseOptions(t, optionsSrc(entry("drafting", ""), "", tc.value))
			switch {
			case tc.want == nil && allow.DecisionButton != nil:
				t.Errorf("DecisionButton = %+v; want nil", *allow.DecisionButton)
			case tc.want != nil && (allow.DecisionButton == nil || *allow.DecisionButton != *tc.want):
				t.Errorf("DecisionButton = %+v; want %+v", allow.DecisionButton, *tc.want)
			}
			if tc.warning != "" && !hasWarning(c, allowKey, tc.warning) {
				t.Errorf("no warning %q at %s; got %v", tc.warning, allowKey, c.Warnings())
			}
			if tc.warning == "" && len(c.Warnings()) != 0 {
				t.Errorf("warnings = %v", c.Warnings())
			}
		})
	}
	t.Run("type without key", func(t *testing.T) {
		_, _, allow, decideMT := parseOptions(t, optionsSrc(entry("drafting", ""), "", ""))
		if allow.DecisionButton != nil || decideMT.DecisionButton != nil {
			t.Error("a type without decision_button has options")
		}
	})
	t.Run("clone", func(t *testing.T) {
		c, _, allow, _ := parseOptions(t, optionsSrc(entry("drafting", ""), "", "{ style: danger }"))
		allow.DecisionButton.Style = ButtonStylePrimary
		ct, _ := c.ChannelType("plan-review")
		if got := ct.MessageTypes["allow"].DecisionButton.Style; got != ButtonStyleDanger {
			t.Errorf("a copy shares DecisionButton: style = %q", got)
		}
	})
}

// TestButtonOptions_Inherit pins AC 2: a preset takes each option it
// leaves unset from its type's decision_button, and its own value wins.
func TestButtonOptions_Inherit(t *testing.T) {
	tests := []struct {
		name, extra, decide string
		want                ButtonOptions
	}{
		{"a preset inherits each unset type option", "", `{ confirm: "Sure?", style: danger }`,
			ButtonOptions{Confirm: ConfirmAsk, ConfirmText: "Sure?", Style: ButtonStyleDanger}},
		{"a preset's style default overrides the type's danger", ", style: default", "{ style: danger }",
			ButtonOptions{Style: ButtonStyleDefault}},
		{"a preset's confirm false overrides the type's text", ", confirm: false", `{ confirm: "Sure?" }`,
			ButtonOptions{Confirm: ConfirmNever}},
		{"a preset's text replaces the type's text", `, confirm: "Mine"`, `{ confirm: "Sure?" }`,
			ButtonOptions{Confirm: ConfirmAsk, ConfirmText: "Mine"}},
		{"a preset's confirm true does not take the type's text", ", confirm: true", `{ confirm: "Sure?" }`,
			ButtonOptions{Confirm: ConfirmAsk}},
		{"a preset keeps its style and inherits confirm", ", style: primary", "{ confirm: true, style: danger }",
			ButtonOptions{Confirm: ConfirmAsk, Style: ButtonStylePrimary}},
		{"a type without decision_button leaves presets as authored", ", style: primary", "",
			ButtonOptions{Style: ButtonStylePrimary}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, decide, _, _ := parseOptions(t, optionsSrc(preset(tc.extra), tc.decide, ""))
			if got := presetOptions(t, decide); got != tc.want {
				t.Errorf("options = %+v; want %+v", got, tc.want)
			}
		})
	}
	t.Run("a refused decision_button gives nothing to inherit", func(t *testing.T) {
		src := optionsSrc(preset(""), `{ confirm: "Sure?", style: Danger }`, "")
		c, problems := compileAnyway(t, src)
		if len(problems) == 0 {
			t.Fatal("the bad style loaded")
		}
		ct := c.channelTypes["plan-review"]
		if ct.MessageTypes["decide"].DecisionButton != nil {
			t.Error("a refused decision_button is set")
		}
		b, ok := ct.session.Branches("review", "decide")[0].Button()
		if !ok || b.Options != (ButtonOptions{}) {
			t.Errorf("preset = %+v %v; want a button without options", b.Options, ok)
		}
	})
}

// compileAnyway runs the validator over src as [Parse] does, but returns
// the compiled config with the problems instead of refusing it, so a test
// can look at what a refused part left behind.
func compileAnyway(t *testing.T, src string) (*Compiled, []Problem) {
	t.Helper()
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(src), &root); err != nil {
		t.Fatal(err)
	}
	var raw rawFile
	dec := yaml.NewDecoder(bytes.NewReader([]byte(src)))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil && !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
	v := &validator{meta: testMeta(t), policy: testPolicy(t), root: &root}
	return v.compile(&raw), v.problems
}

// TestButtonOptions_Problems pins AC 3: each refused value, with its key
// path, on a preset button and on a message type.
func TestButtonOptions_Problems(t *testing.T) {
	long := `"` + strings.Repeat("a", realtimelimits.MaxConfirmTextRunes+1) + `"`
	values := []struct {
		name, key, value, reason string
	}{
		{"unknown style", "style", "ghost", "must be default, primary or danger"},
		{"style wrong case", "style", "Danger", "must be default, primary or danger"},
		{"empty style", "style", `""`, "must be default, primary or danger"},
		{"confirm mapping", "confirm", "{ text: hi }", "must be true, false or the text to show"},
		{"confirm list", "confirm", "[hi]", "must be true, false or the text to show"},
		{"confirm number", "confirm", "3", "quote a number to show it as text"},
		{"confirm unquoted yes", "confirm", "yes", `"yes" is not a boolean`},
		{"confirm unquoted Off", "confirm", "Off", `"Off" is not a boolean`},
		{"confirm empty", "confirm", `""`, "write false to turn the question off, or the text to show"},
		{"confirm blank", "confirm", `"   "`, "write false to turn the question off, or the text to show"},
		{"confirm too long", "confirm", long, "must be at most 300 characters"},
		{"confirm line break", "confirm", `"one\ntwo"`, "must be one line"},
		{"confirm hazard", "confirm", "\"Go\u202e\"", "U+202E, which displays as something other than what it is"},
	}
	for _, tc := range values {
		t.Run("preset "+tc.name, func(t *testing.T) {
			_, err := Parse([]byte(optionsSrc(preset(", "+tc.key+": "+tc.value), "", "")), testMeta(t), testPolicy(t))
			assertProblem(t, err, presetKey+"."+tc.key, tc.reason)
		})
		t.Run("type "+tc.name, func(t *testing.T) {
			src := optionsSrc(entry("drafting", ""), "", "{ "+tc.key+": "+tc.value+" }")
			_, err := Parse([]byte(src), testMeta(t), testPolicy(t))
			assertProblem(t, err, allowKey+"."+tc.key, tc.reason)
		})
	}
	shapes := []struct {
		name, value, key, reason string
	}{
		{"block scalar yes", "\n          confirm: |\n            yes\n", allowKey + ".confirm", `"yes" is not a boolean`},
		{"label", "{ label: Go }", allowKey + ".label", "reads as the message type's label"},
		{"props", "{ props: { decision: approve } }", allowKey + ".props", "posts no preset"},
		{"scalar", "true", allowKey, "must be a mapping"},
	}
	for _, tc := range shapes {
		t.Run("type "+tc.name, func(t *testing.T) {
			_, err := Parse([]byte(optionsSrc(entry("drafting", ""), "", tc.value)), testMeta(t), testPolicy(t))
			assertProblem(t, err, tc.key, tc.reason)
		})
	}
	t.Run("a bad option refuses only its button", func(t *testing.T) {
		_, err := Parse([]byte(optionsSrc(preset(", style: ghost"), "", "")), testMeta(t), testPolicy(t))
		var cfgErr *Error
		if !errors.As(err, &cfgErr) || len(cfgErr.Problems) != 1 {
			t.Errorf("problems = %v; want only the style", err)
		}
	})
}

// TestTypeButton_Checks pins AC 4: decision_button is refused where it
// cannot apply and warns where the graph gives it no effect.
func TestTypeButton_Checks(t *testing.T) {
	t.Run("refused without a session", func(t *testing.T) {
		src := `version: 1
channel_types:
  chat:
    attach_to: [datalek]
    roles:
      member: { from: attached }
    message_types:
      text:
        properties: { body: { type: markdown, required: true } }
        post: [member]
        decision_button: { style: danger }
`
		_, err := Parse([]byte(src), testMeta(t), testPolicy(t))
		assertProblem(t, err, "channel_types.chat.message_types.text.decision_button",
			"only on a channel type with a session")
	})
	t.Run("refused on an ephemeral type", func(t *testing.T) {
		src := strings.Replace(optionsSrc(entry("drafting", ""), "", ""), "      decide:\n",
			"      signal:\n        persist: ephemeral\n        post: [member]\n"+
				"        decision_button: { style: danger }\n      decide:\n", 1)
		_, err := Parse([]byte(src), testMeta(t), testPolicy(t))
		assertProblem(t, err, "channel_types.plan-review.message_types.signal.decision_button",
			"only for log messages")
	})
	t.Run("warns when no person moves the session with the type", func(t *testing.T) {
		src := strings.Replace(optionsSrc(entry("drafting", ""), "", "{ style: danger }"),
			"allow: { to: approved }", "allow: { to: approved, by: trigger }", 1)
		c := parseButtons(t, src)
		if !hasWarning(c, allowKey, "no person moves a session with allow, so decision_button has no effect") {
			t.Errorf("warnings = %v", c.Warnings())
		}
	})
	t.Run("warns when confirm reaches no one-click button", func(t *testing.T) {
		c := parseButtons(t, optionsSrc(entry("drafting", ""), "{ confirm: true }", ""))
		const want = "the type's button opens the composer because decision is required, " +
			"and no preset button of decide uses it"
		if !hasWarning(c, decideDB+".confirm", want) {
			t.Errorf("warnings = %v", c.Warnings())
		}
	})
	t.Run("warns when every preset sets its own confirm", func(t *testing.T) {
		c := parseButtons(t, optionsSrc(preset(", confirm: false"), "{ confirm: true }", ""))
		if !hasWarning(c, decideDB+".confirm", "every entry of decide has a preset button that sets its own confirm") {
			t.Errorf("warnings = %v", c.Warnings())
		}
	})
	t.Run("warns when every preset sets its own style", func(t *testing.T) {
		c := parseButtons(t, optionsSrc(preset(", style: default"), "{ style: danger }", ""))
		if !hasWarning(c, decideDB+".style", "every entry of decide has a preset button that sets its own style") {
			t.Errorf("warnings = %v", c.Warnings())
		}
	})
	t.Run("warns only for the key every preset sets", func(t *testing.T) {
		c := parseButtons(t, optionsSrc(preset(", style: default"), "{ confirm: true, style: danger }", ""))
		w := c.Warnings()
		if len(w) != 1 || !hasWarning(c, decideDB+".style", "sets its own style") {
			t.Errorf("warnings = %v; want only the style", w)
		}
	})
	t.Run("type with an unbuttoned entry loads clean", func(t *testing.T) {
		c := parseButtons(t, optionsSrc(entry("drafting", ""), "", `{ confirm: "Runs the tool.", style: danger }`))
		if w := c.Warnings(); len(w) != 0 {
			t.Errorf("warnings = %v", w)
		}
	})
	t.Run("confirm with a required property and a preset button loads clean", func(t *testing.T) {
		c := parseButtons(t, optionsSrc(docsEntries, "{ confirm: true }", ""))
		if w := c.Warnings(); len(w) != 0 {
			t.Errorf("warnings = %v", w)
		}
	})
	t.Run("style with a required property loads clean", func(t *testing.T) {
		c := parseButtons(t, optionsSrc(entry("drafting", ""), "{ style: danger }", ""))
		if w := c.Warnings(); len(w) != 0 {
			t.Errorf("warnings = %v", w)
		}
	})
}
