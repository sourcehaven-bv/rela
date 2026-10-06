package configedit

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Rename says a property was renamed rather than removed and added, so its
// values move to the new name.
type Rename struct {
	EntityType string `json:"entity_type"`
	From       string `json:"from"`
	To         string `json:"to"`
}

// ValueMapping says where records holding a removed option go. Property is
// the property's name after the save.
type ValueMapping struct {
	EntityType string `json:"entity_type"`
	Property   string `json:"property"`
	From       string `json:"from"`
	To         string `json:"to"`
}

// Counter counts the records of a type whose property holds a value, or any
// value when value is empty. It reads the store as it is before the save.
type Counter interface {
	Count(ctx context.Context, entityType, property, value string) (int, error)
}

// A Problem is something the draft must resolve before it can be saved.
type Problem struct {
	// Code is stable for the browser: value_in_use, type_change,
	// unsupported_change, rename, acl_reference, invalid, locked.
	Code       string `json:"code"`
	Message    string `json:"message"`
	File       File   `json:"file,omitempty"`
	Path       string `json:"path,omitempty"`
	EntityType string `json:"entity_type,omitempty"`
	Property   string `json:"property,omitempty"`
	Value      string `json:"value,omitempty"`
	Count      int    `json:"count,omitempty"`
}

// Step is one migration step as the review screen shows it.
type Step struct {
	Kind       string `json:"kind"` // rename_property, map_values, convert
	EntityType string `json:"entity_type"`
	Property   string `json:"property"`
	From       string `json:"from,omitempty"`
	To         string `json:"to,omitempty"`
	Count      int    `json:"count"`
}

// migrationPlan is what a draft needs done to the records.
type migrationPlan struct {
	steps    []Step
	spec     []datamigration.SpecStep
	problems []Problem
}

// planMigration works out the migration a schema change needs, from the
// user's explicit renames and value choices. It never guesses: a removed
// option still in use without a destination, or a type change it cannot
// convert, is a problem for the user to resolve, not a step.
func planMigration(
	ctx context.Context, before, after *metamodel.Metamodel,
	renames []Rename, mappings []ValueMapping, counts Counter,
) (*migrationPlan, error) {
	p := &migrationPlan{}
	newName := map[[2]string]string{} // (type, old property) → new property
	for _, r := range renames {
		if msg := checkRename(before, after, r); msg != "" {
			p.problems = append(p.problems, Problem{
				Code: "rename", Message: msg, EntityType: r.EntityType, Property: r.From,
			})
			continue
		}
		n, err := counts.Count(ctx, r.EntityType, r.From, "")
		if err != nil {
			return nil, err
		}
		newName[[2]string{r.EntityType, r.From}] = r.To
		p.steps = append(p.steps, Step{
			Kind: "rename_property", EntityType: r.EntityType, Property: r.To, From: r.From, To: r.To, Count: n,
		})
		p.spec = append(p.spec, datamigration.SpecStep{RenameProperty: &datamigration.RenamePropertySpec{
			Entity: r.EntityType, From: r.From, To: r.To,
		}})
	}

	mapped := map[[3]string]string{}
	for _, m := range mappings {
		mapped[[3]string{m.EntityType, m.Property, m.From}] = m.To
	}

	for _, typeName := range sortedKeys(before.Entities) {
		newDef, ok := after.Entities[typeName]
		if !ok {
			continue
		}
		oldDef := before.Entities[typeName]
		for _, oldProp := range sortedKeys(oldDef.Properties) {
			prop := oldProp
			if renamed, ok := newName[[2]string{typeName, oldProp}]; ok {
				prop = renamed
			}
			newPD, ok := newDef.Properties[prop]
			if !ok {
				continue
			}
			oldPD := oldDef.Properties[oldProp]
			err := p.planProperty(ctx, before, after, typeName, oldProp, prop, oldPD, newPD, mapped, counts)
			if err != nil {
				return nil, err
			}
		}
	}

	p.problems = append(p.problems, relationValueProblems(before, after)...)
	shapeDeltas := metamodel.CompareShapes(before.ShapeProjection(), after.ShapeProjection())
	for _, d := range shapeDeltas.ByTier(metamodel.TierMigration) {
		if strings.HasPrefix(d.Kind, "relation_") || strings.HasPrefix(d.Kind, "faces_") {
			p.problems = append(p.problems, Problem{
				Code:    "unsupported_change",
				Message: d.Detail + ". Saving this needs a migration script; use `rela migrate gen` instead.",
			})
		}
	}
	return p, nil
}

