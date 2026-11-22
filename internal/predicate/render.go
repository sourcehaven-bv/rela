package predicate

import (
	"maps"
	"slices"
	"strconv"
	"strings"
)

// render writes n back as source text, close to how an author writes it.
// It names a sub-expression in an error ([Program.ConditionTreeExplained]),
// so it favors readability over a round trip: a table argument lists its
// keys in sorted order, and a decimal or date literal prints its value.
func render(n node) string {
	var b strings.Builder
	renderTo(&b, n)
	return b.String()
}

func renderTo(b *strings.Builder, n node) {
	switch x := n.(type) {
	case *constNode:
		b.WriteString(renderValue(x.v))
	case *varNode:
		b.WriteString(x.name)
	case *attrNode:
		renderTo(b, x.obj)
		b.WriteByte('.')
		b.WriteString(x.name)
	case *callNode:
		b.WriteString(x.name)
		renderArgs(b, x.args)
	case *tableArgNode:
		renderTable(b, x.entries)
	case *relationalNode:
		renderBinary(b, x.lhs, x.op, x.rhs)
	case *logicalNode:
		renderBinary(b, x.lhs, x.op, x.rhs)
	case *notNode:
		b.WriteString("not ")
		renderOperand(b, x.expr)
	case *traversalNode:
		b.WriteString("related(")
		renderTo(b, x.subject)
		b.WriteString(", ")
		for i, rel := range x.spec.Path {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(strconv.Quote(rel))
		}
		b.WriteString(", ...)")
	default:
		b.WriteString("...")
	}
}

func renderArgs(b *strings.Builder, args []node) {
	b.WriteByte('(')
	for i, a := range args {
		if i > 0 {
			b.WriteString(", ")
		}
		renderTo(b, a)
	}
	b.WriteByte(')')
}

func renderTable(b *strings.Builder, entries map[string]Value) {
	b.WriteString("{ ")
	for i, k := range slices.Sorted(maps.Keys(entries)) {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(k)
		b.WriteString(" = ")
		b.WriteString(renderValue(entries[k]))
	}
	b.WriteString(" }")
}

func renderBinary(b *strings.Builder, lhs node, op string, rhs node) {
	renderOperand(b, lhs)
	b.WriteByte(' ')
	b.WriteString(op)
	b.WriteByte(' ')
	renderOperand(b, rhs)
}

// renderOperand parenthesises a compound operand so the rendering keeps
// the tree's grouping.
func renderOperand(b *strings.Builder, n node) {
	switch n.(type) {
	case *relationalNode, *logicalNode, *arithmeticNode, *concatNode:
		b.WriteByte('(')
		renderTo(b, n)
		b.WriteByte(')')
	default:
		renderTo(b, n)
	}
}

// renderValue writes a literal: a string quoted, nil as `nil`, any other
// value in its text form.
func renderValue(v Value) string {
	switch x := v.(type) {
	case String:
		return strconv.Quote(x.v)
	case Nil:
		return "nil"
	case Bool:
		return strconv.FormatBool(x.v)
	case Int:
		return strconv.FormatInt(x.v, 10)
	case Number:
		return strconv.FormatFloat(x.v, 'g', -1, 64)
	case Date:
		return x.v.Format("2006-01-02")
	case Decimal:
		if t, ok := x.Text(); ok {
			return t
		}
		return "..."
	default:
		return "..."
	}
}
