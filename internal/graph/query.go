package graph

import (
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// TraceFrom traces all dependencies from a node by following both outgoing AND incoming edges.
// This allows tracing entities that depend on the given entity (via incoming relations)
// as well as entities the given entity depends on (via outgoing relations).
func (g *Graph) TraceFrom(id string, maxDepth int) *model.TraceResult {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]bool)
	return g.traceBidirectional(id, 0, maxDepth, "", false, visited)
}

// TraceTo traces all upstream dependencies to a node (following incoming edges)
func (g *Graph) TraceTo(id string, maxDepth int) *model.TraceResult {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]bool)
	return g.traceToInternal(id, 0, maxDepth, "", visited)
}

// TraceBoth traces both incoming and outgoing relations from a node
func (g *Graph) TraceBoth(id string, maxDepth int) *model.TraceResult {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]bool)
	return g.traceBidirectional(id, 0, maxDepth, "", false, visited)
}

// traceBidirectional is the shared internal implementation for TraceFrom and TraceBoth.
// It follows both outgoing and incoming edges to traverse the full graph.
func (g *Graph) traceBidirectional(
	id string, depth, maxDepth int, relation string, incoming bool, visited map[string]bool,
) *model.TraceResult {
	if maxDepth > 0 && depth > maxDepth {
		return nil
	}

	node, ok := g.nodes[id]
	if !ok {
		return nil
	}

	result := &model.TraceResult{
		ID:       id,
		Type:     node.Type,
		Title:    node.Title(),
		Depth:    depth,
		Relation: relation,
		Incoming: incoming,
	}

	if visited[id] {
		return result // Prevent cycles
	}
	visited[id] = true

	// Follow outgoing edges (entity -> others)
	for _, edge := range g.outgoing[id] {
		child := g.traceBidirectional(edge.To, depth+1, maxDepth, edge.Type, false, visited)
		if child != nil {
			result.Children = append(result.Children, child)
		}
	}

	// Follow incoming edges (others -> entity) to find entities that depend on this one
	for _, edge := range g.incoming[id] {
		child := g.traceBidirectional(edge.From, depth+1, maxDepth, edge.Type, true, visited)
		if child != nil {
			result.Children = append(result.Children, child)
		}
	}

	return result
}

func (g *Graph) traceToInternal(
	id string, depth, maxDepth int, relation string, visited map[string]bool,
) *model.TraceResult {
	if maxDepth > 0 && depth > maxDepth {
		return nil
	}

	node, ok := g.nodes[id]
	if !ok {
		return nil
	}

	result := &model.TraceResult{
		ID:       id,
		Type:     node.Type,
		Title:    node.Title(),
		Depth:    depth,
		Relation: relation,
	}

	if visited[id] {
		return result // Prevent cycles
	}
	visited[id] = true

	for _, edge := range g.incoming[id] {
		child := g.traceToInternal(edge.From, depth+1, maxDepth, edge.Type, visited)
		if child != nil {
			result.Children = append(result.Children, child)
		}
	}

	return result
}

