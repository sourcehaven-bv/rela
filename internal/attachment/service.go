package attachment

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
	"unicode/utf8"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// # The model: bytes per entity, value per face (BUG-CTUW2N)
//
// Bytes are keyed (entity id, property, storage key) and shared by every
// face of the entity. A face's files are the entries of its OWN property
// value ([metamodel.FileRefs]): each has a display name, unique within the
// face, and a storage key. Listing, download and delete address the display
// name and resolve it to a key through the face's value, so a face never
// sees another face's upload. Writes run under the entity's attachment lock
// ([entitymanager.AttachmentLockKey]) and read the whole family raw: that
// is write-prep, and it must include faces the caller cannot read, or an
// upload could overwrite their bytes. Bytes are deleted when the last face
// stops referencing their key.
//
// Every upload gets a fresh storage key, and its display name is made
// unique against the face's own names only. So an upload on one face
// behaves exactly as if no other face held a file of that name: a file name
// is a property value, and a face's hidden values must not leak through
// another face's upload. A copy carries entries verbatim, keys included, so
// copied faces share the bytes.

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
	key, ok := StorageKey(e, property, fileName)
	if !ok {
		return nil, fmt.Errorf("attachment %s/%s: %w", property, fileName, store.ErrNotFound)
	}
	return s.deps.Store.ReadAttachment(ctx, e.ID, property, key)
}

// References reports whether the face e's own value of property names
// fileName. It is the face gate for the shared bytes.
func References(e *entity.Entity, property, fileName string) bool {
	_, ok := StorageKey(e, property, fileName)
	return ok
}

// StorageKey resolves the display name fileName through the face e's own
// value of property to the store key of its bytes. ok is false when the
// face does not reference the name: the value is the face's capability to
// the shared bytes, so the key must never come from anywhere else.
func StorageKey(e *entity.Entity, property, fileName string) (key string, ok bool) {
	if e == nil {
		return "", false
	}
	ref, ok := ownRef(metamodel.FileRefs(e.Properties[property]), fileName)
	return ref.Key, ok
}

// ownRef returns the entry named name in a face's refs. Names are unique per
// face; should a value hold two (only a hand-edited file could), the first
// in [metamodel.FileRefs] order wins, consistently for every caller.
func ownRef(refs []metamodel.FileRef, name string) (metamodel.FileRef, bool) {
	for _, r := range refs {
		if r.Name == name {
			return r, true
		}
	}
	return metamodel.FileRef{}, false
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
	fileName, err := resolveAttachName(rawFileName, refs.own, maxCount)
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
	fileName, err = resolveAttachName(fileName, refs.own, maxCount)
	if err != nil {
		return nil, err
	}
	fresh, err := newFileRef(e.ID, propName, fileName, refs)
	if err != nil {
		return nil, err
	}

	// Write the new bytes first, under a key nothing references yet. On
	// failure the existing files are untouched.
	if err = s.deps.Store.AttachFile(ctx, e.ID, propName, fresh.Key, spool); err != nil {
		return nil, fmt.Errorf("store attachment: %w", err)
	}

	// The face's new entries come from its value, not from the byte listing,
	// which also holds the other faces' files. At max==1 the upload replaces
	// the face's file; above 1 it is added (its name was suffixed clear of
	// the face's own names).
	var own []metamodel.FileRef
	var dropped []string
	if maxCount <= 1 {
		for _, r := range refs.own {
			dropped = append(dropped, r.Key)
		}
	} else {
		own = slices.Clone(refs.own)
	}
	own = append(own, fresh)
	metamodel.SortFileRefs(own)

	res, err := s.stamp(ctx, e, propName, maxCount, own)
	if err != nil {
		// Nothing references the fresh key, so its bytes go again.
		s.dropUnreferenced(ctx, e.ID, propName, []string{fresh.Key}, nil)
		return nil, fmt.Errorf("update entity (attachment %s): %w", fresh.Entry, err)
	}
	s.dropUnreferenced(ctx, e.ID, propName, dropped, refs.others)

	return &Result{Path: fresh.Entry, FileName: fileName, Entity: res.Entity}, nil
}

// maxKeyAttempts bounds the draws for a storage key no other file of the
// property uses. A draw collides with probability n/2^64 for n files, so a
// second draw is already never needed in practice.
const maxKeyAttempts = 8

