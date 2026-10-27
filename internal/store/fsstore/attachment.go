package fsstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path"

	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// AttachFamilyFile streams r to `<attachKey>/<entityID>/<property>/<fileName>`.
//
// The write goes through RootedFS so the path is validated before it
// reaches the underlying FS. On OS-backed filesystems the data is
// streamed through RootedFS.OpenForWrite; on MemFS the data is buffered
// and written via RootedFS.WriteFile.
//
// If an attachment already exists at this (entityID, property) under a
// different filename, the old file is removed first (1:1 ownership per
// property).
// attachmentKey is the per-file index key: (entityID, property, fileName).
func attachmentKey(entityID, property, fileName string) string {
	return entityID + "/" + property + "/" + fileName
}

func (s *FSStore) attachFile(_ context.Context, entityID, property, fileName string, r io.Reader) error {
	if err := storeutil.ValidateProperty(property); err != nil {
		return err
	}
	if err := store.ValidateFileName(fileName); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Bytes are keyed per bare id and shared by the family: any face of
	// the id makes it exist (BUG-CTUW2N).
	if s.familySize(entityID) == 0 {
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
	// The temp name comes from storage.AttachTempName: a leading-dot prefix
	// no real upload can carry, so the index loader can skip it (a plain
	// ".new" suffix would clash with an upload literally named "x.new"),
	// and a random part, so two processes uploading the same name never
	// share a temp file.
	tmpKey := path.Join(dirKey, storage.AttachTempName())
	// Backstop size guard: cap reads at MaxAttachmentBytes so no caller can
	// write an unbounded attachment (the API layer also caps at ingress).
	hasher := store.NewAttachmentHasher()
	n, err := s.writeAttachment(tmpKey, io.TeeReader(storeutil.LimitAttachmentReader(r), hasher))
	if err != nil {
		_ = s.rooted.Remove(tmpKey) // remove the failed partial; old file intact
		return err
	}
	if err := s.rooted.Rename(tmpKey, fileKey); err != nil {
		_ = s.rooted.Remove(tmpKey)
		return err
	}

	meta := attachMeta{
		entityID: entityID,
		property: property,
		fileName: fileName,
		size:     n,
		sha256:   hasher.Sum(),
	}
	// The stat pins the hash to this file version. Without it the next list
	// hashes the file once more, which is correct, only slower.
	if info, statErr := s.rooted.Stat(fileKey); statErr == nil {
		meta.modTime = info.ModTime()
	}
	s.attachments[key] = meta
	return nil
}

// writeAttachment persists r to the given key. On OsFS-backed stacks
// it streams via RootedFS.OpenForWrite (constant memory); on MemFS it
// buffers via WriteFile since MemFS has no streaming primitive.
//
// Parent directory creation is guaranteed by AttachFamilyFile's MkdirAll
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

// ReadFamilyAttachment returns a streaming reader over the attachment's
// bytes. Callers MUST Close the returned reader.
func (s *FSStore) ReadFamilyAttachment(_ context.Context, entityID, property, fileName string) (io.ReadCloser, error) {
	s.mu.RLock()
	a, ok := s.attachments[attachmentKey(entityID, property, fileName)]
	s.mu.RUnlock()

	if !ok {
		return nil, store.ErrNotFound
	}

	fileKey := path.Join(s.attachKey, a.entityID, a.property, a.fileName)
	return s.rooted.Open(fileKey)
}

func (s *FSStore) deleteAttachment(_ context.Context, entityID, property, fileName string) error {
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

// ListFamilyAttachments lists the family's attachments. Each file is
// stat'ed; one whose size or mtime differs from the index, or that has no
// hash yet, is read and hashed now, and the index is updated. A file
// changed on disk behind the store (a checkout of another revision)
// therefore reports the hash of its current bytes.
//
// A hash is read once per file version: concurrent lists share one read
// (singleflight on the file and its stat), and a cancelled ctx stops it.
func (s *FSStore) ListFamilyAttachments(ctx context.Context, entityID string) ([]store.AttachmentInfo, error) {
	s.mu.RLock()
	if s.familySize(entityID) == 0 {
		s.mu.RUnlock()
		return nil, store.ErrNotFound
	}
	var metas []attachMeta
	for _, a := range s.attachments {
		if a.entityID == entityID {
			metas = append(metas, a)
		}
	}
	s.mu.RUnlock()

	result := make([]store.AttachmentInfo, 0, len(metas))
	for _, a := range metas {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		a = s.refreshAttachmentHash(ctx, a)
		result = append(result, store.AttachmentInfo{
			EntityID: a.entityID,
			Property: a.property,
			FileName: a.fileName,
			Size:     a.size,
			SHA256:   a.sha256,
		})
	}
	return result, nil
}

// refreshAttachmentHash returns a with size, mtime and hash matching the
// file on disk, hashing it when the stat changed or no hash is known. The
// read happens outside s.mu; the index entry is updated afterwards unless
// a write replaced it in the meantime. A file that cannot be read keeps
// its indexed size and reports no hash rather than failing the list.
func (s *FSStore) refreshAttachmentHash(ctx context.Context, a attachMeta) attachMeta {
	fileKey := path.Join(s.attachKey, a.entityID, a.property, a.fileName)
	info, err := s.rooted.Stat(fileKey)
	if err != nil {
		a.sha256 = ""
		return a
	}
	if a.sha256 != "" && info.Size() == a.size && info.ModTime().Equal(a.modTime) {
		return a
	}
	flightKey := fmt.Sprintf("%s\x00%d\x00%d", fileKey, info.Size(), info.ModTime().UnixNano())
	v, err, _ := s.hashFlight.Do(flightKey, func() (any, error) {
		return hashAttachmentFile(ctx, s.rooted, fileKey)
	})
	if err != nil {
		a.sha256 = ""
		return a
	}
	h, ok := v.(fileHash)
	if !ok {
		a.sha256 = ""
		return a
	}
	fresh := a
	fresh.size, fresh.modTime, fresh.sha256 = h.size, info.ModTime(), h.sum

	key := attachmentKey(a.entityID, a.property, a.fileName)
	s.mu.Lock()
	if cur, ok := s.attachments[key]; ok && cur == a {
		s.attachments[key] = fresh
	}
	s.mu.Unlock()
	return fresh
}

// fileHash is the SHA-256 and length of one attachment file.
type fileHash struct {
	sum  string
	size int64
}

// hashAttachmentFile reads the file at key and hashes it. The read stops
// when ctx is cancelled. A package function rather than an FSStore method:
// FSStore is at its plimsoll method cap.
func hashAttachmentFile(ctx context.Context, rooted *storage.RootedFS, key string) (fileHash, error) {
	f, err := rooted.Open(key)
	if err != nil {
		return fileHash{}, err
	}
	defer func() { _ = f.Close() }()
	h := store.NewAttachmentHasher()
	n, err := io.Copy(h, ctxReader{ctx: ctx, r: f})
	if err != nil {
		return fileHash{}, err
	}
	return fileHash{sum: h.Sum(), size: n}, nil
}

// ctxReader stops a copy once its context is cancelled.
type ctxReader struct {
	ctx context.Context //nolint:containedctx // scoped to one io.Copy call
	r   io.Reader
}

func (c ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}

// removeAttachmentDir removes `<attachKey>/<entityID>/` and every
// file underneath. Prunes the in-memory index for the entity
// regardless of whether the on-disk dir exists. Must be called with
// s.mu held.
//
// Called from DeleteFamily and RenameFamily: under the per-entity
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
	maps.Copy(s.attachments, reKey)
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
