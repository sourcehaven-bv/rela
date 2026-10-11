package dataentry

import (
	"context"
	"log/slog"
	"slices"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// relationOrdering is a collection read in relation order: the rows are the
// peers of one anchor's edges over one relation whose side is orderable, and
// they are shown in the order those edges hold. On the outgoing side the
// rows are the targets and the order is `_order_out`; on the incoming side
// the rows are the sources and the order is `_order_in`. A list tab scoped
// to its anchor and a detail-view section over one traversal are the two
// collections that can be one.
//
// Nil: accepted by every method — the collection is not relation-ordered.
type relationOrdering struct {
	relation string
	incoming bool
	// anchor is the address a move PATCH names the anchor by: its id, with
	// its face when only that face's edges are listed (an outgoing,
	// content-scoped relation) or when the anchor has a face at all (the
	// incoming side), because a faced type's bare id names no face.
	anchor string
	// anchorType is the anchor's entity type, which names its route.
	anchorType string
	movable    bool
	keys       map[string]metamodel.OrderKey // row id → its edge's place
	// addresses names, on the incoming side, the source address of each row
	// whose place comes from an edge on a face: a move names the row by it.
	addresses map[string]string
}

// newRelationOrdering builds the ordering for anchor's edges on one side,
// or returns nil when the relation is not orderable on that side or the
// principal may not read the order value: on the incoming side, on any of
// the edges. Ordering by a value the principal cannot see would disclose it.
//
// edges are the anchor's edges of the relation that the collection shows:
// on the outgoing side already narrowed to the anchor's own face, on the
// incoming side the edges into the anchor that the principal may read. The
// caller gates incoming edges before their places are folded into the
// keys, because a source's hidden face may link too and its place must not
// decide where the row shows, nor whether the list is movable.
func newRelationOrdering(
	ctx context.Context, svc affordanceService, meta *metamodel.Metamodel,
	anchor *entityPkg.Entity, relType string, incoming bool, edges []*entityPkg.Relation,
) (*relationOrdering, error) {
	prop := orderPropertyOf(meta, relType, incoming)
	o := &relationOrdering{
		relation:   relType,
		incoming:   incoming,
		anchor:     anchor.ID,
		anchorType: anchor.Type,
		keys:       make(map[string]metamodel.OrderKey, len(edges)),
	}
	shown := prop != ""
	var err error
	switch {
	case !shown:
	case incoming:
		shown, err = o.gateIncoming(ctx, svc, anchor, prop, edges)
	default:
		shown = o.gateOutgoing(ctx, svc, meta, anchor, prop)
	}
	if err != nil || !shown {
		return nil, err
	}
	o.fold(prop, edges)
	return o, nil
}

// gateIncoming reports whether the order of an incoming list may be shown,
// and sets its anchor address and movable. The `visible:` grant on a
// relation's fields resolves against each edge's source, so every source
// must let the principal read the order value.
func (o *relationOrdering) gateIncoming(
	ctx context.Context, svc affordanceService, anchor *entityPkg.Entity, prop string, edges []*entityPkg.Relation,
) (bool, error) {
	groups, err := incomingSources(ctx, svc, o.relation, edges)
	if err != nil {
		return false, err
	}
	probe := map[string]any{prop: 0.0}
	for _, g := range groups {
		if len(g.sources) == 0 {
			return false, nil
		}
		for _, src := range g.sources {
			if src.row == nil {
				return false, nil
			}
			if _, visible := svc.visibleRelationMeta(ctx, src.row, o.relation, probe)[prop]; !visible {
				return false, nil
			}
		}
	}
	if anchor.Face != entityPkg.ImplicitFace {
		o.anchor = entityPkg.FormatStateRef(anchor.ID, anchor.Face)
	}
	o.movable = incomingOrderMovable(ctx, svc, o.relation, groups)
	return true, nil
}

// gateOutgoing reports whether the order of an outgoing list may be shown,
// and sets its anchor address and movable.
func (o *relationOrdering) gateOutgoing(
	ctx context.Context, svc affordanceService, meta *metamodel.Metamodel, anchor *entityPkg.Entity, prop string,
) bool {
	probe := map[string]any{prop: 0.0}
	if _, visible := svc.visibleRelationMeta(ctx, anchor, o.relation, probe)[prop]; !visible {
		return false
	}
	if metamodel.IsContentScoped(meta, o.relation) {
		o.anchor = entityPkg.FormatStateRef(anchor.ID, anchor.Face)
	}
	o.movable = relationOrderMovable(ctx, svc, anchor, o.relation)
	return true
}

// fold records each row's place from edges, and on the incoming side the
// address of each row whose place comes from an edge on a face.
func (o *relationOrdering) fold(prop string, edges []*entityPkg.Relation) {
	tails := map[string]entityPkg.Face{}
	for _, e := range edges {
		k := metamodel.OrderKey{Value: e.Properties[prop], Peer: e.To, Tail: string(e.FromFace)}
		if o.incoming {
			k.Peer = e.From
		}
		// Two tails can join the anchor and one row; the earlier place wins.
		if prev, seen := o.keys[k.Peer]; !seen || metamodel.CompareOrderKeys(k, prev) < 0 {
			o.keys[k.Peer] = k
			tails[k.Peer] = e.FromFace
		}
	}
	if !o.incoming {
		return
	}
	for peer, tail := range tails {
		if tail == entityPkg.ImplicitFace {
			continue
		}
		if o.addresses == nil {
			o.addresses = map[string]string{}
		}
		o.addresses[peer] = entityPkg.FormatStateRef(peer, tail)
	}
}

// incomingSource is the source of one or more edges into an anchor, at
// one tail, with the rows its affordance gate judges.
type incomingSource struct {
	from    string
	tail    entityPkg.Face
	sources []relationSource
}

// incomingSources returns the distinct source tails of edges, each with
// [affordanceService.relationSources] over source rows read in one batch,
// so the cost does not grow with the number of edges.
func incomingSources(
	ctx context.Context, svc affordanceService, relType string, edges []*entityPkg.Relation,
) ([]incomingSource, error) {
	seen := map[entityPkg.Ref]bool{}
	refs := make([]entityPkg.Ref, 0, len(edges))
	for _, e := range edges {
		ref := entityPkg.Ref{ID: e.From, Face: e.FromFace}
		if !seen[ref] {
			seen[ref] = true
			refs = append(refs, ref)
		}
	}
	page, err := batchedSources(ctx, svc, refs, svc.meta().Relations[relType].Scope)
	if err != nil {
		return nil, err
	}
	out := make([]incomingSource, 0, len(refs))
	for _, ref := range refs {
		sources, err := page.relationSources(ctx, nil, ref, string(DirectionIncoming), relType)
		if err != nil {
			return nil, err
		}
		out = append(out, incomingSource{from: ref.ID, tail: ref.Face, sources: sources})
	}
	return out, nil
}

// siblingSources is every row the `_order_in` affordance gate judges for an
// incoming move over edges: the sources of all of them, since a densify may
// rewrite any.
func siblingSources(
	ctx context.Context, svc affordanceService, relType string, edges []*entityPkg.Relation,
) ([]relationSource, error) {
	groups, err := incomingSources(ctx, svc, relType, edges)
	if err != nil {
		return nil, err
	}
	var out []relationSource
	for _, g := range groups {
		out = append(out, g.sources...)
	}
	return out, nil
}

// incomingOrderMovable reports whether the principal may move the edges of
// an incoming list: for every source tail in groups, the `_order_in` field
// is writable and the manager would authorize the update. A move may
// rewrite any of them, so one refusal makes the list not movable.
func incomingOrderMovable(
	ctx context.Context, svc affordanceService, relType string, groups []incomingSource,
) bool {
	order := map[string]any{metamodel.OrderPropertyIn: 0.0}
	for _, g := range groups {
		if len(g.sources) == 0 || g.sources[0].row == nil {
			return false
		}
		if _, denial := svc.relationMetaDenial(ctx, g.sources, relType, order, nil); denial != nil {
			return false
		}
		req := translateRelationWrite(svc.meta(), relType, g.sources[0].row.Type, g.from, g.tail)
		if !svc.acl().AuthorizeWrite(ctx, req).Allow {
			return false
		}
	}
	return true
}

// relationOrderMovable reports whether the principal may move anchor's
// outgoing edges of relType: the write the manager would authorize, and the order
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
	w := &v1.RelationOrder{
		Relation: o.relation, Anchor: o.anchor, AnchorType: o.anchorType, Movable: o.movable, Addresses: o.addresses,
	}
	if o.incoming {
		w.Direction = string(DirectionIncoming)
	}
	return w
}

