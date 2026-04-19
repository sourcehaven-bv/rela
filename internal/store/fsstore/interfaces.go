package fsstore

import (
	"os"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// StoreFS is the byte-level boundary fsstore uses for every data
// read/write. Transforms (encryption today, compression or dedup
// tomorrow) are composed above this interface by the factory;
// fsstore itself never knows which transforms are active.
//
// The interface deliberately excludes directory-topology methods
// (ReadDir/Walk/Open/MkdirAll). Those belong on DirFS, where byte
// I/O is structurally absent — the compiler then prevents any
// fsstore call site from bypassing the transform stack by reaching
// for a raw read.
//
// WriteFile auto-creates parent directories as part of the contract.
// Implementations that do not natively support this (e.g. a bare
// OsFS) must be wrapped in SafeFS, which folds in the MkdirAll.
type StoreFS interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm os.FileMode) error
	Remove(path string) error
	Rename(oldpath, newpath string) error
	Stat(path string) (os.FileInfo, error)
}

// DirFS is the raw directory view fsstore uses for enumeration,
// stat, and temp-file cleanup. It deliberately omits ReadFile /
// WriteFile / Open so that byte I/O is forced through StoreFS and
// cannot silently bypass transforms above it.
//
// The watcher, the consistency verifier, and cleanupTempFiles
// legitimately operate at this raw layer: fsnotify events and
// stat-based reconciliation inherently see on-disk bytes as they
// are, not as a plaintext decorator would present them. DirFS makes
// that scope explicit.
type DirFS interface {
	MkdirAll(path string, perm os.FileMode) error
	ReadDir(path string) ([]os.DirEntry, error)
	Stat(path string) (os.FileInfo, error)
	Walk(root string, fn filepath.WalkFunc) error
	Remove(path string) error
}

// RawReader is the single-method window the watcher uses to read
// the raw on-disk bytes of a file (before any transform). It is
// structurally separate from DirFS so that only code which
// legitimately needs raw bytes (the fsnotify self-echo path) ever
// holds a handle that exposes ReadFile.
type RawReader interface {
	ReadFile(path string) ([]byte, error)
}

// Compile-time assertions: the existing storage.FS satisfies the
// narrower interfaces fsstore consumes. This lets the factory wire
// a single storage.FS today and a decorated StoreFS + raw DirFS
// pair later without changing fsstore call sites.
var (
	_ StoreFS   = storage.FS(nil)
	_ DirFS     = storage.FS(nil)
	_ RawReader = storage.FS(nil)

	// *storage.SafeFS embeds storage.FS and must therefore also
	// satisfy all three interfaces.
	_ StoreFS   = (*storage.SafeFS)(nil)
	_ DirFS     = (*storage.SafeFS)(nil)
	_ RawReader = (*storage.SafeFS)(nil)
)
