package predicate

import (
	"context"
	"errors"
	"fmt"
)

// Func is a host-implemented function callable from a predicate.
// The engine type-checks args against the declared FuncSig at compile
// time; an implementation receives args in the declared types and
// must return a Value of the declared return type.
//
// Func is an interface (not a function type) so implementations can
// carry state and so the engine can pass a context.Context — needed
// once host functions traverse the store, hit caches, or are
// cancellable.
type Func interface {
	Call(ctx context.Context, args []Value) (Value, error)
}

// FuncFunc adapts a Go closure into a Func. Use it when the host
// function has no state of its own.
type FuncFunc func(ctx context.Context, args []Value) (Value, error)

// Call satisfies Func.
func (f FuncFunc) Call(ctx context.Context, args []Value) (Value, error) {
	return f(ctx, args)
}

// Bindings carries the runtime values a predicate evaluates against:
// concrete Values for each declared variable plus implementations for
// each declared host function.
//
// Build a Bindings with NewBindings and the SetVar/SetFunc methods;
// the engine does not expose the underlying maps so callers cannot
// mutate them mid-evaluation. A *Bindings is safe to reuse across
// Eval calls but is not safe for concurrent mutation; build once and
// share the resulting value.
type Bindings struct {
	vars      map[string]Value
	funcs     map[string]Func
	traversal TraversalFunc
}

// NewBindings returns an empty Bindings ready for SetVar / SetFunc.
func NewBindings() *Bindings {
	return &Bindings{
		vars:  map[string]Value{},
		funcs: map[string]Func{},
	}
}

// SetVar binds a value to a variable name. Returns an error on empty
// name or nil value (the typed Nil value is fine; a Go nil interface
// is not).
func (b *Bindings) SetVar(name string, v Value) error {
	if name == "" {
		return errors.New("predicate: bindings: variable name must be non-empty")
	}
	if v == nil {
		return fmt.Errorf("predicate: bindings: variable %q: value must be non-nil (use NewNil() for the predicate nil)", name)
	}
	b.vars[name] = v
	return nil
}

// SetTraversal binds the resolver for the `related(...)` form. A program
// containing a traversal fails at Eval when none is bound: answering false
// would present an unanswerable traversal as a legitimate "no match".
//
// Callers that LOWER traversals into a store query (the pushdown path) never
// need this — the store answers them. It exists for the paths that evaluate
// in Go.
func (b *Bindings) SetTraversal(f TraversalFunc) { b.traversal = f }

// SetFunc binds an implementation to a host function name.
func (b *Bindings) SetFunc(name string, f Func) error {
	if name == "" {
		return errors.New("predicate: bindings: function name must be non-empty")
	}
	if f == nil {
		return fmt.Errorf("predicate: bindings: function %q: implementation must be non-nil", name)
	}
	b.funcs[name] = f
	return nil
}

// lookupVar / lookupFunc are the engine-side accessors.
func (b *Bindings) lookupVar(name string) (Value, bool) {
	if b == nil {
		return nil, false
	}
	v, ok := b.vars[name]
	return v, ok
}

func (b *Bindings) lookupFunc(name string) (Func, bool) {
	if b == nil {
		return nil, false
	}
	f, ok := b.funcs[name]
	return f, ok
}
