package fsstore

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

func (s *FSStore) AttachFile(_ context.Context, entityID, property, fileName string, r io.Reader) error {
	if err := storeutil.ValidateProperty(property); err != nil {
		return err
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.entities[entityID]; !ok {
		return store.ErrNotFound
	}

	// Write file to disk.
	dir := filepath.Join(s.attachDir, entityID, property)
	if err := s.fs.MkdirAll(dir, 0755); err != nil {
		return err
	}

	key := entityID + "/" + property

	// Remove old file if replacing with different name.
	if old, exists := s.attachments[key]; exists && old.fileName != fileName {
		_ = s.fs.Remove(filepath.Join(dir, old.fileName))
	}

	path := filepath.Join(dir, fileName)
	if err := s.fs.WriteFile(path, data, 0644); err != nil {
		return err
	}

	s.attachments[key] = attachMeta{
		entityID: entityID,
		property: property,
		fileName: fileName,
		size:     int64(len(data)),
	}
	return nil
}

func (s *FSStore) ReadAttachment(_ context.Context, entityID, property string) (io.ReadCloser, error) {
	s.mu.RLock()
	key := entityID + "/" + property
	a, ok := s.attachments[key]
	s.mu.RUnlock()

	if !ok {
		return nil, store.ErrNotFound
	}

	path := filepath.Join(s.attachDir, a.entityID, a.property, a.fileName)
	return s.fs.Open(path)
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
	if err := s.fs.Remove(path); err != nil && !os.IsNotExist(err) {
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
