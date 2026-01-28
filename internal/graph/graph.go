package graph

import (
	"fmt"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

// Graph represents an in-memory graph of entities and relations
type Graph struct {
	nodes         map[string]*model.Entity     // ID -> Entity
	edges         []*model.Relation            // All relations
	outgoing      map[string][]*model.Relation // sourceID -> outgoing relations
	incoming      map[string][]*model.Relation // targetID -> incoming relations
	propertyIndex map[string]map[string]int    // propertyName -> value -> count
	mu            sync.RWMutex
}

// New creates a new empty graph
func New() *Graph {
	return &Graph{
		nodes:         make(map[string]*model.Entity),
		edges:         make([]*model.Relation, 0),
		outgoing:      make(map[string][]*model.Relation),
		incoming:      make(map[string][]*model.Relation),
		propertyIndex: make(map[string]map[string]int),
	}
}

// AddNode adds an entity to the graph
func (g *Graph) AddNode(entity *model.Entity) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nodes[entity.ID] = entity
	g.indexEntityProperties(entity)
}

// indexEntityProperties adds entity property values to the property index
func (g *Graph) indexEntityProperties(entity *model.Entity) {
	for propName, value := range entity.Properties {
		strValue := g.valueToString(value)
		if strValue == "" {
			continue
		}

		// Initialize map for this property if needed
		if g.propertyIndex[propName] == nil {
			g.propertyIndex[propName] = make(map[string]int)
		}

		// Increment count for this value
		g.propertyIndex[propName][strValue]++
	}
}

// unindexEntityProperties removes entity property values from the property index
func (g *Graph) unindexEntityProperties(entity *model.Entity) {
	for propName, value := range entity.Properties {
		strValue := g.valueToString(value)
		if strValue == "" {
			continue
		}

		if valMap, ok := g.propertyIndex[propName]; ok {
			if count, exists := valMap[strValue]; exists {
				if count <= 1 {
					delete(valMap, strValue)
					// Clean up empty maps
					if len(valMap) == 0 {
						delete(g.propertyIndex, propName)
					}
				} else {
					valMap[strValue]--
				}
			}
		}
	}
}

// valueToString converts a property value to string
func (g *Graph) valueToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case int, int32, int64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%f", v)
	case bool:
		return fmt.Sprintf("%t", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// UpdateNode updates an existing entity in the graph without affecting its relations.
// Returns false if the node does not exist.
func (g *Graph) UpdateNode(entity *model.Entity) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	oldEntity, ok := g.nodes[entity.ID]
	if !ok {
		return false
	}

	// Remove old property values from index
	g.unindexEntityProperties(oldEntity)

	// Update node
	g.nodes[entity.ID] = entity

	// Add new property values to index
	g.indexEntityProperties(entity)

	return true
}

// GetNode returns an entity by ID
func (g *Graph) GetNode(id string) (*model.Entity, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	node, ok := g.nodes[id]
	return node, ok
}

// RemoveNode removes an entity and its relations from the graph
func (g *Graph) RemoveNode(id string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	entity, ok := g.nodes[id]
	if !ok {
		return false
	}

	// Remove property values from index
	g.unindexEntityProperties(entity)

	delete(g.nodes, id)

	// Remove all relations involving this node
	newEdges := make([]*model.Relation, 0)
	for _, edge := range g.edges {
		if edge.From != id && edge.To != id {
			newEdges = append(newEdges, edge)
		}
	}
	g.edges = newEdges

	// Rebuild adjacency maps
	g.rebuildAdjacency()

	return true
}

// AddEdge adds a relation to the graph
func (g *Graph) AddEdge(relation *model.Relation) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.edges = append(g.edges, relation)
	g.outgoing[relation.From] = append(g.outgoing[relation.From], relation)
	g.incoming[relation.To] = append(g.incoming[relation.To], relation)
}

