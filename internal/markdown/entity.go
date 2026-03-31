package markdown

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// EntityLoadResult contains results from loading entities.
type EntityLoadResult struct {
	Entities   []*model.Entity
	Conflicted []string // Paths of files with git conflicts
}

// ReadEntity reads an entity from a markdown file.
func (f *FileIO) ReadEntity(path string, meta *metamodel.Metamodel) (*model.Entity, error) {
	content, err := f.FS.ReadFile(path)
	if err != nil {
		return nil, err
	}

	doc, err := ParseDocument(string(content))
	if err != nil {
		return nil, err
	}

	id := doc.GetString("id")
	entityType := doc.GetString("type")

	// If type is not specified, try to infer from ID
	if entityType == "" && meta != nil && id != "" {
		entityType = meta.InferEntityType(id)
	}

	// Resolve aliases
	if meta != nil && entityType != "" {
		entityType = meta.ResolveAlias(entityType)
	}

	entity := &model.Entity{
		ID:         id,
		Type:       entityType,
		Properties: make(map[string]interface{}),
		Content:    doc.Content,
		FilePath:   path,
	}

	// Get file modification time
	if info, err := f.FS.Stat(path); err == nil {
		entity.ModTime = info.ModTime()
	}

	// Copy properties from frontmatter (excluding id and type)
	for key, value := range doc.Frontmatter {
		if key != "id" && key != "type" {
			entity.Properties[key] = value
		}
	}

	return entity, nil
}

// FormatEntity returns the formatted markdown content for an entity.
// The optional propertyOrder specifies the order for entity properties (after id and type).
// Both frontmatter ordering and markdown content formatting are applied.
// Uses default line width (80) for paragraph wrapping.
func FormatEntity(entity *model.Entity, propertyOrder []string) (string, error) {
	return FormatEntityWithWidth(entity, propertyOrder, DefaultLineWidth)
}

// FormatEntityWithWidth returns the formatted markdown content for an entity
// with a specific line width for paragraph wrapping.
func FormatEntityWithWidth(entity *model.Entity, propertyOrder []string, lineWidth int) (string, error) {
	frontmatter := make(map[string]interface{})
	frontmatter["id"] = entity.ID
	frontmatter["type"] = entity.Type

	// Copy all properties
	for key, value := range entity.Properties {
		frontmatter[key] = value
	}

	// Build key order: id, type, then property order (or alphabetical for remaining)
	keyOrder := []string{"id", "type"}
	if len(propertyOrder) > 0 {
		keyOrder = append(keyOrder, propertyOrder...)
	}

	// Format markdown content
	content := entity.Content
	if content != "" {
		content = FormatMarkdownWithWidth(content, lineWidth)
	}

	return FormatDocumentOrdered(frontmatter, content, keyOrder)
}

// WriteEntity writes an entity to a markdown file.
// The optional propertyOrder specifies the order for entity properties (after id and type).
func (f *FileIO) WriteEntity(entity *model.Entity, path string, propertyOrder ...[]string) error {
	var order []string
	if len(propertyOrder) > 0 {
		order = propertyOrder[0]
	}

	content, err := FormatEntity(entity, order)
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

// DeleteEntity removes an entity file.
func (f *FileIO) DeleteEntity(path string) error {
	return f.FS.Remove(path)
}

// ListEntityFiles returns all entity markdown files in the entities directory.
func (f *FileIO) ListEntityFiles(entitiesDir string) ([]string, error) {
	var files []string

	err := f.FS.Walk(entitiesDir, func(path string, info os.FileInfo, err error) error {
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

// LoadAllEntities loads all entities from the entities directory using parallel I/O.
func (f *FileIO) LoadAllEntities(entitiesDir string, meta *metamodel.Metamodel) ([]*model.Entity, error) {
	result, err := f.LoadAllEntitiesWithConflicts(entitiesDir, meta)
	if err != nil {
		return nil, err
	}
	return result.Entities, nil
}

// LoadAllEntitiesWithConflicts loads all entities and tracks conflicted files.
func (f *FileIO) LoadAllEntitiesWithConflicts(
	entitiesDir string, meta *metamodel.Metamodel,
) (*EntityLoadResult, error) {
	files, err := f.ListEntityFiles(entitiesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return &EntityLoadResult{Entities: []*model.Entity{}}, nil
		}
		return nil, err
	}

	if len(files) == 0 {
		return &EntityLoadResult{Entities: []*model.Entity{}}, nil
	}

	// Use worker pool for parallel file reading
	numWorkers := runtime.NumCPU()
	if numWorkers > len(files) {
		numWorkers = len(files)
	}

	type loadResult struct {
		entity     *model.Entity
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
				entity, readErr := f.ReadEntity(file, meta)
				if readErr != nil {
					if errors.Is(readErr, ErrConflictedFile) {
						resultChan <- loadResult{conflicted: file}
					}
					// Skip other files that can't be parsed
					continue
				}
				resultChan <- loadResult{entity: entity}
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
	result := &EntityLoadResult{
		Entities:   make([]*model.Entity, 0, len(files)),
		Conflicted: make([]string, 0),
	}
	for lr := range resultChan {
		if lr.entity != nil {
			result.Entities = append(result.Entities, lr.entity)
		}
		if lr.conflicted != "" {
			result.Conflicted = append(result.Conflicted, lr.conflicted)
		}
	}

	return result, nil
}

// EntityFileModTime returns the modification time of an entity file.
func (f *FileIO) EntityFileModTime(path string) (time.Time, error) {
	info, err := f.FS.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}
