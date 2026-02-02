// Package migration provides schema migration support for rela project files.
// It detects deprecated syntax patterns and transforms files while preserving
// comments and formatting using yaml.Node AST manipulation.
package migration

import (
	"gopkg.in/yaml.v3"
)

// FileType identifies which project files a migration applies to.
type FileType string

const (
	FileTypeMetamodel FileType = "metamodel"  // metamodel.yaml
	FileTypeViews     FileType = "views"      // views.yaml (future)
	FileTypeDataEntry FileType = "data-entry" // data-entry.yaml
)

// Migration defines the interface for schema migrations.
// Migrations operate on yaml.Node trees to preserve comments and formatting.
type Migration interface {
	// Name returns a unique identifier for this migration (e.g., "id-type-rename").
	Name() string

	// Description returns a human-readable description of what this migration does.
	Description() string

	// FileTypes returns which file types this migration applies to.
	FileTypes() []FileType

	// Detect checks if the given YAML document needs this migration.
	// It should return true if deprecated patterns are found.
	Detect(doc *yaml.Node) bool

	// Apply transforms the YAML document in-place.
	// It should only be called if Detect returned true.
	Apply(doc *yaml.Node) error
}

// registry holds all registered migrations in order of application.
var registry []Migration

// Register adds a migration to the registry.
// Migrations are applied in registration order.
func Register(m Migration) {
	registry = append(registry, m)
}

// All returns all registered migrations.
func All() []Migration {
	return registry
}

// ForFileType returns migrations that apply to the given file type.
func ForFileType(ft FileType) []Migration {
	var result []Migration
	for _, m := range registry {
		for _, t := range m.FileTypes() {
			if t == ft {
				result = append(result, m)
				break
			}
		}
	}
	return result
}
