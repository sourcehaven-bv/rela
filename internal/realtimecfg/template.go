package realtimecfg

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// Template is one compiled property value of a session output. It is one
// of three forms:
//
//   - a literal, typed as the YAML gave it;
//   - a whole-value reference such as `{{message.risk}}`, which keeps the
//     referenced value's type;
//   - a string with references inside, such as "Contained by
//     {{message.author}}", whose references are substituted as text.
//
// Evaluation is a single pass: a substituted value is never scanned for
// references again, so a message value cannot inject one. `now` and
// `today` come from the notice time in UTC, never from the clock, so a
// retry writes the same values. It is immutable: its fields are
// unexported and never written after it compiles.
type Template struct {
	lit    any
	ref    string
	parts  []templatePart
	target string
	// isLit tells a literal nil apart from no literal; a nil literal never
	// compiles, but the zero Template must not read as one.
	isLit bool
}

// templatePart is one piece of a mixed template: text, or a reference.
type templatePart struct {
	text string
	ref  string
}

// OutputVars are the values a [Template] reads.
type OutputVars struct {
	// Message holds the cause message's properties in their stored form.
	Message map[string]any
	// Author is the cause message's author, as a user id.
	Author string
	// From and To are the states of the move, Generation the session
	// generation after it.
	From, To   string
	Generation int64
	// ChannelKey, ChannelType and ChannelTitle describe the channel.
	ChannelKey, ChannelType, ChannelTitle string
	// At is the notice time. `now` and `today` read it in UTC.
	At time.Time
}

// Literal reports whether the template is a literal: config, not a value
// a poster wrote. A create does not field-gate a literal; a set gates
// every property.
func (t Template) Literal() bool { return t.isLit }

// ReadsTitle reports whether the template reads `{{channel.title}}`, so a
// runner reads the channel head only when an output needs it.
func (t Template) ReadsTitle() bool {
	if t.ref == refChannelTitle {
		return true
	}
	for _, p := range t.parts {
		if p.ref == refChannelTitle {
			return true
		}
	}
	return false
}

// Eval returns the value the template writes into its property. ok is
// false when a whole-value reference names an absent message property; the
// property is then left out of the write. A mixed template substitutes the
// empty string for an absent reference.
func (t Template) Eval(vars OutputVars) (value any, ok bool) {
	switch {
	case t.isLit:
		return literalValue(t.lit, t.target), true
	case t.ref != "":
		return t.refValue(t.ref, vars)
	}
	var b strings.Builder
	for _, p := range t.parts {
		if p.ref == "" {
			b.WriteString(p.text)
			continue
		}
		b.WriteString(refText(p.ref, vars))
	}
	return b.String(), true
}

// literalValue is a literal in the form the entity property stores. YAML
// reads an unquoted timestamp as a time; a date or datetime property
// stores a string.
func literalValue(v any, target string) any {
	tm, ok := v.(time.Time)
	if !ok {
		return v
	}
	if target == metamodel.PropertyTypeDate {
		return tm.UTC().Format(metamodel.DefaultDateFormat)
	}
	return tm.UTC().Format(metamodel.DefaultDatetimeFormat)
}

// refValue is a whole-value reference, typed for the target property.
func (t Template) refValue(ref string, vars OutputVars) (any, bool) {
	if name, isMsg := strings.CutPrefix(ref, msgPrefix); isMsg && name != refAuthorName {
		v, present := vars.Message[name]
		if !present || v == nil {
			return nil, false
		}
		if n, isInt := asInt64(v); isInt && t.target == metamodel.PropertyTypeInteger {
			return n, true
		}
		return v, true
	}
	if ref == refGeneration && t.target == metamodel.PropertyTypeInteger {
		return vars.Generation, true
	}
	return refText(ref, vars), true
}

// refText is a reference's text form.
func refText(ref string, vars OutputVars) string {
	switch ref {
	case refAuthor:
		return vars.Author
	case refFrom:
		return vars.From
	case refTo:
		return vars.To
	case refGeneration:
		return strconv.FormatInt(vars.Generation, 10)
	case refChannelKey:
		return vars.ChannelKey
	case refChannelType:
		return vars.ChannelType
	case refChannelTitle:
		return vars.ChannelTitle
	case refNow:
		return vars.At.UTC().Format(metamodel.DefaultDatetimeFormat)
	case refToday:
		return vars.At.UTC().Format(metamodel.DefaultDateFormat)
	}
	v, ok := vars.Message[strings.TrimPrefix(ref, msgPrefix)]
	if !ok || v == nil {
		return ""
	}
	if s, isStr := v.(string); isStr {
		return s
	}
	return fmt.Sprint(v)
}

// asInt64 reads an integer message value. A value read back from JSON is a
// float64.
func asInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		return int64(n), float64(int64(n)) == n
	}
	return 0, false
}

// The references a template may name.
const (
	msgPrefix       = "message."
	refAuthorName   = "author"
	refAuthor       = msgPrefix + refAuthorName
	refFrom         = "session.from"
	refTo           = "session.to"
	refGeneration   = "session.generation"
	refChannelKey   = "channel.key"
	refChannelType  = "channel.type"
	refChannelTitle = "channel.title"
	refNow          = "now"
	refToday        = "today"
)

const knownRefs = "use message.<property>, message.author, session.from, session.to, session.generation, " +
	"channel.key, channel.type, channel.title, now or today"

// templateTarget is the entity property a template fills.
type templateTarget struct {
	entityType string
	name       string
	def        metamodel.PropertyDef
}

