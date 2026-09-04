package dataentry

import (
	"context"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// entityRef is an entity ADDRESS as it arrives in a URL path segment,
// parsed once at the HTTP boundary: the bare id, or `ID@face` naming one
// stored content state (TKT-SLFURL).
//
// # The face is part of the address, exactly as the id is
//
// A world is a read-side rule that picks a face when the caller names none.
// When the caller DOES name one, there is nothing left for the world to
// decide, so an explicit address is served literally under every world —
// the same row a default-world read of it returns — and the response's
// `_self` round-trips to the row on screen. Before this existed the write
// path accepted `ID@face` (and authorized the face it wrote, BUG-Y0GNSB) while
// the read path treated the whole string as an id: on fsstore that happened
// to hit the right index key, on pgstore it never matched, and under a
// configured `default_world` it 404'd everywhere. A client following the
// server's own `_self` broke on the GET.
//
// # Declared names, stored coordinates
//
// The path carries the operator's DECLARED face name, the same vocabulary
// `_world.face` and `_faces[].label` use. The type's `bare_face` is stored at
// the zero coordinate (design doc §2.1), so `POL-1@draft` with `bare_face:
// draft` addresses the same row as `POL-1` — Explicit is what records that
// the caller spelled it out, which is what exempts it from world resolution.
// A caller cannot tell the two apart by the response, only by what the world
// would otherwise have done with a bare id.
type entityRef struct {
	// ID is the bare entity id. Every ACL row gate keys on this: the row
	// gate is face-blind by design (guard rule 1) and a suffixed string
	// handed to it matches nothing under a query-shaped policy.
	ID string
	// Face is the STORED coordinate the address names; zero for the bare
	// face, whether the path spelled it (`ID@draft`) or not (`ID`).
	Face entity.Face
	// Explicit reports that the path named a face. An explicit address
	// bypasses world resolution; a bare one is resolved by the request's
	// world as before.
	Explicit bool
}

// parseEntityRef parses one path segment into an entityRef, mapping a
// declared face name onto its stored coordinate through the metamodel.
//
// Returns ok=false for anything the grammar rejects — an invalid id, two
// separators, a face that fails [entity.ParseFace]. Callers render that as
// the SAME not-found a missing entity produces: a syntactically impossible
// address cannot name a row, and a distinct 400 would only tell a caller
// which strings are worth probing.
//
// An undeclared face name is NOT rejected here. It maps to itself as a stored
// coordinate and the store answers whether such a row exists — the same
// answer `selfHref` gives an undeclared stored face, so a row written under a
// face the schema has since dropped stays addressable by the `_self` it hands
// out.
//
// m may be nil (bare test fixtures); the name is then taken as stored.
func parseEntityRef(m *metamodel.Metamodel, entityType, raw string) (entityRef, bool) {
	id, face, err := entity.ParseStateRef(raw)
	if err != nil {
		return entityRef{}, false
	}
	if face.IsDefault() {
		return entityRef{ID: id}, true
	}
	stored := metamodel.StoredFace(m, entityType, face.String())
	return entityRef{ID: id, Face: entity.Face(stored), Explicit: true}, true
}

// String renders the address back in its boundary serialization — the bare
// id for the zero coordinate, `ID@face` otherwise. Used for error detail and
// diagnostics, never as a store key.
func (ref entityRef) String() string {
	return entity.FormatStateRef(ref.ID, ref.Face)
}

// contentScopedRelationOn reports the first `scope: content` relation type a
// PATCH body names when the write is addressed to a NON-BARE face — the one
// combination the relation writers cannot honor, because they attach edges to
// the entity's bare tail (entity.RelationOptions carries no face).
//
// Keys that are not a declared relation type (an inverse name, a typo) pass
// through to the ordinary relation validation: an inverse edge's TAIL is the
// other entity, so its face is not this address's concern.
func contentScopedRelationOn(
	m *metamodel.Metamodel, ref entityRef, desired map[string]v1.RelationsUpdate,
) (relType string, refused bool) {
	if ref.Face.IsDefault() || m == nil {
		return "", false
	}
	for key := range desired {
		if def, ok := m.Relations[key]; ok && def.Scope.IsContent() {
			return key, true
		}
	}
	return "", false
}

// addressedProvenance labels a face served because the CALLER NAMED IT, as
// opposed to one the world resolved (see [worldProvenance]).
//
// The rule is `unscoped`: no resolution was applied. That is the same word a
// faceless type and the default world get, and it is the honest one here too
// — the world made no choice, the address did. Reporting the chain position
// the face happens to occupy in the request's world would claim the world
// picked it, and a client keying a stand-in badge on `chain_position` would
// then badge a page the reader navigated to on purpose.
//
// The world NAME is still the request's: neighbors and included peers on
// this response resolve through it, so naming it keeps the block truthful
// about everything on the page that a world did touch.
func addressedProvenance(ctx context.Context, m *metamodel.Metamodel, e *entity.Entity) *v1.EntityWorld {
	if e == nil {
		return nil
	}
	name := worldFromContext(ctx).name
	if name == "" {
		name = defaultWorldName
	}
	return &v1.EntityWorld{
		Name: name,
		Face: metamodel.DeclaredFace(m, e.Type, e.Face.String()),
		Via:  ruleUnscoped,
	}
}
