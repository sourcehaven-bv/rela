package markdown

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

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
	frontmatter := map[string]interface{}{
		"from":     relation.From,
		"relation": relation.Type,
		"to":       relation.To,
	}

	// Add any additional properties
	for key, value := range relation.Properties {
		frontmatter[key] = value
	}

	content, err := FormatDocument(frontmatter, relation.Content)
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
	files, err := f.ListRelationFiles(relationsDir)
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return []*model.Relation{}, nil
	}

	// Use worker pool for parallel file reading
	numWorkers := runtime.NumCPU()
	if numWorkers > len(files) {
		numWorkers = len(files)
	}

	// Channels for work distribution and result collection
	fileChan := make(chan string, len(files))
	resultChan := make(chan *model.Relation, len(files))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range fileChan {
				relation, readErr := f.ReadRelation(file)
				if readErr != nil {
					// Skip files that can't be parsed
					continue
				}
				resultChan <- relation
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
	relations := make([]*model.Relation, 0, len(files))
	for relation := range resultChan {
		relations = append(relations, relation)
	}

	return relations, nil
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
