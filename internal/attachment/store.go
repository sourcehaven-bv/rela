package attachment

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// Store manages content-addressable attachment storage.
type Store struct {
	fs      storage.FS
	rootDir string // Absolute path to project root
}

// NewStore creates a new attachment store.
func NewStore(fs storage.FS, rootDir string) *Store {
	return &Store{
		fs:      fs,
		rootDir: rootDir,
	}
}

// compile-time check that Store satisfies the top-level Manager interface.
var _ Manager = (*Store)(nil)

// AttachFile implements Manager. It drains data, dedupes by content hash, and
// records (entityID, property) on the returned Info alongside the backend key.
// The current-user lookup matches the legacy workspace path — attachments are
// intrinsically user actions, so the caller doesn't need to plumb it through.
func (s *Store) AttachFile(
	_ context.Context, entityID, property, fileName string, data io.Reader,
) (*Info, error) {
	bytesData, err := io.ReadAll(data)
	if err != nil {
		return nil, fmt.Errorf("attachment: read data: %w", err)
	}
	addedBy := ""
	if u, uErr := user.Current(); uErr == nil {
		addedBy = u.Username
	}
	att, err := s.AddBytes(bytesData, fileName, addedBy)
	if err != nil {
		return nil, err
	}
	info := &Info{
		Key:      att.Path,
		EntityID: entityID,
		Property: property,
	}
	if att.Metadata != nil {
		info.OriginalName = att.Metadata.OriginalName
		info.ContentType = att.Metadata.ContentType
		info.Size = att.Metadata.Size
	}
	return info, nil
}

// InfoFor implements Manager. Returns the metadata for a previously stored
// key, or an error if no attachment or metadata is present at that key.
func (s *Store) InfoFor(_ context.Context, key string) (*Info, error) {
	if !s.Exists(key) {
		return nil, fmt.Errorf("attachment: no attachment at %s", key)
	}
	info := &Info{Key: key}
	if meta, err := s.GetMetadata(key); err == nil && meta != nil {
		info.OriginalName = meta.OriginalName
		info.ContentType = meta.ContentType
		info.Size = meta.Size
	}
	return info, nil
}

// AttachmentsDir is the directory name for storing attachments.
const AttachmentsDir = "attachments"

// minHashLength is the minimum length for a valid hash.
const minHashLength = 8

// Attachment represents a stored file with its metadata.
type Attachment struct {
	Hash     string // SHA-256 hash (hex)
	Ext      string // File extension including dot (e.g., ".png")
	Path     string // Relative path: attachments/ab/ab3f8c2e.png
	Metadata *Metadata
}

// Add adds a file to the attachment store.
// It computes the SHA-256 hash, copies the file if not already present,
// and creates a metadata sidecar.
// Returns the attachment with its relative path.
func (s *Store) Add(sourcePath, addedBy string) (*Attachment, error) {
	// Read source file
	data, readErr := s.fs.ReadFile(sourcePath)
	if readErr != nil {
		return nil, fmt.Errorf("read source file: %w", readErr)
	}

	// Compute hash
	hash := HashBytes(data)

	// Get extension from original filename
	ext := strings.ToLower(filepath.Ext(sourcePath))
	if ext == "" {
		ext = ".bin" // Default extension for files without one
	}

	// Construct relative path
	relPath := PathFromHash(hash, ext)
	absPath := filepath.Join(s.rootDir, relPath)

	// Check if file already exists (deduplication)
	if _, statErr := s.fs.Stat(absPath); statErr == nil {
		// File exists, just return the attachment info
		meta, _ := s.GetMetadata(relPath)
		return &Attachment{
			Hash:     hash,
			Ext:      ext,
			Path:     relPath,
			Metadata: meta,
		}, nil
	}

	// Create directory structure
	dir := filepath.Dir(absPath)
	if mkdirErr := s.fs.MkdirAll(dir, 0755); mkdirErr != nil {
		return nil, fmt.Errorf("create directory: %w", mkdirErr)
	}

	// Write file
	if writeErr := s.fs.WriteFile(absPath, data, 0644); writeErr != nil {
		return nil, fmt.Errorf("write attachment: %w", writeErr)
	}

	// Create metadata
	meta := &Metadata{
		OriginalName: filepath.Base(sourcePath),
		ContentType:  detectContentType(ext),
		Size:         int64(len(data)),
		Added:        time.Now().UTC(),
		AddedBy:      addedBy,
	}

	// Write metadata sidecar
	metaPath := MetadataPath(absPath)
	metaData, marshalErr := MarshalMetadata(meta)
	if marshalErr != nil {
		return nil, fmt.Errorf("marshal metadata: %w", marshalErr)
	}
	if writeMetaErr := s.fs.WriteFile(metaPath, metaData, 0644); writeMetaErr != nil {
		return nil, fmt.Errorf("write metadata: %w", writeMetaErr)
	}

	return &Attachment{
		Hash:     hash,
		Ext:      ext,
		Path:     relPath,
		Metadata: meta,
	}, nil
}

