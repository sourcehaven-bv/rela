package dataentry

import (
	"context"
	"fmt"

	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// entitySerializer renders an entity into its V1Entity wire shape. Extracted
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

// toV1 builds the base V1Entity. meta is the request's metamodel snapshot;
// outgoing is the entity's outgoing relations, already loaded by the caller
// (nil omits the relations map — the former includeRelations=false shape).
func (s entitySerializer) toV1(ctx context.Context, e *entityPkg.Entity, outgoing []*entityPkg.Relation, meta *metamodel.Metamodel, plural string) V1Entity {
	v1 := V1Entity{
		ID:         e.ID,
		Type:       e.Type,
		Title:      meta.DisplayTitle(e.ID, e.Type, e.Properties),
		Properties: make(map[string]interface{}),
		Content:    e.Content,
		Self:       fmt.Sprintf("/api/v1/%s/%s", plural, e.ID),
		Actions:    s.affordances.computeActions(ctx, e),
	}

	for k, v := range e.Properties {
		v1.Properties[k] = v
	}

	if e.IsLocked() {
		v1.Inaccessible = make([]V1InaccessibleField, 0, len(e.Inaccessible))
		for _, f := range e.Inaccessible {
			v1.Inaccessible = append(v1.Inaccessible, V1InaccessibleField{
				Name:   f.Name,
				Reason: string(f.Reason),
			})
		}
	}

	if outgoing != nil {
		v1.Relations = make(map[string][]string)
		for _, edge := range outgoing {
			v1.Relations[edge.Type] = append(v1.Relations[edge.Type], edge.To)
		}
	}

	return v1
}

// forWire is the single entry-point every handler that returns a per-entity
// V1Entity should use: toV1 + strip hidden properties + attach the affordance
// maps. Use forWireRelated for entities that appear as list rows or under
// `included` (no affordance maps, but still strip).
func (s entitySerializer) forWire(ctx context.Context, e *entityPkg.Entity, outgoing []*entityPkg.Relation, meta *metamodel.Metamodel, plural string) V1Entity {
	result := s.toV1(ctx, e, outgoing, meta, plural)
	s.affordances.stripHiddenProperties(ctx, e, &result)
	s.affordances.attachEntityAffordances(ctx, e, &result)
	return result
}

// forWireRelated renders an entity that is NOT the per-entity response root —
// list rows, `?include=*` peers, search-result include map. Strips hidden
// properties but omits the `_fields` / `_relations` maps (those ride on
// per-entity responses only). Hidden-field stripping still applies: the wire
// contract is "hidden values never reach the client, regardless of shape."
func (s entitySerializer) forWireRelated(ctx context.Context, e *entityPkg.Entity, outgoing []*entityPkg.Relation, meta *metamodel.Metamodel, plural string) V1Entity {
	result := s.toV1(ctx, e, outgoing, meta, plural)
	s.affordances.stripHiddenProperties(ctx, e, &result)
	return result
}
