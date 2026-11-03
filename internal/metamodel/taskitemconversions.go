package metamodel

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// TaskItemConversionDef is an operator-declared recipe that turns a markdown
// task item in a `from` entity's body into a new `to` entity linked to the
// source. The recipe is the whole mapping: a request names the recipe and
// supplies only the item text and values for required fields the mapping
// leaves empty.
type TaskItemConversionDef struct {
	From     string                     `yaml:"from"`
	To       string                     `yaml:"to"`
	Label    string                     `yaml:"label"`
	Relation TaskItemConversionRelation `yaml:"relation"`
	// Face is the face the new entity is created at. Required when `to`
	// declares faces; refused when it does not.
	Face string `yaml:"face,omitempty"`
	// Item is what the editor does with the task item after the conversion.
	// Empty means [TaskItemLink].
	Item   string            `yaml:"item,omitempty"`
	Fields map[string]string `yaml:"fields"`
}

// TaskItemConversionRelation names the relation that links the source and the
// new entity, and which end the new entity takes.
type TaskItemConversionRelation struct {
	Type string `yaml:"type"`
	// As is the role of the NEW entity: [TaskItemAsTo] makes the source the
	// relation's `from`, [TaskItemAsFrom] makes the new entity the `from`.
	As string `yaml:"as"`
}

// Values of [TaskItemConversionRelation.As].
const (
	TaskItemAsFrom = "from"
	TaskItemAsTo   = "to"
)

// Values of [TaskItemConversionDef.Item].
const (
	TaskItemLink   = "link"
	TaskItemRemove = "remove"
)

// ItemMode returns the effective [TaskItemConversionDef.Item].
func (d TaskItemConversionDef) ItemMode() string {
	if d.Item == "" {
		return TaskItemLink
	}
	return d.Item
}

// TemplateTokenKind classifies one [TemplateToken].
type TemplateTokenKind int

const (
	// TokenLiteral is text copied as is.
	TokenLiteral TemplateTokenKind = iota
	// TokenItemText is the task item's text.
	TokenItemText
	// TokenSource is a property of the source entity, named by Value.
	TokenSource
)

// TemplateToken is one piece of a parsed task-item field template.
type TemplateToken struct {
	Kind  TemplateTokenKind
	Value string
}

var taskItemPlaceholder = regexp.MustCompile(`\{\{\s*([^{}]*?)\s*\}\}`)
var taskItemSourceRef = regexp.MustCompile(`^source\.([A-Za-z0-9_-]+)$`)

// ParseTaskItemTemplate splits a field template into literal text and
// placeholders. The only placeholders are `{{item.text}}` and
// `{{source.<property>}}`; any other `{{...}}`, or an unbalanced brace pair,
// is an error.
//
// Callers expand the tokens one by one and never re-scan the result, so a
// placeholder inside the item text stays literal text.
func ParseTaskItemTemplate(template string) ([]TemplateToken, error) {
	var tokens []TemplateToken
	literal := func(s string) error {
		if strings.Contains(s, "{{") || strings.Contains(s, "}}") {
			return fmt.Errorf("unbalanced `{{` or `}}` in %q", template)
		}
		if s != "" {
			tokens = append(tokens, TemplateToken{Kind: TokenLiteral, Value: s})
		}
		return nil
	}
	rest := 0
	for _, loc := range taskItemPlaceholder.FindAllStringSubmatchIndex(template, -1) {
		if err := literal(template[rest:loc[0]]); err != nil {
			return nil, err
		}
		expr := template[loc[2]:loc[3]]
		switch m := taskItemSourceRef.FindStringSubmatch(expr); {
		case expr == "item.text":
			tokens = append(tokens, TemplateToken{Kind: TokenItemText})
		case m != nil:
			tokens = append(tokens, TemplateToken{Kind: TokenSource, Value: m[1]})
		default:
			return nil, fmt.Errorf("placeholder {{%s}} is not allowed; use {{item.text}} or {{source.<property>}}", expr)
		}
		rest = loc[1]
	}
	if err := literal(template[rest:]); err != nil {
		return nil, err
	}
	return tokens, nil
}

// UsesItemText reports whether a parsed template contains `{{item.text}}`.
func UsesItemText(tokens []TemplateToken) bool {
	for _, t := range tokens {
		if t.Kind == TokenItemText {
			return true
		}
	}
	return false
}

func validateTaskItemConversions(m *Metamodel) []string {
	names := make([]string, 0, len(m.TaskItemConversions))
	for name := range m.TaskItemConversions {
		names = append(names, name)
	}
	sort.Strings(names)
	errs := make([]string, 0, len(names))
	for _, name := range names {
		errs = append(errs, validateTaskItemConversion(m, name, m.TaskItemConversions[name])...)
	}
	return errs
}

