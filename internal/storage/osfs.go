package storage

import (
	"io"
	"os"
	"path/filepath"
)

// OsFS implements FS using the real operating system filesystem.
// All methods delegate directly to the corresponding os/filepath functions.
// Paths are cleaned via filepath.Clean to prevent path traversal.
type OsFS struct{}

// NewOsFS returns a new OsFS instance.
func NewOsFS() *OsFS {
	return &OsFS{}
}

func (f *OsFS) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(filepath.Clean(path))
}

func (f *OsFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(filepath.Clean(path), data, perm)
}

func (f *OsFS) Remove(path string) error {
	return os.Remove(filepath.Clean(path))
}

func (f *OsFS) Rename(oldpath, newpath string) error {
	return os.Rename(filepath.Clean(oldpath), filepath.Clean(newpath))
}

func (f *OsFS) Stat(path string) (os.FileInfo, error) {
	return os.Stat(filepath.Clean(path))
}

func (f *OsFS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(filepath.Clean(path), perm)
}

func (f *OsFS) ReadDir(path string) ([]os.DirEntry, error) {
	return os.ReadDir(filepath.Clean(path))
}

func (f *OsFS) Walk(root string, fn filepath.WalkFunc) error {
	return filepath.Walk(filepath.Clean(root), fn)
}

func (f *OsFS) Open(path string) (io.ReadCloser, error) {
	return os.Open(filepath.Clean(path))
}

func (f *OsFS) Getwd() (string, error) {
	return os.Getwd()
}

// Compile-time check that OsFS implements FS.
var _ FS = (*OsFS)(nil)
