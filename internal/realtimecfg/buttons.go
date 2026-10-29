package realtimecfg

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
	"github.com/Sourcehaven-BV/rela/internal/texthazard"
)

// TransitionButton is the `button:` of one `on.<type>` entry: a one-click
// button that posts the entry's message type with the preset props. A
// click is a normal post; the server checks it as any other.
//
// The load checks that the preset passes the prop rules of a post and the
// type's `when:`, that the entry's own guard holds for it, and which
// earlier entries of the list would take the click first. Those entries
// are recorded, so [SessionState.Offers] hides the button from every
// caller they admit.
type TransitionButton struct {
	// Label is the button's text, trimmed, without emoji style selectors
	// and in NFC.
	Label string
	// props is the preset in stored form.
	props map[string]any
	// shadowing are the earlier entries that take the preset first for a
	// caller they admit: an entry without `when:`, or one whose guard is
	// decidable at load and holds for the preset.
	shadowing []int
	// hidden are the shadowing entries and the earlier entries whose guard
	// the load cannot decide, sorted. A caller one of them admits does not
	// see the button, so a click's outcome is always known.
	hidden []int
}

// Props returns the preset in stored form: a string, an int64, a bool, or
// a []any of strings for a list. The map and its lists are copies.
func (b TransitionButton) Props() map[string]any {
	out := make(map[string]any, len(b.props))
	for k, v := range b.props {
		if list, ok := v.([]any); ok {
			v = slices.Clone(list)
		}
		out[k] = v
	}
	return out
}

// hiddenBy returns the indices of the earlier entries that hide the
// button from a caller they admit. The slice is not a copy.
func (b TransitionButton) hiddenBy() []int { return b.hidden }

// buttonChecker checks `button:` on a validator's behalf. It keeps those
// checks off the validator, which is at its method budget.
type buttonChecker struct{ *validator }

// buttonScope is what the check of one entry's button reads besides the
// entry: its message type, its session and the state that owns it.
type buttonScope struct {
	mt     *MessageType
	s      *Session
	state  string
	limits Limits
}

// compile checks the button of entry j of list, whose raw button is raw,
// and returns it, or nil when it has a problem. Entries before j have
// their index set.
func (v buttonChecker) compile(raw *rawButton, list []Transition, j int, sc buttonScope) *TransitionButton {
	t := list[j]
	key := t.key + ".button"
	ok := true
	// The label is folded as a list item is, so two labels that look the
	// same compare equal and the length counts the served form.
	label := foldDisplay(raw.Label)
	switch {
	case label == "":
		v.add(key+".label", "name the text of the button")
		ok = false
	case utf8.RuneCountInString(label) > buttonRunes:
		v.add(key+".label", fmt.Sprintf("must be at most %d characters", buttonRunes))
		ok = false
	default:
		if r, found := texthazard.Find(label); found {
			v.add(key+".label", fmt.Sprintf("holds %U, which displays as something other than what it is", r))
			ok = false
		}
	}
	classes, problem := personClasses(t, sc.mt)
	if problem != "" {
		v.add(key, problem)
		ok = false
	}
	props, propsOK := v.preset(key+".props", raw.Props, sc)
	if !ok || !propsOK {
		return nil
	}
	b := &TransitionButton{Label: label, props: props}
	if !v.guards(key, b, list, j, classes, sc) {
		return nil
	}
	return b
}

// personClasses returns the roles a load check asks about for t's button:
// t's roles, or the roles that may post its message type when it names
// none. A click is always a person's post, so a trigger is never one.
// problem says why no person could click the button.
func personClasses(t Transition, mt *MessageType) (classes []string, problem string) {
	switch {
	case t.By == TransitionByTrigger:
		return nil, "only a trigger fires this entry, so no person could click its button; drop button or by: trigger"
	case len(t.Roles) > 0:
		return t.Roles, ""
	case len(mt.Post) == 0:
		return nil, fmt.Sprintf("no role may post %s, so no person could click this button", mt.Name)
	}
	return mt.Post, ""
}

