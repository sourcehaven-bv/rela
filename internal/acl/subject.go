package acl

import "github.com/Sourcehaven-BV/rela/internal/entity"

// Subject is what's being written. Sealed: only EntitySubject and
// RelationSubject implement it (via the unexported isSubject method).
// The sum exists so RelationSubject can carry both endpoints
// (FromID/ToID) without the source/target ambiguity that overloading
// EntityType produced in v0.
//
// The sealed sum, visually:
//
//	Subject (sealed)
//	├── EntitySubject   { typ, id, face }  (built via NewEntitySubject /
//	│                   NewFacelessEntitySubject; face zero = default face)
//	│       used for: Create / Update / Delete / Rename of an entity
//	└── RelationSubject { Type, FromType, FromID }
//	                    (default tail only — see authorizeRelationWrite)
//	        used for: Create / Delete of a relation
//
// A nil Subject is a programmer error. AuthorizeWrite panics so the
// bug surfaces at the call site rather than silently denying or
// silently allowing.
type Subject interface{ isSubject() }

// EntitySubject identifies an entity write target.
//
//	Op=Create   → ID is empty (no ID yet at the time of authz).
//	Op=Update   → ID is the entity being mutated.
//	Op=Delete   → ID is the entity being removed.
//	Op=Rename   → ID is the entity before the rename.
//
// # Why the fields are unexported
//
// All three fields are unexported and the value is built only by
// [NewEntitySubject] or [NewFacelessEntitySubject]. This is the whole
// point of the type (BUG-Y0GNSB P5): `face` has a MEANINGFUL zero value
// — the default face — so a composite literal that simply forgot it did
// not fail to build, it authorized the WRONG face and returned 200. A
// constructor argument cannot be forgotten; the compiler asks for it at
// every site.
//
// `typ` and `id` are unexported for the same mechanical reason rather
// than for encapsulation: leaving them exported would keep
// `acl.EntitySubject{Type: t, ID: i}` a legal literal, which is exactly
// the shape of the defect. With all three unexported the literal does
// not compile at all, so there is no silent path back.
//
// The accessors [EntitySubject.Type], [EntitySubject.ID] and
// [EntitySubject.Face] read the value back: the type is also a READ
// surface, consumed by authorizeEntityWrite here and by entitymanager's
// audit records for denied and bypassed writes.
type EntitySubject struct {
	typ string
	id  string

	// face names the CONTENT STATE (face) being written; the zero value
	// is the default state (TKT-C1XUA8).
	//
	// The zero value still MEANS the default face — that semantic is
	// load-bearing and unchanged, and is what
	// TestExistingGrantsUnchangedByFaceField pins: every grant in every
	// existing acl.yaml keeps its meaning, because [GrantsVerbOnState]
	// treats a bare-type grant as covering exactly the default face.
	// What P5 changes is not the meaning of the zero value but who may
	// produce it: only a caller that said so, via
	// [NewFacelessEntitySubject].
	//
	// Why the subject and not the Op: a copy is not a new verb (the copy's
	// own guard is the real gate), and adding one would need grant syntax
	// nobody has asked for in four switch sites. What changes between
	// writing a draft and writing a published face is WHICH FACE, which is
	// a property of the subject.
	face entity.Face
}

// NewEntitySubject builds an entity write subject that names its face.
//
// This is the constructor every write path should use. Source the face
// from the entity actually BEING WRITTEN (the stored row's face, not a
// caller-supplied body face) — the store resolves `ID@face` to a row and
// populates `.Face` on it, and that is the coordinate the write lands at,
// so it is the coordinate the ACL must be asked about.
//
// Passing the zero face here is legal and means the default face. That is
// correct for an unfaced type; it is a deliberate statement either way,
// which is the property P5 buys. If the operation genuinely has NO single
// face, say so with [NewFacelessEntitySubject] instead of passing "".
func NewEntitySubject(typ, id string, face entity.Face) EntitySubject {
	return EntitySubject{typ: typ, id: id, face: face}
}

// NewFacelessEntitySubject builds an entity write subject for an operation
// that has no single face to name, and authorizes it against the DEFAULT
// face.
//
// Use it only when naming a face would assert a NARROWER scope than the
// operation actually has. The one production case is rename: it re-keys
// the whole entity family (fsstore.renameEntity walks stateFamily(oldID)
// and store.RenameEntity takes no Face), so authorizing "rename of the
// draft face" would describe an operation that is not the one running.
//
// It is deliberately a second, longer, differently-named function rather
// than a default or an optional argument: the faceless case must be
// expressible, but it must be LOUD, greppable and justified at the call
// site. Every use should carry a comment saying why the operation spans
// all faces.
func NewFacelessEntitySubject(typ, id string) EntitySubject {
	return EntitySubject{typ: typ, id: id}
}

// Type is the entity type being written.
func (s EntitySubject) Type() string { return s.typ }

// ID is the entity being written; empty for Op=Create, which has no id yet.
func (s EntitySubject) ID() string { return s.id }

// Face is the content state being written; the zero value is the default face.
func (s EntitySubject) Face() entity.Face { return s.face }

func (EntitySubject) isSubject() {} // coverage-ignore: sealing marker: never called at runtime; exists only so
// EntitySubject satisfies the sealed Subject interface (compiler-checked), so no test can reach it without an
// artificial no-op call

// RelationSubject identifies a relation write. v1 evaluates relation
// writes against `FromType` only (matching v0 semantics — see the
// "S13" thread in the TKT-SVXL design log). The v0 quirk of
// EntityType meaning "source type for relation writes" is gone.
//
// The To side is intentionally absent (RR-F9M9): the resolver doesn't
// read it today, and forcing callers to populate it costs an extra
// store round-trip per relation write. A future per-link verdict
// feature that wants asymmetric grants (e.g. "may create editor-of
// edges only to entities of type project") can add it back with a
// clear semantic at that time.
type RelationSubject struct {
	Type     string // relation type (e.g. "editor-of")
	FromType string
	FromID   string

	// FromFace names the SOURCE's content state, for a `scope: content`
	// relation type whose edges belong to one face rather than to the
	// entity as such (TKT-DOFYR1). The zero value is the default state.
	//
	// Only the source carries a face: [entity.Relation] has no ToFace, so
	// there is nothing on the target side to authorize (BUG-64MU2Q).
	//
	// The zero value keeps every existing grant's meaning, for the same
	// reason [EntitySubject.Face] does: a write that names no face
	// addresses the default one, and a bare-type grant covers exactly
	// that. An identity-scoped edge is entity-level and always leaves
	// this zero.
	FromFace entity.Face
}

func (RelationSubject) isSubject() {} // coverage-ignore: sealing marker: never called at runtime; exists only so
// RelationSubject satisfies the sealed Subject interface (compiler-checked), so no test can reach it without an
// artificial no-op call
