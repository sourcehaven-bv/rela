package conflict

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// Conflict marker prefixes
const (
	markerStart = "<<<<<<<"
	markerMid   = "======="
	markerEnd   = ">>>>>>>"
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

	if !markdown.HasConflictMarkers(content) {
		return nil, nil //nolint:nilnil // no conflicts is not an error
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

// FindMarkers locates all conflict marker regions in content.
func FindMarkers(content string) []Marker {
	var markers []Marker

	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0
	var current *Marker

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, markerStart):
			// Start of new conflict
			current = &Marker{
				StartLine: lineNum,
				OursRef:   strings.TrimSpace(strings.TrimPrefix(line, markerStart)),
			}

		case strings.HasPrefix(line, markerMid) && current != nil:
			// Middle marker
			current.MidLine = lineNum

		case strings.HasPrefix(line, markerEnd) && current != nil:
			// End of conflict
			current.EndLine = lineNum
			current.TheirsRef = strings.TrimSpace(strings.TrimPrefix(line, markerEnd))
			markers = append(markers, *current)
			current = nil
		}
	}

	return markers
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
