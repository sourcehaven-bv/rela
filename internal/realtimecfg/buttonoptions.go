package realtimecfg

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
	"github.com/Sourcehaven-BV/rela/internal/texthazard"
	"github.com/Sourcehaven-BV/rela/internal/yamlscalar"
)

// ConfirmMode says whether a decision button asks before it posts.
type ConfirmMode int

const (
	// ConfirmDerived leaves the question to the web app's derived rule:
	// a one-click button asks when every target is final.
	ConfirmDerived ConfirmMode = iota
	// ConfirmAsk always asks before a one-click post.
	ConfirmAsk
	// ConfirmNever never asks.
	ConfirmNever
)

// ButtonStyle is how a decision button looks. The empty value leaves it
// to the web app's derived rule.
type ButtonStyle string

// The styles a decision button may take.
const (
	// ButtonStyleDefault is a plain button, also into a cancelled state.
	ButtonStyleDefault ButtonStyle = "default"
	// ButtonStylePrimary marks the decision the operator expects most.
	ButtonStylePrimary ButtonStyle = "primary"
	// ButtonStyleDanger draws the button in the danger tone.
	ButtonStyleDanger ButtonStyle = "danger"
)

// ButtonOptions are the authored `confirm:` and `style:` of a decision
// button: a preset button's `button:` or a type's `decision_button:`. They
// are presentation for the web app only. A post through MCP, the CLI or
// the REST API is never asked anything, and the server checks it as any
// other.
//
// Each option overrides only its own part of the derived rule. The zero
// value sets neither, so the web app draws the button as it would
// without options.
type ButtonOptions struct {
	// Confirm is whether the button asks before a one-click post.
	Confirm ConfirmMode
	// ConfirmText is the dialog text, folded as a label is; set only with
	// [ConfirmAsk]. Empty means the web app's translated text.
	ConfirmText string
	// Style is the authored style, or empty.
	Style ButtonStyle
}

// inherit returns o with each option o leaves unset taken from from. The
// confirm text goes with its mode, so a preset's `confirm: true` does not
// take the type's text.
func (o ButtonOptions) inherit(from ButtonOptions) ButtonOptions {
	if o.Confirm == ConfirmDerived {
		o.Confirm, o.ConfirmText = from.Confirm, from.ConfirmText
	}
	if o.Style == "" {
		o.Style = from.Style
	}
	return o
}

// optionsOf checks the `confirm:` and `style:` under key and returns them.
// ok is false when either has a problem; each problem names its own key.
func optionsOf(v *validator, key string, confirm rawConfirm, style *string) (ButtonOptions, bool) {
	var o ButtonOptions
	ok := true
	if confirm.set() {
		mode, text, problem := confirmOf(confirm)
		if problem != "" {
			v.add(key+".confirm", problem)
			ok = false
		}
		o.Confirm, o.ConfirmText = mode, text
	}
	if style != nil {
		switch s := ButtonStyle(*style); s {
		case ButtonStyleDefault, ButtonStylePrimary, ButtonStyleDanger:
			o.Style = s
		default:
			v.add(key+".style", fmt.Sprintf("must be default, primary or danger, got %q", *style))
			ok = false
		}
	}
	if !ok {
		return ButtonOptions{}, false
	}
	return o, true
}

// confirmOf reads a `confirm:` that is set: true asks, false never asks,
// and a text asks with that text. The key checker already refused a value
// that is not a single value. problem says why the value is refused.
func confirmOf(c rawConfirm) (mode ConfirmMode, text, problem string) {
	n := c.node
	switch n.Tag {
	case "!!bool":
		var b bool
		if err := n.Decode(&b); err != nil {
			return ConfirmDerived, "", "must be true, false or the text to show"
		}
		if b {
			return ConfirmAsk, "", ""
		}
		return ConfirmNever, "", ""
	case "!!int", "!!float":
		return ConfirmDerived, "", "must be true, false or the text to show; quote a number to show it as text"
	}
	if yamlscalar.PlainYAML11Bool(n) {
		return ConfirmDerived, "", fmt.Sprintf("%q is not a boolean; use true or false, or quote it to show it "+
			"as text", strings.TrimSpace(n.Value))
	}
	text = foldDisplay(n.Value)
	switch {
	case text == "":
		return ConfirmDerived, "", "must not be empty; write false to turn the question off, or the text to show"
	case strings.ContainsAny(text, "\n\r"):
		return ConfirmDerived, "", "must be one line"
	case utf8.RuneCountInString(text) > realtimelimits.MaxConfirmTextRunes:
		return ConfirmDerived, "", fmt.Sprintf("must be at most %d characters",
			realtimelimits.MaxConfirmTextRunes)
	}
	if r, found := texthazard.Find(text); found {
		return ConfirmDerived, "", fmt.Sprintf("holds %U, which displays as something other than what it is", r)
	}
	return ConfirmAsk, text, ""
}

