package markdown

import (
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// SyncData holds the raw entities and relations loaded from markdown files,
// along with any conflicted file paths. The caller is responsible for
// populating the graph and validating relation endpoints.
type SyncData struct {
	Entities   []*model.Entity
	Relations  []*model.Relation
	Conflicted []string
}

// LoadSyncData reads all entity and relation files from disk and returns
// them as raw data. It does NOT populate a graph — that is left to the caller.
// Files with git conflict markers are skipped and tracked in Conflicted.
func (f *FileIO) LoadSyncData(
	ctx *project.Context, meta *metamodel.Metamodel,
) (*SyncData, error) {
	data := &SyncData{}

	// Load all entities (with conflict tracking)
	entityResult, err := f.LoadAllEntitiesWithConflicts(ctx.EntitiesDir, meta)
	if err != nil {
		return nil, err
	}
	data.Entities = entityResult.Entities
	data.Conflicted = append(data.Conflicted, entityResult.Conflicted...)

	// Load all relations (with conflict tracking)
	relationResult, err := f.LoadAllRelationsWithConflicts(ctx.RelationsDir)
	if err != nil {
		return nil, err
	}
	data.Relations = relationResult.Relations
	data.Conflicted = append(data.Conflicted, relationResult.Conflicted...)

	return data, nil
}