func (tt templateTarget) String() string { return tt.entityType + "." + tt.name }

// compileTemplate compiles raw into a template for target, with mt the
// cause message type. It returns the first problem as a reason.
func (v writeChecker) compileTemplate(raw any, tt templateTarget, mt *MessageType) (tpl Template, reason string) {
	s, isStr := raw.(string)
	if !isStr {
		switch raw.(type) {
		case map[string]any, []any:
			return Template{}, "must be a single value"
		case nil:
			return Template{}, "name a value"
		}
		lit := literalValue(raw, tt.def.Type)
		if err := metamodel.NewValidator(v.meta).ValidatePropertyValue(tt.name, &tt.def, lit); err != nil {
			return Template{}, fmt.Sprintf("cannot fill %s: %v", tt, err)
		}
		return Template{lit: raw, isLit: true, target: tt.def.Type}, ""
	}
	if len(s) > realtimelimits.MaxOutputTemplateBytes {
		return Template{}, fmt.Sprintf("a template is at most %d bytes", realtimelimits.MaxOutputTemplateBytes)
	}
	parts, reason := splitTemplate(s)
	if reason != "" {
		return Template{}, reason
	}
	t := Template{target: tt.def.Type}
	switch {
	case len(parts) == 1 && parts[0].ref == "":
		if err := metamodel.NewValidator(v.meta).ValidatePropertyValue(tt.name, &tt.def, s); err != nil {
			return Template{}, fmt.Sprintf("cannot fill %s: %v", tt, err)
		}
		t.lit, t.isLit = s, true
		return t, ""
	case len(parts) == 1:
		if reason := v.wholeRef(parts[0].ref, tt, mt); reason != "" {
			return Template{}, reason
		}
		t.ref = parts[0].ref
		return t, ""
	}
	if tt.def.Type != metamodel.PropertyTypeString {
		return Template{}, fmt.Sprintf("text around a reference fills only a string property; %s is a %s property",
			tt, tt.def.Type)
	}
	for _, p := range parts {
		if p.ref == "" {
			continue
		}
		if reason := textRef(p.ref, mt); reason != "" {
			return Template{}, reason
		}
	}
	t.parts = parts
	return t, ""
}

// splitTemplate splits s into text and `{{reference}}` parts. Spaces
// inside the braces are ignored.
func splitTemplate(s string) (parts []templatePart, reason string) {
	for s != "" {
		i := strings.Index(s, "{{")
		if i < 0 {
			parts = append(parts, templatePart{text: s})
			break
		}
		if i > 0 {
			parts = append(parts, templatePart{text: s[:i]})
		}
		j := strings.Index(s[i:], "}}")
		if j < 0 {
			return nil, "unterminated {{; close the reference with }}"
		}
		ref := strings.TrimSpace(s[i+2 : i+j])
		if ref == "" {
			return nil, "empty reference {{}}"
		}
		parts = append(parts, templatePart{ref: ref})
		s = s[i+j+2:]
	}
	if len(parts) == 0 {
		parts = []templatePart{{}}
	}
	return parts, ""
}

// refKind checks that ref is a known reference and returns, for a message
// property, its declaration.
func refKind(ref string, mt *MessageType) (prop Property, isProp bool, reason string) {
	if strings.HasPrefix(ref, "entity.") || ref == "entity" {
		return Property{}, false, "outputs cannot read the entity; a value from the entity would be written " +
			"with the writer's authority and could copy a hidden field"
	}
	switch ref {
	case refAuthor, refFrom, refTo, refGeneration, refChannelKey, refChannelType, refChannelTitle, refNow, refToday:
		return Property{}, false, ""
	}
	name, isMsg := strings.CutPrefix(ref, msgPrefix)
	if !isMsg {
		return Property{}, false, fmt.Sprintf("unknown reference %s; %s", ref, knownRefs)
	}
	p, ok := mt.Properties[name]
	if !ok {
		return Property{}, false, fmt.Sprintf("message type %s has no property %q", mt.Name, name)
	}
	return p, true, ""
}

// wholeRef checks that a whole-value reference can fill tt.
func (v writeChecker) wholeRef(ref string, tt templateTarget, mt *MessageType) string {
	p, isProp, reason := refKind(ref, mt)
	if reason != "" {
		return reason
	}
	pt := tt.def.Type
	fills := pt == metamodel.PropertyTypeString
	switch {
	case isProp:
		fills = promotable(p, tt.def)
	case ref == refGeneration:
		fills = fills || pt == metamodel.PropertyTypeInteger
	case ref == refNow:
		fills = fills || pt == metamodel.PropertyTypeDatetime
	case ref == refToday:
		fills = fills || pt == metamodel.PropertyTypeDate
	}
	if fills {
		return ""
	}
	if isProp {
		return fmt.Sprintf("message property %s (%s) cannot fill %s, a %s property", p.Name, p.Type, tt, pt)
	}
	return fmt.Sprintf("%s cannot fill %s, a %s property", ref, tt, pt)
}

// textRef checks that a reference inside text reads as text.
func textRef(ref string, mt *MessageType) string {
	p, isProp, reason := refKind(ref, mt)
	if reason != "" || !isProp {
		return reason
	}
	if p.Type == PropertyString || p.Type == PropertyMarkdown || p.Type == PropertyEnum || p.Type == PropertyRef {
		return ""
	}
	return fmt.Sprintf("message property %s (%s) is not text; text around a reference takes only string, "+
		"markdown, enum and ref properties", p.Name, p.Type)
}