// typeButtonChecker checks `decision_button:` on a validator's behalf.
// It keeps those checks off the validator, which is at its method budget.
type typeButtonChecker struct{ *validator }

// check compiles `decision_button:` on every message type of ct and
// merges it into the type's preset buttons. It runs after the session is
// compiled, because it reads the session's entries and buttons.
func (v typeButtonChecker) check(raw *rawChannelType, ct *ChannelType) {
	for _, mn := range sortedKeys(raw.MessageTypes) {
		rd := raw.MessageTypes[mn].DecisionButton
		mt := ct.MessageTypes[mn]
		if rd == nil || mt == nil {
			continue
		}
		key := ct.messageTypeKey(mn) + ".decision_button"
		switch {
		case raw.Session == nil:
			v.add(key, "only on a channel type with a session; a decision button moves a session")
			continue
		case mt.Persist == PersistEphemeral:
			v.add(key, "only for log messages; an ephemeral message never moves a session")
			continue
		case ct.session == nil:
			continue // the session has its own problems
		case !rd.Confirm.set() && rd.Style == nil:
			v.warn(key, "sets neither confirm nor style, so it has no effect")
			continue
		}
		opts, ok := optionsOf(v.validator, key, rd.Confirm, rd.Style)
		if !ok {
			continue
		}
		mt.DecisionButton = &opts
		v.effect(key, ct.session, mt, opts)
		for _, b := range presetButtons(ct.session, mn) {
			from := opts
			if b.Compose {
				// Send is the confirmation of a button that opens the
				// composer (TKT-TXZZZ2), so it takes only the style.
				from.Confirm, from.ConfirmText = ConfirmDerived, ""
			}
			b.Options = b.Options.inherit(from)
		}
	}
}

// effect warns when the options of mt's decision buttons cannot show: no
// person moves the session with mt, every entry has a preset button that
// sets the option itself, or a confirm reaches no one-click button. A
// compose button never takes the type's confirm. It runs before the
// merge, so a preset's Options are its own.
func (v typeButtonChecker) effect(key string, s *Session, mt *MessageType, opts ButtonOptions) {
	clickable, typeButton := false, false
	for _, st := range s.states {
		for _, t := range st.on[mt.Name] {
			if _, problem := personClasses(t, mt); problem == "" {
				clickable = true
				typeButton = typeButton || t.button == nil
			}
		}
	}
	if !clickable {
		v.warn(key, fmt.Sprintf("no person moves a session with %s, so decision_button has no effect", mt.Name))
		return
	}
	presets := presetButtons(s, mt.Name)
	if !typeButton {
		v.presetsOnly(key, mt.Name, opts, presets)
		return
	}
	if opts.Confirm == ConfirmDerived {
		return
	}
	required := ""
	for _, pn := range sortedKeys(mt.Properties) {
		if mt.Properties[pn].Required {
			required = pn
			break
		}
	}
	if required == "" {
		return
	}
	for _, b := range presets {
		if !b.Compose && b.Options.Confirm == ConfirmDerived {
			return
		}
	}
	v.warn(key+".confirm", fmt.Sprintf("confirm has no effect: the type's button opens the composer because "+
		"%s is required, and no preset button of %s uses it", required, mt.Name))
}

// presetsOnly warns for a type with no type button, where an option
// reaches a button only through a preset that leaves it unset.
func (v typeButtonChecker) presetsOnly(key, mn string, opts ButtonOptions, presets []*TransitionButton) {
	ownConfirm, ownStyle, compose := true, true, false
	for _, b := range presets {
		ownConfirm = ownConfirm && (b.Compose || b.Options.Confirm != ConfirmDerived)
		ownStyle = ownStyle && b.Options.Style != ""
		compose = compose || b.Compose
	}
	if opts.Confirm != ConfirmDerived && ownConfirm {
		what := "sets its own confirm"
		if compose {
			what = "sets its own confirm or opens the composer"
		}
		v.warn(key+".confirm", fmt.Sprintf("confirm has no effect: every entry of %s has a preset button "+
			"that %s", mn, what))
	}
	if opts.Style != "" && ownStyle {
		v.warn(key+".style", fmt.Sprintf("style has no effect: every entry of %s has a preset button "+
			"that sets its own style", mn))
	}
}

// presetButtons returns the compiled buttons of every `on.<mn>` entry of
// s. The pointers are the entries' own, so the load can write through
// them before the config is published.
func presetButtons(s *Session, mn string) []*TransitionButton {
	var out []*TransitionButton
	for _, st := range s.states {
		for _, t := range st.on[mn] {
			if t.button != nil {
				out = append(out, t.button)
			}
		}
	}
	return out
}
