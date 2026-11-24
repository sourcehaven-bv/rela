package dataentryconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// AssignTarget is the relation value of a page `assign:` that names the
// entity of the sidebar entry a row was dropped on.
const AssignTarget = "target"

// Assign is what an item gets when it enters a list or is dropped on a
// generated sidebar entry: property values and relations (TKT-FGE5G3I).
//
// On a list, EntityType is the list's own type and is left empty. On a page,
// EntityType names the type of the items dropped on an entry of that page,
// and a relation value of [AssignTarget] relates the item to the page's
// entity. Any other relation value is an entity id.
//
// A value expression is compiled at load by conditionlint, beside the list
// conditions, and evaluated through the seam dataentry declares.
//
// The values come from config, never from a request, and every write goes
// through the caller's own update and relation checks.
type Assign struct {
	EntityType string                 `yaml:"entity_type,omitempty" json:"entity_type,omitempty"`
	Properties map[string]AssignValue `yaml:"properties,omitempty" json:"properties,omitempty"`
	Relations  map[string]string      `yaml:"relations,omitempty" json:"relations,omitempty"`
}

// IsZero reports whether the block assigns nothing.
func (a *Assign) IsZero() bool {
	return a == nil || (len(a.Properties) == 0 && len(a.Relations) == 0)
}

// AssignValue is one assigned property value: a literal, or a predicate
// value expression written as `{expr: "today()"}`.
type AssignValue struct {
	Literal string
	Expr    string
}

// UnmarshalYAML accepts a scalar literal or a mapping with one `expr` key.
func (v *AssignValue) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		v.Literal = node.Value
		return nil
	}
	var m struct {
		Expr string `yaml:"expr"`
	}
	if err := node.Decode(&m); err != nil {
		return err
	}
	if strings.TrimSpace(m.Expr) == "" {
		return errors.New("assign value: a mapping needs a non-empty expr")
	}
	v.Expr = m.Expr
	return nil
}

// MarshalYAML writes the value back in the shape it was read in.
func (v AssignValue) MarshalYAML() (any, error) {
	if v.Expr != "" {
		return map[string]string{"expr": v.Expr}, nil
	}
	return v.Literal, nil
}

// MarshalJSON serves `{"value": ...}` or `{"expr": ...}`.
func (v AssignValue) MarshalJSON() ([]byte, error) {
	if v.Expr != "" {
		return json.Marshal(map[string]string{"expr": v.Expr})
	}
	return json.Marshal(map[string]string{"value": v.Literal})
}

// TypedLiteral converts a literal to the property's wire type: integers and
// booleans travel as JSON numbers and booleans, a decimal and a date are
// checked and stay text. Validation calls it at load, so the handler's call
// cannot fail on a loaded config.
func TypedLiteral(def *metamodel.PropertyDef, lit string) (any, error) {
	switch def.Type {
	case metamodel.PropertyTypeInteger:
		n, err := strconv.ParseInt(lit, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not an integer", lit)
		}
		return n, nil
	case metamodel.PropertyTypeBoolean:
		b, err := strconv.ParseBool(lit)
		if err != nil {
			return nil, fmt.Errorf("%q is not a boolean", lit)
		}
		return b, nil
	case metamodel.PropertyTypeDecimal:
		if _, err := strconv.ParseFloat(lit, 64); err != nil {
			return nil, fmt.Errorf("%q is not a decimal", lit)
		}
	case metamodel.PropertyTypeDate:
		if _, err := time.Parse(def.GetDateFormat(), lit); err != nil {
			return nil, fmt.Errorf("%q is not a date in the format %s", lit, def.GetDateFormat())
		}
	}
	return lit, nil
}

