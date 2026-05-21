package predicate

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
// integer promoted to float64.
func NewNumberFromInt(i int) Number { return Number{v: float64(i)} }

// Float returns the underlying float64.
func (n Number) Float() float64 { return n.v }
func (Number) Type() Type       { return NumberType }
func (Number) sealedValue()     {}

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