// preset checks a button's preset as the channel service checks a post:
// [MessageType.NormalizeProps], then the type's `when:`. It refuses a ref
// or markdown property: a markdown preset could mention people in the
// clicker's name with text they never saw, and a ref names an entity
// whose existence and visibility change behind a label.
func (v buttonChecker) preset(key string, raw rawPreset, sc buttonScope) (map[string]any, bool) {
	mt := sc.mt
	in := make(map[string]any, len(raw))
	ok := true
	for _, name := range sortedKeys(raw) {
		val := presetValue(mt.Properties[name], raw[name].node)
		if p, declared := mt.Properties[name]; declared && val != nil &&
			(p.Type == PropertyRef || p.Type == PropertyMarkdown) {

			why := "a click posts text the clicker never sees"
			if p.Type == PropertyRef {
				why = "a click cannot check the entity it names"
			}
			v.add(key+"."+name, fmt.Sprintf("a preset cannot set a %s property; %s, so a preset sets only "+
				"string, enum, integer, boolean, date and datetime properties", p.Type, why))
			ok = false
			continue
		}
		in[name] = val
	}
	if !ok {
		return nil, false
	}
	np, err := mt.NormalizeProps(in, sc.limits.MessageBytes)
	switch {
	case errors.Is(err, ErrTooManyProps):
		v.add(key, fmt.Sprintf("at most %d properties", realtimelimits.MaxMessageProperties))
		return nil, false
	case errors.Is(err, ErrTooLarge):
		v.add(key, fmt.Sprintf("the preset is over message_bytes (%d bytes)", sc.limits.MessageBytes))
		return nil, false
	case err != nil:
		v.add(key, err.Error())
		return nil, false
	}
	if !v.exactInBrowser(key, np.Props) {
		return nil, false
	}
	if mt.When == nil {
		return np.Props, true
	}
	// A when: on today() would make the file load on one day and fail on
	// the next, as for a trigger's literal message.
	if slices.Contains(mt.When.Functions(), predicatefns.FuncToday) {
		v.add(key, "the when: condition of message type "+mt.Name+" calls today(), so whether this preset "+
			"meets it would depend on the date")
		return nil, false
	}
	if !mt.Holds(context.Background(), mt.PredicateFields(np.Props), time.Time{}) {
		v.add(key, "the preset does not meet the when: condition of message type "+mt.Name)
		return nil, false
	}
	return np.Props, true
}

// maxPresetInt is the largest integer a browser's JSON number carries
// exactly: 2^53.
const maxPresetInt = 1 << 53

// exactInBrowser refuses an integer preset beyond ±[maxPresetInt]. The
// SPA would round it, a click would post the rounded value, and the
// guards would see a value the load never checked.
func (v buttonChecker) exactInBrowser(key string, props map[string]any) bool {
	ok := true
	for _, name := range sortedKeys(props) {
		if n, isInt := props[name].(int64); isInt && (n > maxPresetInt || n < -maxPresetInt) {
			v.add(key+"."+name, fmt.Sprintf("must be between -%d and %d; a browser cannot carry a larger "+
				"integer exactly, so a click would post another value", int64(maxPresetInt), int64(maxPresetInt)))
			ok = false
		}
	}
	return ok
}

// presetValue reads one preset value by the declared type of its
// property p, as JSON would carry it in a post. A boolean needs a YAML
// bool and an integer a YAML int; a list is a sequence of single values;
// every other value is the scalar's text, so an unquoted date stays its
// text. A null value is nil, an absent property. A value of another shape
// passes through for [MessageType.NormalizeProps] to refuse.
func presetValue(p Property, n *yaml.Node) any {
	if n == nil || (n.Kind == yaml.ScalarNode && n.Tag == "!!null") {
		return nil
	}
	if n.Kind == yaml.SequenceNode {
		items := make([]any, 0, len(n.Content))
		for _, c := range n.Content {
			if c = resolveNode(c); c.Tag == "!!null" {
				items = append(items, nil)
			} else {
				items = append(items, c.Value)
			}
		}
		return items
	}
	switch {
	case p.List:
	case p.Type == PropertyBoolean && n.Tag == "!!bool":
		var b bool
		if n.Decode(&b) == nil {
			return b
		}
	case p.Type == PropertyInteger && n.Tag == "!!int":
		var i int64
		if n.Decode(&i) == nil {
			return i
		}
	}
	return n.Value
}

// guards checks the entry's own guard against the preset and records
// which earlier entries take the click first (see [TransitionButton]).
// It refuses a button no caller could see. It adds nothing when a guard
// of the list failed to compile: that already has its problem.
func (v buttonChecker) guards(key string, b *TransitionButton, list []Transition, j int, classes []string,
	sc buttonScope,
) bool {
	t := list[j]
	if slices.ContainsFunc(list[:j+1], func(e Transition) bool { return e.guarded && e.When == nil }) {
		return true
	}
	fields := sc.mt.PredicateFields(b.props)
	if t.When != nil {
		if why := undecidable(t.When); why != "" {
			v.add(key, "the entry's when: "+why+", so whether a click moves the session along this entry is "+
				"unknown when the file loads; a button's own guard may read only the message and session.state")
			return false
		}
		holds, err := loadGuardHolds(t.When, fields, sc)
		switch {
		case err != nil:
			v.add(key, fmt.Sprintf("the entry's when: fails to evaluate for the preset (%v), so a click would "+
				"never move the session", err))
			return false
		case !holds:
			v.add(key, "the entry's when: does not hold for the preset, so a click never moves the session "+
				"along this entry")
			return false
		}
	}
	ok := true
	var undecided []int
	for i, e := range list[:j] {
		if e.By == TransitionByTrigger {
			continue // no person reaches it
		}
		admitsClass := slices.ContainsFunc(classes, func(r string) bool { return entryAdmits(e, posterClass{role: r}) })
		if !e.guarded {
			b.shadowing = append(b.shadowing, i)
			continue
		}
		if undecidable(e.When) != "" {
			undecided = append(undecided, i)
			continue
		}
		holds, err := loadGuardHolds(e.When, fields, sc)
		switch {
		case err != nil && admitsClass:
			v.add(key, fmt.Sprintf("the when: of entry %d fails to evaluate for the preset (%v); that stops "+
				"the decision, so a click would never move the session", i, err))
			ok = false
		case err != nil:
			// Only a caller who also holds a role of entry i reaches it, and
			// for them the decision stops there: hide the button from them.
			undecided = append(undecided, i)
		case holds:
			b.shadowing = append(b.shadowing, i)
		}
	}
	if !ok {
		return false
	}
	b.hidden = append(slices.Clone(b.shadowing), undecided...)
	slices.Sort(b.hidden)
	withButton := t
	withButton.button = b
	reachable := slices.ContainsFunc(classes, func(r string) bool {
		return offers(list, withButton, func(role string) bool { return role == r })
	})
	if !reachable {
		v.add(key, "for every role that could click this button, an earlier entry takes its preset or may "+
			"take it, so no one would ever see the button; move the entry up, or change the earlier entry's "+
			"when or roles")
		return false
	}
	return true
}

