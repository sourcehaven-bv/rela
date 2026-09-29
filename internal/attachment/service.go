package attachment

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// # The model: bytes per entity, value per face (BUG-CTUW2N)
//
// Bytes are keyed (entity id, property, file name) and shared by every face
// of the entity. A face's files are the base names in its OWN property value
// ([metamodel.FileNames]). Listing and download serve only those names, so a
// face never sees another face's upload. Writes run under the entity's
// attachment lock ([entitymanager.AttachmentLockKey]) and read the whole
// family raw: that is write-prep, and it
// must include faces the caller cannot read, or an upload could overwrite
// their bytes. Bytes are deleted when the last face stops referencing them.
//
// Names are unique per entity, so an upload whose name another face uses
// is suffixed (see resolveAttachName). A writer on one face can therefore
// learn that some face of the entity holds a file of that name, but not
// read it. That is an accepted one-bit channel (docs/acl-security.md).

// ErrFaceRequired is returned when a bare id names an entity whose faces
// are all stored at a named face: there is no bare face to attach to, and
// guessing one would put the file on a face the caller did not choose.
var ErrFaceRequired = errors.New("attachment: address one face")

// Resolve reads the face ref addresses, raw. A bare id of a faced entity is
// refused with [ErrFaceRequired], naming the faces.
func (s *Service) Resolve(ctx context.Context, ref entity.Ref) (*entity.Entity, error) {
	e, err := s.deps.Store.GetEntityState(ctx, ref.ID, ref.Face)
	if err == nil {
		return e, nil
	}
	if errors.Is(err, store.ErrNotFound) && ref.Face.IsDefault() {
		family, ferr := s.family(ctx, ref.ID)
		if ferr != nil {
			return nil, fmt.Errorf("get entity %s: %w", ref, ferr)
		}
		if len(family) > 0 {
			addrs := make([]string, 0, len(family))
			for _, f := range family {
				addrs = append(addrs, entity.FormatStateRef(f.ID, f.Face))
			}
			sort.Strings(addrs)
			return nil, fmt.Errorf("%w: %s has faces; use one of %s",
				ErrFaceRequired, ref.ID, strings.Join(addrs, ", "))
		}
	}
	return nil, fmt.Errorf("get entity %s: %w", ref, err)
}

// Attach streams the file at filePath into the store and records it on the
// addressed face's property. Behavior depends on the property's `max`: at
// max==1 (the default) the upload replaces the face's single attachment;
// above 1 it appends up to the cap, auto-suffixing the name on collision
// and erroring when full.
//
// If property is empty, the first file-type property declared on the
// entity type (in alphabetical order — see [findFileProperty]) is
// used.
func (s *Service) Attach(ctx context.Context, ref entity.Ref, filePath, property string) (*Result, error) {
	e, err := s.Resolve(ctx, ref)
	if err != nil {
		return nil, err
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

	propDef, err := filePropertyDef(entityDef, e.Type, propName)
	if err != nil {
		return nil, err
	}

	src, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open source file: %w", err)
	}
	defer src.Close()

	return s.WriteAttachment(ctx, e, propDef, propName, filepath.Base(filePath), src)
}

// Open returns the bytes of one attachment of the face e. The name must be
// one the face's own property value references, else the result wraps
// [store.ErrNotFound], as for a missing file: the value is the face's
// capability to the shared bytes. The caller closes the reader and is
// responsible for ACL gating; e may be a redacted read, in which case a
// hidden property references nothing.
func (s *Service) Open(ctx context.Context, e *entity.Entity, property, fileName string) (io.ReadCloser, error) {
	if !References(e, property, fileName) {
		return nil, fmt.Errorf("attachment %s/%s: %w", property, fileName, store.ErrNotFound)
	}
	return s.deps.Store.ReadAttachment(ctx, e.ID, property, fileName)
}

// References reports whether the face e's own value of property names
// fileName. It is the face gate for the shared bytes.
func References(e *entity.Entity, property, fileName string) bool {
	if e == nil {
		return false
	}
	return slices.Contains(metamodel.FileNames(e.Properties[property]), fileName)
}

// filePropertyDef resolves property to a declared file-type property.
func filePropertyDef(def *metamodel.EntityDef, entityType, property string) (metamodel.PropertyDef, error) {
	propDef, ok := def.Properties[property]
	if !ok {
		return metamodel.PropertyDef{}, fmt.Errorf("property %q not defined for entity type %s", property, entityType)
	}
	if propDef.Type != metamodel.PropertyTypeFile {
		return metamodel.PropertyDef{}, fmt.Errorf("property %q is not a file type (is %s)", property, propDef.Type)
	}
	return propDef, nil
}

