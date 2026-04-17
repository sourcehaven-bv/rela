package workspace

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// --- Entity queries ---

// GetEntity returns an entity by ID.
func (w *Workspace) GetEntity(id string) (*entity.Entity, bool) {
	e, err := w.Store().GetEntity(context.Background(), id)
	if err != nil {
		return nil, false
	}
	return e, true
}

// AllEntities returns all entities in the workspace.
func (w *Workspace) AllEntities() []*entity.Entity {
	return collectEntities(w.Store(), store.EntityQuery{})
}

// EntitiesByType returns all entities of the given type.
func (w *Workspace) EntitiesByType(entityType string) []*entity.Entity {
	return collectEntities(w.Store(), store.EntityQuery{Type: entityType})
}

// EntityCount returns the total number of entities.
func (w *Workspace) EntityCount() int {
	n, _ := w.Store().CountEntities(context.Background(), store.EntityQuery{})
	return n
}

// EntityIDs returns all entity IDs.
func (w *Workspace) EntityIDs() []string {
	entities := collectEntities(w.Store(), store.EntityQuery{})
	ids := make([]string, len(entities))
	for i, e := range entities {
		ids[i] = e.ID
	}
	return ids
}

// --- Relation queries ---

// GetRelation returns a relation by its endpoints and type.
func (w *Workspace) GetRelation(from, relType, to string) (*entity.Relation, bool) {
	r, err := w.Store().GetRelation(context.Background(), from, relType, to)
	if err != nil {
		return nil, false
	}
	return r, true
}

// AllRelations returns all relations in the workspace.
func (w *Workspace) AllRelations() []*entity.Relation {
	return collectRelations(w.Store(), store.RelationQuery{})
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
	var out []*entity.Entity
	for e, err := range s.ListEntities(context.Background(), q) {
		if err != nil {
			return out
		}
		out = append(out, e)
	}
	return out
}

func collectRelations(s store.Store, q store.RelationQuery) []*entity.Relation {
	var out []*entity.Relation
	for r, err := range s.ListRelations(context.Background(), q) {
		if err != nil {
			return out
		}
		out = append(out, r)
	}
	return out
}
