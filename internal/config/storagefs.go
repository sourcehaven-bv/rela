package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// Permission bits reported by [StorageFS.Stat]: config read through a
// Loader is never writable through this view.
const (
	readOnlyDirMode  os.FileMode = 0o555
	readOnlyFileMode os.FileMode = 0o444
)

// errReadOnly is returned by every mutating [StorageFS] method.
var errReadOnly = errors.New("config: project config view is read-only")

// StorageFS adapts a [Loader] to [storage.FS], for readers written against
// absolute paths on a filesystem — the metamodel loader and its include
// resolution, and migration detection.
//
// A path under root maps to the loader name relative to root, so
// "<root>/schema.yaml" reads Load("schema.yaml"). A path outside root is
// reported as absent rather than read from the disk: the view serves
// project config and nothing else, and a schema include that escapes the
// project could not be carried in a database anyway.
//
// Only reads are supported. ReadFile, Open and Stat on a file map to Load;
// ReadDir maps to List. Writes fail with an error, and Walk walks nothing
// for the reason [FSView] documents: a Loader cannot enumerate everything.
//
// Nil: rejected — [NewStorageFS] returns an error.
type StorageFS struct {
	loader Loader
	root   string
	//nolint:containedctx // storage.FS's signatures take no ctx; see [FSView].
	ctx context.Context
}

var _ storage.FS = (*StorageFS)(nil)

// NewStorageFS returns a read-only storage.FS over loader, rooted at root.
// root is made absolute so relative and absolute callers agree.
func NewStorageFS(ctx context.Context, loader Loader, root string) (*StorageFS, error) {
	if loader == nil {
		return nil, errors.New("config: NewStorageFS: loader is required")
	}
	if ctx == nil {
		return nil, errors.New("config: NewStorageFS: context is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("config: NewStorageFS: %w", err)
	}
	return &StorageFS{loader: loader, root: abs, ctx: ctx}, nil
}

// name maps an absolute or root-relative path to a loader name.
func (s *StorageFS) name(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(s.root, abs)
	if err != nil || rel == ".." || filepath.IsAbs(rel) || len(rel) > 2 && rel[:3] == ".."+string(filepath.Separator) {
		return "", &fs.PathError{Op: "open", Path: path, Err: fs.ErrNotExist}
	}
	return filepath.ToSlash(rel), nil
}

// ReadFile returns the bytes of the config file at path.
func (s *StorageFS) ReadFile(path string) ([]byte, error) {
	name, err := s.name(path)
	if err != nil {
		return nil, err
	}
	return s.loader.Load(s.ctx, name)
}

// Open returns a reader over the whole config file at path.
func (s *StorageFS) Open(path string) (io.ReadCloser, error) {
	data, err := s.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

// Stat reports a config file's size, or a directory for the root itself.
// Any other directory is not detectable through a Loader (see [FSView]) and
// reports as absent.
func (s *StorageFS) Stat(path string) (os.FileInfo, error) {
	name, err := s.name(path)
	if err != nil {
		return nil, err
	}
	if name == "." {
		return fileInfo{name: filepath.Base(s.root), dir: true}, nil
	}
	data, err := s.loader.Load(s.ctx, name)
	if err != nil {
		return nil, err
	}
	return fileInfo{name: filepath.Base(name), size: int64(len(data))}, nil
}

// ReadDir lists the regular files directly under path.
func (s *StorageFS) ReadDir(path string) ([]os.DirEntry, error) {
	name, err := s.name(path)
	if err != nil {
		return nil, err
	}
	names, err := s.loader.List(s.ctx, name)
	if err != nil {
		return nil, err
	}
	entries := make([]os.DirEntry, 0, len(names))
	for _, n := range names {
		entries = append(entries, fs.FileInfoToDirEntry(fileInfo{name: n}))
	}
	return entries, nil
}

// Walk visits nothing; see the type comment.
func (s *StorageFS) Walk(string, fs.WalkDirFunc) error { return nil }

// Getwd returns the view's root.
func (s *StorageFS) Getwd() (string, error) { return s.root, nil }

// WriteFile fails: the view is read-only.
func (s *StorageFS) WriteFile(string, []byte, os.FileMode) error { return errReadOnly }

// Remove fails: the view is read-only.
func (s *StorageFS) Remove(string) error { return errReadOnly }

// Rename fails: the view is read-only.
func (s *StorageFS) Rename(string, string) error { return errReadOnly }

// MkdirAll fails: the view is read-only.
func (s *StorageFS) MkdirAll(string, os.FileMode) error { return errReadOnly }

// fileInfo is the minimal os.FileInfo a Loader can back.
type fileInfo struct {
	name string
	size int64
	dir  bool
}

func (f fileInfo) Name() string { return f.name }
func (f fileInfo) Size() int64  { return f.size }
func (f fileInfo) Mode() os.FileMode {
	if f.dir {
		return os.ModeDir | readOnlyDirMode
	}
	return readOnlyFileMode
}
func (f fileInfo) ModTime() time.Time { return time.Time{} }
func (f fileInfo) IsDir() bool        { return f.dir }
func (f fileInfo) Sys() any           { return nil }
