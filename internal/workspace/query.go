package workspace

import (
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// --- Entity queries ---

// GetEntity returns an entity by ID.
func (w *Workspace) GetEntity(id string) (*model.Entity, bool) {
	return w.Graph().GetNode(id)
}

// AllEntities returns all entities in the workspace.
func (w *Workspace) AllEntities() []*model.Entity {
	return w.Graph().AllNodes()
}

// EntitiesByType returns all entities of the given type.
func (w *Workspace) EntitiesByType(entityType string) []*model.Entity {
	return w.Graph().NodesByType(entityType)
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
func (w *Workspace) GetRelation(from, relType, to string) (*model.Relation, bool) {
	return w.Graph().GetEdge(from, relType, to)
}

// AllRelations returns all relations in the workspace.
func (w *Workspace) AllRelations() []*model.Relation {
	return w.Graph().AllEdges()
}

// IncomingRelations returns all relations pointing to the given entity.
func (w *Workspace) IncomingRelations(entityID string) []*model.Relation {
	return w.Graph().IncomingEdges(entityID)
}

// OutgoingRelations returns all relations originating from the given entity.
func (w *Workspace) OutgoingRelations(entityID string) []*model.Relation {
	return w.Graph().OutgoingEdges(entityID)
}

// --- Graph analysis ---

// FindOrphans returns entities with no incoming or outgoing relations.
func (w *Workspace) FindOrphans() []*model.Entity {
	return w.Graph().FindOrphans()
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
