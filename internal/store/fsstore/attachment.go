package fsstore

import (
	"context"
	"errors"
	"io"
	"os"
	"path"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// AttachFile streams r to `<attachKey>/<entityID>/<property>/<fileName>`.
//
// The write goes through RootedFS so the path is validated before it
// reaches the underlying FS. On OS-backed filesystems the data is
// streamed through RootedFS.OpenForWrite; on MemFS the data is buffered
// and written via RootedFS.WriteFile.
//
// If an attachment already exists at this (entityID, property) under a
// different filename, the old file is removed first (1:1 ownership per
// property).
// attachTempPrefix marks an in-progress attachment temp file. A real
// stored attachment name can never start with a dot (store.NormalizeFileName
// trims leading dots), so this prefix is collision-proof; the index loader
// skips files carrying it.
const attachTempPrefix = ".rela-attach-tmp-"

// attachmentKey is the per-file index key: (entityID, property, fileName).
func attachmentKey(entityID, property, fileName string) string {
	return entityID + "/" + property + "/" + fileName
}

func (s *FSStore) AttachFile(_ context.Context, entityID, property, fileName string, r io.Reader) error {
	if err := storeutil.ValidateProperty(property); err != nil {
		return err
	}
	if err := store.ValidateFileName(fileName); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.entities[entityID]; !ok {
		return store.ErrNotFound
	}

	dirKey := path.Join(s.attachKey, entityID, property)
	if err := s.rooted.MkdirAll(dirKey, 0o755); err != nil {
		return err
	}

	key := attachmentKey(entityID, property, fileName)
	fileKey := path.Join(dirKey, fileName)

	// Append: each (entityID, property, fileName) is its own slot. A
	// same-name re-attach replaces only that one file; sibling files on the
	// property are untouched. Cap/suffix policy lives in the write path.
	//
	// Write to a temp file first, then swap, so a failed write (e.g. the
	// backstop size guard tripping) never destroys an existing same-named
	// file — the old bytes and index entry stay intact until the new bytes
	// are fully written.
	//
	// The temp name carries a leading-dot prefix (attachTempPrefix). A stored
	// attachment name can never start with a dot — store.NormalizeFileName
	// trims leading dots — so this marker can never collide with a real
	// upload, and the index loader skips it by that prefix. (A plain ".new"
	// suffix would clash with a legitimate upload literally named "x.new".)
	tmpKey := path.Join(dirKey, attachTempPrefix+fileName)
	// Backstop size guard: cap reads at MaxAttachmentBytes so no caller can
	// write an unbounded attachment (the API layer also caps at ingress).
	n, err := s.writeAttachment(tmpKey, storeutil.LimitAttachmentReader(r))
	if err != nil {
		_ = s.rooted.Remove(tmpKey) // remove the failed partial; old file intact
		return err
	}
	if err := s.rooted.Rename(tmpKey, fileKey); err != nil {
		_ = s.rooted.Remove(tmpKey)
		return err
	}

	s.attachments[key] = attachMeta{
		entityID: entityID,
		property: property,
		fileName: fileName,
		size:     n,
	}
	return nil
}

// writeAttachment persists r to the given key. On OsFS-backed stacks
// it streams via RootedFS.OpenForWrite (constant memory); on MemFS it
// buffers via WriteFile since MemFS has no streaming primitive.
//
// Parent directory creation is guaranteed by AttachFile's MkdirAll
// above.
func (s *FSStore) writeAttachment(key string, r io.Reader) (int64, error) {
	if s.streamingSupported {
		if err := s.streamToFile(key, r); err != nil {
			return 0, err
		}
		info, err := s.rooted.Stat(key)
		if err != nil {
			return 0, err
		}
		return info.Size(), nil
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}
	if err := s.rooted.WriteFile(key, data, 0o644); err != nil {
		return 0, err
	}
	return int64(len(data)), nil
}

// ReadAttachment returns a streaming reader over the attachment's
// bytes. Callers MUST Close the returned reader.
func (s *FSStore) ReadAttachment(_ context.Context, entityID, property, fileName string) (io.ReadCloser, error) {
	s.mu.RLock()
	a, ok := s.attachments[attachmentKey(entityID, property, fileName)]
	s.mu.RUnlock()

	if !ok {
		return nil, store.ErrNotFound
	}

	fileKey := path.Join(s.attachKey, a.entityID, a.property, a.fileName)
	return s.rooted.Open(fileKey)
}

func (s *FSStore) DeleteAttachment(_ context.Context, entityID, property, fileName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := attachmentKey(entityID, property, fileName)
	a, ok := s.attachments[key]
	if !ok {
		return store.ErrNotFound
	}

	fileKey := path.Join(s.attachKey, a.entityID, a.property, a.fileName)
	if err := s.rooted.Remove(fileKey); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	delete(s.attachments, key)
	return nil
}

func (s *FSStore) ListAttachments(_ context.Context, entityID string) ([]store.AttachmentInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.entities[entityID]; !ok {
		return nil, store.ErrNotFound
	}

	var result []store.AttachmentInfo
	for _, a := range s.attachments {
		if a.entityID == entityID {
			result = append(result, store.AttachmentInfo{
				EntityID: a.entityID,
				Property: a.property,
				FileName: a.fileName,
				Size:     a.size,
			})
		}
	}
	return result, nil
}

// removeAttachmentDir removes `<attachKey>/<entityID>/` and every
// file underneath. Prunes the in-memory index for the entity
// regardless of whether the on-disk dir exists. Must be called with
// s.mu held.
//
// Called from DeleteEntity and RenameEntity: under the per-entity
// layout, attachments are 1:1 owned by the entity.
func (s *FSStore) removeAttachmentDir(entityID string) error {
	// Prune in-memory index entries first — runs even when the
	// on-disk dir is gone (e.g. after a workspace-level move).
	for key, a := range s.attachments {
		if a.entityID == entityID {
			delete(s.attachments, key)
		}
	}

	if s.attachKey == "" {
		return nil
	}
	rootKey := path.Join(s.attachKey, entityID)
	if _, err := s.rooted.Stat(rootKey); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}

	propEntries, err := s.rooted.ReadDir(rootKey)
	if err != nil {
		return err
	}
	for _, pe := range propEntries {
		if !pe.IsDir() {
			continue
		}
		propDirKey := path.Join(rootKey, pe.Name())
		fileEntries, err := s.rooted.ReadDir(propDirKey)
		if err != nil {
			return err
		}
		for _, fe := range fileEntries {
			if fe.IsDir() {
				continue
			}
			rmErr := s.rooted.Remove(path.Join(propDirKey, fe.Name()))
			if rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
				return rmErr
			}
		}
		if rmErr := s.rooted.Remove(propDirKey); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
			return rmErr
		}
	}
	if rmErr := s.rooted.Remove(rootKey); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
		return rmErr
	}
	return nil
}