// FindPath finds a path between two nodes (BFS), traversing edges in both directions.
// This treats the graph as undirected for path-finding purposes, allowing paths to be
// found regardless of edge direction.
func (g *Graph) FindPath(fromID, toID string) []model.PathStep {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if fromID == toID {
		if node, ok := g.nodes[fromID]; ok {
			return []model.PathStep{{ID: fromID, Type: node.Type, Title: node.Title()}}
		}
		return nil
	}

	// BFS
	type queueItem struct {
		id   string
		path []model.PathStep
	}

	visited := make(map[string]bool)
	queue := []queueItem{}

	if node, ok := g.nodes[fromID]; ok {
		queue = append(queue, queueItem{
			id:   fromID,
			path: []model.PathStep{{ID: fromID, Type: node.Type, Title: node.Title()}},
		})
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current.id] {
			continue
		}
		visited[current.id] = true

		// Collect all neighbors (both outgoing and incoming edges)
		type neighbor struct {
			id       string
			relation string
		}
		neighbors := make([]neighbor, 0)

		// Add outgoing neighbors
		for _, edge := range g.outgoing[current.id] {
			neighbors = append(neighbors, neighbor{id: edge.To, relation: edge.Type})
		}

		// Add incoming neighbors (traverse against edge direction)
		for _, edge := range g.incoming[current.id] {
			neighbors = append(neighbors, neighbor{id: edge.From, relation: edge.Type})
		}

		for _, nb := range neighbors {
			if nb.id == toID {
				// Found the target
				if node, ok := g.nodes[toID]; ok {
					finalPath := make([]model.PathStep, len(current.path), len(current.path)+1)
					copy(finalPath, current.path)
					finalPath = append(finalPath, model.PathStep{
						ID:       toID,
						Type:     node.Type,
						Title:    node.Title(),
						Relation: nb.relation,
					})
					return finalPath
				}
			}

			if !visited[nb.id] {
				if node, ok := g.nodes[nb.id]; ok {
					newPath := make([]model.PathStep, len(current.path), len(current.path)+1)
					copy(newPath, current.path)
					newPath = append(newPath, model.PathStep{
						ID:       nb.id,
						Type:     node.Type,
						Title:    node.Title(),
						Relation: nb.relation,
					})
					queue = append(queue, queueItem{id: nb.id, path: newPath})
				}
			}
		}
	}

	return nil // No path found
}

// FindOrphans returns all nodes with no connections
func (g *Graph) FindOrphans() []*model.Entity {
	g.mu.RLock()
	defer g.mu.RUnlock()

	orphans := make([]*model.Entity, 0)
	for id, node := range g.nodes {
		if len(g.outgoing[id]) == 0 && len(g.incoming[id]) == 0 {
			orphans = append(orphans, node)
		}
	}
	return orphans
}

// FindClusters returns groups of connected nodes
func (g *Graph) FindClusters() [][]*model.Entity {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]bool)
	clusters := make([][]*model.Entity, 0)

	for id := range g.nodes {
		if visited[id] {
			continue
		}

		cluster := g.collectCluster(id, visited)
		if len(cluster) > 0 {
			clusters = append(clusters, cluster)
		}
	}

	return clusters
}

func (g *Graph) collectCluster(startID string, visited map[string]bool) []*model.Entity {
	cluster := make([]*model.Entity, 0)
	queue := []string{startID}

	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]

		if visited[id] {
			continue
		}
		visited[id] = true

		if node, ok := g.nodes[id]; ok {
			cluster = append(cluster, node)
		}

		// Add connected nodes
		for _, edge := range g.outgoing[id] {
			if !visited[edge.To] {
				queue = append(queue, edge.To)
			}
		}
		for _, edge := range g.incoming[id] {
			if !visited[edge.From] {
				queue = append(queue, edge.From)
			}
		}
	}

	return cluster
}

// HasCycle checks if there's a cycle starting from the given node
func (g *Graph) HasCycle(startID string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	return g.hasCycleInternal(startID, visited, recStack)
}

func (g *Graph) hasCycleInternal(id string, visited, recStack map[string]bool) bool {
	visited[id] = true
	recStack[id] = true

	for _, edge := range g.outgoing[id] {
		if !visited[edge.To] {
			if g.hasCycleInternal(edge.To, visited, recStack) {
				return true
			}
		} else if recStack[edge.To] {
			return true
		}
	}

	recStack[id] = false
	return false
}

// RelationsOfType returns all relations of a specific type
func (g *Graph) RelationsOfType(relationType string) []*model.Relation {
	g.mu.RLock()
	defer g.mu.RUnlock()

	relations := make([]*model.Relation, 0)
	for _, edge := range g.edges {
		if edge.Type == relationType {
			relations = append(relations, edge)
		}
	}
	return relations
}
