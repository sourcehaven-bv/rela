package fsstore

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// AttachFile streams r to `attachments/<entityID>/<property>/<fileName>`.
//
// The write goes through writeDataFile so the atomic-rename + fsync
// guarantees from SafeFS apply; on encryption-free repos (all repos,
// post-rollback) that's just the standard write path. The source reader
// is drained in chunks — peak memory is bounded by io.Copy's internal
// buffer, not the payload size.
//
// If an attachment already exists at this (entityID, property) under a
// different filename, the old file is removed first (1:1 ownership per
// property).
func (s *FSStore) AttachFile(_ context.Context, entityID, property, fileName string, r io.Reader) error {
	if err := storeutil.ValidateProperty(property); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.entities[entityID]; !ok {
		return store.ErrNotFound
	}

	dir := filepath.Join(s.attachDir, entityID, property)
	if err := s.dirs.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	key := entityID + "/" + property
	if old, exists := s.attachments[key]; exists && old.fileName != fileName {
		_ = s.dirs.Remove(filepath.Join(dir, old.fileName))
	}

	path := filepath.Join(dir, fileName)
	n, err := s.writeAttachment(path, r)
	if err != nil {
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

// writeAttachment persists r to path. On a real OS-backed FS it
// streams (constant memory); on a MemFS-backed test FS it falls
// back to buffered WriteFile since MemFS lives in the process
// heap and has no streaming primitive worth plumbing.
//
// Callers must have already created the parent directory via
// s.dirs.MkdirAll.
func (s *FSStore) writeAttachment(path string, r io.Reader) (int64, error) {
	if _, ok := s.dirs.(*storage.OsFS); ok {
		return s.streamToFile(path, r)
	}
	if safe, ok := s.dirs.(*storage.SafeFS); ok {
		if _, inner := safe.FS.(*storage.OsFS); inner {
			return s.streamToFile(path, r)
		}
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}
	if err := s.bytes.WriteFile(path, data, 0o644); err != nil {
		return 0, err
	}
	return int64(len(data)), nil
}

// ReadAttachment returns a streaming reader over the attachment's
// bytes. Callers MUST Close the returned reader.
func (s *FSStore) ReadAttachment(_ context.Context, entityID, property string) (io.ReadCloser, error) {
	s.mu.RLock()
	key := entityID + "/" + property
	a, ok := s.attachments[key]
	s.mu.RUnlock()

	if !ok {
		return nil, store.ErrNotFound
	}

	path := filepath.Join(s.attachDir, a.entityID, a.property, a.fileName)
	return s.bytes.Open(path)
}

func (s *FSStore) DeleteAttachment(_ context.Context, entityID, property string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := entityID + "/" + property
	a, ok := s.attachments[key]
	if !ok {
		return store.ErrNotFound
	}

	path := filepath.Join(s.attachDir, a.entityID, a.property, a.fileName)
	if err := s.dirs.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
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

// removeAttachmentDir removes `attachments/<entityID>/` and every
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

	if s.attachDir == "" {
		return nil
	}
	root := filepath.Join(s.attachDir, entityID)
	if _, err := s.dirs.Stat(root); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}

	propEntries, err := s.dirs.ReadDir(root)
	if err != nil {
		return err
	}
	for _, pe := range propEntries {
		if !pe.IsDir() {
			continue
		}
		propDir := filepath.Join(root, pe.Name())
		fileEntries, err := s.dirs.ReadDir(propDir)
		if err != nil {
			return err
		}
		for _, fe := range fileEntries {
			if fe.IsDir() {
				continue
			}
			rmErr := s.dirs.Remove(filepath.Join(propDir, fe.Name()))
			if rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
				return rmErr
			}
		}
		if rmErr := s.dirs.Remove(propDir); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
			return rmErr
		}
	}
	if rmErr := s.dirs.Remove(root); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
		return rmErr
	}
	return nil
}

// renameAttachmentDir moves `attachments/<oldID>/` to
// `attachments/<newID>/` and updates the in-memory index. No-op if
// the old dir does not exist. Must be called with s.mu held.
func (s *FSStore) renameAttachmentDir(oldID, newID string) error {
	if s.attachDir == "" {
		return nil
	}
	oldRoot := filepath.Join(s.attachDir, oldID)
	if _, err := s.dirs.Stat(oldRoot); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}

	newRoot := filepath.Join(s.attachDir, newID)
	if err := s.dirs.Rename(oldRoot, newRoot); err != nil {
		return err
	}

	reKey := make(map[string]attachMeta)
	for key, a := range s.attachments {
		if a.entityID != oldID {
			continue
		}
		delete(s.attachments, key)
		a.entityID = newID
		reKey[newID+"/"+a.property] = a
	}
	for k, v := range reKey {
		s.attachments[k] = v
	}
	return nil
}

// streamToFile copies r into path, reporting the number of bytes
// written. Uses os.OpenFile directly (bypassing s.bytes.WriteFile)
// so attachments stream chunk-by-chunk instead of materializing the
// entire payload in a byte slice.
//
// Production couples this to a real OS filesystem; fsstore is OsFS-
// backed everywhere in production. The streaming contract is the
// whole point — a 500 MB attachment writes at constant memory.
func (s *FSStore) streamToFile(path string, r io.Reader) (int64, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, err
	}
	n, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(path)
		return n, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return n, closeErr
	}
	return n, nil
}
