package graph

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

// CacheData represents the serialized graph cache
type CacheData struct {
	Version   string            `json:"version"`
	Timestamp time.Time         `json:"timestamp"`
	Nodes     []*model.Entity   `json:"nodes"`
	Edges     []*model.Relation `json:"edges"`
}

const CacheVersion = "1.0"

// SaveCache saves the graph to a JSON cache file
func (g *Graph) SaveCache(path string) error {
	g.mu.RLock()
	nodes := make([]*model.Entity, 0, len(g.nodes))
	for _, node := range g.nodes {
		nodes = append(nodes, node)
	}
	edges := make([]*model.Relation, len(g.edges))
	copy(edges, g.edges)
	g.mu.RUnlock()

	data := CacheData{
		Version:   CacheVersion,
		Timestamp: time.Now(),
		Nodes:     nodes,
		Edges:     edges,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, jsonData, 0644)
}

// LoadCache loads the graph from a JSON cache file
func (g *Graph) LoadCache(path string) error {
	jsonData, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var data CacheData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return err
	}

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

	return nil
}

// CacheExists checks if a cache file exists
func CacheExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// CacheTimestamp returns the timestamp of the cache file
func CacheTimestamp(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}
