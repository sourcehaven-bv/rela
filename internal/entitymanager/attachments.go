package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// File-property values are owned by the attachment paths (BUG-CTUW2N,
// RR-EV48RR).
//
// Attachment bytes are keyed per entity, (id, property, storage key), and
// shared by every face of the entity. A face's file-property value is what
// grants it the bytes it names: listing and download only serve the keys in
// the addressed face's own value ([metamodel.FileRefs]). So the value is a
// capability, and an ordinary write must not be able to set it. Otherwise a
// writer on one face could name a file another face uploaded, and download
// it through the face it can write.
//
// The rule: CreateEntity, UpdateEntity and PatchEntity leave every file
// property unchanged (on create: absent), comparing normalized base names so
// a save that echoes the stored value passes. A change fails with
// [FileWriteError]. The trusted paths that may change a value are the
// attachment service (through [Attachments.StampAttachments]), the copy
// engine, sync ApplyEntity and data migration. Automation-derived values
// are applied after this check, so the automation engine refuses, at load,
// an automation whose `set:` or `create_entity` names a file property.
//
// Every path that removes a reference deletes the bytes no remaining face
// references, under the attachment lock for id: the attachment service, the
// copy engine and DeleteEntityFace. Whole-family deletes remove
// all bytes in the store.

// AttachmentLocker is the keyed lock that serializes changes to one
// entity's attachments: the references in every face's file values and the
// bytes. Declared here, at the consumer. The wiring site supplies the same
// instance the attachment service uses, because a lock only excludes the
// holders of the same instance.
type AttachmentLocker interface {
	// Acquire blocks until key is held or ctx is done, and returns an
	// idempotent release.
	Acquire(ctx context.Context, key string) (release func(), err error)
}

// AttachmentLockKey is the lock key for the attachments of entity id: every
// file property of every face. The attachment service and the manager use
// it, so their critical sections exclude each other. One key per entity,
// not per property: a postgres lock pins a pool connection per held key, so
// a write touching several file properties would otherwise pin several.
func AttachmentLockKey(id string) string {
	return "attachment/" + id
}

// attachmentLockWait bounds how long a manager write waits for the
// attachment lock. It matches the attachment service's own wait.
const attachmentLockWait = 60 * time.Second

// FileWriteError reports a generic write that would change a file
// property's value. Callers may errors.As it into a 422.
type FileWriteError struct {
	EntityType string
	Properties []string
}

func (e *FileWriteError) Error() string {
	return fmt.Sprintf(
		"file properties of entity type %q change only through the attachments API: %v",
		e.EntityType, e.Properties)
}

func fileWriteError(entityType string, names []string) error {
	if len(names) == 0 {
		return nil
	}
	slices.Sort(names)
	return &FileWriteError{EntityType: entityType, Properties: slices.Compact(names)}
}

// sameFileNames reports whether two file-property values reference the same
// names. It ignores storage keys, so it is safe only because every caller
// then pins the stored value ([pinStoredFileValues]): a same-named entry
// with a forged token passes this check but never lands.
func sameFileNames(a, b any) bool {
	return slices.Equal(metamodel.FileNames(a), metamodel.FileNames(b))
}

// rejectFileCreate refuses a create that sets a file property.
func rejectFileCreate(meta *metamodel.Metamodel, entityType string, props map[string]any) error {
	var names []string
	for _, prop := range metamodel.FileProperties(meta, entityType) {
		if len(metamodel.FileNames(props[prop])) > 0 {
			names = append(names, prop)
		}
	}
	return fileWriteError(entityType, names)
}

// rejectFileChanges refuses a whole-entity save that changes a file
// property of old.
func rejectFileChanges(meta *metamodel.Metamodel, old, updated *entity.Entity) error {
	var names []string
	for _, prop := range metamodel.FileProperties(meta, old.Type) {
		if !sameFileNames(old.Properties[prop], updated.Properties[prop]) {
			names = append(names, prop)
		}
	}
	return fileWriteError(old.Type, names)
}

