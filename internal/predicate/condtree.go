package predicate

import "strconv"

// CondTreeOp is the node kind of a [CondTree].
type CondTreeOp int

const (
	// CondTreeCompare is a leaf: [CondTree.Cmp] holds.
	CondTreeCompare CondTreeOp = iota
	// CondTreeTraversal is a leaf: `related(...)` on the record variable,
	// [CondTree.Traversal].
	CondTreeTraversal
	// CondTreeConst is a leaf: the literal [CondTree.Const].
	CondTreeConst
	// CondTreeAnd holds when every argument holds.
	CondTreeAnd
	// CondTreeOr holds when at least one argument holds.
	CondTreeOr
	// CondTreeNot holds when its one argument does not.
	CondTreeNot
)

// CondTree is a boolean condition as [Program.ConditionTree] reads it:
// and, or and not over comparison, traversal and literal leaves. A chain of
// one operator is flattened into one node, so `a and b and c` has three
// arguments rather than nesting.
type CondTree struct {
	Op        CondTreeOp
	Cmp       Comparison
	Traversal TraversalSpec
	Const     bool
	Args      []CondTree
}

// ConditionTree reads the whole program as a [CondTree], for a caller that
// answers the condition in a store query rather than in Go (TKT-8HY05WB).
// It is [Program.ConditionConjunction] without the restriction to a
// top-level AND: `or` and `not` may sit anywhere, and a comparison leaf may
// also be
//
//   - an ordered comparison (<, <=, >, >=) of a string attribute with a
//     string literal (the empty string included), or of an int attribute
//     with an int literal. Comparison.Order holds the operator with the
//     attribute on its left, so `3 < entity.x` reads as `entity.x > 3`
//   - an `==` or `~=` of an int attribute with an int literal
//
// Int comparisons set Comparison.Int and carry the literal in decimal in
// Value. A literal `true` or `false` is a [CondTreeConst] leaf.
//
// ok is false when any part of the program is another shape: a
// value-selecting and/or, a comparison of two attributes, arithmetic, a
// host call outside spec.ConstFuncs, a traversal on anything but
// spec.RecordVar.
//
// The tree describes the program; it does not say a store reads it the same
// way. In particular an ordered comparison on an unset value is an error
// unless the program is evaluated with [NilOrderedIsFalse], and only then
// does it agree with a store's reading.
func (p *Program) ConditionTree(spec PrefilterSpec) (CondTree, bool) {
	if p == nil || spec.RecordVar == "" {
		return CondTree{}, false
	}
	return condTree(p.root, spec)
}

func condTree(n node, spec PrefilterSpec) (CondTree, bool) {
	switch x := n.(type) {
	case *logicalNode:
		if x.selects() {
			return CondTree{}, false
		}
		op := CondTreeAnd
		if x.op == "or" {
			op = CondTreeOr
		}
		out := CondTree{Op: op}
		for _, side := range []node{x.lhs, x.rhs} {
			t, ok := condTree(side, spec)
			if !ok {
				return CondTree{}, false
			}
			if t.Op == op {
				out.Args = append(out.Args, t.Args...)
			} else {
				out.Args = append(out.Args, t)
			}
		}
		return out, true
	case *notNode:
		t, ok := condTree(x.expr, spec)
		if !ok {
			return CondTree{}, false
		}
		return CondTree{Op: CondTreeNot, Args: []CondTree{t}}, true
	case *constNode:
		b, ok := x.v.(Bool)
		if !ok {
			return CondTree{}, false
		}
		return CondTree{Op: CondTreeConst, Const: b.v}, true
	case *traversalNode:
		if !recordTraversal(x, spec) {
			return CondTree{}, false
		}
		return CondTree{Op: CondTreeTraversal, Traversal: x.spec}, true
	case *callNode:
		_, eq, ok := constFuncEquality(x, spec)
		if !ok {
			return CondTree{}, false
		}
		return CondTree{Op: CondTreeCompare, Cmp: Comparison{ConstEquality: eq}}, true
	case *relationalNode:
		c, ok := treeComparison(x, spec)
		if !ok {
			return CondTree{}, false
		}
		return CondTree{Op: CondTreeCompare, Cmp: c}, true
	}
	return CondTree{}, false
}

// treeComparison reads one relational node as a [Comparison] leaf.
func treeComparison(x *relationalNode, spec PrefilterSpec) (Comparison, bool) {
	switch x.op {
	case "==", "~=":
		if c, ok := intComparison(x.lhs, x.rhs, x.op, spec); ok {
			return c, true
		}
		if c, ok := intComparison(x.rhs, x.lhs, x.op, spec); ok {
			return c, true
		}
		_, eq, found := constEqualityFrom(x.lhs, x.rhs, spec)
		if !found {
			_, eq, found = constEqualityFrom(x.rhs, x.lhs, spec)
		}
		if !found {
			return Comparison{}, false
		}
		return Comparison{ConstEquality: eq, NotEqual: x.op == "~="}, true
	case "<", "<=", ">", ">=":
		if c, ok := orderedComparison(x.lhs, x.rhs, x.op, spec); ok {
			return c, true
		}
		return orderedComparison(x.rhs, x.lhs, flipOrder(x.op), spec)
	}
	return Comparison{}, false
}

// intComparison matches `RecordVar.attr <op> <int literal>` with attr in
// lhs.
func intComparison(lhs, rhs node, op string, spec PrefilterSpec) (Comparison, bool) {
	attr, ok := recordAttr(lhs, spec.RecordVar)
	if !ok {
		return Comparison{}, false
	}
	if t := attrTypeOf(lhs); t == nil || !t.equalsType(IntType) {
		return Comparison{}, false
	}
	c, ok := rhs.(*constNode)
	if !ok {
		return Comparison{}, false
	}
	i, ok := c.v.(Int)
	if !ok {
		return Comparison{}, false
	}
	cmp := Comparison{
		ConstEquality: ConstEquality{Attribute: attr, Value: strconv.FormatInt(i.v, 10)},
		Int:           true,
	}
	switch op {
	case "==":
	case "~=":
		cmp.NotEqual = true
	default:
		cmp.Order = op
	}
	return cmp, true
}

// orderedComparison matches `RecordVar.attr <op> <literal>` with attr in
// lhs: a string attribute against a string literal, or an int attribute
// against an int literal.
func orderedComparison(lhs, rhs node, op string, spec PrefilterSpec) (Comparison, bool) {
	attr, ok := recordAttr(lhs, spec.RecordVar)
	if !ok {
		return Comparison{}, false
	}
	if c, isInt := intComparison(lhs, rhs, op, spec); isInt {
		return c, true
	}
	t := attrTypeOf(lhs)
	if t == nil || !t.equalsType(StringType) {
		return Comparison{}, false
	}
	c, ok := rhs.(*constNode)
	if !ok {
		return Comparison{}, false
	}
	s, ok := c.v.(String)
	if !ok {
		return Comparison{}, false
	}
	return Comparison{ConstEquality: ConstEquality{Attribute: attr, Value: s.v}, Order: op}, true
}

// flipOrder returns the operator that reads `a op b` as `b flip a`.
func flipOrder(op string) string {
	switch op {
	case "<":
		return ">"
	case "<=":
		return ">="
	case ">":
		return "<"
	default:
		return "<="
	}
}
