package storage

import (
	"errors"
	"io"
	"io/fs"
	"os"
)

// ErrReadOnly is returned by every write method of [ReadOnlyFS].
var ErrReadOnly = errors.New("storage: filesystem is read-only")

// ReadOnlyFS wraps an FS and refuses every write: WriteFile, Remove, Rename
// and MkdirAll return [ErrReadOnly] without reaching the wrapped FS.
//
// It exists for callers that open a store over a directory they must leave
// byte-identical, such as `rela db import-fs` reading its source project.
// fsstore deletes leftover temp files when it opens; through this wrapper
// that cleanup fails quietly instead of changing the source.
//
// It deliberately does not implement the post-write observer capability, so
// a store opened over it cannot start a watcher.
type ReadOnlyFS struct {
	fs FS
}

// NewReadOnlyFS returns a read-only view of inner.
//
// Nil: rejected — inner must be a usable FS.
func NewReadOnlyFS(inner FS) (*ReadOnlyFS, error) {
	if inner == nil {
		return nil, errors.New("storage: ReadOnlyFS needs an FS")
	}
	return &ReadOnlyFS{fs: inner}, nil
}

func (r *ReadOnlyFS) ReadFile(path string) ([]byte, error) { return r.fs.ReadFile(path) }

func (r *ReadOnlyFS) WriteFile(string, []byte, os.FileMode) error { return ErrReadOnly }

func (r *ReadOnlyFS) Remove(string) error { return ErrReadOnly }

func (r *ReadOnlyFS) Rename(string, string) error { return ErrReadOnly }

func (r *ReadOnlyFS) Stat(path string) (os.FileInfo, error) { return r.fs.Stat(path) }

func (r *ReadOnlyFS) MkdirAll(string, os.FileMode) error { return ErrReadOnly }

func (r *ReadOnlyFS) ReadDir(path string) ([]os.DirEntry, error) { return r.fs.ReadDir(path) }

func (r *ReadOnlyFS) Walk(root string, fn fs.WalkDirFunc) error { return r.fs.Walk(root, fn) }

func (r *ReadOnlyFS) Open(path string) (io.ReadCloser, error) { return r.fs.Open(path) }

func (r *ReadOnlyFS) Getwd() (string, error) { return r.fs.Getwd() }

var _ FS = (*ReadOnlyFS)(nil)