func validateTaskItemConversion(m *Metamodel, name string, d TaskItemConversionDef) []string {
	var errs []string
	bad := func(format string, args ...any) {
		errs = append(errs, fmt.Sprintf("task item conversion %q: "+format, append([]any{name}, args...)...))
	}
	if !conversionNamePattern.MatchString(name) {
		bad("name must match %s", conversionNamePattern.String())
	}
	if strings.TrimSpace(d.Label) == "" {
		bad("label is required")
	}
	if d.Item != "" && d.Item != TaskItemLink && d.Item != TaskItemRemove {
		bad("item must be %q or %q, not %q", TaskItemLink, TaskItemRemove, d.Item)
	}
	from, fromOK := m.GetEntityDef(d.From)
	to, toOK := m.GetEntityDef(d.To)
	if !fromOK {
		bad("from: entity type %q is not declared", d.From)
	}
	if !toOK {
		bad("to: entity type %q is not declared", d.To)
	}
	validateTaskItemRelation(m, d, fromOK && toOK, bad)
	if len(d.Fields) == 0 {
		bad("requires a `fields:` mapping")
	}
	if toOK {
		validateTaskItemFace(d, *to, bad)
		validateTaskItemTarget(d, *to, bad)
		validateMappedTargetProperties(d.To, *to, d.Fields, bad)
	}
	keys := make([]string, 0, len(d.Fields))
	for key := range d.Fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	usesItem := false
	for _, key := range keys {
		tokens, err := ParseTaskItemTemplate(d.Fields[key])
		if err != nil {
			bad("fields.%s: %v", key, err)
			usesItem = true // the parse error is the one to report
			continue
		}
		usesItem = usesItem || UsesItemText(tokens)
		if toOK {
			if property, exists := to.Properties[key]; exists {
				validateTaskItemValueType(m, key, tokens, property, bad)
			}
		}
		if !fromOK {
			continue
		}
		for _, t := range tokens {
			if _, exists := from.Properties[t.Value]; t.Kind == TokenSource && !exists {
				bad("fields.%s: source type %q has no property %q", key, d.From, t.Value)
			}
		}
	}
	if len(keys) > 0 && !usesItem {
		bad("no field uses {{item.text}}, so the item's text would be lost")
	}
	return errs
}

// validateTaskItemValueType refuses a template whose value cannot have the
// target property's type. A lone {{source.<property>}} copies the stored
// value as is. Any other template makes one line of text, which fits a
// string or rrule property, or an enum when it is a literal naming one of
// the enum's values. It never fits a list, a number, a boolean or a date.
func validateTaskItemValueType(
	m *Metamodel, key string, tokens []TemplateToken, property PropertyDef, bad conversionProblem,
) {
	if len(tokens) == 1 && tokens[0].Kind == TokenSource {
		return
	}
	if property.List {
		bad("fields.%s: target property %q is a list, which a text template cannot fill", key, key)
		return
	}
	// A custom type without values is a string with validations.
	values := property.Values
	custom, isCustom := m.Types[property.Type]
	if isCustom {
		values = custom.Values
	}
	switch {
	case len(values) > 0:
		literal, isLiteral := taskItemLiteral(tokens)
		if !isLiteral || !slices.Contains(values, literal) {
			bad("fields.%s: target property %q takes one of %s, so its template must be one of them as plain text",
				key, key, strings.Join(values, ", "))
		}
	case property.Type == PropertyTypeString || property.Type == PropertyTypeRrule || isCustom:
	default:
		bad("fields.%s: target property %q has type %s, which a text template cannot fill", key, key, property.Type)
	}
}

// taskItemLiteral returns the text of a template made of literals only.
func taskItemLiteral(tokens []TemplateToken) (string, bool) {
	var b strings.Builder
	for _, t := range tokens {
		if t.Kind != TokenLiteral {
			return "", false
		}
		b.WriteString(t.Value)
	}
	return b.String(), true
}

func validateTaskItemRelation(m *Metamodel, d TaskItemConversionDef, typesOK bool, bad conversionProblem) {
	rel, exists := m.GetRelationDef(d.Relation.Type)
	if !exists {
		bad("relation.type: relation %q is not declared", d.Relation.Type)
		return
	}
	relFrom, relTo := d.From, d.To
	switch d.Relation.As {
	case TaskItemAsTo:
	case TaskItemAsFrom:
		relFrom, relTo = d.To, d.From
	default:
		bad("relation.as must be %q or %q, not %q", TaskItemAsFrom, TaskItemAsTo, d.Relation.As)
		return
	}
	if !typesOK {
		return
	}
	if !containsTypeOrAny(rel.From, relFrom) {
		bad("relation %q cannot originate from %q", d.Relation.Type, relFrom)
	}
	if !containsTypeOrAny(rel.To, relTo) {
		bad("relation %q cannot target %q", d.Relation.Type, relTo)
	}
}

// validateTaskItemTarget refuses a target type the conversion cannot create.
// The conversion passes no id, so a manual-id type has none to give. A file or
// external reference cannot come from a text template or the required-fields
// dialog, so such a property may be neither mapped nor required.
func validateTaskItemTarget(d TaskItemConversionDef, to EntityDef, bad conversionProblem) {
	if to.IsManualID() {
		bad("to: entity type %q uses manual ids, and a task-item conversion has no id to give it", d.To)
	}
	keys := make([]string, 0, len(to.Properties))
	for key := range to.Properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		property := to.Properties[key]
		if property.Computed != "" || property.Type != PropertyTypeFile && property.Type != PropertyTypeExternalRef {
			continue
		}
		if _, mapped := d.Fields[key]; mapped && property.Type == PropertyTypeFile {
			bad("fields: target property %q is a file and cannot be mapped", key)
		}
		if property.Required {
			bad("target property %q is a required %s and cannot be authored by a task-item conversion", key, property.Type)
		}
	}
}

func validateTaskItemFace(d TaskItemConversionDef, to EntityDef, bad conversionProblem) {
	switch {
	case len(to.Faces) == 0 && d.Face != "":
		bad("face: target type %q has no faces", d.To)
	case len(to.Faces) > 0 && d.Face == "":
		bad("face: target type %q has faces; name the face to create", d.To)
	case d.Face != "":
		if _, exists := to.Faces[d.Face]; !exists {
			bad("face: target type %q has no face %q", d.To, d.Face)
		}
	}
}
