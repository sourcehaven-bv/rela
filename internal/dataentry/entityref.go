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
// # One spelling per face
//
// The path carries the operator's declared face name, the same vocabulary
// `_world.face` and `_faces[].label` use, and that name IS the coordinate the
// row is stored at (BUG-HC6I2T) — so an address needs no translation and no
// face answers to two spellings. An unsuffixed id names a row only for a type
// declaring no faces; for a faced type it names no row at all, and the
// request's world is what turns it into one.
type entityRef struct {
	// ID is the bare entity id. Every ACL row gate keys on this: the row
	// gate is face-blind by design (guard rule 1) and a suffixed string
	// handed to it matches nothing under a query-shaped policy.
	ID string
	// Face is the coordinate the address names; zero when the path named no
	// face, which is a row only for a type declaring none.
	Face entity.Face
	// Explicit reports that the path named a face. An explicit address
	// bypasses world resolution; a bare one is resolved by the request's
	// world as before.
	Explicit bool
}

// parseEntityRef parses one path segment into an entityRef.
//
// Returns ok=false for anything the grammar rejects — an invalid id, two
// separators, a face that fails [entity.ParseFace]. Callers render that as
// the SAME not-found a missing entity produces: a syntactically impossible
// address cannot name a row, and a distinct 400 would only tell a caller
// which strings are worth probing.
//
// An undeclared face name is NOT rejected here. It is taken as the coordinate
// it spells and the store answers whether such a row exists — the same answer
// `selfHref` gives an undeclared face, so a row written under a face the
// schema has since dropped stays addressable by the `_self` it hands out.
// That is why the metamodel is not consulted: a name needs no lookup to
// become a coordinate.
func parseEntityRef(raw string) (entityRef, bool) {
	id, face, err := entity.ParseStateRef(raw)
	if err != nil {
		return entityRef{}, false
	}
	if face.IsDefault() {
		return entityRef{ID: id}, true
	}
	return entityRef{ID: id, Face: face, Explicit: true}, true
}

// bareEntityID parses an address and returns its BARE id, for the surfaces
// that are addressed per entity rather than per row: attachments (files are
// keyed by entity id in every store), documents, commands, scope navigation.
// ok=false for an address the grammar rejects, rendered as the uniform
// not-found by the caller.
func bareEntityID(raw string) (string, bool) {
	ref, ok := parseEntityRef(raw)
	if !ok {
		return "", false
	}
	return ref.ID, true
}

// String renders the address back in its boundary serialization — the bare
// id for the zero coordinate, `ID@face` otherwise. Used for error detail and
// diagnostics, never as a store key.
func (ref entityRef) String() string {
	return entity.FormatStateRef(ref.ID, ref.Face)
}

// contentScopedRelationOn reports the first `scope: content` relation type a
// PATCH body names when the write is addressed to a NAMED face — the one
// combination the relation writers cannot honor, because they attach edges to
// the entity's zero-coordinate tail (entity.RelationOptions carries no face).
//
// Keys resolve exactly as the writer resolves them ([resolveDirection]), so
// the guard and the executor cannot disagree about which entity is the tail:
// an INCOMING edge's tail is the peer and passes through, but a SYMMETRIC
// relation's inverse spelling still makes this address the tail and is
// refused like the canonical name. An unknown key passes through to the
// ordinary relation validation, which rejects it.
func contentScopedRelationOn(
	m *metamodel.Metamodel, ref entityRef, desired map[string]v1.RelationsUpdate,
) (relType string, refused bool) {
	if ref.Face.IsDefault() || m == nil {
		return "", false
	}
	for key := range desired {
		canonical, incoming, ok := resolveDirection(m, key)
		if !ok || incoming {
			continue
		}
		if def, found := m.Relations[canonical]; found && def.Scope.IsContent() {
			return canonical, true
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
func addressedProvenance(ctx context.Context, e *entity.Entity) *v1.EntityWorld {
	if e == nil {
		return nil
	}
	name := worldFromContext(ctx).name
	if name == "" {
		name = defaultWorldName
	}
	return &v1.EntityWorld{
		Name: name,
		Face: e.Face.String(),
		Via:  ruleUnscoped,
	}
}
