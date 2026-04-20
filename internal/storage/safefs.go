package storage

import (
	"os"
	"path/filepath"
	"sync"
)

// WriteObserver is invoked after a successful durable WriteFile,
// with the exact bytes that landed on disk after the atomic rename.
// See SafeFS.OnPostWrite.
type WriteObserver func(path string, data []byte)

// SafeFS wraps an FS and overrides WriteFile with atomic write semantics.
// It writes to a temporary file, fsyncs it, then renames over the target path.
// This prevents partial writes from corrupting files if the process crashes.
//
// Note: The atomic write uses os directly for fsync support. SafeFS is
// intended for production use wrapping OsFS — it does not make sense to
// wrap MemFS with SafeFS.
//
// # Post-write observation
//
// SafeFS exposes a single observer hook via OnPostWrite. The hook
// fires exactly once per successful durable write, with the bytes
// that sit on disk after the atomic rename. Failed writes (the temp
// file was created but the rename failed) do NOT fire the hook.
// The fsstore watcher uses this hook to hash self-writes so it can
// skip its own fsnotify echoes.
type SafeFS struct {
	FS

	mu       sync.RWMutex
	observer WriteObserver
}

// NewSafeFS creates a SafeFS that wraps the given filesystem.
func NewSafeFS(fs FS) *SafeFS {
	return &SafeFS{FS: fs}
}

// OnPostWrite installs obs as the post-write observer. Only one
// observer is supported; a second call replaces the first. Passing
// nil clears the observer.
func (s *SafeFS) OnPostWrite(obs WriteObserver) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observer = obs
}

// WriteFile writes data to path atomically:
//  1. Writes to path + ".tmp" in the same directory
//  2. Fsyncs the temp file
//  3. Renames temp file to final path (atomic on POSIX)
//  4. Fsyncs the parent directory
//  5. Fires the post-write observer with (path, data)
//
// Steps 1-4 are performed even when no observer is installed. Step 5
// is skipped if the rename fails — a failed write leaves no durable
// bytes to observe.
func (s *SafeFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)

	// Ensure directory exists
	if err := s.MkdirAll(dir, 0755); err != nil {
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

	// Notify observer. Only fires on successful durable write.
	s.mu.RLock()
	obs := s.observer
	s.mu.RUnlock()
	if obs != nil {
		obs(path, data)
	}

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
