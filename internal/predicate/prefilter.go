package predicate

import "sort"

// ConstEqualities returns the attributes of recordVar that the program
// constrains, at the TOP LEVEL of an AND-chain, to be equal to a value
// that is constant for one evaluation — either a string literal or a
// field of constVar.
//
// # What it is for
//
// A store can pre-filter rows by an indexed equality far more cheaply
// than the Go pass can reject them, but only if the comparison holds for
// EVERY row the authoritative pass would keep. This reports exactly the
// comparisons for which that is true, so a caller can lower them to a
// backend predicate (see internal/queryplan) while still running the
// full program in Go.
//
// # Why the shape is so restricted
//
// A pre-filter is sound only if it can never remove a row the
// authoritative pass would keep, which forces three restrictions:
//
//   - Top-level AND only. Under an `or`, either side may be false while
//     the program is still true, so pushing one branch would drop rows
//     the program accepts. `not` inverts the sense entirely.
//   - Equality only. Ordered and inequality comparisons have
//     backend-vs-Go semantics that differ on typed and missing values;
//     the store compares by string form, the Go pass by declared type.
//   - Constant right-hand side. A comparison against another entity
//     field varies per row and is not a filter the store can bind.
//
// Values are returned only for comparisons whose other side is a plain
// string: a literal, or a string-typed field of constVar (the request's
// current_user record, whose value the caller supplies at bind time and
// is therefore the same for every row). For a constVar field the
// returned Value is empty and FromVar names the field, because the
// program does not know the identity — the caller does.
//
// Pass "" for constVar to consider literals only.
//
// The result is sorted by attribute name for deterministic output. A
// program that constrains the same attribute twice reports it once, with
// the FIRST binding encountered — a caller must treat the result as a
// subset of the program's constraints, never as the whole of them.
func (p *Program) ConstEqualities(recordVar, constVar string) []ConstEquality {
	if p == nil || recordVar == "" {
		return nil
	}
	seen := map[string]ConstEquality{}
	collectConstEqualities(p.root, recordVar, constVar, seen)
	if len(seen) == 0 {
		return nil
	}
	out := make([]ConstEquality, 0, len(seen))
	for _, eq := range seen {
		out = append(out, eq)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Attribute < out[j].Attribute })
	return out
}

// ConstEquality is one pushable equality reported by
// [Program.ConstEqualities].
//
// Exactly one of Value and FromVar is meaningful: a literal comparison
// sets Value, and a comparison against the constant record's field sets
// FromVar (leaving Value empty for the caller to fill in).
type ConstEquality struct {
	// Attribute is the field of recordVar being constrained.
	Attribute string

	// Value is the string literal the attribute must equal. Meaningful
	// only when FromVar is empty.
	Value string

	// FromVar names the field of constVar supplying the value. Empty for
	// a literal comparison.
	FromVar string
}

// collectConstEqualities walks the top-level AND spine only. It
// deliberately does not descend into `or`, `not`, or any other node:
// see the soundness argument on [Program.ConstEqualities].
func collectConstEqualities(n node, recordVar, constVar string, out map[string]ConstEquality) {
	switch x := n.(type) {
	case *logicalNode:
		if x.op != "and" {
			return
		}
		collectConstEqualities(x.lhs, recordVar, constVar, out)
		collectConstEqualities(x.rhs, recordVar, constVar, out)
	case *relationalNode:
		if x.op != "==" {
			return
		}
		attr, eq, ok := constEqualityFrom(x.lhs, x.rhs, recordVar, constVar)
		if !ok {
			attr, eq, ok = constEqualityFrom(x.rhs, x.lhs, recordVar, constVar)
		}
		if !ok {
			return
		}
		if _, dup := out[attr]; !dup {
			out[attr] = eq
		}
	}
}

// constEqualityFrom matches `recordVar.attr == <const>` in one operand
// order, returning the attribute and the constant side.
func constEqualityFrom(
	lhs, rhs node, recordVar, constVar string,
) (string, ConstEquality, bool) {
	attr, ok := recordAttr(lhs, recordVar)
	if !ok {
		return "", ConstEquality{}, false
	}
	// Only string-valued equalities are reported. A store pre-filter
	// compares by string form, so a typed comparison (int, date, bool)
	// could disagree with the Go pass about the same row — exactly the
	// widening the belt-and-braces design exists to prevent.
	if attr := attrTypeOf(lhs); attr != nil && !attr.equalsType(StringType) {
		return "", ConstEquality{}, false
	}
	switch c := rhs.(type) {
	case *constNode:
		s, isStr := c.v.(String)
		if !isStr {
			return "", ConstEquality{}, false
		}
		return attr, ConstEquality{Attribute: attr, Value: s.String()}, true
	case *attrNode:
		if constVar == "" {
			return "", ConstEquality{}, false
		}
		field, isVar := recordAttr(c, constVar)
		if !isVar || !c.typ.equalsType(StringType) {
			return "", ConstEquality{}, false
		}
		return attr, ConstEquality{Attribute: attr, FromVar: field}, true
	}
	return "", ConstEquality{}, false
}

// recordAttr reports the field name when n is a direct attribute access
// on the named record variable.
func recordAttr(n node, recordVar string) (string, bool) {
	a, ok := n.(*attrNode)
	if !ok {
		return "", false
	}
	v, ok := a.obj.(*varNode)
	if !ok || v.name != recordVar {
		return "", false
	}
	return a.name, true
}

// attrTypeOf returns the static type of an attribute node, or nil.
func attrTypeOf(n node) Type {
	if a, ok := n.(*attrNode); ok {
		return a.typ
	}
	return nil
}
