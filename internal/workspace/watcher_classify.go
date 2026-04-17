package workspace

import (
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// reloadAction describes what the workspace should do in response to file changes.
type reloadAction int

const (
	// actionSync means entity/relation data changed — graph re-sync only.
	actionSync reloadAction = iota
	// actionNotify means non-data files changed (views, config) — just notify consumers.
	actionNotify
)

// classifyDataEvents inspects a batch of data-directory change events and
// returns actionSync when any event touches an entity or relation file,
// actionNotify otherwise. Schema changes are handled by the metamodel
// loader's subscription and never reach this classifier.
//
// viewsPath, entitiesDir, and relationsDir must be absolute, cleaned paths.
func classifyDataEvents(
	events []storage.ChangeEvent,
	viewsPath string,
	entitiesDir string,
	relationsDir string,
) reloadAction {
	cleanViews := filepath.Clean(viewsPath)
	cleanEntities := filepath.Clean(entitiesDir)
	cleanRelations := filepath.Clean(relationsDir)

	for _, e := range events {
		p := filepath.Clean(e.Path)
		if p == cleanViews {
			continue
		}
		if isUnder(p, cleanEntities) || isUnder(p, cleanRelations) {
			return actionSync
		}
	}
	return actionNotify
}

// isUnder checks if path is under dir (using cleaned paths).
func isUnder(path, dir string) bool {
	// Add trailing separator so "/entities" doesn't match "/entities-extra"
	dirPrefix := dir + string(filepath.Separator)
	return len(path) > len(dirPrefix) && path[:len(dirPrefix)] == dirPrefix
}
