package markdown

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

// RelationLoadResult contains results from loading relations.
type RelationLoadResult struct {
	Relations  []*model.Relation
	Conflicted []string // Paths of files with git conflicts
}

// ReadRelation reads a relation from a markdown file.
func (f *FileIO) ReadRelation(path string) (*model.Relation, error) {
	content, err := f.FS.ReadFile(path)
	if err != nil {
		return nil, err
	}

	doc, err := ParseDocument(string(content))
	if err != nil {
		return nil, err
	}

	relation := &model.Relation{
		From:     doc.GetString("from"),
		Type:     doc.GetString("relation"),
		To:       doc.GetString("to"),
		Content:  doc.Content,
		FilePath: path,
	}

	// Get file modification time
	if info, err := f.FS.Stat(path); err == nil {
		relation.ModTime = info.ModTime()
	}

	// Copy any additional properties
	relation.Properties = make(map[string]interface{})
	for key, value := range doc.Frontmatter {
		if key != "from" && key != "relation" && key != "to" {
			relation.Properties[key] = value
		}
	}

	return relation, nil
}

// WriteRelation writes a relation to a markdown file.
func (f *FileIO) WriteRelation(relation *model.Relation, path string) error {
	content, err := FormatRelation(relation)
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := f.FS.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return f.FS.WriteFile(path, []byte(content), 0644)
}

// FormatRelation returns the formatted markdown content for a relation.
// Frontmatter keys are ordered: from, relation, to, then extras alphabetically.
// Markdown content is also formatted.
func FormatRelation(relation *model.Relation) (string, error) {
	frontmatter := map[string]interface{}{
		"from":     relation.From,
		"relation": relation.Type,
		"to":       relation.To,
	}

	// Add any additional properties
	for key, value := range relation.Properties {
		frontmatter[key] = value
	}

	// Key order: from, relation, to, then extras alphabetically
	keyOrder := []string{"from", "relation", "to"}

	// Format markdown content
	content := relation.Content
	if content != "" {
		content = FormatMarkdown(content)
	}

	return FormatDocumentOrdered(frontmatter, content, keyOrder)
}

// DeleteRelation removes a relation file.
func (f *FileIO) DeleteRelation(path string) error {
	return f.FS.Remove(path)
}

// ListRelationFiles returns all relation markdown files in the relations directory.
func (f *FileIO) ListRelationFiles(relationsDir string) ([]string, error) {
	var files []string

	if _, err := f.FS.Stat(relationsDir); os.IsNotExist(err) {
		return files, nil
	}

	entries, err := f.FS.ReadDir(relationsDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			files = append(files, filepath.Join(relationsDir, entry.Name()))
		}
	}

	return files, nil
}

// LoadAllRelations loads all relations from the relations directory using parallel I/O.
func (f *FileIO) LoadAllRelations(relationsDir string) ([]*model.Relation, error) {
	result, err := f.LoadAllRelationsWithConflicts(relationsDir)
	if err != nil {
		return nil, err
	}
	return result.Relations, nil
}

// LoadAllRelationsWithConflicts loads all relations and tracks conflicted files.
func (f *FileIO) LoadAllRelationsWithConflicts(relationsDir string) (*RelationLoadResult, error) {
	files, err := f.ListRelationFiles(relationsDir)
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return &RelationLoadResult{Relations: []*model.Relation{}}, nil
	}

	// Use worker pool for parallel file reading
	numWorkers := runtime.NumCPU()
	if numWorkers > len(files) {
		numWorkers = len(files)
	}

	type loadResult struct {
		relation   *model.Relation
		conflicted string // Non-empty if file has conflicts
	}

	// Channels for work distribution and result collection
	fileChan := make(chan string, len(files))
	resultChan := make(chan loadResult, len(files))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range fileChan {
				relation, readErr := f.ReadRelation(file)
				if readErr != nil {
					if errors.Is(readErr, ErrConflictedFile) {
						resultChan <- loadResult{conflicted: file}
					}
					// Skip other files that can't be parsed
					continue
				}
				resultChan <- loadResult{relation: relation}
			}
		}()
	}

	// Send files to workers
	for _, file := range files {
		fileChan <- file
	}
	close(fileChan)

	// Wait for all workers to finish, then close result channel
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	result := &RelationLoadResult{
		Relations:  make([]*model.Relation, 0, len(files)),
		Conflicted: make([]string, 0),
	}
	for lr := range resultChan {
		if lr.relation != nil {
			result.Relations = append(result.Relations, lr.relation)
		}
		if lr.conflicted != "" {
			result.Conflicted = append(result.Conflicted, lr.conflicted)
		}
	}

	return result, nil
}

// RelationFilename generates a filename for a relation.
func RelationFilename(from, relationType, to string) string {
	return fmt.Sprintf("%s--%s--%s.md", from, relationType, to)
}

// ParseRelationFilename extracts from, relation, to from a filename.
func ParseRelationFilename(filename string) (from, relationType, to string, ok bool) {
	// Remove .md extension
	name := strings.TrimSuffix(filename, ".md")

	// Split by --
	parts := strings.Split(name, "--")
	if len(parts) != 3 {
		return "", "", "", false
	}

	// Reject empty parts
	if parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", false
	}

	return parts[0], parts[1], parts[2], true
}
