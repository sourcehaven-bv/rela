package predicate

import (
	"context"
	"fmt"
)

// Eval evaluates the program against bindings and returns the result
// value or an *EvalError. Safe to call concurrently with distinct
// bindings (see doc.go).
//
// The context is threaded through to any host function the program
// invokes. A nil bindings argument is treated as empty; refer to any
// declared variable or function and Eval returns an *EvalError.
func (p *Program) Eval(ctx context.Context, b *Bindings, opts ...EvalOption) (Value, error) {
	cfg := evalOptions{stepBudget: defaultStepBudget}
	for _, o := range opts {
		o(&cfg)
	}
	state := &evalState{
		bindings:    b,
		stepBudget:  cfg.stepBudget,
		stepCounter: 0,
	}
	return state.eval(ctx, p.root)
}

// evalState carries the per-Eval mutable state. Each Eval call gets a
// fresh state; nothing here is shared with the *Program. ctx is
// threaded through the eval methods rather than stored on the struct
// (golangci-lint containedctx).
type evalState struct {
	bindings    *Bindings
	stepBudget  int
	stepCounter int
}

func (s *evalState) tick() error {
	s.stepCounter++
	if s.stepCounter > s.stepBudget {
		return &EvalError{Reason: fmt.Sprintf("step budget exceeded (limit %d)", s.stepBudget)}
	}
	return nil
}

func (s *evalState) eval(ctx context.Context, n node) (Value, error) {
	if err := s.tick(); err != nil {
		return nil, err
	}
	switch x := n.(type) {
	case *constNode:
		return x.v, nil
	case *varNode:
		return s.evalVar(x)
	case *attrNode:
		return s.evalAttr(ctx, x)
	case *callNode:
		return s.evalCall(ctx, x)
	case *tableArgNode:
		// Table-arg nodes are handled inside evalCall via the
		// per-call dispatch; reaching here is a bug.
		return nil, &EvalError{Reason: "internal: tableArgNode outside call context"}
	case *relationalNode:
		return s.evalRelational(ctx, x)
	case *logicalNode:
		return s.evalLogical(ctx, x)
	case *notNode:
		return s.evalNot(ctx, x)
	default:
		return nil, &EvalError{Reason: fmt.Sprintf("internal: unknown IR node %T", n)}
	}
}

func (s *evalState) evalVar(n *varNode) (Value, error) {
	v, ok := s.bindings.lookupVar(n.name)
	if !ok {
		return nil, &EvalError{Reason: fmt.Sprintf("binding %q not provided", n.name)}
	}
	if !runtimeTypeAccepts(n.typ, v) {
		return nil, &EvalError{Reason: fmt.Sprintf("binding %q: expected %s, got %s", n.name, n.typ.typeName(), v.Type().typeName())}
	}
	return v, nil
}

// runtimeTypeAccepts checks that a runtime Value is shape-compatible
// with a declared static type. Field-level RecordType validation is a
// compile-time concern; at runtime we only check the value variant.
// A declared RecordType is satisfied by any Record value; a declared
// ListType by any List value. Scalars must match exactly.
func runtimeTypeAccepts(expected Type, got Value) bool {
	switch expected.(type) {
	case RecordType:
		_, ok := got.(Record)
		return ok
	case ListType:
		_, ok := got.(List)
		return ok
	}
	return expected.equalsType(got.Type())
}

func (s *evalState) evalAttr(ctx context.Context, n *attrNode) (Value, error) {
	obj, err := s.eval(ctx, n.obj)
	if err != nil {
		return nil, err
	}
	rec, ok := obj.(Record)
	if !ok {
		return nil, &EvalError{Reason: fmt.Sprintf("attribute %q: expected record, got %s", n.name, obj.Type().typeName())}
	}
	v, present := rec.Get(n.name)
	if !present {
		// The type checker promised this field exists on the declared
		// record shape, but the binding may have omitted it. Treat as
		// nil so the rule author can write `entity.optional == nil`
		// to test for absence.
		return NewNil(), nil
	}
	return v, nil
}

func (s *evalState) evalCall(ctx context.Context, n *callNode) (Value, error) {
	// Look up the host function before evaluating args. A missing host
	// fn fails fast and avoids spending the step budget on subtree
	// evaluation whose result we wouldn't use anyway (RR-CIGK).
	fn, ok := s.bindings.lookupFunc(n.name)
	if !ok {
		return nil, &EvalError{Reason: fmt.Sprintf("host function %q not provided", n.name)}
	}

	args := make([]Value, len(n.args))
	for i, a := range n.args {
		// Table-arg nodes don't go through the normal eval path: they
		// carry constants, so we can materialize the Record directly.
		if ta, ok := a.(*tableArgNode); ok {
			args[i] = NewRecord(copyEntries(ta.entries))
			continue
		}
		v, err := s.eval(ctx, a)
		if err != nil {
			return nil, err
		}
		args[i] = v
	}

	out, err := fn.Call(ctx, args)
	if err != nil {
		return nil, &EvalError{Reason: fmt.Sprintf("host function %q: %v", n.name, err)}
	}
	if out == nil {
		return nil, &EvalError{Reason: fmt.Sprintf("host function %q returned nil; use NewNil() to return the predicate nil value", n.name)}
	}
	if !runtimeTypeAccepts(n.typ, out) {
		return nil, &EvalError{Reason: fmt.Sprintf("host function %q: declared return %s, got %s", n.name, n.typ.typeName(), out.Type().typeName())}
	}
	return out, nil
}

