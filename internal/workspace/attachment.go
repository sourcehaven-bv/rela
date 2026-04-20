package workspace

import (
	"context"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// AttachmentInfo contains information about an attachment on an entity.
type AttachmentInfo struct {
	Property    string
	Path        string
	FileName    string
	ContentType string
	Size        int64
}

// AttachResult contains the outcome of attaching a file.
type AttachResult struct {
	Path     string
	FileName string
}

// AttachFile streams the file at filePath into the store at
// `attachments/<entityID>/<property>/<base(filePath)>` and records the
// path on the entity's property.
//
// If property is empty, the first file-type property defined on the
// entity type is used.
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

	src, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open source file: %w", err)
	}
	defer src.Close()

	fileName := filepath.Base(filePath)
	if err := w.Store().AttachFile(ctx, entityID, propName, fileName, src); err != nil {
		return nil, fmt.Errorf("store attachment: %w", err)
	}

	key := filepath.ToSlash(filepath.Join("attachments", entityID, propName, fileName))
	e.SetString(propName, key)
	if _, err := w.EntityManager().UpdateEntity(ctx, e); err != nil {
		return nil, fmt.Errorf("update entity: %w", err)
	}

	return &AttachResult{Path: key, FileName: fileName}, nil
}

// ListAttachments returns all attachments for an entity. Content type
// is inferred from the filename extension on the fly; there is no
// persisted metadata sidecar.
func (w *Workspace) ListAttachments(entityID string) ([]AttachmentInfo, error) {
	ctx := context.Background()

	items, err := w.Store().ListAttachments(ctx, entityID)
	if err != nil {
		return nil, err
	}

	infos := make([]AttachmentInfo, 0, len(items))
	for _, it := range items {
		path := filepath.ToSlash(filepath.Join("attachments", it.EntityID, it.Property, it.FileName))
		infos = append(infos, AttachmentInfo{
			Property:    it.Property,
			Path:        path,
			FileName:    it.FileName,
			ContentType: contentTypeForName(it.FileName),
			Size:        it.Size,
		})
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

// contentTypeForName infers a MIME type from a filename extension.
// Falls back to application/octet-stream — browsers render that as a
// download prompt, which is the right default for unknown types.
func contentTypeForName(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		return "application/octet-stream"
	}
	if mt := mime.TypeByExtension(ext); mt != "" {
		return mt
	}
	return "application/octet-stream"
}
