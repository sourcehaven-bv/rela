package workspace

import (
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/repository"
)

// reloadAction describes what the workspace should do in response to file changes.
type reloadAction int

const (
	// actionReload means the metamodel changed — full reload needed.
	actionReload reloadAction = iota
	// actionSync means only entity/relation data changed — graph re-sync only.
	actionSync
	// actionNotify means non-data files changed (views, config) — just notify consumers.
	actionNotify
)

// classifyEvents inspects a batch of file change events and determines what
// action the workspace should take. If ANY event touches a schema file,
// the result is actionReload. If no schema files but entity/relation files
// changed, the result is actionSync. Otherwise actionNotify.
//
// schemaFiles must contain absolute, cleaned paths. viewsPath must also be
// absolute and cleaned.
func classifyEvents(
	events []repository.ChangeEvent,
	schemaFiles []string,
	viewsPath string,
	entitiesDir string,
	relationsDir string,
) reloadAction {
	schemaSet := make(map[string]bool, len(schemaFiles))
	for _, f := range schemaFiles {
		schemaSet[filepath.Clean(f)] = true
	}
	cleanViews := filepath.Clean(viewsPath)
	cleanEntities := filepath.Clean(entitiesDir)
	cleanRelations := filepath.Clean(relationsDir)

	hasData := false
	for _, e := range events {
		p := filepath.Clean(e.Path)

		// Schema file?
		if schemaSet[p] {
			return actionReload
		}

		// Views file? Treat as notify-only (views are loaded on demand)
		if p == cleanViews {
			continue
		}

		// Entity or relation file?
		if isUnder(p, cleanEntities) || isUnder(p, cleanRelations) {
			hasData = true
		}
	}

	if hasData {
		return actionSync
	}
	return actionNotify
}

// isUnder checks if path is under dir (using cleaned paths).
func isUnder(path, dir string) bool {
	// Add trailing separator so "/entities" doesn't match "/entities-extra"
	dirPrefix := dir + string(filepath.Separator)
	return len(path) > len(dirPrefix) && path[:len(dirPrefix)] == dirPrefix
}
