package workspace

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// Snapshot is a point-in-time, read-only view of the workspace state.
// Consumers should call Workspace.Snapshot() once at the top of an
// operation and use the returned Snapshot for all reads within that
// scope. This guarantees a coherent view: graph, metamodel, and search
// index all come from the same reload epoch.
//
// Snapshot replaces the pattern of calling ws.Graph() and ws.Meta()
// independently, which can observe different epochs if a reload lands
// between the two calls.
type Snapshot struct {
	s *workspaceState
}

// Graph returns the in-memory graph from this snapshot.
func (snap *Snapshot) Graph() *graph.Graph { return snap.s.graph }

// Meta returns the metamodel from this snapshot.
func (snap *Snapshot) Meta() *metamodel.Metamodel { return snap.s.meta }

// GetEntity returns an entity by ID, or (nil, false) if not found.
func (snap *Snapshot) GetEntity(id string) (*model.Entity, bool) {
	return snap.s.graph.GetNode(id)
}

// AllEntities returns all entities in the graph.
func (snap *Snapshot) AllEntities() []*model.Entity {
	return snap.s.graph.AllNodes()
}

// EntitiesByType returns all entities of the given type.
func (snap *Snapshot) EntitiesByType(entityType string) []*model.Entity {
	return snap.s.graph.NodesByType(entityType)
}

// GetRelation returns a relation by from/type/to, or (nil, false).
func (snap *Snapshot) GetRelation(from, relType, to string) (*model.Relation, bool) {
	return snap.s.graph.GetEdge(from, relType, to)
}

// AllRelations returns all relations in the graph.
func (snap *Snapshot) AllRelations() []*model.Relation {
	return snap.s.graph.AllEdges()
}

// IncomingRelations returns all relations pointing to the given entity.
func (snap *Snapshot) IncomingRelations(id string) []*model.Relation {
	return snap.s.graph.IncomingEdges(id)
}

// OutgoingRelations returns all relations from the given entity.
func (snap *Snapshot) OutgoingRelations(id string) []*model.Relation {
	return snap.s.graph.OutgoingEdges(id)
}

// --- Graph-vocabulary aliases ---
// These mirror graph.Graph method names so that *Snapshot can satisfy
// consumer-defined GraphReader interfaces without renaming.

// GetNode returns an entity by ID, or (nil, false) if not found.
// Alias for GetEntity.
func (snap *Snapshot) GetNode(id string) (*model.Entity, bool) {
	return snap.s.graph.GetNode(id)
}

// AllNodes returns all entities in the graph. Alias for AllEntities.
func (snap *Snapshot) AllNodes() []*model.Entity {
	return snap.s.graph.AllNodes()
}

// AllEdges returns all relations in the graph. Alias for AllRelations.
func (snap *Snapshot) AllEdges() []*model.Relation {
	return snap.s.graph.AllEdges()
}

// GetEdge returns a relation by from/type/to, or (nil, false).
// Alias for GetRelation.
func (snap *Snapshot) GetEdge(from, relType, to string) (*model.Relation, bool) {
	return snap.s.graph.GetEdge(from, relType, to)
}

// OutgoingEdges returns all relations from the given entity.
// Alias for OutgoingRelations.
func (snap *Snapshot) OutgoingEdges(id string) []*model.Relation {
	return snap.s.graph.OutgoingEdges(id)
}

// IncomingEdges returns all relations pointing to the given entity.
// Alias for IncomingRelations.
func (snap *Snapshot) IncomingEdges(id string) []*model.Relation {
	return snap.s.graph.IncomingEdges(id)
}

// --- Additional query methods ---

// NodesByType returns all entities of the given type.
func (snap *Snapshot) NodesByType(entityType string) []*model.Entity {
	return snap.s.graph.NodesByType(entityType)
}

// RelationsOfType returns all relations of the given type.
func (snap *Snapshot) RelationsOfType(relationType string) []*model.Relation {
	return snap.s.graph.RelationsOfType(relationType)
}

// TraceFrom traces all dependencies from a node.
func (snap *Snapshot) TraceFrom(id string, maxDepth int) *model.TraceResult {
	return snap.s.graph.TraceFrom(id, maxDepth)
}

// TraceTo traces all upstream dependencies to a node.
func (snap *Snapshot) TraceTo(id string, maxDepth int) *model.TraceResult {
	return snap.s.graph.TraceTo(id, maxDepth)
}

// FindPath finds a path between two nodes.
func (snap *Snapshot) FindPath(fromID, toID string) []model.PathStep {
	return snap.s.graph.FindPath(fromID, toID)
}

// FindOrphans returns entities with no relations.
func (snap *Snapshot) FindOrphans() []*model.Entity {
	return snap.s.graph.FindOrphans()
}

// NodeCount returns the number of entities in the graph.
func (snap *Snapshot) NodeCount() int {
	return snap.s.graph.NodeCount()
}

// EdgeCount returns the number of relations in the graph.
func (snap *Snapshot) EdgeCount() int {
	return snap.s.graph.EdgeCount()
}

// Search performs a full-text search and returns matching entities with scores.
func (snap *Snapshot) Search(words, phrases []string, limit int) ([]*model.Entity, []float64, error) {
	if snap.s.searchIdx == nil {
		return nil, nil, fmt.Errorf("search index not available")
	}
	results, err := snap.s.searchIdx.Search(words, phrases, limit)
	if err != nil {
		return nil, nil, err
	}
	entities := make([]*model.Entity, 0, len(results))
	scores := make([]float64, 0, len(results))
	for _, r := range results {
		if e, ok := snap.s.graph.GetNode(r.ID); ok {
			entities = append(entities, e)
			scores = append(scores, r.Score)
		}
	}
	return entities, scores, nil
}
