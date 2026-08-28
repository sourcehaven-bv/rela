package predicate

import "sort"

// Program is a compiled predicate, ready for repeated evaluation.
//
// A Program is immutable after Compile. It carries no mutable state,
// no caches, and no per-instance memoization. Multiple goroutines
// may call Eval on the same Program concurrently with their own
// Bindings.
type Program struct {
	root        node
	resultTyp   Type
	env         *Env
	attributes  map[string]map[string]struct{}
	sqlPortable bool
}

// ResultType returns the static type of the program's top-level
// expression.
func (p *Program) ResultType() Type { return p.resultTyp }

// Attributes returns the statically referenced fields of recordVar in sorted
// order. Dynamic attribute access is rejected by the compiler, so this is a
// complete dependency set for an entity record.
func (p *Program) Attributes(recordVar string) []string {
	set := p.attributes[recordVar]
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// SQLPortable reports whether every node and host function in the program has
// declared target-neutral semantics suitable for a future SQL lowering.
func (p *Program) SQLPortable() bool { return p.sqlPortable }

func (p *Program) inspect() {
	p.attributes = map[string]map[string]struct{}{}
	p.sqlPortable = true
	var visit func(node)
	visit = func(n node) {
		switch x := n.(type) {
		case *constNode, *varNode:
		case *attrNode:
			if v, ok := x.obj.(*varNode); ok {
				if p.attributes[v.name] == nil {
					p.attributes[v.name] = map[string]struct{}{}
				}
				p.attributes[v.name][x.name] = struct{}{}
			}
			visit(x.obj)
		case *callNode:
			if sig, ok := p.env.lookupFunc(x.name); !ok || !sig.SQLPortable {
				p.sqlPortable = false
			}
			for _, a := range x.args {
				visit(a)
			}
		case *tableArgNode:
			p.sqlPortable = false
		case *relationalNode:
			visit(x.lhs)
			visit(x.rhs)
		case *logicalNode:
			visit(x.lhs)
			visit(x.rhs)
		case *notNode:
			visit(x.expr)
		case *arithmeticNode:
			visit(x.lhs)
			visit(x.rhs)
		case *unaryMinusNode:
			visit(x.expr)
		case *concatNode:
			visit(x.lhs)
			visit(x.rhs)
		}
	}
	visit(p.root)
}

// EvalOption configures a single Eval call. Options stack
// left-to-right; later options override earlier ones.
type EvalOption func(*evalOptions)

type evalOptions struct {
	stepBudget int
}

// defaultStepBudget is the per-Eval node-visit cap. Tuned generous
// for hand-written rules; aggressive against adversarial input.
const defaultStepBudget = 10_000

// WithStepBudget overrides the per-Eval step budget. Must be > 0;
// values <= 0 are clamped to 1 so a misconfigured caller cannot
// disable the budget.
func WithStepBudget(n int) EvalOption {
	return func(o *evalOptions) {
		if n <= 0 {
			n = 1
		}
		o.stepBudget = n
	}
}
