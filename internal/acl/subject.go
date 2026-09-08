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
//	├── EntitySubject   { Type, ID, Face }
//	│                   (Face zero = default face)
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
type EntitySubject struct {
	Type string
	ID   string

	// Face names the CONTENT STATE (face) being written; the zero value
	// is the default state (TKT-C1XUA8).
	//
	// The zero value is what makes this field safe to add under every
	// existing caller: a write that does not name a face addresses the
	// default one, and [GrantsVerbOnState] treats a bare-type grant as
	// covering exactly that. So every grant in every existing acl.yaml
	// keeps its meaning, which is the property
	// TestExistingGrantsUnchangedByFaceField pins.
	//
	// Why the subject and not the Op: a copy is not a new verb (the copy's
	// own guard is the real gate), and adding one would need grant syntax
	// nobody has asked for in four switch sites. What changes between
	// writing a draft and writing a published face is WHICH FACE, which is
	// a property of the subject.
	Face entity.Face
}

func (EntitySubject) isSubject() {}

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
}

func (RelationSubject) isSubject() {}
