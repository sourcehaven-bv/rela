package dataentry

import (
	"context"
	"slices"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// relationOrdering is a collection read in relation order: the rows are the
// targets of one anchor's edges over one relation whose outgoing side is
// orderable, and they are shown in the order those edges hold. A list tab
// scoped to its anchor and a detail-view section over one traversal are the
// two collections that can be one.
//
// Nil: accepted by every method — the collection is not relation-ordered.
type relationOrdering struct {
	relation string
	// anchor is the address a move PATCH names the anchor by: its id, with
	// its face when the relation is content-scoped, because only that face's
	// edges are listed and a faced type's bare id names no face.
	anchor string
	// anchorType is the anchor's entity type, which names its route.
	anchorType string
	movable    bool
	keys       map[string]metamodel.OrderKey // row id → its edge's place
}

// newRelationOrdering builds the ordering for anchor's edges, or returns nil
// when the relation is not orderable on the outgoing side or the principal
// may not read the order value. Ordering by a value the principal cannot
// see would disclose it.
//
// edges are the anchor's edges of the relation that the collection shows,
// already narrowed to the anchor's own face.
func newRelationOrdering(
	ctx context.Context, svc affordanceService, meta *metamodel.Metamodel,
	anchor *entityPkg.Entity, relType string, edges []*entityPkg.Relation,
) *relationOrdering {
	def, ok := meta.Relations[relType]
	if !ok || def.OutgoingOrderProperty() == "" {
		return nil
	}
	probe := map[string]any{metamodel.OrderPropertyOut: 0.0}
	if _, visible := svc.visibleRelationMeta(ctx, anchor, relType, probe)[metamodel.OrderPropertyOut]; !visible {
		return nil
	}
	addr := anchor.ID
	if metamodel.IsContentScoped(meta, relType) {
		addr = entityPkg.FormatStateRef(anchor.ID, anchor.Face)
	}
	o := &relationOrdering{
		relation:   relType,
		anchor:     addr,
		anchorType: anchor.Type,
		movable:    relationOrderMovable(ctx, svc, anchor, relType),
		keys:       make(map[string]metamodel.OrderKey, len(edges)),
	}
	for _, e := range edges {
		k := metamodel.OrderKey{Value: e.Properties[metamodel.OrderPropertyOut], Peer: e.To, Tail: string(e.FromFace)}
		// Two tails of the anchor can reach one row; the earlier place wins.
		if prev, seen := o.keys[e.To]; !seen || metamodel.CompareOrderKeys(k, prev) < 0 {
			o.keys[e.To] = k
		}
	}
	return o
}

// relationOrderMovable reports whether the principal may move anchor's
// edges of relType: the write the manager would authorize, and the order
// field writable under the meta-field affordances — the same two gates a
// position PATCH meets.
func relationOrderMovable(
	ctx context.Context, svc affordanceService, anchor *entityPkg.Entity, relType string,
) bool {
	var tail entityPkg.Face
	if metamodel.IsContentScoped(svc.meta(), relType) {
		tail = anchor.Face
	}
	sources, err := svc.relationSources(ctx, anchor, entityPkg.Ref{ID: anchor.ID, Face: tail}, "", relType)
	if err != nil {
		return false
	}
	order := map[string]any{metamodel.OrderPropertyOut: 0.0}
	if _, denial := svc.relationMetaDenial(ctx, sources, relType, order, nil); denial != nil {
		return false
	}
	req := translateRelationWrite(svc.meta(), relType, anchor.Type, anchor.ID, tail)
	return svc.acl().AuthorizeWrite(ctx, req).Allow
}

// sort orders rows by their edges' places, with [metamodel.CompareOrderKeys].
func (o *relationOrdering) sort(rows []*entityPkg.Entity) {
	if o == nil {
		return
	}
	slices.SortStableFunc(rows, func(a, b *entityPkg.Entity) int {
		return metamodel.CompareOrderKeys(o.keyOf(a.ID), o.keyOf(b.ID))
	})
}

func (o *relationOrdering) keyOf(id string) metamodel.OrderKey {
	if k, ok := o.keys[id]; ok {
		return k
	}
	return metamodel.OrderKey{Peer: id}
}

// wire is the ordering as the client sees it.
func (o *relationOrdering) wire() *v1.RelationOrder {
	if o == nil {
		return nil
	}
	return &v1.RelationOrder{Relation: o.relation, Anchor: o.anchor, AnchorType: o.anchorType, Movable: o.movable}
}

// sectionOrdering returns the relation ordering a flat section is shown in,
// or nil. A section qualifies when its rows come from exactly one
// non-recursive `follow:` rule from `entry` (see [viewResult.EntryEdges])
// over a relation orderable on the outgoing side, and the section declares
// neither `sort:` nor `group_by:`: an author's own sort wins, and grouping
// re-orders rows by group.
//
// A package function rather than a viewsHandler method: the handler is at
// its plimsoll method line.
func sectionOrdering(ctx context.Context, h *viewsHandler, sec ViewSection, result *viewResult) *relationOrdering {
	if len(sec.Sort) > 0 || sec.GroupBy != "" || sec.Display == dataentryconfig.DisplayNested {
		return nil
	}
	ee, ok := result.EntryEdges[sec.Source]
	if !ok || result.Entry == nil {
		return nil
	}
	return newRelationOrdering(ctx, h.affordances, h.schema().Meta, result.Entry, ee.relation, ee.edges)
}
