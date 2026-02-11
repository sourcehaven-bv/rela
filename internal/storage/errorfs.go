package storage

import (
	"io"
	"os"
	"path/filepath"
)

// ErrorFS wraps an FS and returns an error for specified operations.
// Useful for testing error handling paths.
type ErrorFS struct {
	FS        FS
	WalkError error // Error to return from Walk
}

// NewErrorFS creates an ErrorFS wrapping the given FS.
func NewErrorFS(fs FS) *ErrorFS {
	return &ErrorFS{FS: fs}
}

func (e *ErrorFS) ReadFile(path string) ([]byte, error) {
	return e.FS.ReadFile(path)
}

func (e *ErrorFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	return e.FS.WriteFile(path, data, perm)
}

func (e *ErrorFS) Remove(path string) error {
	return e.FS.Remove(path)
}

func (e *ErrorFS) Rename(oldpath, newpath string) error {
	return e.FS.Rename(oldpath, newpath)
}

func (e *ErrorFS) Stat(path string) (os.FileInfo, error) {
	return e.FS.Stat(path)
}

func (e *ErrorFS) MkdirAll(path string, perm os.FileMode) error {
	return e.FS.MkdirAll(path, perm)
}

func (e *ErrorFS) ReadDir(path string) ([]os.DirEntry, error) {
	return e.FS.ReadDir(path)
}

func (e *ErrorFS) Walk(root string, fn filepath.WalkFunc) error {
	if e.WalkError != nil {
		return e.WalkError
	}
	return e.FS.Walk(root, fn)
}

func (e *ErrorFS) Open(path string) (io.ReadCloser, error) {
	return e.FS.Open(path)
}

func (e *ErrorFS) Getwd() (string, error) {
	return e.FS.Getwd()
}

// Compile-time check that ErrorFS implements FS.
var _ FS = (*ErrorFS)(nil)
