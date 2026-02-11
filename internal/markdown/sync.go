package markdown

import (
	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// SyncFromFiles rebuilds the graph from markdown files.
// Files with git conflict markers are skipped and tracked in result.Conflicted.
func (f *FileIO) SyncFromFiles(
	ctx *project.Context, meta *metamodel.Metamodel, g *graph.Graph,
) (*model.SyncResult, error) {
	result := &model.SyncResult{}

	// Clear the graph
	g.Clear()

	// Load all entities (with conflict tracking)
	entityResult, err := f.LoadAllEntitiesWithConflicts(ctx.EntitiesDir, meta)
	if err != nil {
		return nil, err
	}

	for _, entity := range entityResult.Entities {
		g.AddNode(entity)
		result.EntitiesLoaded++
	}
	result.Conflicted = append(result.Conflicted, entityResult.Conflicted...)

	// Load all relations (with conflict tracking)
	relationResult, err := f.LoadAllRelationsWithConflicts(ctx.RelationsDir)
	if err != nil {
		return nil, err
	}

	for _, relation := range relationResult.Relations {
		// Validate that both ends exist
		if _, ok := g.GetNode(relation.From); !ok {
			result.Errors = append(result.Errors, &model.SyncError{
				File:    relation.FilePath,
				Message: "source entity not found: " + relation.From,
			})
			continue
		}
		if _, ok := g.GetNode(relation.To); !ok {
			result.Errors = append(result.Errors, &model.SyncError{
				File:    relation.FilePath,
				Message: "target entity not found: " + relation.To,
			})
			continue
		}

		g.AddEdge(relation)
		result.RelationsLoaded++
	}
	result.Conflicted = append(result.Conflicted, relationResult.Conflicted...)

	return result, nil
}
