package predicate

// OrderedComparison is one leaf of [Program.OrderedConjunction]:
// `RecordVar.Attribute <Op> <operand>`, with the attribute always on the
// left. The operand is a constant (Literal) or a call with no arguments to
// one of the caller's named functions (Func).
type OrderedComparison struct {
	Attribute string
	// Op is one of ==, <, <=, >, >=. A leaf written with the attribute on
	// the right is flipped: `today() <= edge.x` reads as `edge.x >= today()`.
	Op string
	// Literal is the constant operand, as compiled: a Date for a date
	// attribute (its literal parsed against the attribute's layout), a
	// String for a string one. Nil when Func is set.
	Literal Value
	// Func names the zero-argument function operand, empty for a Literal.
	Func string
}

// OrderedConjunction splits the program into the comparisons of its
// top-level AND spine, for a caller that answers each comparison exactly
// outside the engine (the conditional ACL grant, TKT-TKSCSBT). ok is true
// only when every leaf compares a direct attribute of recordVar with a
// constant or with a call, without arguments, to a function in funcs.
// Anything else (`or`, `not`, `~=`, two attributes, a call with arguments)
// makes ok false.
//
// Unlike [Program.Conjunction] it admits ordered comparisons and leaves
// their meaning to the caller, which must decide per attribute type whether
// it can answer them exactly.
func (p *Program) OrderedConjunction(recordVar string, funcs map[string]bool) ([]OrderedComparison, bool) {
	if p == nil || recordVar == "" {
		return nil, false
	}
	var leaves []node
	collectConjuncts(p.root, &leaves)
	out := make([]OrderedComparison, 0, len(leaves))
	for _, n := range leaves {
		rel, isRel := n.(*relationalNode)
		if !isRel {
			return nil, false
		}
		cmp, ok := orderedLeaf(rel.lhs, rel.rhs, rel.op, recordVar, funcs)
		if !ok {
			cmp, ok = orderedLeaf(rel.rhs, rel.lhs, flipOp(rel.op), recordVar, funcs)
		}
		if !ok {
			return nil, false
		}
		out = append(out, cmp)
	}
	return out, len(out) > 0
}

// orderedLeaf matches `recordVar.attr op operand` in one operand order.
func orderedLeaf(lhs, rhs node, op, recordVar string, funcs map[string]bool) (OrderedComparison, bool) {
	switch op {
	case "==", "<", "<=", ">", ">=":
	default:
		return OrderedComparison{}, false
	}
	attr, ok := recordAttr(lhs, recordVar)
	if !ok {
		return OrderedComparison{}, false
	}
	switch c := rhs.(type) {
	case *constNode:
		switch c.v.(type) {
		case String, Date:
			return OrderedComparison{Attribute: attr, Op: op, Literal: c.v}, true
		}
	case *callNode:
		if len(c.args) == 0 && funcs[c.name] {
			return OrderedComparison{Attribute: attr, Op: op, Func: c.name}, true
		}
	}
	return OrderedComparison{}, false
}

// flipOp returns the operator that reads the same comparison with its
// operands swapped.
func flipOp(op string) string {
	switch op {
	case "<":
		return ">"
	case "<=":
		return ">="
	case ">":
		return "<"
	case ">=":
		return "<="
	}
	return op
}
