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

// toV1 builds the base v1.Entity, a pure transform. outgoing and incoming are
// relations loaded by the caller (nil omits them). Incoming edges go under
// the relation's INVERSE key (inverseRelationKey), which the SPA computes the
// same way. visibleNeighbors, when non-nil, emits a neighbor id only if it is
// true in the set, so `relations` never carries an ACL-hidden neighbor's id;
// nil disables the filter.
func (s entitySerializer) toV1(
	ctx context.Context, e *entityPkg.Entity, outgoing, incoming []*entityPkg.Relation,
	visibleNeighbors map[string]bool, meta *metamodel.Metamodel, plural string,
) v1.Entity {
	out := v1.Entity{
		ID:          e.ID,
		Type:        e.Type,
		Title:       meta.DisplayTitle(e.ID, e.Type, e.Properties),
		Properties:  make(map[string]any),
		Content:     e.Content,
		EntityLinks: v1.EntityLinks{Self: selfHref(plural, e)},
		Actions:     s.affordances.computeActions(ctx, e),
	}

	maps.Copy(out.Properties, e.Properties)
	for _, name := range meta.RollupNames(e.Type) {
		delete(out.Properties, name)
	}

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

// forWireScoped is forWire with an explicit neighbor-visibility filter, for
// the world path, so the `relations` map names only neighbors the world
// resolves and the principal may read. A nil visibleNeighbors disables the
// filter; an empty non-nil map blanks every relation.
func (s entitySerializer) forWireScoped(
	ctx context.Context, e *entityPkg.Entity, outgoing []*entityPkg.Relation,
	visibleNeighbors map[string]bool, meta *metamodel.Metamodel, plural string,
) v1.Entity {
	result := s.toV1(ctx, e, outgoing, nil, visibleNeighbors, meta, plural)
	s.affordances.stripHiddenProperties(ctx, e, &result)
	s.affordances.attachEntityAffordances(ctx, e, &result)
	return result
}

// forWireHistoricalReveal renders a historical snapshot for an audit reader
// holding acl.PermHistoryReadRedacted. It is the ONLY serializer path that
// does not strip hidden properties. Callers MUST gate it on that permission.
// It omits the `_fields` / `_relations` maps; its `_actions` map is computed
// against the live graph and is only an advisory UI hint.
func (s entitySerializer) forWireHistoricalReveal(
	ctx context.Context, e *entityPkg.Entity, meta *metamodel.Metamodel, plural string,
) v1.Entity {
	return s.toV1(ctx, e, nil, nil, nil, meta, plural)
}

// forWireRelated renders an entity that is not the per-entity response root:
// list rows, `?include=*` peers, search includes. It strips hidden properties
// and omits the `_fields` / `_relations` maps. Pass incoming edges (nil to
// omit). visibleNeighbors gates the neighbor ids in the relations map so a
// hidden neighbor's id never reaches the wire; nil disables the filter.
func (s entitySerializer) forWireRelated(
	ctx context.Context, e *entityPkg.Entity, outgoing, incoming []*entityPkg.Relation,
	visibleNeighbors map[string]bool, meta *metamodel.Metamodel, plural string,
) v1.Entity {
	result := s.toV1(ctx, e, outgoing, incoming, visibleNeighbors, meta, plural)
	s.affordances.stripHiddenProperties(ctx, e, &result)
	return result
}

// selfHref addresses the ROW this response describes, face included, so a
// GET-`_self` / PATCH-`_self` loop edits the face it displayed. A faced type
// has no bare address and always carries its coordinate. A type with no faces
// keeps the bare href.
func selfHref(plural string, e *entityPkg.Entity) string {
	if e.Face.IsImplicit() {
		return fmt.Sprintf("/api/v1/%s/%s", plural, e.ID)
	}
	return fmt.Sprintf("/api/v1/%s/%s@%s", plural, e.ID, e.Face.String())
}
