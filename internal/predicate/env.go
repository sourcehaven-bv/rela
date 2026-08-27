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
	IntType    Type = primitiveType{"int"}
	// DateType is the bare date type (no parse layout). It is
	// equalsType-compatible with any DateTypeWithLayout(...) — see that
	// constructor. Use DateTypeWithLayout at the metamodel->Env adapter
	// so string date literals coerce against the field's real format.
	DateType   Type = dateType{}
	StringType Type = primitiveType{"string"}
	NilType    Type = primitiveType{"nil"}
	// AnyType only appears in host-function signatures: it accepts
	// any Value. Use sparingly — it short-circuits the type checker.
	AnyType Type = primitiveType{"any"}
)

// DateTypeWithLayout returns a date type descriptor that additionally
// carries the Go time layout used to parse bare string/number date
// literals against this field at COMPILE time (see walkRelational
// literal coercion, RR-A3EZR). It is equalsType-compatible with the
// bare DateType singleton — a value is still a Date, comparisons are
// still date-ordered — so only the compile-time coercion path reads the
// layout; eval never does. The metamodel->Env adapter supplies the
// layout (e.g. "2006-01-02" for date, time.RFC3339 for datetime) so the
// predicate package need not import metamodel (arch_test forbids it).
//
// An empty layout falls back to the built-in default layouts at
// coercion time (see coerceDateLiteral).
func DateTypeWithLayout(layout string) Type { return dateType{layout: layout} }

// dateType is the parameterized date descriptor. Its zero value (empty
// layout) is what the DateType singleton is, so DateType and any
// DateTypeWithLayout(...) are mutually equalsType-compatible.
type dateType struct{ layout string }

func (dateType) typeName() string { return "date" }
func (dateType) equalsType(o Type) bool {
	_, ok := o.(dateType)
	return ok
}
func (dateType) sealedType() {}

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
	// SQLPortable reports that this function has target-neutral semantics
	// which a future SQL lowering may reproduce exactly. False is the safe
	// default: host functions must opt in deliberately.
	SQLPortable bool
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
