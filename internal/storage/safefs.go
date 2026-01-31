package storage

import (
	"os"
	"path/filepath"
)

// SafeFS wraps an FS and overrides WriteFile with atomic write semantics.
// It writes to a temporary file, fsyncs it, then renames over the target path.
// This prevents partial writes from corrupting files if the process crashes.
//
// Note: The atomic write uses os directly for fsync support. SafeFS is
// intended for production use wrapping OsFS — it does not make sense to
// wrap MemFS with SafeFS.
type SafeFS struct {
	FS
}

// NewSafeFS creates a SafeFS that wraps the given filesystem.
func NewSafeFS(fs FS) *SafeFS {
	return &SafeFS{FS: fs}
}

// WriteFile writes data to path atomically:
//  1. Writes to path + ".tmp" in the same directory
//  2. Fsyncs the temp file
//  3. Renames temp file to final path (atomic on POSIX)
//  4. Fsyncs the parent directory
func (s *SafeFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)

	// Ensure directory exists
	if err := s.FS.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmpPath := path + ".tmp"

	// Create and write temp file using os directly for fsync support
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}

	if _, writeErr := f.Write(data); writeErr != nil {
		f.Close()
		os.Remove(tmpPath)
		return writeErr
	}

	// Fsync to ensure data reaches disk
	if syncErr := f.Sync(); syncErr != nil {
		f.Close()
		os.Remove(tmpPath)
		return syncErr
	}

	if closeErr := f.Close(); closeErr != nil {
		os.Remove(tmpPath)
		return closeErr
	}

	// Atomic rename: temp → final
	if renameErr := os.Rename(tmpPath, path); renameErr != nil {
		os.Remove(tmpPath)
		return renameErr
	}

	// Fsync parent directory to persist the rename
	syncDir(dir)

	return nil
}

// syncDir fsyncs a directory to ensure metadata (renames) are persisted.
// Errors are ignored since the file content is already safe.
func syncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = d.Sync()
	d.Close()
}