// newFileRef is the entry for a fresh upload of name: a new random token,
// re-drawn while its key is referenced or has bytes, so the upload can never
// overwrite or adopt bytes that exist.
func newFileRef(entityID, property, name string, r refState) (metamodel.FileRef, error) {
	for range maxKeyAttempts {
		var b [metamodel.FileTokenLen / 2]byte
		if _, err := rand.Read(b[:]); err != nil {
			return metamodel.FileRef{}, fmt.Errorf("attachment key: %w", err)
		}
		tok := hex.EncodeToString(b[:])
		key := metamodel.FileKey(tok, name)
		ownKey := slices.ContainsFunc(r.own, func(o metamodel.FileRef) bool { return o.Key == key })
		if ownKey || slices.Contains(r.bytes, key) || slices.Contains(r.others, key) {
			continue
		}
		return metamodel.FileRef{
			Entry: metamodel.KeyedFileEntry(entityID, property, tok, name), Name: name, Key: key,
		}, nil
	}
	return metamodel.FileRef{}, errors.New("attachment key: no free storage key")
}

// refState is the reference state of one (entity, property) at one moment:
// the addressed face's own entries, the storage keys every other face of
// the family references, and the keys that have bytes.
type refState struct {
	own    []metamodel.FileRef
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
		if f.Face == e.Face {
			out.own = metamodel.FileRefs(f.Properties[property])
			found = true
			continue
		}
		out.others = append(out.others, metamodel.FileKeys(f.Properties[property])...)
	}
	if !found {
		return refState{}, fmt.Errorf("get entity %s: %w", entity.FormatStateRef(e.ID, e.Face), store.ErrNotFound)
	}
	sort.Strings(out.others)
	out.others = slices.Compact(out.others)
	out.bytes, err = s.byteKeys(ctx, e.ID, property)
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

// dropUnreferenced deletes the bytes of storage keys no other face
// references. The caller holds the entity's attachment lock and has already
// stamped the face without them (or failed to stamp fresh bytes). A failure
// leaves unreachable bytes (nothing lists or serves a key no face
// references), so it is logged and does not fail the write; the manager's
// next sweep of the entity collects them. The deletes outlive a cancelled
// request: the write they follow has already happened.
func (s *Service) dropUnreferenced(ctx context.Context, id, property string, keys, others []string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), lockWait)
	defer cancel()
	for _, key := range keys {
		if slices.Contains(others, key) {
			continue
		}
		err := s.deps.Store.DeleteAttachment(ctx, id, property, key)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			slog.Warn("attachment: unreferenced bytes left behind",
				"entity", id, "property", property, "file", key, "err", err)
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
var ErrBusy = errors.New("attachment: another write to this entity's attachments is in progress")

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
// no other face references their key. Deleting a name the face does not
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
	if _, ok := ownRef(refs.own, fileName); !ok {
		return false, nil
	}
	// Every entry of that name goes: names are unique per face, but a
	// hand-edited value may repeat one, and deleting only the first would
	// leave the file listed as if the delete had not happened.
	var rest []metamodel.FileRef
	var gone, kept []string
	for _, r := range refs.own {
		if r.Name == fileName {
			gone = append(gone, r.Key)
			continue
		}
		rest = append(rest, r)
		kept = append(kept, r.Key)
	}
	if _, err := s.stamp(ctx, e, propName, propDef.FileMax(), rest); err != nil {
		return false, fmt.Errorf("update entity: %w", err)
	}
	s.dropUnreferenced(ctx, e.ID, propName, gone, append(kept, refs.others...))
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
			fileName = refs.own[0].Name
		default:
			names := make([]string, 0, len(refs.own))
			for _, r := range refs.own {
				names = append(names, r.Name)
			}
			return "", fmt.Errorf("%w: property %q holds %d files; specify which to detach: %v",
				ErrNoFileToDetach, property, len(refs.own), names)
		}
	}
	existed, err := s.deleteFile(ctx, e, propDef, property, fileName, refs)
	if err != nil || !existed {
		return "", err
	}
	return fileName, nil
}

