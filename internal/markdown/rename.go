package markdown

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// UpdateEntityType reads an entity file, updates the type field, and writes it back.
func UpdateEntityType(path, newType string, meta *metamodel.Metamodel) error {
	entity, err := ReadEntity(path, meta)
	if err != nil {
		return fmt.Errorf("failed to read entity: %w", err)
	}

	entity.Type = newType

	if err := WriteEntity(entity, path); err != nil {
		return fmt.Errorf("failed to write entity: %w", err)
	}

	return nil
}

// UpdateEntityTypesInDir updates the `type` field in all entity markdown files in a directory.
// Returns the number of files updated and any error encountered.
func UpdateEntityTypesInDir(dir, newType string, meta *metamodel.Metamodel) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read directory: %w", err)
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		if err := UpdateEntityType(path, newType, meta); err != nil {
			return count, fmt.Errorf("failed to update %s: %w", entry.Name(), err)
		}
		count++
	}

	return count, nil
}
