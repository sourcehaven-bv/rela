package workspace

import (
	"fmt"
	"os/user"

	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// AttachmentInfo contains information about an attachment on an entity.
type AttachmentInfo struct {
	Property     string
	Path         string
	OriginalName string
	ContentType  string
	Size         int64
}

// AttachResult contains the outcome of attaching a file.
type AttachResult struct {
	Path         string
	OriginalName string
	Deduplicated bool // true if file already existed in store
}

// AttachFile attaches a file to an entity's file property.
// If property is empty, it uses the first file-type property defined for the entity type.
// The file is stored in the content-addressable attachment store and the entity is updated.
func (w *Workspace) AttachFile(entityID, filePath, property string) (*AttachResult, error) {
	// Get entity from graph
	entity, ok := w.graph.GetNode(entityID)
	if !ok {
		return nil, fmt.Errorf("entity not found: %s", entityID)
	}

	meta := w.Meta()

	// Get entity definition
	entityDef, ok := meta.GetEntityDef(entity.Type)
	if !ok {
		return nil, fmt.Errorf("unknown entity type: %s", entity.Type)
	}

	// Determine which property to use
	propName := property
	if propName == "" {
		propName = findFileProperty(entityDef)
		if propName == "" {
			return nil, fmt.Errorf("no file property defined for entity type %s; specify property explicitly", entity.Type)
		}
	}

	// Validate property exists and is file type
	propDef, ok := entityDef.Properties[propName]
	if !ok {
		return nil, fmt.Errorf("property %q not defined for entity type %s", propName, entity.Type)
	}
	if propDef.Type != metamodel.PropertyTypeFile {
		return nil, fmt.Errorf("property %q is not a file type (is %s)", propName, propDef.Type)
	}

	// Get current user for metadata
	addedBy := ""
	if u, err := user.Current(); err == nil {
		addedBy = u.Username
	}

	// Create attachment store and add file
	store := attachment.NewStore(w.FS(), w.Paths().Root)
	att, err := store.Add(filePath, addedBy)
	if err != nil {
		return nil, fmt.Errorf("failed to store attachment: %w", err)
	}

	// Clone before mutation so we can diff old vs new
	oldEntity := entity.Clone()

	// Update entity property with the attachment path
	entity.SetString(propName, att.Path)

	// Write through workspace (validates, persists, updates graph+cache)
	if _, err := w.UpdateEntity(entity, oldEntity); err != nil {
		return nil, fmt.Errorf("failed to update entity: %w", err)
	}

	result := &AttachResult{
		Path:         att.Path,
		OriginalName: att.Metadata.OriginalName,
		Deduplicated: false, // TODO: detect if file was deduplicated
	}

	return result, nil
}

// ListAttachments returns all attachments for an entity.
func (w *Workspace) ListAttachments(entityID string) ([]AttachmentInfo, error) {
	// Get entity from graph
	entity, ok := w.graph.GetNode(entityID)
	if !ok {
		return nil, fmt.Errorf("entity not found: %s", entityID)
	}

	meta := w.Meta()

	// Get entity definition
	entityDef, ok := meta.GetEntityDef(entity.Type)
	if !ok {
		return nil, fmt.Errorf("unknown entity type: %s", entity.Type)
	}

	// Create attachment store for metadata lookup
	store := attachment.NewStore(w.FS(), w.Paths().Root)

	var infos []AttachmentInfo

	// Iterate over all file-type properties
	for propName, propDef := range entityDef.Properties {
		if propDef.Type != metamodel.PropertyTypeFile {
			continue
		}

		val, ok := entity.Properties[propName]
		if !ok || val == nil {
			continue
		}

		// Extract paths from property value
		paths := extractPaths(val)

		for _, path := range paths {
			info := AttachmentInfo{
				Property: propName,
				Path:     path,
			}

			// Try to get metadata
			if meta, err := store.GetMetadata(path); err == nil {
				info.OriginalName = meta.OriginalName
				info.ContentType = meta.ContentType
				info.Size = meta.Size
			}

			infos = append(infos, info)
		}
	}

	return infos, nil
}

// findFileProperty returns the first file-type property name for an entity definition.
func findFileProperty(entityDef *metamodel.EntityDef) string {
	for name, prop := range entityDef.Properties {
		if prop.Type == metamodel.PropertyTypeFile {
			return name
		}
	}
	return ""
}

// extractPaths extracts attachment paths from a property value.
func extractPaths(val interface{}) []string {
	if val == nil {
		return nil
	}

	switch v := val.(type) {
	case string:
		if v != "" {
			return []string{v}
		}
	case []interface{}:
		var paths []string
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				paths = append(paths, s)
			}
		}
		return paths
	case []string:
		return v
	}

	return nil
}
