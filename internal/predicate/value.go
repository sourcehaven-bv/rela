package predicate

import (
	"math/big"
	"strings"
	"time"
)

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

// Decimal is an exact decimal value backed by [big.Rat]. It is its own
// variant so a decimal property compares and adds without binary float
// drift: 0.1 + 0.2 == 0.3 holds. Only +, - and * apply to it, so every
// result stays a finite decimal that [Decimal.Text] renders exactly.
type Decimal struct{ v *big.Rat }

// NewDecimal constructs a Decimal. It copies r, so the caller may keep
// using r.
//
// Nil: rejected for r — a nil value is [Nil], not a Decimal.
func NewDecimal(r *big.Rat) Decimal { return Decimal{v: new(big.Rat).Set(r)} }

// Rat returns a copy of the underlying value.
func (d Decimal) Rat() *big.Rat { return new(big.Rat).Set(d.v) }

// Text renders the value in plain decimal notation with as few digits
// after the point as represent it exactly ("12.5", "-3"). ok is false for
// a value with no finite decimal form, which no Decimal operation
// produces.
func (d Decimal) Text() (text string, ok bool) {
	return ratText(d.v)
}

func (Decimal) Type() Type   { return DecimalType }
func (Decimal) sealedValue() {}

// decimalBase2 and decimalBase5 are the prime factors of 10: a fraction is
// a finite decimal exactly when its denominator has no other.
const (
	decimalBase2 = 2
	decimalBase5 = 5
)

// ratText renders r with the fewest digits after the point that represent
// it exactly. ok is false when the denominator has a prime factor other
// than 2 and 5, which no finite decimal represents.
func ratText(r *big.Rat) (string, bool) {
	den := new(big.Int).Set(r.Denom())
	two, five := big.NewInt(decimalBase2), big.NewInt(decimalBase5)
	twos, fives := 0, 0
	m := new(big.Int)
	for m.Mod(den, two).Sign() == 0 {
		den.Quo(den, two)
		twos++
	}
	for m.Mod(den, five).Sign() == 0 {
		den.Quo(den, five)
		fives++
	}
	if den.Cmp(big.NewInt(1)) != 0 {
		return "", false
	}
	return r.FloatString(max(twos, fives)), true
}

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

// maxDecimalLiteral caps a decimal literal's length, as the metamodel caps
// a stored decimal's.
const maxDecimalLiteral = 64

// parseDecimalText reads plain decimal notation, `[+-]?digits(.digits)?`.
// Exponents are refused: big.Rat would accept "1e1000000" and build a
// huge number from a short literal.
func parseDecimalText(s string) (*big.Rat, bool) {
	if s == "" || len(s) > maxDecimalLiteral {
		return nil, false
	}
	body := strings.TrimLeft(s, "+-")
	if len(s)-len(body) > 1 {
		return nil, false
	}
	intPart, frac, hasPoint := strings.Cut(body, ".")
	if !isDigits(intPart) || (hasPoint && !isDigits(frac)) {
		return nil, false
	}
	return new(big.Rat).SetString(s)
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
