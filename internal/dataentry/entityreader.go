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

func (er entityReader) relations(ctx context.Context, id string, dir store.Direction) []*entity.Relation {
	rels, err := listRelationsCtx(ctx, er.store, store.RelationQuery{EntityID: id, Direction: dir})
	if err != nil {
		slog.Warn("dataentry: entityReader: listing relations failed; result truncated",
			"entity", id, "direction", dir, "err", err)
	}
	return rels
}