// validateAssigns checks every list and page `assign:` and every action
// `assign:` against the schema.
func validateAssigns(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string
	for _, id := range slices.Sorted(maps.Keys(cfg.Lists)) {
		l := cfg.Lists[id]
		if l.Assign == nil {
			continue
		}
		at := fmt.Sprintf("list %q: assign", id)
		if l.Assign.EntityType != "" && l.Assign.EntityType != l.EntityType {
			errs = append(errs, at+": entity_type is the list's own type; leave it out")
		}
		errs = append(errs, checkAssign(at, l.EntityType, "", l.Assign, meta)...)
	}
	for _, id := range slices.Sorted(maps.Keys(cfg.Pages)) {
		p := cfg.Pages[id]
		if p.Assign == nil {
			continue
		}
		at := fmt.Sprintf("pages[%s]: assign", id)
		if p.EntityType == "" {
			errs = append(errs, at+": only an entity page (entity_type) takes assign")
			continue
		}
		if p.Assign.EntityType == "" {
			errs = append(errs, at+": entity_type is required: the type of the items dropped on the page's entries")
			continue
		}
		errs = append(errs, checkAssign(at, p.Assign.EntityType, p.EntityType, p.Assign, meta)...)
	}
	for _, id := range slices.Sorted(maps.Keys(cfg.Actions)) {
		a := cfg.Actions[id]
		if a.Assign == "" {
			continue
		}
		l, ok := cfg.Lists[a.Assign]
		if !ok || l.Assign.IsZero() {
			errs = append(errs, fmt.Sprintf("actions: %q assign: list %q does not exist or has no assign", id, a.Assign))
		}
		// The action is a key for the list's assign, which a drop on the
		// list's entry applies to anyone who may update the rows. A
		// permission, scope or when on it would gate the key and not the
		// write, so it is refused rather than left to look like a gate.
		if a.Permission != "" || a.AvailableOn != nil || a.When != "" {
			errs = append(errs, fmt.Sprintf(
				"actions: %q assign: permission, available_on and when are not available on an assign action; "+
					"the write is gated by the rows' update permission", id))
		}
	}
	return errs
}

// assignActionModeError refuses an action with assign beside another mode.
func assignActionModeError(id string, a Action) string {
	if len(a.Set) > 0 || a.Script != "" || a.LLM != nil {
		return fmt.Sprintf("actions: %q has assign beside set, script or llm (must have one)", id)
	}
	return ""
}

// checkAssign validates one block. targetType is the page's entity type, or
// "" on a list, where [AssignTarget] is refused.
func checkAssign(at, itemType, targetType string, a *Assign, meta *metamodel.Metamodel) []string {
	if a.IsZero() {
		return []string{at + ": assigns nothing; set properties or relations"}
	}
	entDef, ok := meta.GetEntityDef(itemType)
	if !ok {
		return []string{fmt.Sprintf("%s: unknown entity type %q", at, itemType)}
	}
	var errs []string
	for _, prop := range slices.Sorted(maps.Keys(a.Properties)) {
		errs = append(errs, checkAssignProperty(at, prop, a.Properties[prop], entDef, meta)...)
	}
	for _, rel := range slices.Sorted(maps.Keys(a.Relations)) {
		val := a.Relations[rel]
		def, ok := meta.GetRelationDef(rel)
		switch {
		case !ok:
			errs = append(errs, fmt.Sprintf("%s: unknown relation %q", at, rel))
			continue
		case metamodel.RelationEndCode(def, itemType, true) != "":
			errs = append(errs, fmt.Sprintf("%s: relation %q does not start at %q", at, rel, itemType))
			continue
		case strings.TrimSpace(val) == "":
			errs = append(errs, fmt.Sprintf("%s: relation %q needs a value", at, rel))
		case val == AssignTarget && targetType == "":
			errs = append(errs, fmt.Sprintf(
				"%s: relation %q: %q is only available on a page assign", at, rel, AssignTarget))
		case val == AssignTarget && metamodel.RelationEndCode(def, targetType, false) != "":
			errs = append(errs, fmt.Sprintf("%s: relation %q does not end at the page's type %q", at, rel, targetType))
		}
	}
	return errs
}

func checkAssignProperty(
	at, prop string, v AssignValue, entDef *metamodel.EntityDef, meta *metamodel.Metamodel,
) []string {
	def, ok := entDef.Properties[prop]
	switch {
	case !ok:
		return []string{fmt.Sprintf("%s: property %q is not a property of the item type", at, prop)}
	case def.Computed != "" || def.Rollup != nil:
		return []string{fmt.Sprintf("%s: property %q is computed and cannot be assigned", at, prop)}
	}
	if v.Expr == "" && v.Literal == "" {
		// yaml.v3 decodes `~` and an empty value to the zero value without
		// calling UnmarshalYAML, so a null is caught here.
		return []string{fmt.Sprintf(`%s: property %q has no value; write {expr: "nil"} to unset it`, at, prop)}
	}
	if v.Expr != "" {
		// Compiled by conditionlint, which holds the predicate engine.
		return nil
	}
	if _, err := TypedLiteral(&def, v.Literal); err != nil {
		return []string{fmt.Sprintf("%s: property %q: %v", at, prop, err)}
	}
	values := def.Values
	if def.Type != metamodel.PropertyTypeEnum {
		values = meta.Types[def.Type].Values
	}
	if len(values) > 0 && !slices.Contains(values, v.Literal) {
		return []string{fmt.Sprintf("%s: property %q: %q is not one of %v", at, prop, v.Literal, values)}
	}
	return nil
}