// AddBytes adds data directly to the attachment store.
func (s *Store) AddBytes(data []byte, originalName, addedBy string) (*Attachment, error) {
	// Compute hash
	hash := HashBytes(data)

	// Get extension from original filename
	ext := strings.ToLower(filepath.Ext(originalName))
	if ext == "" {
		ext = ".bin"
	}

	// Construct relative path
	relPath := PathFromHash(hash, ext)
	absPath := filepath.Join(s.rootDir, relPath)

	// Check if file already exists (deduplication)
	if _, err := s.fs.Stat(absPath); err == nil {
		meta, _ := s.GetMetadata(relPath)
		return &Attachment{
			Hash:     hash,
			Ext:      ext,
			Path:     relPath,
			Metadata: meta,
		}, nil
	}

	// Create directory structure
	dir := filepath.Dir(absPath)
	if err := s.fs.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	// Write file
	if err := s.fs.WriteFile(absPath, data, 0644); err != nil {
		return nil, fmt.Errorf("write attachment: %w", err)
	}

	// Create metadata
	meta := &Metadata{
		OriginalName: originalName,
		ContentType:  detectContentType(ext),
		Size:         int64(len(data)),
		Added:        time.Now().UTC(),
		AddedBy:      addedBy,
	}

	// Write metadata sidecar
	metaPath := MetadataPath(absPath)
	metaData, err := MarshalMetadata(meta)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}
	if err := s.fs.WriteFile(metaPath, metaData, 0644); err != nil {
		return nil, fmt.Errorf("write metadata: %w", err)
	}

	return &Attachment{
		Hash:     hash,
		Ext:      ext,
		Path:     relPath,
		Metadata: meta,
	}, nil
}

// Get retrieves an attachment's data by its relative path.
func (s *Store) Get(relPath string) ([]byte, error) {
	absPath := filepath.Join(s.rootDir, relPath)
	return s.fs.ReadFile(absPath)
}

// GetMetadata retrieves an attachment's metadata by its relative path.
func (s *Store) GetMetadata(relPath string) (*Metadata, error) {
	absPath := filepath.Join(s.rootDir, relPath)
	metaPath := MetadataPath(absPath)

	data, err := s.fs.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("read metadata: %w", err)
	}

	return UnmarshalMetadata(data)
}

// Exists checks if an attachment exists by its relative path.
func (s *Store) Exists(relPath string) bool {
	absPath := filepath.Join(s.rootDir, relPath)
	_, err := s.fs.Stat(absPath)
	return err == nil
}

// Remove deletes an attachment and its metadata sidecar.
func (s *Store) Remove(relPath string) error {
	absPath := filepath.Join(s.rootDir, relPath)
	metaPath := MetadataPath(absPath)

	// Remove metadata first (non-fatal if missing)
	_ = s.fs.Remove(metaPath)

	// Remove the attachment file
	if err := s.fs.Remove(absPath); err != nil {
		return fmt.Errorf("remove attachment: %w", err)
	}

	return nil
}

// GCResult contains the results of garbage collection.
type GCResult struct {
	Removed   []string // Paths that were removed
	Reclaimed int64    // Bytes reclaimed
}

