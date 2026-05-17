// Package attachment exposes the CLI-shaped facade for managing
// entity file attachments (attach, list). The service depends only
// on the focused primitives it needs (Store, Meta, EntityManager) so
// it can be constructed at any wiring site.
package attachment

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Info describes a single file attachment on an entity.
type Info struct {
	Property    string
	Path        string
	FileName    string
	ContentType string
	Size        int64
}

// Result is the outcome of [Service.Attach].
type Result struct {
	Path     string
	FileName string
}

// Deps is the dependency bundle [New] requires. Every field is
// mandatory; [New] returns an error if any is nil.
type Deps struct {
	Store         store.Store
	Meta          *metamodel.Metamodel
	EntityManager entitymanager.EntityManager
}

// Service implements the attachment-facade methods that CLI invokes.
// Constructed once at the wiring site and shared across subcommands.
type Service struct {
	deps Deps
}

// New constructs a Service. Returns an error if any required
// dependency is nil — CLAUDE.md "constructors reject nil required
// fields."
func New(d Deps) (*Service, error) {
	if d.Store == nil {
		return nil, errors.New("attachment: Store is required")
	}
	if d.Meta == nil {
		return nil, errors.New("attachment: Meta is required")
	}
	if d.EntityManager == nil {
		return nil, errors.New("attachment: EntityManager is required")
	}
	return &Service{deps: d}, nil
}

// Attach streams the file at filePath into the store at
// `attachments/<entityID>/<property>/<base(filePath)>` and records
// the path on the entity's property. The stored file overwrites any
// existing attachment at the same path — file-type properties hold
// at most one attachment.
//
// If property is empty, the first file-type property declared on the
// entity type (in alphabetical order — see [findFileProperty]) is
// used.
func (s *Service) Attach(ctx context.Context, entityID, filePath, property string) (*Result, error) {
	e, err := s.deps.Store.GetEntity(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("get entity %s: %w", entityID, err)
	}

	entityDef, ok := s.deps.Meta.GetEntityDef(e.Type)
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
	if err := s.deps.Store.AttachFile(ctx, entityID, propName, fileName, src); err != nil {
		return nil, fmt.Errorf("store attachment: %w", err)
	}

	key := filepath.ToSlash(filepath.Join("attachments", entityID, propName, fileName))
	e.SetString(propName, key)
	if _, err := s.deps.EntityManager.UpdateEntity(ctx, e); err != nil {
		// The file landed on disk before UpdateEntity ran, so a failure
		// here leaves an orphan. CleanupOrphanedTempFiles (analysis
		// facade) sweeps these — surface the path in the error so the
		// operator can find it.
		return nil, fmt.Errorf("update entity (attachment %s orphaned; run `rela gc --temp-files`): %w", key, err)
	}

	return &Result{Path: key, FileName: fileName}, nil
}

// List returns all attachments for an entity. Content type is
// inferred from the filename extension on the fly; there is no
// persisted metadata sidecar.
func (s *Service) List(ctx context.Context, entityID string) ([]Info, error) {
	items, err := s.deps.Store.ListAttachments(ctx, entityID)
	if err != nil {
		return nil, err
	}

	infos := make([]Info, 0, len(items))
	for _, it := range items {
		path := filepath.ToSlash(filepath.Join("attachments", it.EntityID, it.Property, it.FileName))
		infos = append(infos, Info{
			Property:    it.Property,
			Path:        path,
			FileName:    it.FileName,
			ContentType: contentTypeForName(it.FileName),
			Size:        it.Size,
		})
	}
	return infos, nil
}
