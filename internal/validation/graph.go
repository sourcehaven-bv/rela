package validation

import "context"

// Direction selects which end of an edge the subject sits on.
//
// It is declared here, in validation-local terms, rather than reusing
// store.Direction: internal/validation may not depend on internal/store
// (.go-arch-lint.yml), and the gate should state what it needs of the graph
// without naming a storage type. The adapter translates.
type Direction int

const (
	// DirectionOutgoing counts edges whose SOURCE is the subject. The zero
	// value, so a constraint that says nothing about direction behaves
	// exactly as every rule written before direction existed.
	DirectionOutgoing Direction = iota

	// DirectionIncoming counts edges whose TARGET is the subject.
	DirectionIncoming
)

// String renders the direction as it is spelled in schema.yaml.
func (d Direction) String() string {
	if d == DirectionIncoming {
		return "incoming"
	}
	return "outgoing"
}

// Related is ONE EDGE, with the entity at its far end resolved where that
// was possible.
//
// # One element per edge — ids may repeat
//
// A relation's identity is (From, FromFace, Type, To), not the triple: two
// edges on the same triple with different tail faces are two relations (see
// [entity.Relation.Key]). A subject can therefore hold several edges of one
// type to the SAME target, and the gate counts each of them.
//
// So this is deliberately not a set of targets. Collapsing it to one entry
// per distinct id would silently re-scope every constraint that counts
// face-tailed edges — the kind of change that looks like a tidy-up and
// alters verdicts. A future batched implementation must preserve this.
//
// # Resolved is the fail-closed signal
//
// Resolved=false means "this edge exists, but its far entity could not be
// read" — a dangling reference, or a target the reader may not see. That is
// NOT the same as "no edge", and the distinction is load-bearing: a `max:`
// bound counts an unevaluable target as matching, so folding an unresolved
// edge away would let a `max: 0` gate report satisfied precisely because
// the thing it guards against could not be checked.
//
// When Resolved is false, Type and Properties carry no meaning.
type Related struct {
	// ID is the far entity's id: the edge's target for an outgoing
	// constraint, its source for an incoming one. The adapter applies
	// direction, so a caller never picks an end — picking the wrong one
	// yields plausible numbers about the wrong entities.
	ID string

	// Type is the far entity's declared type. Empty when !Resolved.
	Type string

	// Properties are the far entity's properties, for `where` matching.
	// Nil when !Resolved.
	Properties map[string]any

	// Resolved reports whether the far entity was read. See the type doc.
	Resolved bool
}

// Graph is the graph-read surface a relation-cardinality gate needs.
//
// Declared at the consumer (CLAUDE.md "interfaces at the call site"), in the
// same shape internal/acl uses for [acl.Graph]: the wiring site supplies an
// implementation, and this package holds no store handle and no opinion about
// how an edge is resolved. That is what lets a caller decide — later, without
// touching the evaluator — that targets should resolve within a particular
// world.
//
// Nil: rejected — [Service.Check] reports a constraint it cannot evaluate
// rather than treating it as satisfied. A gate that silently passes because
// nothing wired it is indistinguishable from a gate that verified something.
type Graph interface {
	// RelatedEntities returns one [Related] per edge of relType incident to
	// subjectID on the side dir selects, in graph order.
	//
	// resolveFar asks for the far entities to be READ. Pass false when the
	// caller will only count edges: a constraint with no `where` and no
	// `target_type` inspects nothing about the far end, and reading it anyway
	// costs one lookup per edge per entity — a per-row read on what is a
	// collection scan. Elements then come back with Resolved=false and ID set,
	// which is all such a caller needs.
	//
	// An error means the edge lookup itself failed; the caller reports the
	// constraint as unevaluable rather than counting zero. A far entity that
	// could not be read is NOT an error — it comes back with Resolved=false
	// so the caller keeps its per-bound fail-closed choice.
	//
	// Note the two sources of Resolved=false are deliberately the same value:
	// "not read because nobody asked" and "asked, could not read". A caller
	// passing false has said it will not inspect the far end, so it must not
	// then branch on Resolved.
	RelatedEntities(
		ctx context.Context, subjectID, relType string, dir Direction, resolveFar bool,
	) ([]Related, error)
}

// NullGraph is a [Graph] with no edges, for tests that build a Service but
// exercise no relation constraint. Production wiring always supplies a real
// store-backed implementation.
type NullGraph struct{}

// RelatedEntities always returns no edges.
func (NullGraph) RelatedEntities(
	context.Context, string, string, Direction, bool,
) ([]Related, error) {
	return nil, nil
}
