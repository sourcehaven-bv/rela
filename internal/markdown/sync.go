package markdown

import (
	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// SyncResult contains statistics from a sync operation
type SyncResult struct {
	EntitiesLoaded  int
	RelationsLoaded int
	Errors          []error
}

// SyncFromFiles rebuilds the graph from markdown files
func SyncFromFiles(ctx *project.Context, meta *metamodel.Metamodel, g *graph.Graph) (*SyncResult, error) {
	result := &SyncResult{}

	// Clear the graph
	g.Clear()

	// Load all entities
	entities, err := LoadAllEntities(ctx.EntitiesDir, meta)
	if err != nil {
		return nil, err
	}

	for _, entity := range entities {
		g.AddNode(entity)
		result.EntitiesLoaded++
	}

	// Load all relations
	relations, err := LoadAllRelations(ctx.RelationsDir)
	if err != nil {
		return nil, err
	}

	for _, relation := range relations {
		// Validate that both ends exist
		if _, ok := g.GetNode(relation.From); !ok {
			result.Errors = append(result.Errors, &SyncError{
				File:    relation.FilePath,
				Message: "source entity not found: " + relation.From,
			})
			continue
		}
		if _, ok := g.GetNode(relation.To); !ok {
			result.Errors = append(result.Errors, &SyncError{
				File:    relation.FilePath,
				Message: "target entity not found: " + relation.To,
			})
			continue
		}

		g.AddEdge(relation)
		result.RelationsLoaded++
	}

	return result, nil
}

// SyncError represents an error during sync
type SyncError struct {
	File    string
	Message string
}

func (e *SyncError) Error() string {
	return e.File + ": " + e.Message
}