// rejectFilePatch refuses a patch that changes a file property of stored,
// other than allowed (the property [Attachments.StampAttachments] writes;
// empty for PatchEntity).
func rejectFilePatch(meta *metamodel.Metamodel, stored *entity.Entity, p entity.Patch, allowed string) error {
	fileProps := metamodel.FileProperties(meta, stored.Type)
	if allowed != "" && !slices.Contains(fileProps, allowed) {
		return fmt.Errorf("entitymanager: %q is not a file property of entity type %q", allowed, stored.Type)
	}
	var names []string
	for _, prop := range fileProps {
		if prop == allowed {
			continue
		}
		if v, ok := p.Properties[prop]; ok && !sameFileNames(stored.Properties[prop], v) {
			names = append(names, prop)
		}
		if slices.Contains(p.MetaUnset, prop) && len(metamodel.FileNames(stored.Properties[prop])) > 0 {
			names = append(names, prop)
		}
	}
	return fileWriteError(stored.Type, names)
}

// pinStoredFileValues sets every file property of updated, other than
// except, back to stored's exact value. The reject checks compare base
// names, so a save that echoes the value in another form (a different
// prefix, list order or scalar/list shape) passes them; pinning keeps that
// form off the stored row, so a value only ever holds what the attachment
// paths wrote.
func pinStoredFileValues(meta *metamodel.Metamodel, stored, updated *entity.Entity, except string) {
	if updated.Properties == nil {
		updated.Properties = map[string]any{}
	}
	for _, prop := range metamodel.FileProperties(meta, stored.Type) {
		if prop == except {
			continue
		}
		if v, ok := stored.Properties[prop]; ok {
			updated.Properties[prop] = v
		} else {
			delete(updated.Properties, prop)
		}
	}
}

// Attachments is the manager's surface for the attachment service: the one
// write that may change a file property's value, and the lock that
// serializes it. A separate type rather than Manager methods, so the
// permission is held only by the code handed this value.
type Attachments struct {
	m *Manager
}

// AttachmentsOf returns the attachment surface of m. The zero Attachments
// fails every call.
//
// Nil: rejected. m, and m's AttachmentLocker, are required: without the
// lock the service and the manager could not exclude each other.
func AttachmentsOf(m *Manager) (Attachments, error) {
	if m == nil {
		return Attachments{}, errors.New("entitymanager: AttachmentsOf: manager is nil")
	}
	if m.deps.AttachmentLocker == nil {
		return Attachments{}, errors.New("entitymanager: AttachmentsOf: manager has no AttachmentLocker")
	}
	return Attachments{m: m}, nil
}

// Acquire takes the manager's attachment lock for key; see
// [AttachmentLockKey].
func (a Attachments) Acquire(ctx context.Context, key string) (func(), error) {
	if a.m == nil {
		return nil, errAttachmentsUnavailable
	}
	return a.m.deps.AttachmentLocker.Acquire(ctx, key)
}

// errAttachmentsUnavailable is returned by the zero [Attachments]: a wiring
// site whose manager has no AttachmentLocker (its metamodel declared no file
// property) holds the zero value, and a later schema with a file property
// must fail the attachment write rather than panic.
var errAttachmentsUnavailable = errors.New(
	"entitymanager: attachments unavailable: the manager was built without an AttachmentLocker")

// StampAttachments sets the file property prop of the face ref to value. It
// is PatchEntity with the file-property rule lifted for prop only: it runs
// the same authorization, field gate, validation, automation and audit. The
// caller holds the attachment lock for ref.ID and keeps the bytes
// consistent with the value.
func (a Attachments) StampAttachments(
	ctx context.Context, ref entity.Ref, prop string, value any,
) (*entity.UpdateResult, error) {
	if a.m == nil {
		return nil, errAttachmentsUnavailable
	}
	if ref.ID == "" {
		return nil, errors.New("entitymanager: StampAttachments: id is empty")
	}
	if prop == "" {
		return nil, errors.New("entitymanager: StampAttachments: property is empty")
	}
	p := entity.Patch{Properties: map[string]any{prop: value}}
	return patchWithRetry(ctx, true, func(pin bool) (*entity.UpdateResult, error) {
		return a.m.patchEntityOnce(withStoreAttribution(ctx), ref.String(), p, pin, prop)
	})
}

