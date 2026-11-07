package flowcfg

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Template references. A template value is a YAML literal, or a string in
// which `{{ref}}` placeholders are replaced when the output runs.
const (
	// RefSubjectID is the subject's id.
	RefSubjectID = "subject.id"
	// RefRunID is the run's id.
	RefRunID = "run.id"
	// RefNow is the time the output runs, in RFC 3339.
	RefNow = "now"
	// RefToday is the date the output runs, as YYYY-MM-DD.
	RefToday = "today"
	// refSubjectProp starts a reference to a property of the subject.
	refSubjectProp = "subject."
)

// Template is one compiled output property value.
type Template struct {
	// literal is the value when the template has no placeholder.
	literal any
	isLit   bool
	parts   []templatePart
}

// templatePart is a run of text, or one placeholder when ref is set.
type templatePart struct {
	text string
	ref  string
}

// RunID names one run of a flow ([flow.RunID]).
type RunID = flow.RunID

// Vars are what a template reads when its output runs.
type Vars struct {
	// Subject is the entity the run is about.
	Subject entity.Ref
	Run     RunID
	Now     time.Time
	// Props holds the subject's properties as the engine read them.
	Props map[string]any
}

// Literal reports whether the template has no placeholder.
func (t Template) Literal() bool { return t.isLit }

// Refs returns the references the template reads, in order of first use.
func (t Template) Refs() []string {
	var out []string
	for _, p := range t.parts {
		if p.ref != "" && !slices.Contains(out, p.ref) {
			out = append(out, p.ref)
		}
	}
	return out
}

// Eval returns the template's value for vars. A template that is one
// placeholder keeps the referenced value's type, so a whole
// `{{subject.due}}` copies a date as a date; any other template is text.
// ok is false when a whole placeholder names a subject property that has
// no value, so the output leaves the property alone.
func (t Template) Eval(vars Vars) (value any, ok bool) {
	if t.isLit {
		return t.literal, true
	}
	if len(t.parts) == 1 && t.parts[0].ref != "" {
		return refValue(t.parts[0].ref, vars)
	}
	var b strings.Builder
	for _, p := range t.parts {
		if p.ref == "" {
			b.WriteString(p.text)
			continue
		}
		if v, has := refValue(p.ref, vars); has {
			fmt.Fprint(&b, v)
		}
	}
	return b.String(), true
}

func refValue(ref string, vars Vars) (any, bool) {
	switch ref {
	case RefSubjectID:
		return vars.Subject.ID, true
	case RefRunID:
		return string(vars.Run), true
	case RefNow:
		return vars.Now.UTC().Format(time.RFC3339), true
	case RefToday:
		return vars.Now.UTC().Format(time.DateOnly), true
	}
	v, ok := vars.Props[strings.TrimPrefix(ref, refSubjectProp)]
	if !ok || v == nil {
		return nil, false
	}
	return v, true
}

// splitTemplate splits s into text and placeholders. reason is non-empty
// for an unclosed or empty placeholder.
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
		j := strings.Index(s[i+2:], "}}")
		if j < 0 {
			return nil, "has an unclosed {{"
		}
		ref := strings.TrimSpace(s[i+2 : i+2+j])
		if ref == "" {
			return nil, "has an empty {{}}"
		}
		parts = append(parts, templatePart{ref: ref})
		s = s[i+2+j+2:]
	}
	return parts, ""
}

// textTypes are the property types a template with text around a
// placeholder, or a reference of another type, may write.
var textTypes = []string{"string", "markdown", "text"}

// compileTemplate compiles one output property value written to target,
// the property name. A literal must pass the property's validation.
// Subject references resolve against subjectType. The returned reason is
// non-empty when the value cannot be compiled.
func (l *loader) compileTemplate(
	name string, value any, target metamodel.PropertyDef, subjectType string,
) (tpl Template, reason string) {
	s, isString := value.(string)
	if !isString || !strings.Contains(s, "{{") {
		if err := metamodel.NewValidator(l.meta).ValidatePropertyValue(name, &target, value); err != nil {
			return Template{}, err.Error()
		}
		return Template{literal: value, isLit: true}, ""
	}
	parts, reason := splitTemplate(s)
	if reason != "" {
		return Template{}, reason
	}
	whole := len(parts) == 1
	for _, p := range parts {
		if p.ref == "" {
			continue
		}
		refType, reason := l.refType(p.ref, subjectType)
		if reason != "" {
			return Template{}, reason
		}
		if whole && refType != "" && refType == target.Type {
			if reason := sameValues(p.ref, l.subjectProp(p.ref, subjectType), target); reason != "" {
				return Template{}, reason
			}
			continue
		}
		if !slices.Contains(textTypes, target.Type) {
			return Template{}, fmt.Sprintf("writes {{%s}} (%s) into a %s property; only a whole placeholder of the "+
				"same type, or a text property, may take it", p.ref, refTypeName(refType), target.Type)
		}
	}
	return Template{parts: parts}, ""
}

// refType is the property type ref yields: "" for an id, which only a
// text property takes. reason is non-empty for an unknown reference, a
// roll-up, or a subject property acl.yaml restricts. A roll-up is folded
// per reader, so a run would copy an aggregate over rows its readers may
// not see.
func (l *loader) refType(ref, subjectType string) (typ, reason string) {
	switch ref {
	case RefSubjectID, RefRunID:
		return "", ""
	case RefNow:
		return "datetime", ""
	case RefToday:
		return "date", ""
	}
	name, ok := strings.CutPrefix(ref, refSubjectProp)
	if !ok {
		return "", fmt.Sprintf("names {{%s}}; a template may read subject.id, subject.<property>, run.id, now "+
			"and today", ref)
	}
	def, _ := l.meta.GetEntityDef(subjectType)
	pd, ok := def.Properties[name]
	if !ok {
		return "", fmt.Sprintf("names {{%s}}, but %s has no property %q", ref, subjectType, name)
	}
	if pd.Rollup != nil {
		return "", fmt.Sprintf("names {{%s}}, a roll-up; its value depends on who reads it", ref)
	}
	if l.policy != nil && l.policy.FieldRestricted(subjectType, name) {
		return "", fmt.Sprintf("names {{%s}}, which a visible: grant in acl.yaml restricts; the output would "+
			"copy its value where its readers may not see it", ref)
	}
	return pd.Type, ""
}

// subjectProp returns the subject property ref names, or nil for any
// other reference.
func (l *loader) subjectProp(ref, subjectType string) *metamodel.PropertyDef {
	name, ok := strings.CutPrefix(ref, refSubjectProp)
	if !ok {
		return nil
	}
	def, _ := l.meta.GetEntityDef(subjectType)
	pd, ok := def.Properties[name]
	if !ok {
		return nil
	}
	return &pd
}

// sameValues refuses an inline enum copied into an inline enum that does
// not allow every one of its values.
func sameValues(ref string, from *metamodel.PropertyDef, target metamodel.PropertyDef) string {
	if from == nil || len(from.Values) == 0 {
		return ""
	}
	for _, v := range from.Values {
		if !slices.Contains(target.Values, v) {
			return fmt.Sprintf("writes {{%s}}, which may be %q, into a property that does not allow it", ref, v)
		}
	}
	return ""
}

func refTypeName(typ string) string {
	if typ == "" {
		return "an id"
	}
	return typ
}
