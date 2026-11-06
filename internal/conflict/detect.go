package conflict

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// DetectAll scans all entity and relation files for git conflicts.
func DetectAll(ctx *project.Context) (*DetectResult, error) {
	result := &DetectResult{
		Files: make([]ConflictedFile, 0),
	}

	// Scan entity files
	entityFiles, err := listMarkdownFiles(ctx.EntitiesDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	for _, path := range entityFiles {
		conflicted, detectErr := DetectInFile(path)
		if detectErr != nil {
			continue // Skip files we can't read
		}
		if conflicted != nil {
			// Try to determine entity type from path
			conflicted.EntityType = inferEntityType(path, ctx.EntitiesDir)
			result.Files = append(result.Files, *conflicted)
		}
	}

	// Scan relation files
	relationFiles, err := listMarkdownFiles(ctx.RelationsDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	for _, path := range relationFiles {
		conflicted, detectErr := DetectInFile(path)
		if detectErr != nil {
			continue // Skip files we can't read
		}
		if conflicted != nil {
			result.Files = append(result.Files, *conflicted)
		}
	}

	return result, nil
}

// DetectInFile checks a single file for git conflicts.
// Returns nil, nil if the file has no conflicts (this is not an error condition).
func DetectInFile(path string) (*ConflictedFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	markers := FindMarkers(string(content))
	if len(markers) == 0 {
		return nil, nil //nolint:nilnil // no conflicts is not an error
	}

	return &ConflictedFile{
		Path:    path,
		Markers: markers,
	}, nil
}

// FindMarkers locates the conflict blocks in content, as
// [markdown.ConflictBlocks] defines them.
func FindMarkers(content string) []Marker {
	return markdown.ConflictBlocks(content)
}

// HasConflicts checks if content has any conflict markers.
func HasConflicts(content string) bool {
	return markdown.HasConflictMarkersString(content)
}

// listMarkdownFiles returns all .md files in a directory tree.
func listMarkdownFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".md") {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

// inferEntityType tries to determine entity type from file path.
// Entity files are stored as entities/<type>/<id>.md
func inferEntityType(path, entitiesDir string) string {
	rel, err := filepath.Rel(entitiesDir, path)
	if err != nil {
		return ""
	}

	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) >= 2 {
		return parts[0]
	}
	return ""
}