// fileProperty resolves ref raw and property to a declared file-type
// property of its type.
func (s *Service) fileProperty(
	ctx context.Context, ref entity.Ref, property string,
) (*entity.Entity, metamodel.PropertyDef, error) {
	e, err := s.Resolve(ctx, ref)
	if err != nil {
		return nil, metamodel.PropertyDef{}, err
	}
	entityDef, ok := s.deps.Meta.GetEntityDef(e.Type)
	if !ok {
		return nil, metamodel.PropertyDef{}, fmt.Errorf("unknown entity type: %s", e.Type)
	}
	propDef, err := filePropertyDef(entityDef, e.Type, property)
	if err != nil {
		return nil, metamodel.PropertyDef{}, err
	}
	return e, propDef, nil
}

// ErrAtCapacity is returned by [Service.WriteAttachment] when a multi-file
// property already holds its `max` attachments on the face. Callers (the
// HTTP handler) map it to a 409.
var ErrAtCapacity = errors.New("attachment: property already holds the maximum number of attachments")

// WriteAttachment is the shared write-path policy used by both the CLI
// (Attach) and the data-entry HTTP upload handler, so the cap/suffix/
// stamp rules live in exactly one place. The face e, its property def, and
// the (already-gated) property name are passed in; r supplies the bytes.
//
// Ordering is deliberate to avoid data loss: the new file is written
// FIRST, then the face's value is stamped, and only then are the names the
// face dropped (at max==1, its previous file) deleted, when no other face
// references them. A failure mid-way leaves the existing attachment intact.
//
// The cap, the name choice and the stamp are read-modify-writes over the
// family's references, so they run under the entity's attachment lock. It
// works across processes when Deps.Locker is backed by the
// shared store. The slow steps stay outside the lock: the processor (a scan
// may take up to a minute) and reading r, which is spooled to a temporary
// file first, so a slow client cannot hold the lock.
func (s *Service) WriteAttachment(
	ctx context.Context, e *entity.Entity, propDef metamodel.PropertyDef, propName, rawFileName string, r io.Reader,
) (*Result, error) {
	maxCount := propDef.FileMax()

	// Fail fast on a full property before reading or scanning any bytes. The
	// check is repeated under the lock below, where it counts.
	refs, err := s.references(ctx, e, propName)
	if err != nil {
		return nil, err
	}
	fileName, err := resolveAttachName(rawFileName, refs, maxCount)
	if err != nil {
		return nil, err
	}

	// Inspect / transform the bytes before persisting. A processor may buffer
	// (when it needs the full file), reject the upload (wrapping
	// ErrRejected), or rewrite the stream and the file name.
	pc := ProcessContext{EntityID: e.ID, EntityType: e.Type, Property: propName, FileName: fileName}
	r, fileName, err = runProcessor(ctx, s.deps.Processor, pc, r, store.MaxAttachmentBytes)
	if err != nil {
		return nil, err
	}
	spool, err := spoolAttachment(r)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = spool.Close()
		_ = os.Remove(spool.Name())
	}()

	release, err := s.lockForWrite(ctx, e, propName)
	if err != nil {
		return nil, err
	}
	defer release()

	refs, err = s.references(ctx, e, propName)
	if err != nil {
		return nil, err
	}
	// Resolve against the references as they are now: a concurrent upload
	// may have filled the property or taken the name since the check above,
	// and a transform may have changed the name.
	fileName, err = resolveAttachName(fileName, refs, maxCount)
	if err != nil {
		return nil, err
	}

	// Write the new bytes first. On failure the existing files are untouched.
	if err = s.deps.Store.AttachFile(ctx, e.ID, propName, fileName, spool); err != nil {
		return nil, fmt.Errorf("store attachment: %w", err)
	}

	// The face's new names come from its value, not from the byte listing,
	// which also holds the other faces' files.
	var names, dropped []string
	if maxCount <= 1 {
		names = []string{fileName}
		for _, old := range refs.own {
			if old != fileName {
				dropped = append(dropped, old)
			}
		}
	} else {
		names = append(names, refs.own...)
		if !slices.Contains(names, fileName) {
			names = append(names, fileName)
		}
		sort.Strings(names)
	}

	key := attachPath(e.ID, propName, fileName)
	res, err := s.stamp(ctx, e, propName, maxCount, names)
	if err != nil {
		// Nothing references fresh bytes until the stamp lands, so they go
		// again. Bytes replaced in place (the face's own name) cannot be
		// restored; the error names the path.
		if !slices.Contains(refs.own, fileName) && !slices.Contains(refs.others, fileName) {
			s.dropUnreferenced(ctx, e.ID, propName, []string{fileName}, nil)
		}
		return nil, fmt.Errorf("update entity (attachment %s): %w", key, err)
	}
	s.dropUnreferenced(ctx, e.ID, propName, dropped, refs.others)

	return &Result{Path: key, FileName: fileName, Entity: res.Entity}, nil
}

