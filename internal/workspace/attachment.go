package workspace

import (
	"bytes"
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
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

// Attachments returns the top-level attachment manager backing this workspace.
// Callers that only need pure byte storage (e.g. upload endpoints) use it
// directly; the workspace itself composes it with entity-property updates
// in AttachFile/ListAttachments below.
func (w *Workspace) Attachments() attachment.Manager {
	return w.attachmentStore()
}

// attachmentStore returns the concrete content-addressable backend.
// Used by workspace internals that need backend-specific operations
// (GC) beyond the Manager interface.
func (w *Workspace) attachmentStore() *attachment.Store {
	return attachment.NewStore(w.FS(), w.FS(), w.Paths().Root)
}

// AttachFile attaches a file to an entity's file property.
// If property is empty, it uses the first file-type property defined for the entity type.
// The file is stored via the attachment manager and the entity is updated.
func (w *Workspace) AttachFile(entityID, filePath, property string) (*AttachResult, error) {
	ctx := context.Background()

	e, err := w.Store().GetEntity(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("entity not found: %s", entityID)
	}

	meta := w.Meta()
	entityDef, ok := meta.GetEntityDef(e.Type)
	if !ok {
		return nil, fmt.Errorf("unknown entity type: %s", e.Type)
	}

	propName := property
	if propName == "" {
		propName = findFileProperty(entityDef)
		if propName == "" {
			return nil, fmt.Errorf("no file property defined for entity type %s; specify property explicitly", e.Type)
		}
	}

	propDef, ok := entityDef.Properties[propName]
	if !ok {
		return nil, fmt.Errorf("property %q not defined for entity type %s", propName, e.Type)
	}
	if propDef.Type != metamodel.PropertyTypeFile {
		return nil, fmt.Errorf("property %q is not a file type (is %s)", propName, propDef.Type)
	}

	data, err := w.FS().ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read source file: %w", err)
	}

	info, err := w.Attachments().AttachFile(ctx, entityID, propName, filePath, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to store attachment: %w", err)
	}

	e.SetString(propName, info.Key)
	if _, err := w.EntityManager().UpdateEntity(ctx, e); err != nil {
		return nil, fmt.Errorf("failed to update entity: %w", err)
	}

	return &AttachResult{
		Path:         info.Key,
		OriginalName: info.OriginalName,
		Deduplicated: false, // TODO: detect if file was deduplicated
	}, nil
}

// ListAttachments returns all attachments for an entity.
func (w *Workspace) ListAttachments(entityID string) ([]AttachmentInfo, error) {
	ctx := context.Background()

	e, err := w.Store().GetEntity(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("entity not found: %s", entityID)
	}

	meta := w.Meta()
	entityDef, ok := meta.GetEntityDef(e.Type)
	if !ok {
		return nil, fmt.Errorf("unknown entity type: %s", e.Type)
	}

	mgr := w.Attachments()
	var infos []AttachmentInfo
	for propName, propDef := range entityDef.Properties {
		if propDef.Type != metamodel.PropertyTypeFile {
			continue
		}
		val, ok := e.Properties[propName]
		if !ok || val == nil {
			continue
		}
		for _, path := range extractPaths(val) {
			info := AttachmentInfo{Property: propName, Path: path}
			if meta, err := mgr.InfoFor(ctx, path); err == nil {
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

// GCAttachmentsResult contains the outcome of garbage collecting attachments.
type GCAttachmentsResult struct {
	Removed   []string // Paths that were (or would be) removed
	Reclaimed int64    // Bytes reclaimed (or that would be reclaimed)
}

// GCAttachments removes unreferenced attachment files from the content-
// addressable backend. If dryRun is true, it returns what would be removed
// without actually removing.
func (w *Workspace) GCAttachments(dryRun bool) (*GCAttachmentsResult, error) {
	referencedPaths := w.collectReferencedAttachmentPaths()
	cas := w.attachmentStore()

	gcResult, err := cas.GC(referencedPaths)
	if err != nil {
		return nil, fmt.Errorf("gc failed: %w", err)
	}

	result := &GCAttachmentsResult{
		Removed:   gcResult.Removed,
		Reclaimed: gcResult.Reclaimed,
	}

	if !dryRun && len(gcResult.Removed) > 0 {
		if err := cas.RemoveUnreferenced(gcResult); err != nil {
			return nil, fmt.Errorf("failed to remove files: %w", err)
		}
	}

	return result, nil
}

// collectReferencedAttachmentPaths returns all attachment paths referenced by entities.
func (w *Workspace) collectReferencedAttachmentPaths() []string {
	meta := w.Meta()
	var paths []string
	for _, e := range collectEntities(w.Store(), store.EntityQuery{}) {
		entityDef, ok := meta.GetEntityDef(e.Type)
		if !ok {
			continue
		}
		for propName, propDef := range entityDef.Properties {
			if propDef.Type != metamodel.PropertyTypeFile {
				continue
			}
			paths = append(paths, extractPaths(e.Properties[propName])...)
		}
	}
	return paths
}