// undecidable says why prog's result is unknown at load, or "" when the
// preset and the owning state decide it: it calls no today() and reads no
// field of `session` other than `session.state`.
func undecidable(prog *predicate.Program) string {
	switch {
	case slices.Contains(prog.Functions(), predicatefns.FuncToday):
		return "calls today()"
	case prog.UsesWhole("session"):
		return "reads the whole session"
	}
	for _, a := range prog.Attributes("session") {
		if a != "state" {
			return "reads session." + a
		}
	}
	return ""
}

// loadGuardHolds evaluates a decidable guard for the preset fields in the
// owning state, through [GuardHolds] as the decider does. The other
// session facts are zero, which a decidable guard does not read.
func loadGuardHolds(prog *predicate.Program, fields map[string]predicate.Value, sc buttonScope) (bool, error) {
	counters := make(map[string]predicate.Value, len(sc.s.guardReminders))
	for _, name := range sc.s.guardReminders {
		counters[name] = predicate.NewInt(0)
	}
	facts := GuardFacts{State: sc.state, Reminders: predicate.NewRecord(counters)}
	return GuardHolds(context.Background(), prog, fields, facts, time.Time{})
}

// states checks the buttons of each state of s together: two buttons
// with the same label that one caller could both see are refused, more
// than [buttonItems] buttons in a state warn, and a button labeled as a
// message type that a person's post may move out of the state warns,
// because that type's own button may show beside it.
func (v buttonChecker) states(s *Session, ct *ChannelType, statesKey string) {
	for _, st := range s.states {
		var buttons []Transition
		for _, mn := range sortedKeys(st.on) {
			for _, t := range st.on[mn] {
				if t.button != nil {
					buttons = append(buttons, t)
				}
			}
		}
		if len(buttons) > buttonItems {
			v.warn(statesKey+"."+st.Name+".on", fmt.Sprintf("%d preset buttons; more than %d crowd the "+
				"session header", len(buttons), buttonItems))
		}
		for j, b := range buttons {
			label := foldDisplay(b.button.Label)
			for _, a := range buttons[:j] {
				if strings.EqualFold(foldDisplay(a.button.Label), label) && !exclusive(a, b) {
					v.add(b.key+".button.label", fmt.Sprintf("the button of %s in state %s already reads %q; "+
						"one caller could see both, so give them different labels", a.key, st.Name, a.button.Label))
					break
				}
			}
			if mn := typeLabeled(st, ct, label); mn != "" {
				v.warn(b.key+".button.label", fmt.Sprintf("equals the label of message type %s, whose own "+
					"button may show beside it; give one of them another label", mn))
			}
		}
	}
}

// typeLabeled returns the first message type, by name, whose folded label
// is label and that a person's post may move out of st, or "". A type only
// a trigger moves has no button to show beside a preset button.
func typeLabeled(st SessionState, ct *ChannelType, label string) string {
	for _, mn := range sortedKeys(st.on) {
		if !slices.ContainsFunc(st.on[mn], func(e Transition) bool { return e.By != TransitionByTrigger }) {
			continue
		}
		if mt := ct.MessageTypes[mn]; mt != nil && strings.EqualFold(foldDisplay(mt.Label), label) {
			return mn
		}
	}
	return ""
}

// exclusive reports whether no caller can see the buttons of both a and
// b, a listed first: they are entries of one type and a takes b's preset
// first, so every caller a admits loses b's button.
func exclusive(a, b Transition) bool {
	return a.MessageType == b.MessageType && a.index < b.index && slices.Contains(b.button.shadowing, a.index)
}