// refState is the reference state of one (entity, property) at one moment:
// the addressed face's own names, the names every other face of the family
// references, and the names that have bytes.
type refState struct {
	own    []string
	others []string
	bytes  []string
}

// references reads the family raw and the property's bytes. The face must
// still exist: a face deleted since the caller read it wraps
// [store.ErrNotFound].
func (s *Service) references(ctx context.Context, e *entity.Entity, property string) (refState, error) {
	family, err := s.family(ctx, e.ID)
	if err != nil {
		return refState{}, fmt.Errorf("read entity family %s: %w", e.ID, err)
	}
	var out refState
	found := false
	for _, f := range family {
		names := metamodel.FileNames(f.Properties[property])
		if f.Face == e.Face {
			out.own = names
			found = true
			continue
		}
		out.others = append(out.others, names...)
	}
	if !found {
		return refState{}, fmt.Errorf("get entity %s: %w", entity.FormatStateRef(e.ID, e.Face), store.ErrNotFound)
	}
	sort.Strings(out.others)
	out.others = slices.Compact(out.others)
	out.bytes, err = s.byteNames(ctx, e.ID, property)
	if err != nil {
		return refState{}, fmt.Errorf("list attachments: %w", err)
	}
	return out, nil
}

// family returns every stored face of id, raw. IDs-scoped, never a scan.
func (s *Service) family(ctx context.Context, id string) ([]*entity.Entity, error) {
	var family []*entity.Entity
	for e, err := range s.deps.Store.ListEntities(ctx, store.EntityQuery{IDs: []string{id}, AllStates: true}) {
		if err != nil {
			return nil, err
		}
		if e.ID == id {
			family = append(family, e)
		}
	}
	return family, nil
}

// dropUnreferenced deletes the bytes of names no other face references.
// The caller holds the entity's attachment lock and has already stamped the
// face without them (or failed to stamp fresh bytes). A failure leaves
// unreachable bytes (nothing lists or serves a name no face references), so
// it is logged and does not fail the write; the manager's next sweep of the
// entity collects them. The deletes outlive a cancelled request: the write
// they follow has already happened.
func (s *Service) dropUnreferenced(ctx context.Context, id, property string, names, others []string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), lockWait)
	defer cancel()
	for _, name := range names {
		if slices.Contains(others, name) {
			continue
		}
		err := s.deps.Store.DeleteAttachment(ctx, id, property, name)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			slog.Warn("attachment: unreferenced bytes left behind",
				"entity", id, "property", property, "file", name, "err", err)
		}
	}
}

// lockWait bounds how long a writer waits for another writer to the same
// entity's attachments. The holder never scans or reads an upload under the
// lock, but its stamp runs the entity's on-update automations, whose Lua may
// take up to the default script timeout (30s). The wait is twice that, so a
// holder that is still making progress does not cause a 503.
const lockWait = 60 * time.Second

// ErrBusy is returned when another writer held the same entity's
// attachment lock for longer than the service waits, and stands for a full [Limiter] at the
// upload handlers. Nothing was written; the caller may retry. The HTTP
// handler maps it to a 503.
var ErrBusy = errors.New("attachment: another write to this property is in progress")

// lockForWrite takes the entity's attachment lock and then re-authorizes the write, so
// the decision holds for everything done under the lock. On a denial the lock
// is released and the authorizer's error returned.
func (s *Service) lockForWrite(ctx context.Context, e *entity.Entity, propName string) (func(), error) {
	release, err := s.lockProperty(ctx, e.ID, propName)
	if err != nil {
		return nil, err
	}
	if err := s.deps.Authorizer.AuthorizeAttachmentWrite(ctx, e); err != nil {
		release()
		return nil, err
	}
	return release, nil
}

