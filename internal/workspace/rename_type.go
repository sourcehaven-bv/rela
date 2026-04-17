package workspace

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// RenameEntityType renames an entity type across the project:
// metamodel, entity directory, entity files, and templates.
// Returns the number of entity files updated.
func (w *Workspace) RenameEntityType(oldType, newType, newPlural string) (int, error) {
	fs := w.FS()
	meta := w.Meta()
	paths := w.Paths()

	oldDef, ok := meta.GetEntityDef(oldType)
	if !ok {
		return 0, fmt.Errorf("unknown entity type: %s", oldType)
	}
	oldPlural := oldDef.GetDirPlural(oldType)

	// 1. Update metamodel.yaml
	if err := metamodel.RenameEntityType(paths.MetamodelPath, oldType, newType, fs); err != nil {
		return 0, fmt.Errorf("failed to update metamodel: %w", err)
	}

	oldDir := paths.EntityTypeDirWithPlural(oldPlural)
	newDir := paths.EntityTypeDirWithPlural(newPlural)

	// 2. Rename entity directory
	if _, err := fs.Stat(oldDir); err == nil {
		if err := fs.Rename(oldDir, newDir); err != nil {
			return 0, fmt.Errorf("failed to rename directory: %w", err)
		}
	}

	// 3. Update entity files in the new directory
	var count int
	if _, err := fs.Stat(newDir); err == nil {
		var updateErr error
		count, updateErr = markdown.NewFileIO(fs).UpdateEntityTypesInDir(newDir, newType, meta)
		if updateErr != nil {
			return 0, fmt.Errorf("failed to update entity files: %w", updateErr)
		}
	}

	// 4. Rename template if it exists
	oldTemplatePath := paths.EntityTemplatePath(oldType)
	if _, err := fs.Stat(oldTemplatePath); err == nil {
		newTemplatePath := paths.EntityTemplatePath(newType)
		_ = fs.MkdirAll(paths.EntityTemplatesDir, 0755)
		_ = fs.Rename(oldTemplatePath, newTemplatePath)
	}

	// 5. Remove cache (stale after rename)
	_ = fs.Remove(paths.CachePath)

	return count, nil
}