// acquireAttachmentLock takes the attachment lock for id. With no locker it
// is a no-op: New admits a nil locker only when no type has a file property.
func acquireAttachmentLock(ctx context.Context, locker AttachmentLocker, id string) (func(), error) {
	if locker == nil {
		return func() {}, nil
	}
	waitCtx, cancel := context.WithTimeout(ctx, attachmentLockWait)
	defer cancel()
	release, err := locker.Acquire(waitCtx, AttachmentLockKey(id))
	if err != nil {
		return nil, fmt.Errorf("entitymanager: lock attachments of %s: %w", id, err)
	}
	return release, nil
}

// cleanupContext is the context for work that follows a committed write: a
// client that disconnects after the commit must not cancel the cleanup, or
// the bytes it would have removed stay behind.
func cleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), attachmentLockWait)
}

// sweepUnreferencedFiles deletes every byte of id that no stored face
// references. The caller holds the attachment lock for id, so no upload is
// between writing its bytes and stamping them. Sweeping the whole entity,
// rather than the names one write dropped, also collects bytes an earlier
// cleanup failed to remove and names a concurrent stamp added after the
// caller's read. A failure is returned: the caller's write has committed, so
// it reports the leftover bytes rather than failing.
func sweepUnreferencedFiles(ctx context.Context, st store.Store, id string) error {
	infos, err := st.ListAttachments(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("entitymanager: list attachments of %s: %w", id, err)
	}
	if len(infos) == 0 {
		return nil
	}
	family, err := familyRows(ctx, st, id)
	if err != nil {
		return fmt.Errorf("entitymanager: read family %s: %w", id, err)
	}
	var errs []error
	for _, info := range infos {
		key := info.FileName
		if familyReferences(family, info.Property, key) {
			continue
		}
		derr := st.DeleteAttachment(ctx, id, info.Property, key)
		if derr != nil && !errors.Is(derr, store.ErrNotFound) {
			errs = append(errs, fmt.Errorf("delete attachment %s/%s/%s: %w", id, info.Property, key, derr))
		}
	}
	return errors.Join(errs...)
}

// releaseUnreferencedFiles runs [sweepUnreferencedFiles] under the
// attachment lock, after a committed write that removed a face or changed
// its file values. The write has already committed, so a failure is logged
// rather than returned: failing the call would report a write that happened
// as one that did not. Leftover bytes are unreachable (no face lists or
// serves them), and the next sweep of the entity collects them.
func releaseUnreferencedFiles(ctx context.Context, deps Deps, id string) {
	ctx, cancel := cleanupContext(ctx)
	defer cancel()
	release, err := acquireAttachmentLock(ctx, deps.AttachmentLocker, id)
	if err == nil {
		defer release()
		err = sweepUnreferencedFiles(ctx, deps.Store, id)
	}
	if err != nil {
		slog.Warn("entitymanager: unreferenced attachment bytes left behind", "entity", id, "err", err)
	}
}

// familyReferences reports whether any face in family references the
// storage key on prop.
func familyReferences(family []*entity.Entity, prop, key string) bool {
	for _, f := range family {
		if slices.Contains(metamodel.FileKeys(f.Properties[prop]), key) {
			return true
		}
	}
	return false
}

// CarryFileValues returns props with every file property of entityType
// replaced by live's value (removed when live has none). History restore
// uses it: a restore keeps the face's current file values and never
// re-introduces an old name, which a later upload on another face may have
// reused. live may be nil (a re-created entity has no live value).
func CarryFileValues(
	meta *metamodel.Metamodel, entityType string, props map[string]any, live *entity.Entity,
) map[string]any {
	out := make(map[string]any, len(props))
	maps.Copy(out, props)
	for _, prop := range metamodel.FileProperties(meta, entityType) {
		delete(out, prop)
		if live == nil {
			continue
		}
		if v, ok := live.Properties[prop]; ok {
			out[prop] = v
		}
	}
	return out
}
