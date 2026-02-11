package storage

import (
	"io"
	"os"
	"path/filepath"
)

// OsFS implements FS using the real operating system filesystem.
// All methods delegate directly to the corresponding os/filepath functions.
type OsFS struct{}

// NewOsFS returns a new OsFS instance.
func NewOsFS() *OsFS {
	return &OsFS{}
}

func (f *OsFS) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (f *OsFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

func (f *OsFS) Remove(path string) error {
	return os.Remove(path)
}

func (f *OsFS) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func (f *OsFS) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (f *OsFS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (f *OsFS) ReadDir(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}

func (f *OsFS) Walk(root string, fn filepath.WalkFunc) error {
	return filepath.Walk(root, fn)
}

func (f *OsFS) Open(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

func (f *OsFS) Getwd() (string, error) {
	return os.Getwd()
}

// Compile-time check that OsFS implements FS.
var _ FS = (*OsFS)(nil)