// lockProperty acquires the entity's attachment lock, which serializes
// writers to all of its file properties. The deadline applies to the wait only; the returned
// release must be called once the write is done.
func (s *Service) lockProperty(ctx context.Context, entityID, propName string) (func(), error) {
	waitCtx, cancel := context.WithTimeout(ctx, lockWait)
	defer cancel()
	release, err := s.deps.Locker.Acquire(waitCtx, entitymanager.AttachmentLockKey(entityID))
	if err != nil {
		// Only our own wait bound is "busy"; a caller that gave up is not.
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			return nil, fmt.Errorf("%w (%s/%s)", ErrBusy, entityID, propName)
		}
		return nil, fmt.Errorf("lock attachment property %s/%s: %w", entityID, propName, err)
	}
	return release, nil
}

// spoolAttachment copies r to a temporary file and rewinds it. The copy is
// capped at [store.MaxAttachmentBytes], the same limit every store enforces.
func spoolAttachment(r io.Reader) (*os.File, error) {
	f, err := os.CreateTemp("", "rela-attachment-*")
	if err != nil {
		return nil, fmt.Errorf("spool attachment: %w", err)
	}
	fail := func(err error) (*os.File, error) {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return nil, err
	}
	if _, err := io.Copy(f, store.CapAttachmentReader(r, store.MaxAttachmentBytes)); err != nil {
		if errors.Is(err, store.ErrAttachmentTooLarge) {
			return fail(fmt.Errorf("store attachment: %w", err))
		}
		return fail(fmt.Errorf("spool attachment: %w", err))
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fail(fmt.Errorf("spool attachment: %w", err))
	}
	return f, nil
}

// DeleteAttachment removes one file from the face's property, shared with
// the HTTP delete handler. The caller gates the read; the write is
// authorized by Deps.Authorizer under the attachment lock. The bytes go when
// no other face references the name. Deleting a name the face does not
// reference changes nothing and succeeds.
func (s *Service) DeleteAttachment(
	ctx context.Context, e *entity.Entity, propDef metamodel.PropertyDef, propName, fileName string,
) error {
	release, err := s.lockForWrite(ctx, e, propName)
	if err != nil {
		return err
	}
	defer release()
	refs, err := s.references(ctx, e, propName)
	if err != nil {
		return err
	}
	_, err = s.deleteFile(ctx, e, propDef, propName, fileName, refs)
	return err
}

// deleteFile is [Service.DeleteAttachment], also reporting whether the face
// referenced the file. The caller holds the property's lock and read refs
// under it.
func (s *Service) deleteFile(
	ctx context.Context, e *entity.Entity, propDef metamodel.PropertyDef, propName, fileName string, refs refState,
) (bool, error) {
	// A name the face does not reference changes nothing, so it must not
	// write the entity: a write is audited and runs `on: update`
	// automations. It must not touch the bytes either: they may be another
	// face's.
	if !slices.Contains(refs.own, fileName) {
		return false, nil
	}
	names := slices.DeleteFunc(slices.Clone(refs.own), func(n string) bool { return n == fileName })
	if _, err := s.stamp(ctx, e, propName, propDef.FileMax(), names); err != nil {
		return false, fmt.Errorf("update entity: %w", err)
	}
	s.dropUnreferenced(ctx, e.ID, propName, []string{fileName}, refs.others)
	return true, nil
}

// ErrNoFileToDetach is returned by [Service.DetachFile] when no file name was
// given and the property holds no file, or several.
var ErrNoFileToDetach = errors.New("no file to detach")

// Detach removes one attachment from a file-type property of the face ref
// addresses; see [Service.DetachFile] for the fileName rules and the result.
func (s *Service) Detach(ctx context.Context, ref entity.Ref, property, fileName string) (string, error) {
	e, propDef, err := s.fileProperty(ctx, ref, property)
	if err != nil {
		return "", err
	}
	return s.DetachFile(ctx, e, propDef, property, fileName)
}

