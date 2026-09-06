package predicate

import "sort"

// PrefilterSpec names the variables and host functions
// [Program.ConstEqualities] recognizes as request-constant comparisons.
//
// The predicate engine knows nothing about entities or users; the caller
// tells it which record is the row under test, which record is constant
// for the request, and which of its own host functions are equality
// sugar over that constant.
type PrefilterSpec struct {
	// RecordVar is the record whose attributes are being constrained (the
	// entity under test). Required.
	RecordVar string

	// ConstVar is the record whose fields are constant for one evaluation
	// (the request's current_user). Optional: empty considers string
	// literals only.
	ConstVar string

	// ConstFuncs maps a host-function name to the field of ConstVar its
	// single argument is compared against. Listing a function here is the
	// caller's ASSERTION about its semantics, which the engine cannot
	// verify: `f(RecordVar.attr)` must mean exactly `RecordVar.attr ==
	// ConstVar.<field>` when attr is a string, and "some element of
	// RecordVar.attr equals ConstVar.<field>" when attr is a list of
	// strings. The argument's static type decides which reading applies;
	// a call whose argument is anything but a direct attribute of
	// RecordVar is ignored.
	ConstFuncs map[string]string
}

// ConstEqualities returns the attributes of the spec's RecordVar that the
// program constrains, at the TOP LEVEL of an AND-chain, to be equal to a
// value that is constant for one evaluation — a string literal, a field of
// ConstVar, or a ConstFuncs call over the attribute.
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
// string: a literal, or a string-typed field of ConstVar (whose value the
// caller supplies at bind time and is therefore the same for every row).
// For a ConstVar field the returned Value is empty and FromVar names the
// field, because the program does not know the identity — the caller
// does.
//
// A ConstFuncs call over a LIST attribute is reported with List set: the
// constraint is membership, not scalar equality, and a caller lowering it
// must pick a store operation with that meaning.
//
// The result is sorted by attribute name for deterministic output. A
// program that constrains the same attribute twice reports it once, with
// the FIRST binding encountered — a caller must treat the result as a
// subset of the program's constraints, never as the whole of them.
func (p *Program) ConstEqualities(spec PrefilterSpec) []ConstEquality {
	if p == nil || spec.RecordVar == "" {
		return nil
	}
	seen := map[string]ConstEquality{}
	collectConstEqualities(p.root, spec, seen)
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
	// Attribute is the field of RecordVar being constrained.
	Attribute string

	// Value is the string literal the attribute must equal. Meaningful
	// only when FromVar is empty.
	Value string

	// FromVar names the field of ConstVar supplying the value. Empty for
	// a literal comparison.
	FromVar string

	// List reports that Attribute is a list of strings and the constraint
	// is MEMBERSHIP — some element equals the value — rather than scalar
	// equality. Only a ConstFuncs call produces it: the language has no
	// list equality operator.
	List bool
}

// collectConstEqualities walks the top-level AND spine only. It
// deliberately does not descend into `or`, `not`, or any other node:
// see the soundness argument on [Program.ConstEqualities].
func collectConstEqualities(n node, spec PrefilterSpec, out map[string]ConstEquality) {
	var (
		attr string
		eq   ConstEquality
		ok   bool
	)
	switch x := n.(type) {
	case *logicalNode:
		if x.op != "and" {
			return
		}
		collectConstEqualities(x.lhs, spec, out)
		collectConstEqualities(x.rhs, spec, out)
		return
	case *relationalNode:
		if x.op != "==" {
			return
		}
		attr, eq, ok = constEqualityFrom(x.lhs, x.rhs, spec)
		if !ok {
			attr, eq, ok = constEqualityFrom(x.rhs, x.lhs, spec)
		}
	case *callNode:
		attr, eq, ok = constFuncEquality(x, spec)
	}
	if !ok {
		return
	}
	if _, dup := out[attr]; !dup {
		out[attr] = eq
	}
}

// constEqualityFrom matches `RecordVar.attr == <const>` in one operand
// order, returning the attribute and the constant side.
func constEqualityFrom(lhs, rhs node, spec PrefilterSpec) (string, ConstEquality, bool) {
	attr, ok := recordAttr(lhs, spec.RecordVar)
	if !ok {
		return "", ConstEquality{}, false
	}
	// Only string-valued equalities are reported. A store pre-filter
	// compares by string form, so a typed comparison (int, date, bool)
	// could disagree with the Go pass about the same row — exactly the
	// widening the belt-and-braces design exists to prevent.
	if t := attrTypeOf(lhs); t != nil && !t.equalsType(StringType) {
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
		if spec.ConstVar == "" {
			return "", ConstEquality{}, false
		}
		field, isVar := recordAttr(c, spec.ConstVar)
		if !isVar || !c.typ.equalsType(StringType) {
			return "", ConstEquality{}, false
		}
		return attr, ConstEquality{Attribute: attr, FromVar: field}, true
	}
	return "", ConstEquality{}, false
}

// constFuncEquality matches `f(RecordVar.attr)` for an f the spec lists,
// reading it as equality (string attr) or membership (list-of-string
// attr) against the ConstVar field the spec maps f to.
func constFuncEquality(call *callNode, spec PrefilterSpec) (string, ConstEquality, bool) {
	field, listed := spec.ConstFuncs[call.name]
	if !listed || spec.ConstVar == "" || len(call.args) != 1 {
		return "", ConstEquality{}, false
	}
	attr, ok := recordAttr(call.args[0], spec.RecordVar)
	if !ok {
		return "", ConstEquality{}, false
	}
	switch t := attrTypeOf(call.args[0]); {
	case t == nil:
		return "", ConstEquality{}, false
	case t.equalsType(StringType):
		return attr, ConstEquality{Attribute: attr, FromVar: field}, true
	case t.equalsType(ListType{Elem: StringType}):
		return attr, ConstEquality{Attribute: attr, FromVar: field, List: true}, true
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