// sectionOrdering returns the relation ordering a flat section is shown in,
// or nil. A section qualifies when its rows come from exactly one
// non-recursive `follow:` or `follow_incoming:` rule from `entry` (see
// [viewResult.EntryEdges]) over a relation orderable on that side, and the
// section declares neither `sort:` nor `group_by:`: an author's own sort
// wins, and grouping re-orders rows by group.
//
// A package function rather than a viewsHandler method: the handler is at
// its plimsoll method line.
func sectionOrdering(ctx context.Context, h *viewsHandler, sec ViewSection, result *viewResult) *relationOrdering {
	if len(sec.Sort) > 0 || sec.GroupBy != "" || sec.Display == dataentryconfig.DisplayNested {
		return nil
	}
	meta := h.schema().Meta
	ee, ok := result.EntryEdges[sec.Source]
	if !ok || result.Entry == nil || orderPropertyOf(meta, ee.relation, ee.incoming) == "" {
		return nil
	}
	edges := ee.edges
	if ee.incoming {
		var err error
		if edges, err = h.visible.readableRelations(ctx, edges); err != nil {
			// No order rather than an order from a partial answer.
			slog.ErrorContext(ctx, "section relation order: gate failed", "section", sec.Source, "err", err)
			return nil
		}
	}
	o, err := newRelationOrdering(ctx, h.affordances, meta, result.Entry, ee.relation, ee.incoming, edges)
	if err != nil {
		slog.ErrorContext(ctx, "section relation order failed", "section", sec.Source, "err", err)
		return nil
	}
	return o
}
