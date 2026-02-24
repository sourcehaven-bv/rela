package graph

import (
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// CacheData holds graph data for serialization. The repository layer
// decides how (and where) to marshal and persist this data.
type CacheData struct {
	Nodes []*model.Entity
	Edges []*model.Relation
}

// Snapshot returns a copy of the graph's nodes and edges for external
// serialization. The caller owns the returned slices.
func (g *Graph) Snapshot() *CacheData {
	g.mu.RLock()
	nodes := make([]*model.Entity, 0, len(g.nodes))
	for _, node := range g.nodes {
		nodes = append(nodes, node)
	}
	edges := make([]*model.Relation, len(g.edges))
	copy(edges, g.edges)
	g.mu.RUnlock()

	return &CacheData{
		Nodes: nodes,
		Edges: edges,
	}
}

// Restore replaces the graph contents with the provided cache data.
// It rebuilds all internal indexes (adjacency maps, property index).
func (g *Graph) Restore(data *CacheData) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Clear existing data
	g.nodes = make(map[string]*model.Entity)
	g.edges = make([]*model.Relation, 0)
	g.outgoing = make(map[string][]*model.Relation)
	g.incoming = make(map[string][]*model.Relation)

	// Load nodes
	for _, node := range data.Nodes {
		g.nodes[node.ID] = node
	}

	// Load edges
	for _, edge := range data.Edges {
		g.edges = append(g.edges, edge)
		g.outgoing[edge.From] = append(g.outgoing[edge.From], edge)
		g.incoming[edge.To] = append(g.incoming[edge.To], edge)
	}

	// Rebuild property index from loaded nodes
	g.propertyIndex = make(map[string]map[string]int)
	for _, node := range g.nodes {
		g.indexEntityProperties(node)
	}
}
