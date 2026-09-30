package entity

// Address is an entity address as a caller wrote it: a bare id, or an id with
// a named face ("ID@face"). It differs from [Ref] in one way that matters: a
// bare id does NOT name the implicit face. It names no face at all, and only
// a resolver may turn it into a Ref, by choosing a face in a world (reads) or
// by counting the candidate faces (writes). See TKT-7IZHP0 design §2.
//
// The fields are unexported so that the one question that separates the two
// cases, [Address.Named], cannot be skipped by reading a Face field that is
// zero for both "no face named" and "the implicit face".
//
// The zero Address has no id and addresses nothing.
type Address struct {
	id    string
	face  Face
	named bool
}

// ParseAddress parses the boundary serialization "ID" or "ID@face" through
// [ParseStateRef]. "ID" yields an unnamed address; "ID@face" a named one.
func ParseAddress(s string) (Address, error) {
	id, face, err := ParseStateRef(s)
	if err != nil {
		return Address{}, err
	}
	if face.IsImplicit() {
		return BareAddress(id), nil
	}
	return Address{id: id, face: face, named: true}, nil
}

// BareAddress is the address of id with no face named. Like [Ref] it does
// not validate id; use [ParseAddress] for external input.
func BareAddress(id string) Address { return Address{id: id} }

// AddressOf is the address that names ref's face, the implicit face
// included. Named on the result always returns (ref, true).
//
// Its [Address.String] of an implicit-face ref is the bare id, which
// [ParseAddress] reads back as unnamed: the wire has no spelling for
// "the implicit face, named" (DEC-NPZICR ruling 1).
func AddressOf(ref Ref) Address { return Address{id: ref.ID, face: ref.Face, named: true} }

// ID returns the entity id the address names.
func (a Address) ID() string { return a.id }

// Named returns the Ref the address names and true, or the zero Ref and
// false when the address names no face.
func (a Address) Named() (Ref, bool) {
	if !a.named {
		return Ref{}, false
	}
	return Ref{ID: a.id, Face: a.face}, true
}

// String renders the boundary serialization via [FormatStateRef]: the bare
// id when no face is named or the named face is the implicit one.
func (a Address) String() string { return FormatStateRef(a.id, a.face) }
