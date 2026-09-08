package dataentry

import (
	"context"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// entityReader is the ungated entity/relation read seam over the store.
// Extracted from App (TKT-N26KLB): a single-dependency leaf shared by the read
// handlers, the affordance service, the relation handlers, and (eventually) the
// write handlers. It is deliberately UNGATED — ACL scoping is applied elsewhere
// (visibleReader for the gated single-GET / include-filter path; the analyze
// gate at the issue boundary). These helpers are the raw store reads those
// gated paths and the internal machinery build on.
//
// # It is DEFAULT-WORLD-ONLY, deliberately (TKT-DN37J2)
//
// Every read here goes to the store unresolved, so it returns each entity's
// DEFAULT state — the draft face, under the design doc's example layout.
// That is a decision, not an oversight, and it is safe because of two things
// together:
//
//   - The routes this reader serves (relations, attachments, export,
//     sub-resources, views) are REFUSED a non-default world by
//     worldCapablePath, so no `?world=` request reaches those calls.
//   - The two world-CAPABLE handlers (the collection list and the
//     single-entity GET) call this reader only on their DEFAULT-world branch.
//     Since TKT-WRLDAPI item 4 they have a world branch that goes through
//     worldNeighbors instead, which resolves each link through the request's
//     world (RULING 12).
//
// If a route is ever added to that allowlist, its use of this reader must be
// converted first — a world-bound response assembled partly from
// world-resolved rows and partly from these is the mixed-face bug that would
// be hardest to see, because the entity would look right and its neighbors
// would not. TestWorldCapableRoutesDoNotUseUngatedReader is the guard.
type entityReader struct {
	store store.Store
}

// getEntity looks up an entity by ID via the store.
func (er entityReader) getEntity(ctx context.Context, id string) (*entity.Entity, bool) {
	e, err := er.store.GetEntity(ctx, id)
	if err != nil {
		return nil, false
	}
	return e, true
}

// entityType returns the type of the entity with the given ID, or empty
// string if it can't be resolved. The relation GET handlers call it on a
// relation endpoint's ID to emit a `type` field per edge, so SPA clients can
// construct JSON:API §9 resource identifiers without guessing — but the
// operation is just "look up an entity, return its type", nothing
// relation-specific.
func (er entityReader) entityType(ctx context.Context, id string) string {
	if e, ok := er.getEntity(ctx, id); ok {
		return e.Type
	}
	return ""
}

// outgoingRelations returns all outgoing relations for id.
//
// A store error truncates the slice to what was read before the failure. The
// callers are response-serialization paths where a partial relations map is
// degraded-but-usable and a hard failure would 500 the whole entity; we log
// the error (so it isn't invisible) rather than propagate it. TODO(TKT-N26KLB):
// the relations map silently dropping edges on a store error is a latent
// correctness gap inherited from App — revisit whether these paths should
// surface a partial-result warning.
func (er entityReader) outgoingRelations(ctx context.Context, id string) []*entity.Relation {
	return er.relations(ctx, id, store.DirectionOutgoing)
}

// incomingRelations returns all incoming relations for id. Same error handling
// as outgoingRelations.
func (er entityReader) incomingRelations(ctx context.Context, id string) []*entity.Relation {
	return er.relations(ctx, id, store.DirectionIncoming)
}

// pageRelations loads every edge touching any of entities in ONE query and
// splits them per row: outgoing[i] holds the edges whose source is
// entities[i], incoming[i] those whose target is. Index-aligned with
// entities; a row with no edges keeps a nil entry. An edge between two page
// rows appears in both rows' slices, once each — the same result the former
// per-row outgoing+incoming pair produced (TKT-1U8XYN).
func (er entityReader) pageRelations(
	ctx context.Context, entities []*entity.Entity,
) (outgoing, incoming [][]*entity.Relation) {
	outgoing = make([][]*entity.Relation, len(entities))
	incoming = make([][]*entity.Relation, len(entities))
	if len(entities) == 0 {
		return outgoing, incoming
	}
	rowIdx := make(map[string]int, len(entities))
	ids := make([]string, 0, len(entities))
	for i, e := range entities {
		if _, dup := rowIdx[e.ID]; dup {
			continue
		}
		rowIdx[e.ID] = i
		ids = append(ids, e.ID)
	}
	rels, err := listRelationsCtx(ctx, er.store, store.RelationQuery{EntityIDs: ids, Direction: store.DirectionBoth})
	if err != nil {
		slog.Warn("dataentry: entityReader: listing page relations failed; result truncated",
			"rows", len(ids), "err", err)
	}
	for _, r := range rels {
		if i, ok := rowIdx[r.From]; ok {
			outgoing[i] = append(outgoing[i], r)
		}
		if i, ok := rowIdx[r.To]; ok {
			incoming[i] = append(incoming[i], r)
		}
	}
	return outgoing, incoming
}

func (er entityReader) relations(ctx context.Context, id string, dir store.Direction) []*entity.Relation {
	rels, err := listRelationsCtx(ctx, er.store, store.RelationQuery{EntityID: id, Direction: dir})
	if err != nil {
		slog.Warn("dataentry: entityReader: listing relations failed; result truncated",
			"entity", id, "direction", dir, "err", err)
	}
	return rels
}