func (p *migrationPlan) planProperty(
	ctx context.Context, before, after *metamodel.Metamodel,
	typeName, oldProp, prop string, oldPD, newPD metamodel.PropertyDef,
	mapped map[[3]string]string, counts Counter,
) error {
	oldVals, newVals := optionValues(before, oldPD), optionValues(after, newPD)
	if typeChanged(before, after, oldPD, newPD) {
		n, err := counts.Count(ctx, typeName, oldProp, "")
		if err != nil || n == 0 {
			return err
		}
		if oldPD.List != newPD.List || oldPD.Format != newPD.Format || len(newVals) > 0 || !coercible(newPD.Type) {
			p.problems = append(p.problems, Problem{
				Code: "type_change", EntityType: typeName, Property: prop, Count: n,
				Message: fmt.Sprintf("%d %s records hold a value for %s, and its new type cannot be converted to automatically.",
					n, typeName, prop),
			})
			return nil
		}
		p.steps = append(p.steps, Step{Kind: "convert", EntityType: typeName, Property: prop, To: newPD.Type, Count: n})
		p.spec = append(p.spec, datamigration.SpecStep{Convert: &datamigration.ConvertSpec{
			Entity: typeName, Property: prop, ToType: newPD.Type,
		}})
		return nil
	}
	if len(oldVals) == 0 {
		return nil
	}
	keep := map[string]bool{}
	for _, v := range newVals {
		keep[v] = true
	}
	mapping := map[string]string{}
	total := 0
	for _, v := range oldVals {
		if keep[v] {
			continue
		}
		n, err := counts.Count(ctx, typeName, oldProp, v)
		if err != nil {
			return err
		}
		if n == 0 {
			continue
		}
		to, ok := mapped[[3]string{typeName, prop, v}]
		if !ok || !keep[to] {
			p.problems = append(p.problems, Problem{
				Code: "value_in_use", EntityType: typeName, Property: prop, Value: v, Count: n,
				Message: fmt.Sprintf("%d %s records have %s %q, which is removed. Choose where they go.", n, typeName, prop, v),
			})
			continue
		}
		mapping[v] = to
		total += n
		p.steps = append(p.steps, Step{Kind: "map_values", EntityType: typeName, Property: prop, From: v, To: to, Count: n})
	}
	if len(mapping) > 0 {
		p.spec = append(p.spec, datamigration.SpecStep{MapValues: &datamigration.MapValuesSpec{
			Entity: typeName, Property: prop, Mapping: mapping,
		}})
	}
	return nil
}

func checkRename(before, after *metamodel.Metamodel, r Rename) string {
	oldDef, ok := before.Entities[r.EntityType]
	if !ok {
		return fmt.Sprintf("entity type %q does not exist", r.EntityType)
	}
	newDef, ok := after.Entities[r.EntityType]
	switch {
	case !ok:
		return fmt.Sprintf("entity type %q is removed", r.EntityType)
	case r.From == r.To:
		return "a property cannot be renamed to itself"
	case !hasKey(oldDef.Properties, r.From):
		return fmt.Sprintf("%s has no property %q to rename", r.EntityType, r.From)
	case hasKey(newDef.Properties, r.From):
		return fmt.Sprintf("%s still has a property %q after the rename", r.EntityType, r.From)
	case !hasKey(newDef.Properties, r.To):
		return fmt.Sprintf("%s has no property %q after the rename", r.EntityType, r.To)
	case hasKey(oldDef.Properties, r.To):
		return fmt.Sprintf("%s already has a property %q", r.EntityType, r.To)
	}
	return ""
}

// relationValueProblems refuses removing an option from a choice list a
// relation property uses: the migration steps rewrite entities only, so
// relations holding the option would be left with a value nothing accepts.
func relationValueProblems(before, after *metamodel.Metamodel) []Problem {
	var out []Problem
	for _, relName := range sortedKeys(before.Relations) {
		rel, ok := after.Relations[relName]
		if !ok {
			continue
		}
		for _, propName := range sortedKeys(before.Relations[relName].Properties) {
			newPD, ok := rel.Properties[propName]
			if !ok {
				continue
			}
			keep := map[string]bool{}
			for _, v := range optionValues(after, newPD) {
				keep[v] = true
			}
			for _, v := range optionValues(before, before.Relations[relName].Properties[propName]) {
				if !keep[v] {
					out = append(out, Problem{
						Code: "unsupported_change", Property: propName, Value: v,
						Message: fmt.Sprintf("option %q is used by relation %s, and relation values cannot be migrated here.",
							v, relName),
					})
				}
			}
		}
	}
	return out
}

