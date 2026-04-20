// Package storage provides filesystem abstraction for rela's file I/O operations.
//
// The package defines a layered architecture:
//   - FS interface: raw file operations, swappable for tests
//   - SafeFS: wraps FS with atomic writes (crash-safe)
//   - Repository: domain-level CRUD for entities/relations/metamodel/cache
package storage

import (
	"io"
	"io/fs"
	"os"
)

// FS abstracts filesystem operations used throughout rela.
// The default production implementation is OsFS which delegates to the os package.
// Tests can use MemFS for fast, deterministic, in-memory file operations.
type FS interface {
	// ReadFile reads the named file and returns its contents.
	ReadFile(path string) ([]byte, error)

	// WriteFile writes data to the named file, creating it if necessary.
	WriteFile(path string, data []byte, perm os.FileMode) error

	// Remove removes the named file or empty directory.
	Remove(path string) error

	// Rename renames (moves) oldpath to newpath.
	Rename(oldpath, newpath string) error

	// Stat returns file info for the named file.
	Stat(path string) (os.FileInfo, error)

	// MkdirAll creates a directory path and all parents that do not yet exist.
	MkdirAll(path string, perm os.FileMode) error

	// ReadDir reads the named directory and returns its directory entries sorted by name.
	ReadDir(path string) ([]os.DirEntry, error)

	// Walk walks the file tree rooted at root, calling fn for each
	// file or directory. Uses fs.WalkDirFunc (Go 1.16+) so entries
	// expose fs.DirEntry rather than os.FileInfo — no stat syscall
	// per entry unless the callback explicitly asks for one via
	// d.Info().
	Walk(root string, fn fs.WalkDirFunc) error

	// Open opens the named file for reading.
	Open(path string) (io.ReadCloser, error)

	// Getwd returns the current working directory.
	Getwd() (string, error)
}