func copyEntries(in map[string]Value) map[string]Value {
	out := make(map[string]Value, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (s *evalState) evalRelational(ctx context.Context, n *relationalNode) (Value, error) {
	lhs, err := s.eval(ctx, n.lhs)
	if err != nil {
		return nil, err
	}
	rhs, err := s.eval(ctx, n.rhs)
	if err != nil {
		return nil, err
	}
	switch n.op {
	case "==":
		return NewBool(valuesEqual(lhs, rhs)), nil
	case "~=":
		return NewBool(!valuesEqual(lhs, rhs)), nil
	case "<", "<=", ">", ">=":
		return evalOrdered(n.op, lhs, rhs)
	default:
		return nil, &EvalError{Reason: fmt.Sprintf("internal: unknown relational op %q", n.op)}
	}
}

// valuesEqual implements Lua-flavored equality (see doc.go). nil
// only equals nil; mismatched types are never equal except via nil.
func valuesEqual(a, b Value) bool {
	if _, an := a.(Nil); an {
		_, bn := b.(Nil)
		return bn
	}
	if _, bn := b.(Nil); bn {
		return false
	}
	switch av := a.(type) {
	case Bool:
		bv, ok := b.(Bool)
		return ok && av.v == bv.v
	case Number:
		bv, ok := b.(Number)
		return ok && av.v == bv.v
	case String:
		bv, ok := b.(String)
		return ok && av.v == bv.v
	}
	// Records / lists are forbidden at compile time, so reaching this
	// branch is an internal invariant violation.
	return false
}

func evalOrdered(op string, a, b Value) (Value, error) {
	// Both sides were already type-checked at compile to be the same
	// scalar type (number or string). The defensive ", ok" guards
	// here protect against an IR/type-checker drift bug rather than
	// any reachable Lua input.
	switch av := a.(type) {
	case Number:
		bv, ok := b.(Number)
		if !ok {
			return nil, &EvalError{Reason: fmt.Sprintf("internal: ordered cmp %q: rhs type %s mismatches lhs Number", op, b.Type().typeName())}
		}
		return NewBool(cmpNumber(op, av.v, bv.v)), nil
	case String:
		bv, ok := b.(String)
		if !ok {
			return nil, &EvalError{Reason: fmt.Sprintf("internal: ordered cmp %q: rhs type %s mismatches lhs String", op, b.Type().typeName())}
		}
		return NewBool(cmpString(op, av.v, bv.v)), nil
	}
	return nil, &EvalError{Reason: fmt.Sprintf("ordered comparison %q on unsupported type %s", op, a.Type().typeName())}
}

func cmpNumber(op string, x, y float64) bool {
	switch op {
	case "<":
		return x < y
	case "<=":
		return x <= y
	case ">":
		return x > y
	case ">=":
		return x >= y
	}
	return false
}

func cmpString(op, x, y string) bool {
	switch op {
	case "<":
		return x < y
	case "<=":
		return x <= y
	case ">":
		return x > y
	case ">=":
		return x >= y
	}
	return false
}

func (s *evalState) evalLogical(ctx context.Context, n *logicalNode) (Value, error) {
	lhs, err := s.eval(ctx, n.lhs)
	if err != nil {
		return nil, err
	}
	lb, ok := lhs.(Bool)
	if !ok {
		return nil, &EvalError{Reason: fmt.Sprintf("'%s' lhs: expected bool, got %s", n.op, lhs.Type().typeName())}
	}
	// Short-circuit, Lua-style.
	switch n.op {
	case "and":
		if !lb.v {
			return NewBool(false), nil
		}
	case "or":
		if lb.v {
			return NewBool(true), nil
		}
	}
	rhs, err := s.eval(ctx, n.rhs)
	if err != nil {
		return nil, err
	}
	rb, ok := rhs.(Bool)
	if !ok {
		return nil, &EvalError{Reason: fmt.Sprintf("'%s' rhs: expected bool, got %s", n.op, rhs.Type().typeName())}
	}
	return rb, nil
}

func (s *evalState) evalNot(ctx context.Context, n *notNode) (Value, error) {
	v, err := s.eval(ctx, n.expr)
	if err != nil {
		return nil, err
	}
	b, ok := v.(Bool)
	if !ok {
		return nil, &EvalError{Reason: "'not': expected bool, got " + v.Type().typeName()}
	}
	return NewBool(!b.v), nil
}
