package predicate

import "time"

// Value is the sealed sum type the evaluator operates on. The
// unexported sealedValue method prevents external packages from
// inventing new variants; everything that round-trips through the
// engine must be one of the constructors below.
type Value interface {
	Type() Type
	sealedValue()
}

// Bool is a concrete-typed boolean value.
type Bool struct{ v bool }

// NewBool constructs a Bool value.
func NewBool(b bool) Bool { return Bool{v: b} }

// Bool returns the underlying Go bool.
func (b Bool) Bool() bool { return b.v }
func (Bool) Type() Type   { return BoolType }
func (Bool) sealedValue() {}

// Number is a concrete numeric value, backed by float64. There is no
// separate integer type — see doc.go ("Numeric model").
type Number struct{ v float64 }

// NewNumber constructs a Number value from a float64.
func NewNumber(f float64) Number { return Number{v: f} }

// NewNumberFromInt constructs a Number value from a Go int, with the
// integer promoted to float64. Use it for a field declared NumberType.
// For a field declared IntType use [NewInt] instead — binding a Number
// to an IntType field fails the runtime type check at Eval (RR-4189H).
func NewNumberFromInt(i int) Number { return Number{v: float64(i)} }

// Float returns the underlying float64.
func (n Number) Float() float64 { return n.v }
func (Number) Type() Type       { return NumberType }
func (Number) sealedValue()     {}

// Int is a concrete integer value backed by int64. It is a SEPARATE
// variant from Number (which is float64) so integer-typed properties
// compare exactly — no lossy float64 round-trip past 2^53 and no
// lexicographic "10" < "9" surprise. A predicate never mixes Int and
// Number in a comparison: the type checker (checkRelational) requires
// same-type operands, and literal coercion (walkRelational) retypes a
// number-literal RHS to Int when the LHS attribute is IntType.
type Int struct{ v int64 }

// NewInt constructs an Int value. Bind it to a field declared IntType.
// Do NOT use [NewNumberFromInt] for an IntType field — that returns a
// Number (float64), which fails the runtime type check against an
// IntType binding at Eval (RR-4189H).
func NewInt(i int64) Int { return Int{v: i} }

// Int64 returns the underlying int64.
func (i Int) Int64() int64 { return i.v }
func (Int) Type() Type     { return IntType }
func (Int) sealedValue()   {}

// Date is a concrete instant value backed by time.Time. It backs both
// the metamodel `date` and `datetime` property types; comparison is
// always instant-granular (a bare date is midnight in its parsed
// location), matching internal/filter's matchDate. The time.Time is
// parsed at compile or bind time — never at Eval — so the engine keeps
// its no-I/O-at-eval invariant (see doc.go, RR-A3EZR).
type Date struct{ v time.Time }

// NewDate constructs a Date value from a time.Time.
func NewDate(t time.Time) Date { return Date{v: t} }

// Time returns the underlying time.Time.
func (d Date) Time() time.Time { return d.v }
func (Date) Type() Type        { return DateType }
func (Date) sealedValue()      {}

// String is a concrete string value. Lua strings are byte-strings; we
// preserve any bytes the caller binds, including embedded null bytes.
type String struct{ v string }

// NewString constructs a String value.
func NewString(s string) String { return String{v: s} }

// String returns the underlying Go string.
func (s String) String() string { return s.v }
func (String) Type() Type       { return StringType }
func (String) sealedValue()     {}

// Nil is the predicate engine's nil value. Distinct from Go's nil and
// from a missing binding.
type Nil struct{}

// NewNil constructs a Nil value.
func NewNil() Nil { return Nil{} }

// Type returns NilType.
func (Nil) Type() Type   { return NilType }
func (Nil) sealedValue() {}

// Record is a named-field bundle, the value-form of a Lua table used
// for entity-shape access (entity.status). Field access happens at
// eval time through the AttrGet IR op.
type Record struct {
	fields map[string]Value
}

// NewRecord constructs a Record from a map. The returned Record
// retains the supplied map by reference — callers must not mutate
// it after the call (RR-AJS4). Pass a freshly built map if the
// caller intends to keep working with one of its own.
func NewRecord(fields map[string]Value) Record {
	if fields == nil {
		fields = map[string]Value{}
	}
	return Record{fields: fields}
}

// Get returns the field value and a present flag.
func (r Record) Get(name string) (Value, bool) {
	v, ok := r.fields[name]
	return v, ok
}

func (Record) Type() Type   { return RecordType{} }
func (Record) sealedValue() {}

// List is an ordered sequence of values, currently unused by the
// expression grammar (no list literals) but reachable through host
// functions whose return type is a list. Reserved here so the surface
// is stable.
type List struct {
	elems []Value
}

// NewList constructs a List. As with NewRecord, the returned List
// retains the supplied slice by reference — callers must not mutate
// it after the call (RR-AJS4).
func NewList(elems []Value) List { return List{elems: elems} }

// Elems returns the underlying slice. Callers must not mutate.
func (l List) Elems() []Value { return l.elems }
func (List) Type() Type       { return ListType{} }
func (List) sealedValue()     {}
