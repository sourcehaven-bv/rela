package metamodel

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// RollupFunc names the aggregate a [RollupDef] computes.
type RollupFunc string

// The roll-up functions. Exactly one is declared per roll-up.
const (
	RollupCount  RollupFunc = "count"
	RollupExists RollupFunc = "exists"
	RollupMin    RollupFunc = "min"
	RollupMax    RollupFunc = "max"
	RollupSum    RollupFunc = "sum"
	RollupLatest RollupFunc = "latest_by"
)

// RollupDef declares a property whose value aggregates the entities related
// through one relation, or through a path of up to [MaxRollupPath] relations
// (TKT-6I2OPG, TKT-LTSNHD):
//
//	bewijs_aantal:
//	  type: integer
//	  rollup: { via: evidencedBy, count: true, where: { status: geldig } }
//	laatste_testresultaat:
//	  type: testresultaat
//	  rollup: { via: testedBy, pick: resultaat, latest_by: datum }
//	gedekt:
//	  type: boolean
//	  rollup: { via: [verwijst_naar, wordt_geimplementeerd_door], exists: true }
//
// The value is computed when the entity is read, with the reader's access:
// a related entity the reader cannot read does not count, an entity on the
// path the reader cannot read breaks the path, and a related entity on which
// the reader cannot see a property the roll-up reads is left out. A related
// entity reached by several paths counts once. A path that leads back to the
// source type can reach the source itself, and then counts it. It is never stored, so it
// cannot be written. A list filters and sorts on it by computing it for
// every scoped row first.
//
// The loader checks the declaration's shape. The checks that need the
// relation and the related type (does `via` exist, is `pick` declared there,
// do the types agree, is there a chain) run in internal/rollup, at the same
// point computed expressions are compiled.
type RollupDef struct {
	// Via is the path from the entity to the related entities: one relation
	// name, or a list of up to [MaxRollupPath]. A canonical name walks
	// FROM -> TO, the relation's inverse id walks TO -> FROM, as in
	// related().
	Via StringOrSlice `yaml:"via"`
	// Type picks the related type when the last relation's far side names
	// several.
	Type string `yaml:"type,omitempty"`
	// Where keeps the related entities whose properties equal these values.
	// It filters the far end of the path only.
	Where map[string]string `yaml:"where,omitempty"`

	Count    bool   `yaml:"count,omitempty"`
	Exists   bool   `yaml:"exists,omitempty"`
	Min      string `yaml:"min,omitempty"`
	Max      string `yaml:"max,omitempty"`
	Sum      string `yaml:"sum,omitempty"`
	Pick     string `yaml:"pick,omitempty"`
	LatestBy string `yaml:"latest_by,omitempty"`
}

// MaxRollupPath is the most relations a roll-up's via may name. Each hop is
// one more join in the statement that computes the roll-up for a page, and
// the number of (entity, related entity) pairs it carries grows with the
// fan-out of every hop. Three covers the GRC paths (requirement -> control
// -> measure -> evidence); a longer chain is better modeled with a direct
// relation.
const MaxRollupPath = 3

// RollupNames returns the roll-up properties of entityType, sorted. The
// entity read surfaces (data-entry, MCP show_entity, CLI show and list) drop
// these names from a stored row before they add the computed values, so they
// never serve a value stored under a roll-up name. Other readers (Lua,
// search hits) see the stored row as it is.
func (m *Metamodel) RollupNames(entityType string) []string {
	def, ok := m.GetEntityDef(entityType)
	if !ok {
		return nil
	}
	var out []string
	for name, pd := range def.Properties {
		if pd.Rollup != nil {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

// Func returns the declared function and the related property it reads
// ("" for count and exists). For latest_by the property is the PICK; the
// ordering property is [RollupDef.LatestBy]. ok is false unless exactly one
// function is declared.
func (r *RollupDef) Func() (fn RollupFunc, property string, ok bool) {
	var found []RollupFunc
	if r.Count {
		found, fn = append(found, RollupCount), RollupCount
	}
	if r.Exists {
		found, fn = append(found, RollupExists), RollupExists
	}
	for _, c := range []struct {
		fn   RollupFunc
		prop string
	}{{RollupMin, r.Min}, {RollupMax, r.Max}, {RollupSum, r.Sum}, {RollupLatest, r.LatestBy}} {
		if strings.TrimSpace(c.prop) != "" {
			found, fn, property = append(found, c.fn), c.fn, c.prop
		}
	}
	if fn == RollupLatest {
		property = r.Pick
	}
	return fn, property, len(found) == 1
}

// validateRollupShape checks what the loader can check without the related
// type: exactly one function, latest_by with pick and pick only with
// latest_by, the declared type of count and exists, and no option a stored
// property has.
func validateRollupShape(schemaName, propName string, pd PropertyDef, allowRollup bool) []string {
	r := pd.Rollup
	if r == nil {
		return nil
	}
	at := fmt.Sprintf("%s: property %q: rollup", schemaName, propName)
	var errs []string
	if !allowRollup {
		errs = append(errs, at+" is only supported on entity properties")
	}
	switch {
	case len(r.Via) == 0, len(r.Via) == 1 && strings.TrimSpace(r.Via[0]) == "":
		// A blank scalar via is a missing via, not an empty path element.
		errs = append(errs, at+" needs via: the relation to aggregate over")
	case len(r.Via) > MaxRollupPath:
		errs = append(errs, fmt.Sprintf("%s: via names %d relations; a roll-up follows at most %d",
			at, len(r.Via), MaxRollupPath))
	default:
		for i, name := range r.Via {
			if strings.TrimSpace(name) == "" {
				errs = append(errs, fmt.Sprintf("%s: via[%d] is empty", at, i))
			}
		}
	}
	fn, _, ok := r.Func()
	if !ok {
		errs = append(errs, at+" must declare exactly one of count, exists, min, max, sum or latest_by")
	}
	latestBy, pick := strings.TrimSpace(r.LatestBy) != "", strings.TrimSpace(r.Pick) != ""
	if latestBy && !pick {
		errs = append(errs, at+": latest_by needs pick: the property to read from the latest entity")
	}
	if pick && !latestBy {
		errs = append(errs, at+": pick is only valid with latest_by")
	}
	for _, name := range slices.Sorted(maps.Keys(r.Where)) {
		if strings.TrimSpace(r.Where[name]) == "" {
			errs = append(errs, fmt.Sprintf("%s: where %q has an empty value", at, name))
		}
	}
	if ok {
		switch {
		case fn == RollupCount && pd.Type != PropertyTypeInteger:
			errs = append(errs, fmt.Sprintf("%s: count needs type integer, not %q", at, pd.Type))
		case fn == RollupExists && pd.Type != PropertyTypeBoolean:
			errs = append(errs, fmt.Sprintf("%s: exists needs type boolean, not %q", at, pd.Type))
		}
	}
	for _, c := range []struct {
		set  bool
		name string
	}{
		{pd.Computed != "", "computed"},
		{pd.List, "list"},
		{pd.Required, "required"},
		{pd.Default != "", "default"},
		{pd.Unique, "unique"},
		{pd.Type == PropertyTypeFile, "a file type"},
	} {
		if c.set {
			errs = append(errs, fmt.Sprintf("%s cannot be combined with %s: a roll-up is computed on read and never stored",
				at, c.name))
		}
	}
	return errs
}
