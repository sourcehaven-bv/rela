package workspace

import (
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// --- Entity queries ---

// GetEntity returns an entity by ID.
func (w *Workspace) GetEntity(id string) (*entity.Entity, bool) {
	n, ok := w.Graph().GetNode(id)
	if !ok {
		return nil, false
	}
	return model.EntityToDomain(n), true
}

// AllEntities returns all entities in the workspace.
func (w *Workspace) AllEntities() []*entity.Entity {
	nodes := w.Graph().AllNodes()
	out := make([]*entity.Entity, len(nodes))
	for i, n := range nodes {
		out[i] = model.EntityToDomain(n)
	}
	return out
}

// EntitiesByType returns all entities of the given type.
func (w *Workspace) EntitiesByType(entityType string) []*entity.Entity {
	nodes := w.Graph().NodesByType(entityType)
	out := make([]*entity.Entity, len(nodes))
	for i, n := range nodes {
		out[i] = model.EntityToDomain(n)
	}
	return out
}

// EntityCount returns the total number of entities.
func (w *Workspace) EntityCount() int {
	return w.Graph().NodeCount()
}

// EntityIDs returns all entity IDs.
func (w *Workspace) EntityIDs() []string {
	return w.Graph().AllIDs()
}

// --- Relation queries ---

// GetRelation returns a relation by its endpoints and type.
func (w *Workspace) GetRelation(from, relType, to string) (*entity.Relation, bool) {
	r, ok := w.Graph().GetEdge(from, relType, to)
	if !ok {
		return nil, false
	}
	return model.RelationToDomain(r), true
}

// AllRelations returns all relations in the workspace.
func (w *Workspace) AllRelations() []*entity.Relation {
	edges := w.Graph().AllEdges()
	out := make([]*entity.Relation, len(edges))
	for i, r := range edges {
		out[i] = model.RelationToDomain(r)
	}
	return out
}

// IncomingRelations returns all relations pointing to the given entity.
func (w *Workspace) IncomingRelations(entityID string) []*entity.Relation {
	edges := w.Graph().IncomingEdges(entityID)
	out := make([]*entity.Relation, len(edges))
	for i, r := range edges {
		out[i] = model.RelationToDomain(r)
	}
	return out
}

// OutgoingRelations returns all relations originating from the given entity.
func (w *Workspace) OutgoingRelations(entityID string) []*entity.Relation {
	edges := w.Graph().OutgoingEdges(entityID)
	out := make([]*entity.Relation, len(edges))
	for i, r := range edges {
		out[i] = model.RelationToDomain(r)
	}
	return out
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

