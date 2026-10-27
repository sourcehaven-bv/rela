// Package rollup compiles the `rollup:` properties of a metamodel
// (TKT-6I2OPG) into [Column] values: the relation to walk, the related type, the
// aggregate and the related properties it reads.
//
// A roll-up is computed when an entity is read, with the reader's access, and
// never stored. This package only checks and describes the declarations; the
// gated computation lives in internal/visibility, and the aggregate itself in
// the store ([store.Rollup]). The loader in internal/metamodel checks the shape
// of one declaration; [Compile] checks what needs the related type.
package rollup

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Column is one compiled roll-up property.
type Column struct {
	// Property is the roll-up property on the source type.
	Property string
	// Func is the declared function.
	Func metamodel.RollupFunc
	// Relation is the canonical relation walked; SourceIsTo says the edge
	// runs from the related entity to the source (an inverse via).
	Relation   string
	SourceIsTo bool
	// Target is the related entity type.
	Target string
	// Where are the equalities a related entity must meet, sorted by
	// property.
	Where []store.PropPredicate
	// Value is the related property aggregated (min, max, sum) or picked
	// (latest_by); By orders latest_by. Numeric compares Value as a number.
	Value   string
	By      string
	Numeric bool
	// Reads are the related properties whose value decides whether a row
	// counts: the where keys, the min/max/sum property and latest_by. The
	// pick is not among them; a hidden pick withholds the value only.
	Reads []string
}

// StoreFunc maps the declared function onto the store's aggregate. exists
// is a count read as "more than zero".
func (c Column) StoreFunc() store.RollupFunc {
	switch c.Func {
	case metamodel.RollupMin:
		return store.RollupMin
	case metamodel.RollupMax:
		return store.RollupMax
	case metamodel.RollupSum:
		return store.RollupSum
	case metamodel.RollupLatest:
		return store.RollupLatest
	case metamodel.RollupCount, metamodel.RollupExists:
	}
	return store.RollupCount
}

// ValueOf turns a store result into the property's value, or reports false
// when the roll-up has none (no related row supplied one). count and exists
// always have a value.
func (c Column) ValueOf(res store.RollupResult) (any, bool) {
	switch c.Func {
	case metamodel.RollupCount:
		return res.Count, true
	case metamodel.RollupExists:
		return res.Count > 0, true
	case metamodel.RollupMin, metamodel.RollupMax, metamodel.RollupSum, metamodel.RollupLatest:
	}
	return res.Value, res.Value != nil
}

// Set is the compiled roll-ups of a metamodel.
//
// Nil: accepted by every method; a nil Set has no roll-ups.
type Set struct {
	byType map[string][]Column
}

// For returns the roll-ups of an entity type, sorted by property. The
// slice is shared; callers must not change it.
func (s *Set) For(entityType string) []Column {
	if s == nil {
		return nil
	}
	return s.byType[entityType]
}

// Is reports whether property is a roll-up of entityType.
func Is(meta *metamodel.Metamodel, entityType, property string) bool {
	if meta == nil {
		return false
	}
	def, ok := meta.Entities[entityType]
	if !ok {
		return false
	}
	pd, ok := def.Properties[property]
	return ok && pd.Rollup != nil
}

