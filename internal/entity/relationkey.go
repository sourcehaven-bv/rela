package entity

// RelationKey is the identity of one relation: the source, its tail face,
// the relation type and the target. The tail is part of the identity
// (TKT-C1XUA8): two edges on one triple with different tails are two
// relations. The zero FromFace is an identity-scoped edge, or the implicit
// face of a faceless source; both are real coordinates, not a default.
// Heads are entity-level, so there is no ToFace.
//
// RelationKey is comparable and is meant to be used as a map key.
type RelationKey struct {
	From     string
	FromFace Face
	Type     string
	To       string
}

// Identity returns the key that identifies r.
func (r *Relation) Identity() RelationKey {
	return RelationKey{From: r.From, FromFace: r.FromFace, Type: r.Type, To: r.To}
}

// Tail returns the address of the face row the edge attaches to.
func (k RelationKey) Tail() Ref { return Ref{ID: k.From, Face: k.FromFace} }

// String renders the key's text form. The format is stable: it is what
// [Relation.Key] returns, and pgstore's relation paging cursor encodes and
// parses it, so a change here breaks cursors that clients already hold.
//
// The FROM slot carries the tail face via [FormatStateRef] (TKT-DOFYR1).
// The face grammar forbids "--", which keeps the text unambiguous, and a
// key with the zero tail renders as the historical "FROM--type--TO".
func (k RelationKey) String() string {
	return FormatStateRef(k.From, k.FromFace) + "--" + k.Type + "--" + k.To
}