// renameAttachmentDir moves `<attachKey>/<oldID>/` to
// `<attachKey>/<newID>/` and updates the in-memory index. No-op if
// the old dir does not exist. Must be called with s.mu held.
func (s *FSStore) renameAttachmentDir(oldID, newID string) error {
	if s.attachKey == "" {
		return nil
	}
	oldRootKey := path.Join(s.attachKey, oldID)
	if _, err := s.rooted.Stat(oldRootKey); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}

	newRootKey := path.Join(s.attachKey, newID)
	if err := s.rooted.Rename(oldRootKey, newRootKey); err != nil {
		return err
	}

	reKey := make(map[string]attachMeta)
	for key, a := range s.attachments {
		if a.entityID != oldID {
			continue
		}
		delete(s.attachments, key)
		a.entityID = newID
		reKey[attachmentKey(newID, a.property, a.fileName)] = a
	}
	for k, v := range reKey {
		s.attachments[k] = v
	}
	return nil
}

// streamToFile copies r into the file at key using RootedFS.OpenForWrite.
// Returns nil on success. Callers must ensure the underlying FS supports
// streaming (RootedFS.SupportsStreaming) before calling.
func (s *FSStore) streamToFile(key string, r io.Reader) error {
	wc, err := s.rooted.OpenForWrite(key, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(wc, r)
	closeErr := wc.Close()
	if copyErr != nil {
		_ = s.rooted.Remove(key)
		return copyErr
	}
	if closeErr != nil {
		_ = s.rooted.Remove(key)
		return closeErr
	}
	return nil
}
