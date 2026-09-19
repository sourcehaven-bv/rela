package predicate

import (
	"context"
	"fmt"
	"sort"

	"github.com/yuin/gopher-lua/ast"
)

// FuncRelated is the reserved name of the traversal form:
//
//	related(entity, 'rel-x', { type = 'ticket', status = 'done' })
//	related(entity, { 'rel-x', 'rel-y' }, { type = 'C', prop = 'foo' })
//
// It is ordinary Lua to the author — a call with a string (or list of
// strings) and a table constructor — but it is NOT a host function. The
// walker recognizes the name and compiles the whole form to a
// [traversalNode] at COMPILE time.
//
// That distinction is the point, not an implementation detail. A host
// function's signature would have to declare SQLPortable=false (it reads
// the graph, which no FuncSig can express), and one non-portable call taints
// the WHOLE program — so spelling traversal as a host function would silently
// disable pushdown for every other clause in the same condition. Compiling it
// statically keeps the program portable and gives the lowering an exact,
// inspectable description of what to push down.
const FuncRelated = "related"

// TraversalSpec is the compile-time description of one traversal, exposed so
// a lowering can turn it into a store predicate without re-parsing anything.
//
// It carries no evaluation behavior and no store types: internal/predicate
// depends on nothing (arch_test), so the CALLER maps these names onto its own
// graph vocabulary.
type TraversalSpec struct {
	// Path is the relation types to follow, in order. Always at least one.
	Path []string

	// EntityType is the `type =` ascription, or "" when the author omitted
	// it. Omission is legal HERE — the engine cannot know whether a relation
	// has one target type or several — and is rejected by the metamodel-aware
	// layer when the relation's target is a union.
	EntityType string

	// Props are the remaining table keys: property name → required value.
	// Sorted access is via [TraversalSpec.PropNames] so a lowering emits
	// predicates in a deterministic order.
	Props map[string]Value
}

