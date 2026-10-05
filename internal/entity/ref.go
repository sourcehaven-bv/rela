package entity

import "fmt"

// Ref is the address of one face row: an entity id and a face coordinate.
// A faceless type's only face is the implicit face "" (DEC-NPZICR ruling 1),
// so its Ref has a zero Face. A Ref with no ID addresses nothing.
//
// Ref is comparable and is meant to be used as a map key. Its text form is
// the boundary serialization of [FormatStateRef] ("ID" or "ID@face"), so a
// Ref is a JSON string and a JSON map key with no wire change.
type Ref struct {
	ID   string
	Face Face
}

// ParseRef parses the boundary serialization "ID" or "ID@face" through
// [ParseStateRef]. A bare id yields a Ref with the zero face.
func ParseRef(s string) (Ref, error) {
	id, p, err := ParseStateRef(s)
	if err != nil {
		return Ref{}, err
	}
	return Ref{ID: id, Face: p}, nil
}

// String renders the boundary serialization via [FormatStateRef]. Like that
// function it does not validate; use [Ref.MarshalText] where an invalid
// value must be refused.
func (r Ref) String() string { return FormatStateRef(r.ID, r.Face) }

// IsZero reports whether r has no ID. Such a Ref addresses nothing,
// whatever its face.
func (r Ref) IsZero() bool { return r.ID == "" }

// MarshalText renders the boundary serialization, and refuses any Ref whose
// text does not parse back to the same value. A marshaled Ref therefore
// always round-trips, and the rule lives in [ParseRef] alone.
func (r Ref) MarshalText() ([]byte, error) {
	s := r.String()
	back, err := ParseRef(s)
	if err != nil {
		return nil, fmt.Errorf("invalid ref %#v: %w", r, err)
	}
	if back != r {
		return nil, fmt.Errorf("invalid ref %#v: not canonical (reads back as %#v)", r, back)
	}
	return []byte(s), nil
}

// UnmarshalText parses b with [ParseRef]. On error r is left unchanged.
// Nil: not supported, panics; r must point at a Ref to fill.
func (r *Ref) UnmarshalText(b []byte) error {
	parsed, err := ParseRef(string(b))
	if err != nil {
		return err
	}
	*r = parsed
	return nil
}
