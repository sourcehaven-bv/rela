package workspace

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Thin wrappers over w.Store() kept for callers that still want the
// convenience signatures. Bulk iteration helpers (AllEntities,
// EntitiesByType, EntityCount, EntityIDs, AllRelations) used to live
// here but were removed — no non-test code called them, and direct
// store queries are clearer at call sites.

// GetEntity returns an entity by ID.
func (w *Workspace) GetEntity(id string) (*entity.Entity, bool) {
	e, err := w.Store().GetEntity(context.Background(), id)
	if err != nil {
		return nil, false
	}
	return e, true
}

// GetRelation returns a relation by its endpoints and type.
func (w *Workspace) GetRelation(from, relType, to string) (*entity.Relation, bool) {
	r, err := w.Store().GetRelation(context.Background(), from, relType, to)
	if err != nil {
		return nil, false
	}
	return r, true
}

// IncomingRelations returns all relations pointing to the given entity.
func (w *Workspace) IncomingRelations(entityID string) []*entity.Relation {
	return collectRelations(w.Store(), store.RelationQuery{
		EntityID:  entityID,
		Direction: store.DirectionIncoming,
	})
}

// OutgoingRelations returns all relations originating from the given entity.
func (w *Workspace) OutgoingRelations(entityID string) []*entity.Relation {
	return collectRelations(w.Store(), store.RelationQuery{
		EntityID:  entityID,
		Direction: store.DirectionOutgoing,
	})
}

func collectEntities(s store.Store, q store.EntityQuery) []*entity.Entity {
	out := make([]*entity.Entity, 0)
	for e, err := range s.ListEntities(context.Background(), q) {
		if err != nil {
			return out
		}
		out = append(out, e)
	}
	return out
}

func collectRelations(s store.Store, q store.RelationQuery) []*entity.Relation {
	out := make([]*entity.Relation, 0)
	for r, err := range s.ListRelations(context.Background(), q) {
		if err != nil {
			return out
		}
		out = append(out, r)
	}
	return out
}