// byteKeys lists the storage keys that have bytes on the property, in
// stable order. A real store error is returned (not swallowed) because the
// write path chooses keys against this list: acting on a degraded view could
// overwrite bytes another face references.
func (s *Service) byteKeys(ctx context.Context, entityID, property string) ([]string, error) {
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

// stamp records refs as the face's property value through the manager's
// one file-property write (see [Stamper]). e supplies identity only and is
// not modified.
func (s *Service) stamp(
	ctx context.Context, e *entity.Entity, property string, maxCount int, refs []metamodel.FileRef,
) (*entity.UpdateResult, error) {
	return s.deps.EntityManager.StampAttachments(ctx, entity.Ref{ID: e.ID, Face: e.Face}, property,
		stampValue(maxCount, refs))
}

// stampValue is the property value for a face's entries: a scalar path for
// a single-cap property (empty when none), a list of paths for a multi-cap
// property. Existing entries are written verbatim, so a file stored before
// storage tokens keeps its key. A single-cap property still holding several
// entries (its `max` was lowered) keeps them all as a list: writing only the
// first would drop the others' references without deleting their bytes.
func stampValue(maxCount int, refs []metamodel.FileRef) any {
	if maxCount <= 1 && len(refs) <= 1 {
		if len(refs) == 0 {
			return ""
		}
		return refs[0].Entry
	}
	paths := make([]string, 0, len(refs))
	for _, r := range refs {
		paths = append(paths, r.Entry)
	}
	return paths
}

// resolveAttachName applies the filename policy for an attach: normalize
// (NormalizeFileName always yields a usable name, never ""), then make it
// unique among the face's own names. Only the face's own names count: the
// bytes get a fresh storage key ([newFileRef]), so no other face's file
// can collide with the upload or show through its name.
//
// At max==1 the upload replaces the face's file, so its name is kept. At
// max>1 the face's own names are taken, and the cap counts them:
// [ErrAtCapacity] when full.
func resolveAttachName(rawName string, own []metamodel.FileRef, maxCount int) (string, error) {
	name := capNameLen(store.NormalizeFileName(rawName))
	if maxCount <= 1 {
		return name, nil
	}
	if len(own) >= maxCount {
		return "", ErrAtCapacity
	}
	return store.SuffixOnCollision(name, func(c string) bool {
		_, taken := ownRef(own, c)
		return taken
	}), nil
}

// maxNameBytes caps a display name so its storage key ("<token>-<name>",
// plus a " (n)" suffix at max>1) still fits the 255-byte file-name limit of
// the filesystem backend.
const maxNameBytes = 255 - metamodel.FileTokenLen - 1 - len(" (99)")

// capNameLen shortens name to [maxNameBytes], cutting the stem on a rune
// boundary and keeping the extension when it is short enough to keep.
func capNameLen(name string) string {
	if len(name) <= maxNameBytes {
		return name
	}
	ext := filepath.Ext(name)
	if len(ext) > maxNameBytes/2 {
		ext = ""
	}
	stem := strings.TrimSuffix(name, ext)
	limit := maxNameBytes - len(ext)
	for limit > 0 && !utf8.RuneStart(stem[limit]) {
		limit--
	}
	return stem[:limit] + ext
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
	faced := faceFiles(e, items)
	infos := make([]Info, 0, len(faced))
	for _, f := range faced {
		infos = append(infos, Info{
			Property:    f.info.Property,
			Path:        f.entry,
			FileName:    f.info.FileName,
			ContentType: contentTypeForName(f.info.FileName),
			Size:        f.info.Size,
		})
	}
	return infos, nil
}

// FaceAttachments filters an entity's byte listing to the files the face e
// references, in (property, name) order. Each result's FileName is the
// face's display name, not the storage key the listing held.
func FaceAttachments(e *entity.Entity, items []store.AttachmentInfo) []store.AttachmentInfo {
	faced := faceFiles(e, items)
	out := make([]store.AttachmentInfo, 0, len(faced))
	for _, f := range faced {
		out = append(out, f.info)
	}
	return out
}

// faceFile is one of a face's files: its byte listing with the display
// name, and the value entry that references it.
type faceFile struct {
	info  store.AttachmentInfo
	entry string
}

// faceFiles joins the byte listing items (by storage key) with the face e's
// own entries, in (property, name) order. A face entry with no bytes is
// left out.
func faceFiles(e *entity.Entity, items []store.AttachmentInfo) []faceFile {
	var out []faceFile
	if e == nil {
		return out
	}
	refs := make(map[string][]metamodel.FileRef)
	for _, it := range items {
		own, seen := refs[it.Property]
		if !seen {
			own = metamodel.FileRefs(e.Properties[it.Property])
			refs[it.Property] = own
		}
		for _, r := range own {
			// List only what download serves: the entry a name resolves to.
			if first, _ := ownRef(own, r.Name); r.Key != it.FileName || first != r {
				continue
			}
			info := it
			info.FileName = r.Name
			out = append(out, faceFile{info: info, entry: r.Entry})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].info.Property != out[j].info.Property {
			return out[i].info.Property < out[j].info.Property
		}
		return out[i].info.FileName < out[j].info.FileName
	})
	return out
}
