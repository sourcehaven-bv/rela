package markdown

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

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

// WriteEntity writes an entity to a markdown file.
func (f *FileIO) WriteEntity(entity *model.Entity, path string) error {
	frontmatter := make(map[string]interface{})
	frontmatter["id"] = entity.ID
	frontmatter["type"] = entity.Type

	// Add properties in a consistent order
	// First the common ones
	if title := entity.GetString("title"); title != "" {
		frontmatter["title"] = title
	}
	if status := entity.GetString("status"); status != "" {
		frontmatter["status"] = status
	}

	// Then the rest
	for key, value := range entity.Properties {
		if key != "title" && key != "status" {
			frontmatter[key] = value
		}
	}

	content, err := FormatDocument(frontmatter, entity.Content)
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
	files, err := f.ListEntityFiles(entitiesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*model.Entity{}, nil
		}
		return nil, err
	}

	if len(files) == 0 {
		return []*model.Entity{}, nil
	}

	// Use worker pool for parallel file reading
	numWorkers := runtime.NumCPU()
	if numWorkers > len(files) {
		numWorkers = len(files)
	}

	// Channels for work distribution and result collection
	fileChan := make(chan string, len(files))
	resultChan := make(chan *model.Entity, len(files))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range fileChan {
				entity, readErr := f.ReadEntity(file, meta)
				if readErr != nil {
					// Skip files that can't be parsed
					continue
				}
				resultChan <- entity
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
	entities := make([]*model.Entity, 0, len(files))
	for entity := range resultChan {
		entities = append(entities, entity)
	}

	return entities, nil
}

// EntityFileModTime returns the modification time of an entity file.
func (f *FileIO) EntityFileModTime(path string) (time.Time, error) {
	info, err := f.FS.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}