// GC removes unreferenced attachments from the store.
// referencedPaths should contain all attachment paths currently referenced by entities.
func (s *Store) GC(referencedPaths []string) (*GCResult, error) {
	// Build set of referenced paths
	referenced := make(map[string]bool)
	for _, p := range referencedPaths {
		// Normalize path
		referenced[filepath.ToSlash(p)] = true
	}

	result := &GCResult{}
	attachmentsDir := filepath.Join(s.rootDir, AttachmentsDir)

	// Check if attachments directory exists
	if _, err := s.fs.Stat(attachmentsDir); errors.Is(err, os.ErrNotExist) {
		return result, nil
	}

	// Walk attachments directory
	err := s.fs.Walk(attachmentsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Skip metadata files
		if strings.HasSuffix(path, ".yaml") {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(s.rootDir, path)
		if err != nil {
			return nil //nolint:nilerr // Skip files we can't get relative path for
		}
		relPath = filepath.ToSlash(relPath)

		// Check if referenced
		if !referenced[relPath] {
			// Not referenced, remove it
			result.Reclaimed += info.Size()
			result.Removed = append(result.Removed, relPath)
		}

		return nil
	})

	return result, err
}

// RemoveUnreferenced removes the files listed in a GC result.
func (s *Store) RemoveUnreferenced(result *GCResult) error {
	for _, relPath := range result.Removed {
		if err := s.Remove(relPath); err != nil {
			return err
		}
	}
	return nil
}

// List returns all attachment paths in the store.
func (s *Store) List() ([]string, error) {
	var paths []string
	attachmentsDir := filepath.Join(s.rootDir, AttachmentsDir)

	// Check if attachments directory exists
	if _, err := s.fs.Stat(attachmentsDir); errors.Is(err, os.ErrNotExist) {
		return paths, nil
	}

	err := s.fs.Walk(attachmentsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and metadata files
		if info.IsDir() || strings.HasSuffix(path, ".yaml") {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(s.rootDir, path)
		if err != nil {
			return nil //nolint:nilerr // Skip files we can't get relative path for
		}

		paths = append(paths, filepath.ToSlash(relPath))
		return nil
	})

	return paths, err
}

// detectContentType returns a MIME type based on file extension.
func detectContentType(ext string) string {
	// Try standard library first
	mimeType := mime.TypeByExtension(ext)
	if mimeType != "" {
		return mimeType
	}

	// Fallback for common types
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain"
	case ".md":
		return "text/markdown"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	default:
		return "application/octet-stream"
	}
}

// ValidatePath checks if a path is a valid attachment path.
func ValidatePath(path string) error {
	hash, ext, ok := ParsePath(path)
	if !ok {
		return fmt.Errorf("invalid attachment path format: %s", path)
	}
	if len(hash) < minHashLength {
		return fmt.Errorf("hash too short: %s", hash)
	}
	if ext == "" {
		return errors.New("missing file extension")
	}
	return nil
}

// Reader returns a reader for the attachment data.
func (s *Store) Reader(relPath string) (*bytes.Reader, error) {
	data, err := s.Get(relPath)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

// ListWithMetadata returns all attachments with their metadata.
func (s *Store) ListWithMetadata() ([]*Attachment, error) {
	paths, err := s.List()
	if err != nil {
		return nil, err
	}

	attachments := make([]*Attachment, 0, len(paths))
	for _, relPath := range paths {
		hash, ext, ok := ParsePath(relPath)
		if !ok {
			continue
		}
		meta, _ := s.GetMetadata(relPath)
		attachments = append(attachments, &Attachment{
			Hash:     hash,
			Ext:      ext,
			Path:     relPath,
			Metadata: meta,
		})
	}

	return attachments, nil
}

// UpdateDisplayName updates the original-name field in an attachment's metadata.
func (s *Store) UpdateDisplayName(relPath, newName string) error {
	absPath := filepath.Join(s.rootDir, relPath)
	metaPath := MetadataPath(absPath)

	// Load existing metadata or create new
	meta, err := s.GetMetadata(relPath)
	if err != nil {
		// If no metadata exists, create minimal metadata
		meta = &Metadata{
			OriginalName: newName,
			ContentType:  detectContentType(filepath.Ext(relPath)),
			Added:        time.Now().UTC(),
		}
	} else {
		meta.OriginalName = newName
	}

	// Write updated metadata
	metaData, err := MarshalMetadata(meta)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	if err := s.fs.WriteFile(metaPath, metaData, 0644); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}

	return nil
}