// RemoveEdge removes a specific relation from the graph
func (g *Graph) RemoveEdge(from, relationType, to string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	found := false
	newEdges := make([]*model.Relation, 0, len(g.edges))
	for _, edge := range g.edges {
		if edge.From == from && edge.Type == relationType && edge.To == to {
			found = true
		} else {
			newEdges = append(newEdges, edge)
		}
	}
	g.edges = newEdges

	if found {
		g.rebuildAdjacency()
	}

	return found
}

// GetEdge returns a specific relation if it exists
func (g *Graph) GetEdge(from, relationType, to string) (*model.Relation, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, edge := range g.edges {
		if edge.From == from && edge.Type == relationType && edge.To == to {
			return edge, true
		}
	}
	return nil, false
}

// OutgoingEdges returns all outgoing relations from a node
func (g *Graph) OutgoingEdges(id string) []*model.Relation {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.outgoing[id]
}

// IncomingEdges returns all incoming relations to a node
func (g *Graph) IncomingEdges(id string) []*model.Relation {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.incoming[id]
}

// AllNodes returns all entities in the graph
func (g *Graph) AllNodes() []*model.Entity {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodes := make([]*model.Entity, 0, len(g.nodes))
	for _, node := range g.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// AllEdges returns all relations in the graph
func (g *Graph) AllEdges() []*model.Relation {
	g.mu.RLock()
	defer g.mu.RUnlock()

	edges := make([]*model.Relation, len(g.edges))
	copy(edges, g.edges)
	return edges
}

// NodesByType returns all entities of a given type
func (g *Graph) NodesByType(entityType string) []*model.Entity {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodes := make([]*model.Entity, 0)
	for _, node := range g.nodes {
		if node.Type == entityType {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// NodeCount returns the number of nodes in the graph
func (g *Graph) NodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes)
}

// EdgeCount returns the number of edges in the graph
func (g *Graph) EdgeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.edges)
}

// AllIDs returns all entity IDs in the graph
func (g *Graph) AllIDs() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	ids := make([]string, 0, len(g.nodes))
	for id := range g.nodes {
		ids = append(ids, id)
	}
	return ids
}

// IDsByType returns all entity IDs of a given type
func (g *Graph) IDsByType(entityType string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	ids := make([]string, 0)
	for id, node := range g.nodes {
		if node.Type == entityType {
			ids = append(ids, id)
		}
	}
	return ids
}

// Clear removes all nodes and edges from the graph
func (g *Graph) Clear() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nodes = make(map[string]*model.Entity)
	g.edges = make([]*model.Relation, 0)
	g.outgoing = make(map[string][]*model.Relation)
	g.incoming = make(map[string][]*model.Relation)
}

// rebuildAdjacency rebuilds the outgoing/incoming maps from edges
// Must be called with lock held
func (g *Graph) rebuildAdjacency() {
	g.outgoing = make(map[string][]*model.Relation)
	g.incoming = make(map[string][]*model.Relation)

	for _, edge := range g.edges {
		g.outgoing[edge.From] = append(g.outgoing[edge.From], edge)
		g.incoming[edge.To] = append(g.incoming[edge.To], edge)
	}
}

// PropertyValuesByFrequency holds a property value and its frequency
type PropertyValuesByFrequency struct {
	Value string
	Count int
}

// GetPropertyValues returns property values sorted by frequency (most common first)
func (g *Graph) GetPropertyValues(propertyName string, limit int) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	values, ok := g.propertyIndex[propertyName]
	if !ok || len(values) == 0 {
		return nil
	}

	// Convert map to slice for sorting
	type valueCount struct {
		value string
		count int
	}
	sorted := make([]valueCount, 0, len(values))
	for val, count := range values {
		sorted = append(sorted, valueCount{value: val, count: count})
	}

	// Sort by count descending, then by value alphabetically
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].count > sorted[i].count ||
				(sorted[j].count == sorted[i].count && sorted[j].value < sorted[i].value) {

				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Extract values up to limit
	result := make([]string, 0, limit)
	for i := 0; i < len(sorted) && i < limit; i++ {
		result = append(result, sorted[i].value)
	}

	return result
}