// Compile checks every roll-up of meta against the related type and returns
// the compiled set, or one error listing every problem.
//
// Nil: meta rejected — a required input from the wiring site.
func Compile(meta *metamodel.Metamodel) (*Set, error) {
	if meta == nil {
		return nil, errors.New("rollup: Compile: nil metamodel")
	}
	set := &Set{byType: map[string][]Column{}}
	var problems []string
	for _, typeName := range slices.Sorted(maps.Keys(meta.Entities)) {
		def := meta.Entities[typeName]
		for _, name := range slices.Sorted(maps.Keys(def.Properties)) {
			pd := def.Properties[name]
			if pd.Rollup == nil {
				continue
			}
			col, errs := compileColumn(meta, typeName, name, pd)
			if len(errs) > 0 {
				problems = append(problems, errs...)
				continue
			}
			set.byType[typeName] = append(set.byType[typeName], col)
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return nil, fmt.Errorf("rollup: invalid roll-up properties:\n  %s", strings.Join(problems, "\n  "))
	}
	return set, nil
}

func compileColumn(
	meta *metamodel.Metamodel, typeName, name string, pd metamodel.PropertyDef,
) (col Column, problems []string) {
	r := pd.Rollup
	at := fmt.Sprintf("entity %q property %q: rollup", typeName, name)
	fn, prop, ok := r.Func()
	if !ok {
		// The loader reports the shape; nothing more to check.
		return Column{}, []string{at + " must declare exactly one function"}
	}
	via := strings.TrimSpace(r.Via)
	hops, err := predicatefns.ResolveTraversal(meta, typeName, predicate.TraversalSpec{
		Subject: "entity", Path: []string{via}, EntityType: strings.TrimSpace(r.Type),
	})
	if err != nil {
		return Column{}, []string{fmt.Sprintf("%s via %q: %s", at, via, strings.TrimPrefix(err.Error(), "related: "))}
	}
	hop := hops[0]
	if metamodel.IsContentScoped(meta, hop.Relation) {
		return Column{}, []string{fmt.Sprintf(
			"%s via %q: relation %q is content-scoped; a roll-up follows identity-scoped relations only",
			at, via, hop.Relation)}
	}
	target, ok := meta.Entities[hop.Target]
	if !ok {
		return Column{}, []string{fmt.Sprintf("%s via %q: related type %q is not an entity type", at, via, hop.Target)}
	}
	col = Column{
		Property: name, Func: fn, Relation: hop.Relation, SourceIsTo: hop.Incoming, Target: hop.Target,
	}
	related := func(role, p string) (metamodel.PropertyDef, bool) {
		rp, ok := target.Properties[p]
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("%s: %s %q is not a property of %q", at, role, p, hop.Target))
		case rp.Rollup != nil:
			problems = append(problems, fmt.Sprintf(
				"%s: %s %q on %q is itself a roll-up; roll-ups cannot be chained", at, role, p, hop.Target))
		case rp.List:
			problems = append(problems, fmt.Sprintf("%s: %s %q on %q is a list", at, role, p, hop.Target))
		default:
			return rp, true
		}
		return metamodel.PropertyDef{}, false
	}

	for _, key := range slices.Sorted(maps.Keys(r.Where)) {
		value := strings.TrimSpace(r.Where[key])
		rp, ok := related("where", key)
		if !ok {
			continue
		}
		pred, problem := wherePredicate(meta, key, value, rp)
		if problem != "" {
			problems = append(problems, fmt.Sprintf("%s: where %q: %s", at, key, problem))
			continue
		}
		col.Where = append(col.Where, pred)
		col.Reads = append(col.Reads, key)
	}

	compileFunc(&col, fn, strings.TrimSpace(prop), r, pd, related, &problems, at)
	slices.Sort(col.Reads)
	col.Reads = slices.Compact(col.Reads)
	return col, problems
}