// optionValues is a property's choice list, inline or named.
func optionValues(m *metamodel.Metamodel, pd metamodel.PropertyDef) []string {
	if len(pd.Values) > 0 {
		return pd.Values
	}
	if t, ok := m.Types[pd.Type]; ok {
		return t.Values
	}
	return nil
}

// typeChanged reports a change in what kind of value a property holds. A
// switch between two choice lists is not one when every option survives;
// removed options are handled as values.
func typeChanged(before, after *metamodel.Metamodel, oldPD, newPD metamodel.PropertyDef) bool {
	if oldPD.List != newPD.List || oldPD.Format != newPD.Format {
		return true
	}
	oldEnum, newEnum := len(optionValues(before, oldPD)) > 0, len(optionValues(after, newPD)) > 0
	if oldEnum && newEnum {
		return false
	}
	if oldEnum != newEnum {
		return true
	}
	return baseType(before, oldPD.Type) != baseType(after, newPD.Type)
}

// baseType is the built-in type behind a property type. A named type with
// no values is a pattern-checked string.
func baseType(m *metamodel.Metamodel, t string) string {
	if _, ok := m.Types[t]; ok {
		return "string"
	}
	return t
}

func coercible(t string) bool {
	switch t {
	case "string", "integer", "boolean", "date", "datetime":
		return true
	}
	return false
}

// buildMigration turns a plan into a migration file, or nil when the change
// needs none.
func buildMigration(
	plan *migrationPlan, before, after *metamodel.Metamodel, title string, now time.Time,
) (*datamigration.Draft, error) {
	return datamigration.Build(datamigration.Spec{
		Description: title,
		From:        before.ShapeProjection(),
		To:          after.ShapeProjection(),
		Steps:       plan.spec,
	}, now)
}

// aclReferences returns a problem for each name the save removes that
// acl.yaml mentions. Role grants and client restrictions match types,
// properties and option values by name, so renaming one can silently widen
// access: a client restricted from `salary` could read `annual_salary`.
// The match is deliberately coarse (the name as a whole word anywhere in the
// file); a false alarm costs the user an edit to acl.yaml, a miss costs an
// access-control hole.
func aclReferences(aclText []byte, before, after *metamodel.Metamodel) []Problem {
	if len(aclText) == 0 {
		return nil
	}
	var out []Problem
	check := func(kind, name string) {
		re := regexp.MustCompile(`(^|[^A-Za-z0-9_-])` + regexp.QuoteMeta(name) + `($|[^A-Za-z0-9_-])`)
		if re.Match(aclText) {
			out = append(out, Problem{
				Code: "acl_reference", Value: name,
				Message: fmt.Sprintf("acl.yaml refers to %s %q, which this change removes or renames. "+
					"Update acl.yaml first.", kind, name),
			})
		}
	}
	for _, t := range sortedKeys(before.Entities) {
		newDef, ok := after.Entities[t]
		if !ok {
			check("entity type", t)
			continue
		}
		for _, prop := range sortedKeys(before.Entities[t].Properties) {
			if !hasKey(newDef.Properties, prop) {
				check("property", prop)
			}
		}
	}
	for _, r := range sortedKeys(before.Relations) {
		if !hasKey(after.Relations, r) {
			check("relation", r)
		}
	}
	removedValues := map[string]bool{}
	kept := declaredOptions(after)
	for v := range declaredOptions(before) {
		if !kept[v] {
			removedValues[v] = true
		}
	}
	for _, v := range sortedKeys(removedValues) {
		check("option", v)
	}
	return out
}

// declaredOptions is every option value the metamodel declares, on custom types
// and on properties alike.
func declaredOptions(m *metamodel.Metamodel) map[string]bool {
	vals := map[string]bool{}
	for _, ct := range m.Types {
		for _, v := range ct.Values {
			vals[v] = true
		}
	}
	for _, e := range m.Entities {
		for _, pd := range e.Properties {
			for _, v := range pd.Values {
				vals[v] = true
			}
		}
	}
	return vals
}

func hasKey[V any](m map[string]V, k string) bool {
	_, ok := m[k]
	return ok
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
