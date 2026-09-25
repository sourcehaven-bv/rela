// Package attachment exposes the CLI-shaped facade for managing
// entity file attachments (attach, list). The service depends only
// on the focused primitives it needs (Store, Meta, EntityManager) so
// it can be constructed at any wiring site.
package attachment

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/entity"
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

// Result is the outcome of [Service.Attach] and [Service.WriteAttachment].
type Result struct {
	Path     string
	FileName string

	// Entity is the post-write entity as stored, unredacted. A caller that
	// serves it must apply its own read-side redaction.
	Entity *entity.Entity
}

// EntityPatcher is the write surface the attachment service needs: recording
// an attachment changes exactly one property, the file property's stamped
// path(s). See the internal/entitymanager package doc for the consumer-side
// rule this follows (TKT-IVSJV6).
//
// PatchEntity rather than UpdateEntity: a whole-entity save would hold the
// record across the processor run (a scan may take up to a minute) and
// overwrite any property edited meanwhile. A patch names only the file
// property, so the service needs the entity's identity, never an owned full
// record, and a caller may hand it an entity read through a redacting reader.
type EntityPatcher interface {
	PatchEntity(ctx context.Context, id string, p entity.Patch) (*entity.UpdateResult, error)
}

// Deps is the dependency bundle [New] requires. Store, Meta and
// EntityManager are mandatory; [New] returns an error if any is nil.
// Processor is optional — when nil the service uses [NoopProcessor] and the
// write path stays zero-copy.
type Deps struct {
	Store         store.Store
	Meta          *metamodel.Metamodel
	EntityManager EntityPatcher

	// Processor inspects/rewrites attachment bytes before they are persisted
	// (scan, MIME validation, transform). Optional; defaults to [NoopProcessor].
	Processor Processor
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
	if d.Processor == nil {
		d.Processor = NoopProcessor{}
	}
	return &Service{deps: d}, nil
}

// Attach streams the file at filePath into the store at
// `attachments/<entityID>/<property>/<fileName>` and records the path(s)
// on the entity's property. Behavior depends on the property's `max`: at
// max==1 (the default) the upload replaces the single attachment; above 1
// it appends up to the cap, auto-suffixing the name on collision and
// erroring when full.
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

	return s.WriteAttachment(ctx, e, propDef, propName, filepath.Base(filePath), src)
}

// Open returns the bytes of one attachment. The caller closes the reader and
// is responsible for ACL gating. A missing file wraps [store.ErrNotFound].
func (s *Service) Open(ctx context.Context, entityID, property, fileName string) (io.ReadCloser, error) {
	return s.deps.Store.ReadAttachment(ctx, entityID, property, fileName)
}

// fileProperty loads the raw entity and resolves property to a declared
// file-type property of its type.
func (s *Service) fileProperty(
	ctx context.Context, entityID, property string,
) (*entity.Entity, metamodel.PropertyDef, error) {
	e, err := s.deps.Store.GetEntity(ctx, entityID)
	if err != nil {
		return nil, metamodel.PropertyDef{}, fmt.Errorf("get entity %s: %w", entityID, err)
	}
	entityDef, ok := s.deps.Meta.GetEntityDef(e.Type)
	if !ok {
		return nil, metamodel.PropertyDef{}, fmt.Errorf("unknown entity type: %s", e.Type)
	}
	propDef, ok := entityDef.Properties[property]
	if !ok {
		return nil, metamodel.PropertyDef{}, fmt.Errorf("property %q not defined for entity type %s", property, e.Type)
	}
	if propDef.Type != metamodel.PropertyTypeFile {
		return nil, metamodel.PropertyDef{}, fmt.Errorf("property %q is not a file type (is %s)", property, propDef.Type)
	}
	return e, propDef, nil
}

// ErrAtCapacity is returned by [Service.WriteAttachment] when a multi-file
// property already holds its `max` attachments. Callers (the HTTP handler)
// map it to a 409.
var ErrAtCapacity = errors.New("attachment: property already holds the maximum number of attachments")