// compileFunc fills col's value, order and reads for its function fn over
// the related property prop, adding each problem found to problems.
func compileFunc(
	col *Column, fn metamodel.RollupFunc, prop string, r *metamodel.RollupDef, pd metamodel.PropertyDef,
	related func(role, p string) (metamodel.PropertyDef, bool), problems *[]string, at string,
) {
	switch fn {
	case metamodel.RollupCount, metamodel.RollupExists:
	case metamodel.RollupMin, metamodel.RollupMax:
		if rp, ok := related(string(fn), prop); ok {
			switch {
			case !slices.Contains([]string{
				metamodel.PropertyTypeInteger, metamodel.PropertyTypeDate, metamodel.PropertyTypeDatetime,
			}, rp.Type):
				*problems = append(*problems, fmt.Sprintf(
					"%s: %s %q has type %q; min and max need integer, date or datetime", at, fn, prop, rp.Type))
			case rp.Type != pd.Type:
				*problems = append(*problems, fmt.Sprintf(
					"%s: %s %q has type %q, but the roll-up is declared %q", at, fn, prop, rp.Type, pd.Type))
			}
		}
		col.Value, col.Numeric = prop, pd.Type == metamodel.PropertyTypeInteger
		col.Reads = append(col.Reads, prop)
	case metamodel.RollupSum:
		if rp, ok := related("sum", prop); ok && rp.Type != metamodel.PropertyTypeInteger {
			*problems = append(*problems, fmt.Sprintf("%s: sum %q has type %q; sum needs integer", at, prop, rp.Type))
		}
		if pd.Type != metamodel.PropertyTypeInteger {
			*problems = append(*problems, fmt.Sprintf("%s: sum needs type integer, not %q", at, pd.Type))
		}
		col.Value, col.Numeric = prop, true
		col.Reads = append(col.Reads, prop)
	case metamodel.RollupLatest:
		by := strings.TrimSpace(r.LatestBy)
		if rp, ok := related("latest_by", by); ok &&
			rp.Type != metamodel.PropertyTypeDate && rp.Type != metamodel.PropertyTypeDatetime {

			*problems = append(*problems, fmt.Sprintf(
				"%s: latest_by %q has type %q; it needs date or datetime", at, by, rp.Type))
		}
		if rp, ok := related("pick", prop); ok {
			if problem := sameType(pd, rp); problem != "" {
				*problems = append(*problems, fmt.Sprintf("%s: pick %q %s", at, prop, problem))
			}
		}
		col.Value, col.By = prop, by
		col.Reads = append(col.Reads, by)
	}
}

// wherePredicate is the store equality for one where entry, or why the
// value cannot equal the property.
func wherePredicate(
	meta *metamodel.Metamodel, key, value string, rp metamodel.PropertyDef,
) (pred store.PropPredicate, problem string) {
	pred = store.PropPredicate{Property: key, Op: store.PropEqual, Value: value}
	switch {
	case rp.Type == metamodel.PropertyTypeBoolean:
		if value != "true" && value != "false" {
			return pred, fmt.Sprintf("%q is not true or false", value)
		}
	case rp.Type == metamodel.PropertyTypeInteger:
		if _, err := strconv.ParseInt(value, 10, 64); err != nil {
			return pred, fmt.Sprintf("%q is not an integer", value)
		}
	case metamodel.StringShaped(meta, rp):
		if values := enumValues(meta, rp); values != nil && !slices.Contains(values, value) {
			return pred, fmt.Sprintf("%q is not one of %s", value, strings.Join(values, ", "))
		}
		pred.Scalar = true
	default:
		return pred, fmt.Sprintf("type %q cannot be compared in a where", rp.Type)
	}
	return pred, ""
}

// enumValues is the value set of an enum-shaped property, or nil.
func enumValues(meta *metamodel.Metamodel, pd metamodel.PropertyDef) []string {
	if len(pd.Values) > 0 {
		return pd.Values
	}
	if ct, ok := meta.Types[pd.Type]; ok && len(ct.Values) > 0 {
		return ct.Values
	}
	return nil
}

// sameType reports why a picked property's type differs from the roll-up's
// declared type, or "".
func sameType(rollup, picked metamodel.PropertyDef) string {
	if picked.Type != rollup.Type {
		return fmt.Sprintf("has type %q, but the roll-up is declared %q", picked.Type, rollup.Type)
	}
	if picked.Type == metamodel.PropertyTypeEnum && !slices.Equal(slices.Sorted(slices.Values(picked.Values)),
		slices.Sorted(slices.Values(rollup.Values))) {

		return "is an enum with other values than the roll-up"
	}
	if picked.Type == metamodel.PropertyTypeFile || picked.Type == metamodel.PropertyTypeExternalRef {
		return fmt.Sprintf("has type %q, which a roll-up cannot pick", picked.Type)
	}
	return ""
}