// PropNames returns the property keys in sorted order.
func (s TraversalSpec) PropNames() []string {
	out := make([]string, 0, len(s.Props))
	for k := range s.Props {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// traversalNode is the compiled traversal. It evaluates to a bool via a
// caller-supplied resolver (see [Bindings.SetTraversal]); with no resolver
// bound, Eval fails rather than guessing an answer.
type traversalNode struct {
	subject node
	spec    TraversalSpec
}

func (*traversalNode) resultType() Type { return BoolType }
func (*traversalNode) sealedNode()      {}

// TraversalFunc answers one compiled traversal against the graph. Bound on
// [Bindings] by a caller that has a store; the engine itself never reads one.
//
// Nil: a program containing a traversal fails at Eval when none is bound.
// That is deliberate — returning false would silently turn an unanswerable
// traversal into "no match", which reads as a legitimate result.
type TraversalFunc func(subject Value, spec TraversalSpec) (bool, error)

// walkRelated compiles the `related(...)` form.
//
// Shape: related(<record>, <string|{strings}>, [{table}]). The subject must be
// a record-typed expression (the entity in scope); the path is a string
// literal or a list of them; the optional third argument is the constraint
// table, whose `type` key is the entity-type ascription and whose remaining
// keys are property equalities.
func (w *walker) walkRelated(e *ast.FuncCallExpr) (node, error) {
	if len(e.Args) < 2 || len(e.Args) > 3 {
		return nil, &CompileError{Line: e.Line(), Reason: fmt.Sprintf(
			"%s: expected 2 or 3 args (subject, relation path, optional constraints), got %d",
			FuncRelated, len(e.Args))}
	}

	subject, err := w.walkExpr(e.Args[0])
	if err != nil {
		return nil, err
	}
	if _, ok := subject.resultType().(RecordType); !ok {
		return nil, &CompileError{Line: e.Line(), Reason: fmt.Sprintf(
			"%s: first argument must be an entity record, got %s",
			FuncRelated, subject.resultType().typeName())}
	}

	path, err := relationPath(e.Args[1], e.Line())
	if err != nil {
		return nil, err
	}

	spec := TraversalSpec{Path: path, Props: map[string]Value{}}
	if len(e.Args) == 3 {
		if err := w.applyConstraints(e, e.Args[2], &spec); err != nil {
			return nil, err
		}
	}

	return &traversalNode{subject: subject, spec: spec}, nil
}

// applyConstraints folds the optional constraint table into spec: the `type`
// key becomes the entity-type ascription, every other key a property equality.
func (w *walker) applyConstraints(call *ast.FuncCallExpr, arg ast.Expr, spec *TraversalSpec) error {
	tbl, ok := arg.(*ast.TableExpr)
	if !ok {
		return &CompileError{Line: call.Line(),
			Reason: FuncRelated + ": constraints must be a table literal ({type='x', prop='y'})"}
	}
	n, err := w.walkTableArg(tbl)
	if err != nil {
		return err
	}
	targ, ok := n.(*tableArgNode)
	if !ok {
		// coverage-ignore: invariant: walkTableArg returns *tableArgNode or an error, both handled above
		return &CompileError{Line: call.Line(), Reason: FuncRelated + ": internal: malformed constraint table"}
	}
	for k, v := range targ.entries {
		if k != "type" {
			spec.Props[k] = v
			continue
		}
		str, isStr := v.(String)
		if !isStr {
			return &CompileError{Line: call.Line(), Reason: FuncRelated + ": 'type' must be a string literal"}
		}
		spec.EntityType = str.String()
	}
	return nil
}

// relationPath accepts either 'rel-x' or {'rel-x', 'rel-y'} and returns the
// hop list. A chain is written as a LIST rather than by nesting calls because
// the nested spelling would make the subject of the inner call a bool (the
// outer call's result type), which cannot type-check.
func relationPath(e ast.Expr, line int) ([]string, error) {
	switch x := e.(type) {
	case *ast.StringExpr:
		if x.Value == "" {
			return nil, &CompileError{Line: line, Reason: FuncRelated + ": relation type must be non-empty"}
		}
		return []string{x.Value}, nil
	case *ast.TableExpr:
		if len(x.Fields) == 0 {
			return nil, &CompileError{Line: line, Reason: FuncRelated + ": relation path must name at least one relation type"}
		}
		out := make([]string, 0, len(x.Fields))
		for _, f := range x.Fields {
			if f.Key != nil {
				return nil, &CompileError{Line: line, Reason: FuncRelated + ": relation path must be a plain list ({'a', 'b'})"}
			}
			s, ok := f.Value.(*ast.StringExpr)
			if !ok || s.Value == "" {
				return nil, &CompileError{Line: line, Reason: FuncRelated + ": relation path entries must be non-empty string literals"}
			}
			out = append(out, s.Value)
		}
		return out, nil
	default:
		return nil, &CompileError{Line: line, Reason: fmt.Sprintf(
			"%s: relation path must be a string or a list of strings, got %s", FuncRelated, astKindName(e))}
	}
}

// Traversals returns every traversal the program compiles, in the order
// [Program.inspect] visits them. A lowering uses this to build store
// predicates; an empty result means the program needs no graph access beyond
// its bindings.
//
// Collected during inspect rather than by a second walk, so the exhaustive
// node switch that already has to stay current is the ONLY one — a new node
// type forgotten here would silently drop a traversal from the lowering while
// the program still evaluated it, which is a pushdown that returns more rows
// than the condition allows.
func (p *Program) Traversals() []TraversalSpec { return p.traversals }

// evalTraversal answers a traversal via the bound resolver.
func (s *evalState) evalTraversal(ctx context.Context, n *traversalNode) (Value, error) {
	if s.bindings.traversal == nil {
		return nil, &EvalError{Reason: fmt.Sprintf(
			"%s: no traversal resolver is bound; this program must be lowered into a "+
				"store query or evaluated with Bindings.SetTraversal", FuncRelated)}
	}
	subject, err := s.eval(ctx, n.subject)
	if err != nil {
		return nil, err
	}
	ok, err := s.bindings.traversal(subject, n.spec)
	if err != nil {
		return nil, &EvalError{Reason: fmt.Sprintf("%s: %s", FuncRelated, err.Error())}
	}
	return NewBool(ok), nil
}