// WriteAttachment is the shared write-path policy used by both the CLI
// (Attach) and the data-entry HTTP upload handler, so the cap/suffix/
// stamp rules live in exactly one place. The entity, its property def, and
// the (already-gated) property name are passed in; r supplies the bytes.
//
// Ordering is deliberate to avoid data loss: the new file is written
// FIRST, and only after it lands are superseded files removed (at max==1,
// the other files on the property). A store failure mid-write therefore
// leaves the existing attachment intact.
//
// Cap enforcement (the `max` ceiling) reads the current file set then
// writes, which is not atomic — it assumes writers to a given
// (entity, property) are serialized. The HTTP path holds a per-App write
// mutex; concurrent CLI invocations against the same property could race
// and overshoot the cap. That's acceptable for the single-writer CLI use.
func (s *Service) WriteAttachment(
	ctx context.Context, e *entity.Entity, propDef metamodel.PropertyDef, propName, rawFileName string, r io.Reader,
) (*Result, error) {
	maxCount := propDef.FileMax()

	existing, err := s.attachmentFileNames(ctx, e.ID, propName)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}

	fileName, err := resolveAttachName(rawFileName, existing, maxCount)
	if err != nil {
		return nil, err
	}

	// Inspect / transform the bytes before persisting. With the default
	// no-op processor this threads r straight through (zero-copy); a real
	// processor may buffer (when it needs the full file), reject the upload
	// (wrapping ErrRejected), or rewrite the stream and the file name.
	pc := ProcessContext{EntityID: e.ID, EntityType: e.Type, Property: propName, FileName: fileName}
	r, fileName, err = runProcessor(ctx, s.deps.Processor, pc, r, store.MaxAttachmentBytes)
	if err != nil {
		return nil, err
	}
	// A transform that changed the name may collide with an existing file or
	// exceed the cap; re-resolve against the current set.
	fileName, err = resolveAttachName(fileName, existing, maxCount)
	if err != nil {
		return nil, err
	}

	// Write the new bytes first. On failure the existing files are untouched.
	if err = s.deps.Store.AttachFile(ctx, e.ID, propName, fileName, r); err != nil {
		return nil, fmt.Errorf("store attachment: %w", err)
	}

	// Compute the post-write file set without a second ListAttachments: at
	// max==1 it's just the new file (and we delete the rest); above 1 it's
	// the prior set plus the new name (a same-name upload replaced in place,
	// so dedupe).
	var names []string
	if maxCount <= 1 {
		for _, old := range existing {
			if old == fileName {
				continue
			}
			_ = s.deps.Store.DeleteAttachment(ctx, e.ID, propName, old)
		}
		names = []string{fileName}
	} else {
		names = append(names, existing...)
		if !slices.Contains(names, fileName) {
			names = append(names, fileName)
		}
		sort.Strings(names)
	}

	key := attachPath(e.ID, propName, fileName)
	res, err := s.stamp(ctx, e, propName, maxCount, names)
	if err != nil {
		// The file landed on disk before the stamp ran, so a failure here
		// leaves an orphan. CleanupOrphanedTempFiles (analysis facade)
		// sweeps these — surface the path in the error so the operator can
		// find it.
		return nil, fmt.Errorf("update entity (attachment %s orphaned; run `rela gc --temp-files`): %w", key, err)
	}

	return &Result{Path: key, FileName: fileName, Entity: res.Entity}, nil
}

// DeleteAttachment removes one file from a property and re-stamps it,
// shared with the HTTP delete handler. The caller is responsible for ACL
// gating; this performs the store delete + property re-stamp + persist.
// Deleting a file that is not there still re-stamps and succeeds.
func (s *Service) DeleteAttachment(
	ctx context.Context, e *entity.Entity, propDef metamodel.PropertyDef, propName, fileName string,
) error {
	_, err := s.deleteFile(ctx, e, propDef, propName, fileName)
	return err
}

// deleteFile is [Service.DeleteAttachment], also reporting whether the file
// was there.
func (s *Service) deleteFile(
	ctx context.Context, e *entity.Entity, propDef metamodel.PropertyDef, propName, fileName string,
) (bool, error) {
	err := s.deps.Store.DeleteAttachment(ctx, e.ID, propName, fileName)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return false, fmt.Errorf("delete attachment: %w", err)
	}
	existed := err == nil
	names, err := s.attachmentFileNames(ctx, e.ID, propName)
	if err != nil {
		return false, fmt.Errorf("list attachments: %w", err)
	}
	// A retried delete changes nothing, so it must not write the entity: a
	// write is audited and runs `on: update` automations. A stale property
	// value is still repaired.
	if !existed && sameStamp(e.Properties[propName], stampValue(e.ID, propName, propDef.FileMax(), names)) {
		return false, nil
	}
	if _, err := s.stamp(ctx, e, propName, propDef.FileMax(), names); err != nil {
		return false, fmt.Errorf("update entity: %w", err)
	}
	return existed, nil
}

// ErrNoFileToDetach is returned by [Service.DetachFile] when no file name was
// given and the property holds no file, or several.
var ErrNoFileToDetach = errors.New("no file to detach")

