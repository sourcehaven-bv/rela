package dataentry

import (
	"context"
	"fmt"
	"maps"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// entitySerializer renders an entity into its v1.Entity wire shape. Extracted
// from App (TKT-N26KLB). It does NO loading and holds no snapshot — the caller
// passes everything the transform needs as values: the entity, its already-
// loaded outgoing relations (nil to omit the relations map), and the metamodel
// snapshot (for DisplayTitle). Loading and snapshotting are the handler's job
// (it already holds both); serialization is a pure transform. The only field is
// the affordance service, which computes the per-request _actions / _fields /
// _relations maps and strips hidden fields (ACL-evaluated, hence the ctx).
type entitySerializer struct {
	affordances affordanceService
}

// toV1 builds the base v1.Entity. meta is the request's metamodel snapshot;
// outgoing is the entity's outgoing relations, already loaded by the caller
// (nil omits the relations map — the former includeRelations=false shape).
// incoming is the entity's incoming relations; when non-nil each incoming
// edge is added to the relations map under its relation's INVERSE key
// (inverseRelationKey), so an incoming list column has a distinct lookup key
// that never collides with the same relation used outgoing. The SPA computes
// the identical key from the column's relation + the metamodel inverse — see
// MECHANISM.md. Loading incoming edges is the caller's job; this stays a pure
// transform.
//
// visibleNeighbors gates the neighbor IDs written into the relations map
// (RR-HJV8CP): when non-nil, an outgoing target (edge.To) or incoming source
// (edge.From) is emitted ONLY if visibleNeighbors[id] is true, so the wire's
// `relations` map can never carry an ACL-hidden neighbor's raw ID (which would
// leak past the `included` map that is already visibility-filtered). Passing
// nil disables the filter — the per-entity / search / include shapes that
// serialize their own already-gated edges keep byte-identical output. The set
// is computed once per page by visibleRelationIDs (batched by type); this
// transform only consults it.
func (s entitySerializer) toV1(
	ctx context.Context, e *entityPkg.Entity, outgoing, incoming []*entityPkg.Relation,
	visibleNeighbors map[string]bool, meta *metamodel.Metamodel, plural string,
) v1.Entity {
	out := v1.Entity{
		ID:         e.ID,
		Type:       e.Type,
		Title:      meta.DisplayTitle(e.ID, e.Type, e.Properties),
		Properties: make(map[string]any),
		Content:    e.Content,
		Self:       selfHref(plural, e, meta),
		Actions:    s.affordances.computeActions(ctx, e),
	}

	maps.Copy(out.Properties, e.Properties)

	if e.IsLocked() {
		out.Inaccessible = make([]v1.InaccessibleField, 0, len(e.Inaccessible))
		for _, f := range e.Inaccessible {
			out.Inaccessible = append(out.Inaccessible, v1.InaccessibleField{
				Name:   f.Name,
				Reason: string(f.Reason),
			})
		}
	}

	if outgoing != nil || incoming != nil {
		// neighborVisible reports whether an edge endpoint may appear on the
		// wire. When visibleNeighbors is nil the filter is off (caller passes
		// already-gated edges); otherwise only IDs in the set survive.
		neighborVisible := func(id string) bool {
			return visibleNeighbors == nil || visibleNeighbors[id]
		}
		out.Relations = make(map[string][]string)
		for _, edge := range outgoing {
			if !neighborVisible(edge.To) {
				continue
			}
			out.Relations[edge.Type] = append(out.Relations[edge.Type], edge.To)
		}
		// Incoming edges are keyed by the relation's inverse name so an
		// `direction: incoming` column looks them up without colliding with
		// outgoing edges of the same relation. edge.From is the source
		// entity ID (resolved to a title client-side via ?include=*).
		for _, edge := range incoming {
			if !neighborVisible(edge.From) {
				continue
			}
			relDef, ok := meta.Relations[edge.Type]
			if !ok {
				continue
			}
			key := inverseRelationKey(edge.Type, relDef)
			out.Relations[key] = append(out.Relations[key], edge.From)
		}
	}

	return out
}

// forWire is the single entry-point every handler that returns a per-entity
// v1.Entity should use: toV1 + strip hidden properties + attach the affordance
// maps. Use forWireRelated for entities that appear as list rows or under
// `included` (no affordance maps, but still strip).
//
// The edges passed here are assumed ALREADY GATED — it forwards a nil
// neighbor filter. A caller holding edges whose heads have not been through
// the row gate must use [entitySerializer.forWireScoped] instead.
func (s entitySerializer) forWire(
	ctx context.Context, e *entityPkg.Entity, outgoing []*entityPkg.Relation,
	meta *metamodel.Metamodel, plural string,
) v1.Entity {
	return s.forWireScoped(ctx, e, outgoing, nil, meta, plural)
}

// forWireScoped is forWire with an explicit neighbor-visibility filter.
//
// It exists for the world path (TKT-WRLDAPI item 4), where the edges reaching
// the serializer were resolved by the world-scoped seam and their heads gated
// as a batch. Passing that set through means the `relations` map can only
// name a neighbor that this world resolves AND the principal may read —
// the same guarantee forWireRelated already gives list rows via
// visibleNeighbors, which the per-entity path previously had no way to
// express.
//
// visibleNeighbors nil disables the filter, which is what forWire passes: the
// default-world path's edges come from a reader whose heads the caller has
// already accounted for, and threading an empty (non-nil) map there would
// silently blank every relation.
func (s entitySerializer) forWireScoped(
	ctx context.Context, e *entityPkg.Entity, outgoing []*entityPkg.Relation,
	visibleNeighbors map[string]bool, meta *metamodel.Metamodel, plural string,
) v1.Entity {
	// Per-entity responses carry outgoing edges only; incoming edges reach the
	// SPA via the dedicated /relations endpoint, not the top-level relations
	// map. Incoming list columns are served by forWireRelated (list rows).
	result := s.toV1(ctx, e, outgoing, nil, visibleNeighbors, meta, plural)
	s.affordances.stripHiddenProperties(ctx, e, &result)
	s.affordances.attachEntityAffordances(ctx, e, &result)
	return result
}

// forWireHistoricalReveal renders a historical snapshot for an audit reader
// holding acl.PermHistoryReadRedacted (TKT-73C6B2): it SKIPS
// stripHiddenProperties entirely, emitting every frozen field — OVERRIDE
// semantics, the field-grained sibling of acl.PermHistoryRead's all-or-nothing
// audit power. This is the ONLY serializer path that intentionally does not
// strip; every ordinary read path (forWire / forWireRelated) strips. Callers
// MUST gate this on the permission (the history handler does) — it is not a
// general-purpose entry point. It omits the `_fields` / `_relations` affordance
// maps (those ride only on forWire, the per-entity live shape). It does carry
// the base `_actions` map from toV1 — computed against the LIVE graph, so on a
// snapshot it is at best advisory and, for a deleted entity, describes write
// affordances that no longer apply; it is a boolean UI hint, not a data leak,
// and the server re-authorizes every write regardless. The ordinary
// (non-reveal) history path shares that same toV1-`_actions` property.
func (s entitySerializer) forWireHistoricalReveal(
	ctx context.Context, e *entityPkg.Entity, meta *metamodel.Metamodel, plural string,
) v1.Entity {
	return s.toV1(ctx, e, nil, nil, nil, meta, plural)
}

// forWireRelated renders an entity that is NOT the per-entity response root —
// list rows, `?include=*` peers, search-result include map. Strips hidden
// properties but omits the `_fields` / `_relations` maps (those ride on
// per-entity responses only). Hidden-field stripping still applies: the wire
// contract is "hidden values never reach the client, regardless of shape."
// Pass incoming edges (nil to omit) so `direction: incoming` list columns can
// resolve their source entities under the relation's inverse key.
//
// visibleNeighbors gates the neighbor IDs (both outgoing targets and incoming
// sources) written into the relations map so a hidden neighbor's ID never
// leaks onto the wire (RR-HJV8CP). The list handler computes it once per page
// via visibleRelationIDs (batched by type — no per-id gate calls) and
// threads it here. Pass nil to disable filtering when the caller already
// serializes only gated edges (e.g. the search/include shapes pass nil
// relations anyway).
func (s entitySerializer) forWireRelated(
	ctx context.Context, e *entityPkg.Entity, outgoing, incoming []*entityPkg.Relation,
	visibleNeighbors map[string]bool, meta *metamodel.Metamodel, plural string,
) v1.Entity {
	result := s.toV1(ctx, e, outgoing, incoming, visibleNeighbors, meta, plural)
	s.affordances.stripHiddenProperties(ctx, e, &result)
	return result
}

// selfHref addresses the ROW this response describes, face included.
//
// `_self` used to be built from e.ID alone, which made it the bare id even
// when the response was a non-bare face — read `GUIDE-1@nl`, get back a
// pointer to `GUIDE-1`. A client doing the ordinary GET-`_self` / PATCH-`_self`
// loop then edited a DIFFERENT content state than the one it was displaying,
// silently.
//
// That is the same wrong-face write the `?world=` write refusal
// (422 world_read_only) exists to prevent, so handing it back in the response
// body defeated the guard rather than complementing it.
//
// The bare face keeps the bare href: it IS the bare id, and appending its
// declared name would break every existing client for no gain. A type with no
// faces is unaffected. This is deliberately NOT unified with [v1.Face.Ref],
// which spells the bare face as `ID@<bare_face>` so a face SWITCH can name it
// literally under a world: a WRITE never takes a world, so a PATCH or DELETE
// to the bare id always lands on the bare face — the row a page showing the
// bare face describes — and the bare href is therefore a correct write
// address everywhere. Only a READ of a bare id is world-resolved, and the
// face switcher is the one place the SPA reads by address (RR-RN3YGR).
func selfHref(plural string, e *entityPkg.Entity, meta *metamodel.Metamodel) string {
	if e.Face.IsDefault() {
		return fmt.Sprintf("/api/v1/%s/%s", plural, e.ID)
	}
	declared := metamodel.DeclaredFace(meta, e.Type, e.Face.String())
	if declared == "" {
		// An undeclared stored face: address it by its stored coordinate
		// rather than dropping it, so the pointer still round-trips to the row
		// the reader is looking at.
		declared = e.Face.String()
	}
	return fmt.Sprintf("/api/v1/%s/%s@%s", plural, e.ID, declared)
}
