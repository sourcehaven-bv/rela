package workspace

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/model"
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

// --- Graph analysis ---

// FindOrphans returns entities with no incoming or outgoing relations.
func (w *Workspace) FindOrphans() []*entity.Entity {
	nodes := w.Graph().FindOrphans()
	out := make([]*entity.Entity, len(nodes))
	for i, n := range nodes {
		out[i] = model.EntityToDomain(n)
	}
	return out
}

// TraceResult is re-exported from model for consumers.
type TraceResult = model.TraceResult

// TraceFrom traces all paths from the given entity (outgoing direction).
func (w *Workspace) TraceFrom(entityID string, maxDepth int) *TraceResult {
	return w.Graph().TraceFrom(entityID, maxDepth)
}

// TraceTo traces all paths to the given entity (incoming direction).
func (w *Workspace) TraceTo(entityID string, maxDepth int) *TraceResult {
	return w.Graph().TraceTo(entityID, maxDepth)
}

// PathStep is re-exported from model for consumers.
type PathStep = model.PathStep

// FindPath finds the shortest path between two entities.
func (w *Workspace) FindPath(from, to string) []PathStep {
	return w.Graph().FindPath(from, to)
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
