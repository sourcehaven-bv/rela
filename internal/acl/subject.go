package acl

// Subject is what's being written. Sealed: only EntitySubject and
// RelationSubject implement it (via the unexported isSubject method).
// The sum exists so RelationSubject can carry both endpoints
// (FromID/ToID) without the source/target ambiguity that overloading
// EntityType produced in v0.
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
