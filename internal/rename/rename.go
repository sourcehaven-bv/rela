// Package rename defines the public types for entity rename operations.
//
// Historically this package also contained the rename orchestration as
// a free function `Rename(repo, meta, g, ...)`. The orchestration moved
// into the workspace package as `Workspace.Rename` so it could use the
// workspace.Tx primitive (TKT-PNPI / PR #351). The types remain here as
// DTOs returned by `Workspace.Rename` and consumed by the CLI rename
// command — they live in their own package to avoid pulling the
// workspace into the CLI's import graph for type-only references.
package rename

// Options configures the rename operation.
type Options struct {
	DryRun bool // If true, return what would change without making changes
}

// RelationRef identifies a relation by its three-part key.
type RelationRef struct {
	From string `json:"from"`
	Type string `json:"type"`
	To   string `json:"to"`
}

// Result contains the outcome of a rename operation.
type Result struct {
	OldID            string
	NewID            string
	EntityType       string
	EntityFile       string        // Path to new entity file
	RelationsUpdated []RelationRef // Relations that were updated
	OldFilesDeleted  []string      // Files that were deleted
}