// Detach removes one attachment from a file-type property of entityID; see
// [Service.DetachFile] for the fileName rules and the result.
func (s *Service) Detach(ctx context.Context, entityID, property, fileName string) (string, error) {
	e, propDef, err := s.fileProperty(ctx, entityID, property)
	if err != nil {
		return "", err
	}
	return s.DetachFile(ctx, e, propDef, property, fileName)
}

// DetachFile removes one attachment from a file-type property and returns
// the name of the file it removed. A named file is deleted idempotently: when
// it is not there, DetachFile succeeds and returns "". An empty fileName
// removes the property's only file, and errors when the property holds none
// or several (the caller must disambiguate). The caller is responsible for
// ACL gating.
func (s *Service) DetachFile(
	ctx context.Context, e *entity.Entity, propDef metamodel.PropertyDef, property, fileName string,
) (string, error) {
	if fileName == "" {
		existing, err := s.attachmentFileNames(ctx, e.ID, property)
		if err != nil {
			return "", fmt.Errorf("list attachments: %w", err)
		}
		switch len(existing) {
		case 0:
			return "", fmt.Errorf("%w: property %q has no attachment", ErrNoFileToDetach, property)
		case 1:
			fileName = existing[0]
		default:
			return "", fmt.Errorf("%w: property %q holds %d files; specify which to detach: %v",
				ErrNoFileToDetach, property, len(existing), existing)
		}
	}
	existed, err := s.deleteFile(ctx, e, propDef, property, fileName)
	if err != nil || !existed {
		return "", err
	}
	return fileName, nil
}

// attachmentFileNames lists the file names currently on the property, in
// stable order. A real store error is returned (not swallowed) because the
// write path makes cap / replace decisions from this list — acting on a
// degraded view could overshoot the cap or skip the replace-delete.
func (s *Service) attachmentFileNames(ctx context.Context, entityID, property string) ([]string, error) {
	infos, err := s.deps.Store.ListAttachments(ctx, entityID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, info := range infos {
		if info.Property == property {
			names = append(names, info.FileName)
		}
	}
	sort.Strings(names)
	return names, nil
}

// stamp records names as the property's value with a patch that names only
// that property (see [EntityPatcher]). e supplies identity only and is not
// modified.
func (s *Service) stamp(
	ctx context.Context, e *entity.Entity, property string, maxCount int, names []string,
) (*entity.UpdateResult, error) {
	patch := entity.Patch{Properties: map[string]any{property: stampValue(e.ID, property, maxCount, names)}}
	return s.deps.EntityManager.PatchEntity(ctx, entity.FormatStateRef(e.ID, e.Face), patch)
}

// stampValue is the property value for a known set of file names: a scalar
// path for a single-cap property (empty when none), a list of paths for a
// multi-cap property.
func stampValue(entityID, property string, maxCount int, names []string) any {
	if maxCount <= 1 {
		if len(names) == 0 {
			return ""
		}
		return attachPath(entityID, property, names[0])
	}
	paths := make([]string, 0, len(names))
	for _, n := range names {
		paths = append(paths, attachPath(entityID, property, n))
	}
	return paths
}

// sameStamp reports whether a stored property value equals want, a
// [stampValue] result. A list may load as []string or []any, and an unset
// value equals the empty stamp.
func sameStamp(stored, want any) bool {
	switch w := want.(type) {
	case string:
		s, _ := stored.(string)
		return s == w
	case []string:
		var got []string
		switch v := stored.(type) {
		case []string:
			got = v
		case []any:
			for _, x := range v {
				str, ok := x.(string)
				if !ok {
					return false
				}
				got = append(got, str)
			}
		case nil:
		default:
			return false
		}
		return slices.Equal(got, w)
	}
	return false
}

func attachPath(entityID, property, fileName string) string {
	return filepath.ToSlash(filepath.Join("attachments", entityID, property, fileName))
}

// resolveAttachName applies the filename policy for an attach: normalize
// (NormalizeFileName always yields a usable name, never ""), then at max>1
// auto-suffix on collision and return ErrAtCapacity when full. At max==1
// the name is whatever was uploaded (the caller replaces).
func resolveAttachName(rawName string, existing []string, maxCount int) (string, error) {
	name := store.NormalizeFileName(rawName)
	if maxCount <= 1 {
		return name, nil
	}
	if len(existing) >= maxCount {
		return "", ErrAtCapacity
	}
	have := make(map[string]bool, len(existing))
	for _, f := range existing {
		have[f] = true
	}
	// Auto-suffix on collision so a duplicate upload adds a distinct file.
	return store.SuffixOnCollision(name, func(c string) bool { return have[c] }), nil
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
