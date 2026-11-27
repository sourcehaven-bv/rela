package visibility

import (
	"errors"
	"fmt"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// A page filter on a property the caller does not see on every row
// (TKT-9BKVQ9I) is answered in the store, not refused and not judged in Go.
//
// The Go reading of a filter judges the REDACTED row: where the caller may
// not see property p, the row holds no p, so a comparison on p answers as
// it does for an unset value. The store holds the real value. So each part
// S of the filter that reads p alone is rewritten as
//
//	(V and S) or (not V and S0)
//
// where V is the row condition under which the caller's `visible:` grants
// show p, and S0 is the constant S takes on a row without p. The rewrite is
// exact, so the page and its total are the rows the Go reading keeps, and
// the store compares a hidden value only where its answer is replaced by
// S0.
//
// V comes from the caller's compiled field scope ([FieldScoper]), the one
// search applies and TestSearchFieldScope_AgreesWithFieldVerdicts pins to
// the redactor. A grant shape the store cannot express refuses the filter;
// nothing approximates V, so a refusal never becomes a wider answer. Every
// refusal is decided from the policy, the caller's roles and the filter's
// shape, never from a stored row.

// errFieldCondUnsupported reports a field grant whose row condition has no
// store form for this read.
var errFieldCondUnsupported = errors.New("a field grant's condition cannot be applied in the store")

// hiddenPropConds returns, for each property in hidden, the row condition
// under which fs shows it on the rows of entityType. scopeFaces is the set
// of faces a returned row can have (nil: any face).
func hiddenPropConds(
	fs *search.FieldScope, entityType string, hidden []string, scopeFaces []entity.Face,
) (map[string]store.Cond, error) {
	out := make(map[string]store.Cond, len(hidden))
	for _, p := range hidden {
		v, err := visibleCond(fs, entityType, p, scopeFaces)
		if err != nil {
			return nil, fmt.Errorf("property %q: %w", p, err)
		}
		out[p] = v
	}
	return out, nil
}

// visibleCond is the row condition under which fs shows prop: true when
// fs does not gate it, otherwise the OR of the conditions of the grants
// that list it, which is false when none does.
func visibleCond(fs *search.FieldScope, entityType, prop string, scopeFaces []entity.Face) (store.Cond, error) {
	if !fs.Gates(prop) {
		return store.AndCond(), nil
	}
	var arms []store.Cond
	for _, g := range fs.Grants {
		if !slices.Contains(g.Fields, prop) {
			continue
		}
		c, err := grantCond(g.When, entityType, scopeFaces)
		if err != nil {
			return store.Cond{}, err
		}
		if v, known := c.Constant(); known && v {
			return store.AndCond(), nil
		}
		arms = append(arms, c)
	}
	return store.OrCond(arms...), nil
}

// grantCond is the row condition of one grant: true for a nil When. A face
// set is decided against the faces a returned row can have: true when they
// all lie in it, false when none does, and refused otherwise, since a Cond
// has no face test.
func grantCond(when *store.GraphQuery, entityType string, scopeFaces []entity.Face) (store.Cond, error) {
	if when == nil {
		return store.AndCond(), nil
	}
	if when.EntityType != "" && when.EntityType != entityType {
		return store.Cond{}, fmt.Errorf("%w: it is for type %q", errFieldCondUnsupported, when.EntityType)
	}
	q := *when
	if len(q.FaceIn) > 0 {
		switch {
		case scopeFaces != nil && !slices.ContainsFunc(scopeFaces, func(f entity.Face) bool {
			return slices.Contains(q.FaceIn, f)
		}):
			return store.OrCond(), nil
		case scopeFaces != nil && !slices.ContainsFunc(scopeFaces, func(f entity.Face) bool {
			return !slices.Contains(q.FaceIn, f)
		}):
			q.FaceIn = nil
		default:
			return store.Cond{}, fmt.Errorf("%w: it applies on some faces only", errFieldCondUnsupported)
		}
	}
	c, err := q.RowCond()
	if err != nil {
		return store.Cond{}, fmt.Errorf("%w: %w", errFieldCondUnsupported, err)
	}
	return c, nil
}

// guardHiddenProps rewrites a gated page filter, props ANDed with where,
// so that every comparison on a property in conds reads it only where
// conds shows it (see the file comment). Props on such a property move
// into the returned where; the others stay in the returned props.
func guardHiddenProps(
	props []store.PropPredicate, where *store.Cond, conds map[string]store.Cond,
) ([]store.PropPredicate, *store.Cond, error) {
	var (
		kept     []store.PropPredicate
		byHidden = map[string][]store.Cond{}
		order    []string
	)
	for _, p := range props {
		if _, hidden := conds[p.Property]; !hidden {
			kept = append(kept, p)
			continue
		}
		if _, seen := byHidden[p.Property]; !seen {
			order = append(order, p.Property)
		}
		byHidden[p.Property] = append(byHidden[p.Property], store.PropLeaf(p))
	}
	var args []store.Cond
	for _, name := range order {
		g, err := guard(store.AndCond(byHidden[name]...), name, conds[name])
		if err != nil {
			return nil, nil, err
		}
		args = append(args, g)
	}
	if where != nil {
		w, err := guardTree(*where, conds)
		if err != nil {
			return nil, nil, err
		}
		args = append(args, w)
	}
	switch len(args) {
	case 0:
		return kept, nil, nil
	case 1:
		return kept, &args[0], nil
	default:
		w := store.AndCond(args...)
		return kept, &w, nil
	}
}

// guardTree rewrites c. The largest subtree that reads one hidden property
// and no relation is guarded as a whole, so V appears once per such
// subtree rather than once per comparison: `entity.n == 3` lowers to two
// comparisons, and V can carry several relation leaves of its own.
func guardTree(c store.Cond, conds map[string]store.Cond) (store.Cond, error) {
	if p, only := soleProperty(c); only {
		v, hidden := conds[p]
		if !hidden {
			return c, nil
		}
		return guard(c, p, v)
	}
	if len(c.Args) == 0 {
		return c, nil
	}
	out := store.Cond{Op: c.Op, Args: make([]store.Cond, 0, len(c.Args))}
	for _, a := range c.Args {
		ga, err := guardTree(a, conds)
		if err != nil {
			return store.Cond{}, err
		}
		out.Args = append(out.Args, ga)
	}
	return out, nil
}

// soleProperty reports the one property c compares, when c has at least
// one comparison, all on that property, and no relation leaf.
func soleProperty(c store.Cond) (string, bool) {
	prop, only, found := "", true, false
	c.Walk(func(n store.Cond) bool {
		switch n.Op {
		case store.CondRelated:
			only = false
		case store.CondProp:
			if found && n.Prop.Property != prop {
				only = false
			}
			prop, found = n.Prop.Property, true
		default:
		}
		return only
	})
	return prop, only && found
}

// guard rewrites s, which compares prop alone, as `(v and s) or (not v
// and s0)` with s0 the constant s takes on a row without prop, folded.
func guard(s store.Cond, prop string, v store.Cond) (store.Cond, error) {
	s0, err := absentValue(s)
	if err != nil {
		return store.Cond{}, fmt.Errorf("property %q: %w", prop, err)
	}
	return guardWith(s, s0, v), nil
}

// guardWith is `(v and s) or (not v and s0)`, folded.
func guardWith(s store.Cond, s0 bool, v store.Cond) store.Cond {
	if known, isConst := v.Constant(); isConst {
		if known {
			return s
		}
		return constCond(s0)
	}
	if s0 {
		return store.OrCond(store.NotCond(v), s)
	}
	return store.AndCond(v, s)
}

// guardFilledTree is [guardTree] for a condition whose comparisons read a
// declared default where the row holds no value. fill rewrites a subtree
// for the default. A hidden property reads as unset, never as its default,
// so s0 is taken from the subtree before fill.
func guardFilledTree(
	c store.Cond, conds map[string]store.Cond, fill func(store.Cond) (store.Cond, error),
) (store.Cond, error) {
	if p, only := soleProperty(c); only {
		s, err := fill(c)
		if err != nil {
			return store.Cond{}, err
		}
		v, hidden := conds[p]
		if !hidden {
			return s, nil
		}
		s0, err := absentValue(c)
		if err != nil {
			return store.Cond{}, fmt.Errorf("property %q: %w", p, err)
		}
		return guardWith(s, s0, v), nil
	}
	if len(c.Args) == 0 {
		return fill(c)
	}
	out := store.Cond{Op: c.Op, Args: make([]store.Cond, 0, len(c.Args))}
	for _, a := range c.Args {
		ga, err := guardFilledTree(a, conds, fill)
		if err != nil {
			return store.Cond{}, err
		}
		out.Args = append(out.Args, ga)
	}
	return out, nil
}

func constCond(v bool) store.Cond {
	if v {
		return store.AndCond()
	}
	return store.OrCond()
}

// absentValue evaluates c on a row without the property its comparisons
// read.
func absentValue(c store.Cond) (bool, error) {
	switch c.Op {
	case store.CondProp:
		return absentLeaf(c.Prop)
	case store.CondNot:
		if len(c.Args) != 1 {
			return false, fmt.Errorf("%w: a malformed not", errFieldCondUnsupported)
		}
		v, err := absentValue(c.Args[0])
		return !v, err
	case store.CondAnd, store.CondOr:
		decider := c.Op == store.CondOr
		for _, a := range c.Args {
			v, err := absentValue(a)
			if err != nil {
				return false, err
			}
			if v == decider {
				return decider, nil
			}
		}
		return !decider, nil
	default:
		return false, fmt.Errorf("%w: a relation in a guarded comparison", errFieldCondUnsupported)
	}
}

// absentLeaf is the answer of p on a row without p.Property, for the
// comparisons queryplan.LowerCondition emits: an equality or membership on
// a non-empty value and an ordered comparison do not hold, and
// [store.PropNotEqualOrEmpty] ("not v, or unset") does. Any other
// comparison is refused rather than guessed: the rewrite is only exact
// where this answer is.
func absentLeaf(p store.PropPredicate) (bool, error) {
	switch p.Op {
	case store.PropEqual:
		if p.Value == "" || p.Kind != store.OrderText {
			break
		}
		return false, nil
	case store.PropNotEqualOrEmpty:
		if p.Kind != store.OrderText {
			break
		}
		return true, nil
	case store.PropLess, store.PropLessEqual, store.PropGreater, store.PropGreaterEqual:
		if p.Kind != store.OrderString && p.Kind != store.OrderInteger {
			break
		}
		return false, nil
	default:
	}
	return false, fmt.Errorf("%w: comparison on a property hidden on some rows", errFieldCondUnsupported)
}
