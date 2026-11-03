package storage

import (
	"errors"
	"fmt"
	"io/fs"
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
//
// # Post-write observation
//
// SafeFS exposes an observer hook via OnPostWrite. Each installed
// observer fires exactly once per successful durable write, with the
// bytes that sit on disk after the atomic rename. Failed writes (the
// temp file was created but the rename failed) do NOT fire the hook.
// The fsstore watcher uses this hook to hash self-writes so it can
// skip its own fsnotify echoes.
type SafeFS struct {
	FS

	hook postWriteHook
}

// NewSafeFS creates a SafeFS that wraps the given filesystem.
func NewSafeFS(fs FS) *SafeFS {
	return &SafeFS{FS: fs}
}

// OnPostWrite installs obs as a post-write observer and returns the
// function that uninstalls exactly that observer. Any number of
// observers may be installed; they fire in installation order. A nil
// obs installs nothing and returns a no-op remover.
func (s *SafeFS) OnPostWrite(obs WriteObserver) (remove func()) {
	return s.hook.add(obs)
}

// WriteFile writes data to path atomically:
//  1. Writes to a new temp file in the same directory (see [TempPath])
//  2. Fsyncs the temp file
//  3. Renames temp file to final path (atomic on POSIX)
//  4. Fsyncs the parent directory
//  5. Fires the post-write observers with (path, data)
//
// Steps 1-4 are performed even when no observer is installed. Step 5
// is skipped if the rename fails — a failed write leaves no durable
// bytes to observe.
//
// Each call uses its own temp file, so concurrent writers of one path,
// in this process or another, never write into each other's temp file:
// the file at path is always one writer's complete bytes. The temp file
// is created with O_EXCL and perm, so the umask applies as for any new
// file and a planted symlink at the temp name is not followed.
func (s *SafeFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)

	// Ensure directory exists
	if err := s.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Create and write temp file using os directly for fsync support
	f, tmpPath, err := createTemp(path, perm)
	if err != nil {
		return err
	}

	// coverage-ignore-start: os-fs-event: Write on a freshly opened O_WRONLY temp file only fails on a full disk / OS
	// I/O fault, not injectable
	// in a unit test
	if _, writeErr := f.Write(data); writeErr != nil {
		f.Close()
		os.Remove(tmpPath)
		return writeErr
	}
	// coverage-ignore-end

	// Fsync to ensure data reaches disk
	// coverage-ignore-start: os-fs-event: fsync failure is an OS/disk-level fault not deterministically reproducible in
	// a unit test
	if syncErr := f.Sync(); syncErr != nil {
		f.Close()
		os.Remove(tmpPath)
		return syncErr
	}
	// coverage-ignore-end

	// coverage-ignore-start: os-fs-event: Close on a successfully written+synced fd only fails on a deferred OS I/O
	// error, not injectable in a
	// unit test
	if closeErr := f.Close(); closeErr != nil {
		os.Remove(tmpPath)
		return closeErr
	}
	// coverage-ignore-end

	// Atomic rename: temp → final
	if renameErr := os.Rename(tmpPath, path); renameErr != nil {
		os.Remove(tmpPath)
		return renameErr
	}

	// Fsync parent directory to persist the rename
	syncDir(dir)

	// Notify observers. Only fires on successful durable write.
	s.hook.fire(path, data)

	return nil
}

// tempCreateAttempts bounds the retries when a temp name already exists.
// With 64 random bits a clash with another writer is practically
// impossible; the bound stops a loop if a file system reports ErrExist
// for every name.
const tempCreateAttempts = 5

// createTemp creates a new temp file for an atomic write of path and
// returns it with its name.
func createTemp(path string, perm os.FileMode) (*os.File, string, error) {
	for range tempCreateAttempts {
		tmpPath := TempPath(path)
		f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
		if !errors.Is(err, fs.ErrExist) {
			return f, tmpPath, err
		}
	}
	// coverage-ignore-start: os-fs-event: reached only after repeated clashes of 64 random bits
	return nil, "", fmt.Errorf("create temp file for %s: %w", path, fs.ErrExist)
	// coverage-ignore-end
}

// syncDir fsyncs a directory to ensure metadata (renames) are persisted.
// Errors are ignored since the file content is already safe.
func syncDir(dir string) {
	d, err := os.Open(dir)
	// coverage-ignore-start: os-fs-event: syncDir runs on a directory just created by MkdirAll and written into;
	// os.Open failing requires the dir
	// to vanish mid-write, not deterministically reachable
	if err != nil {
		return
	}
	// coverage-ignore-end
	_ = d.Sync()
	d.Close()
}