// DetachFile removes one attachment from the face's file-type property and
// returns the name of the file it removed. A named file is deleted
// idempotently: when the face does not reference it, DetachFile succeeds and
// returns "". An empty fileName removes the face's only file, and errors
// when it holds none or several (the caller must disambiguate). The caller
// gates the read; the write is authorized by Deps.Authorizer under the
// attachment lock.
func (s *Service) DetachFile(
	ctx context.Context, e *entity.Entity, propDef metamodel.PropertyDef, property, fileName string,
) (string, error) {
	release, err := s.lockForWrite(ctx, e, property)
	if err != nil {
		return "", err
	}
	defer release()
	refs, err := s.references(ctx, e, property)
	if err != nil {
		return "", err
	}
	if fileName == "" {
		switch len(refs.own) {
		case 0:
			return "", fmt.Errorf("%w: property %q has no attachment", ErrNoFileToDetach, property)
		case 1:
			fileName = refs.own[0]
		default:
			return "", fmt.Errorf("%w: property %q holds %d files; specify which to detach: %v",
				ErrNoFileToDetach, property, len(refs.own), refs.own)
		}
	}
	existed, err := s.deleteFile(ctx, e, propDef, property, fileName, refs)
	if err != nil || !existed {
		return "", err
	}
	return fileName, nil
}

// byteNames lists the file names that have bytes on the property, in
// stable order. A real store error is returned (not swallowed) because the
// write path chooses names from this list: acting on a degraded view could
// overwrite bytes another face references.
func (s *Service) byteNames(ctx context.Context, entityID, property string) ([]string, error) {
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

// stamp records names as the face's property value through the manager's
// one file-property write (see [Stamper]). e supplies identity only and is
// not modified.
func (s *Service) stamp(
	ctx context.Context, e *entity.Entity, property string, maxCount int, names []string,
) (*entity.UpdateResult, error) {
	return s.deps.EntityManager.StampAttachments(ctx, entity.Ref{ID: e.ID, Face: e.Face}, property,
		stampValue(e.ID, property, maxCount, names))
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

func attachPath(entityID, property, fileName string) string {
	return filepath.ToSlash(filepath.Join("attachments", entityID, property, fileName))
}

// resolveAttachName applies the filename policy for an attach: normalize
// (NormalizeFileName always yields a usable name, never ""), then
// auto-suffix against the names already taken.
//
// Taken are the other faces' names and every name with bytes: bytes are
// shared, so reusing either would overwrite a file another face serves. At
// max==1 the face's own file is replaced in place when no other face
// references it. At max>1 the face's own names are taken too, and the cap
// counts them: [ErrAtCapacity] when full.
func resolveAttachName(rawName string, r refState, maxCount int) (string, error) {
	name := store.NormalizeFileName(rawName)
	taken := make(map[string]bool, len(r.others)+len(r.bytes)+len(r.own))
	for _, f := range r.others {
		taken[f] = true
	}
	for _, f := range r.bytes {
		taken[f] = true
	}
	if maxCount <= 1 {
		for _, f := range r.own {
			if !slices.Contains(r.others, f) {
				delete(taken, f)
			}
		}
	} else {
		if len(r.own) >= maxCount {
			return "", ErrAtCapacity
		}
		for _, f := range r.own {
			taken[f] = true
		}
	}
	return store.SuffixOnCollision(name, func(c string) bool { return taken[c] }), nil
}

// List returns the attachments of the face ref addresses; see
// [Service.ListFace].
func (s *Service) List(ctx context.Context, ref entity.Ref) ([]Info, error) {
	e, err := s.Resolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	return s.ListFace(ctx, e)
}

// ListFace returns the attachments of the face e: the names its own file
// properties reference that have bytes. A face never lists another face's
// upload. Content type is inferred from the file name on the fly; there is
// no persisted metadata sidecar. e may be a redacted read, in which case a
// hidden property lists nothing.
func (s *Service) ListFace(ctx context.Context, e *entity.Entity) ([]Info, error) {
	items, err := s.deps.Store.ListAttachments(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	infos := make([]Info, 0, len(items))
	for _, it := range FaceAttachments(e, items) {
		infos = append(infos, Info{
			Property:    it.Property,
			Path:        attachPath(it.EntityID, it.Property, it.FileName),
			FileName:    it.FileName,
			ContentType: contentTypeForName(it.FileName),
			Size:        it.Size,
		})
	}
	return infos, nil
}

// FaceAttachments filters an entity's byte listing to the files the face e
// references, in (property, name) order.
func FaceAttachments(e *entity.Entity, items []store.AttachmentInfo) []store.AttachmentInfo {
	out := make([]store.AttachmentInfo, 0, len(items))
	for _, it := range items {
		if References(e, it.Property, it.FileName) {
			out = append(out, it)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Property != out[j].Property {
			return out[i].Property < out[j].Property
		}
		return out[i].FileName < out[j].FileName
	})
	return out
}
