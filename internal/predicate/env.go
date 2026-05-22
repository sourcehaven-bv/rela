package predicate

import (
	"errors"
	"fmt"
)

// Type is the static type a Value can carry. The type system is
// deliberately tiny: it only needs to discriminate the cases the
// expression grammar can express.
type Type interface {
	// typeName is for error messages.
	typeName() string
	// equalsType compares two type descriptors for compatibility.
	equalsType(Type) bool
	sealedType()
}

// Scalar primitives. All values of a given scalar type are
// interchangeable for type-check purposes.
type primitiveType struct{ name string }

func (p primitiveType) typeName() string { return p.name }
func (p primitiveType) equalsType(o Type) bool {
	op, ok := o.(primitiveType)
	return ok && op.name == p.name
}
func (primitiveType) sealedType() {}

// Public type descriptors callers use when declaring an env.
var (
	BoolType   Type = primitiveType{"bool"}
	NumberType Type = primitiveType{"number"}
	StringType Type = primitiveType{"string"}
	NilType    Type = primitiveType{"nil"}
	// AnyType only appears in host-function signatures: it accepts
	// any Value. Use sparingly — it short-circuits the type checker.
	AnyType Type = primitiveType{"any"}
)

// Record is a named-field type descriptor. Used both as a static
// declaration and as a runtime Value (see value.go). The fields map
// declares attribute name → type for an entity-like structure.
type RecordType map[string]Type

func (r RecordType) typeName() string { return "record" }
func (r RecordType) equalsType(o Type) bool {
	or, ok := o.(RecordType)
	if !ok || len(or) != len(r) {
		return false
	}
	for k, v := range r {
		ov, ok := or[k]
		if !ok || !v.equalsType(ov) {
			return false
		}
	}
	return true
}
func (RecordType) sealedType() {}

// ListType is a homogeneous list type descriptor.
type ListType struct{ Elem Type }

func (ListType) typeName() string { return "list" }
func (l ListType) equalsType(o Type) bool {
	ol, ok := o.(ListType)
	return ok && l.Elem.equalsType(ol.Elem)
}
func (ListType) sealedType() {}

// FuncSig declares a host function's parameter and return types. A
// non-nil Variadic indicates the function accepts zero or more extra
// arguments of that type after the fixed Params.
type FuncSig struct {
	Params   []Type
	Variadic Type
	Return   Type
}

// Env declares the variables and functions a predicate may reference.
// Build one before calling Compile.
//
// Env is mutable until the first Compile that uses it; callers should
// finish declarations before any compile. Concurrent declares are not
// safe; declare then share.
type Env struct {
	vars  map[string]Type
	funcs map[string]FuncSig
}

// NewEnv constructs an empty Env.
func NewEnv() *Env {
	return &Env{
		vars:  map[string]Type{},
		funcs: map[string]FuncSig{},
	}
}

// DeclareVar registers a variable name and its type. Returns an error
// if name is already declared (as a var or as a func).
func (e *Env) DeclareVar(name string, t Type) error {
	if name == "" {
		return errors.New("predicate: env: variable name must be non-empty")
	}
	if t == nil {
		return fmt.Errorf("predicate: env: variable %q: type must be non-nil", name)
	}
	if _, exists := e.vars[name]; exists {
		return fmt.Errorf("predicate: env: variable %q already declared", name)
	}
	if _, exists := e.funcs[name]; exists {
		return fmt.Errorf("predicate: env: name %q already declared as a function", name)
	}
	e.vars[name] = t
	return nil
}

// DeclareFunc registers a host function name and its signature.
//
// The return type must be a scalar (bool, number, string, nil).
// Record and list return types are rejected (RR-93UN): the engine's
// runtime type check does not reach into a returned Record's fields,
// so a downstream entity.attribute access on a host-returned record
// could observe a typed field whose runtime type differs from the
// declared type. Until the type checker is extended to re-validate
// nested values, host functions return scalars only. (Current use
// cases — has_role, has_relation, count_relations — all do.)
func (e *Env) DeclareFunc(name string, sig FuncSig) error {
	if name == "" {
		return errors.New("predicate: env: function name must be non-empty")
	}
	if sig.Return == nil {
		return fmt.Errorf("predicate: env: function %q: return type must be non-nil", name)
	}
	switch sig.Return.(type) {
	case RecordType:
		return fmt.Errorf("predicate: env: function %q: record return types are not supported", name)
	case ListType:
		return fmt.Errorf("predicate: env: function %q: list return types are not supported", name)
	}
	for i, p := range sig.Params {
		if p == nil {
			return fmt.Errorf("predicate: env: function %q: param %d type must be non-nil", name, i)
		}
	}
	if _, exists := e.funcs[name]; exists {
		return fmt.Errorf("predicate: env: function %q already declared", name)
	}
	if _, exists := e.vars[name]; exists {
		return fmt.Errorf("predicate: env: name %q already declared as a variable", name)
	}
	e.funcs[name] = sig
	return nil
}

func (e *Env) lookupVar(name string) (Type, bool) {
	t, ok := e.vars[name]
	return t, ok
}

func (e *Env) lookupFunc(name string) (FuncSig, bool) {
	s, ok := e.funcs[name]
	return s, ok
}
