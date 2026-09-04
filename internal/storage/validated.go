package storage

import (
	"io"
	"io/fs"
	"os"
)

// ValidatedPath is an absolute filesystem path that has passed
// RootedFS.resolve: the key it was built from contained no traversal
// syntax (no "..", no absolute prefix, no backslash, colon, control
// character, empty segment or Windows reserved name), and the path is
// the join of that key onto a RootedFS root.
//
// The single string field is unexported and there is no exported
// constructor, so no package outside storage can produce one. A
// function taking a ValidatedPath therefore has a compile-time
// guarantee that the path passed validation — the barrier that was
// previously only a documented convention (see the RootedFS type doc)
// is now carried by the type system.
//
// This is what validatedFS exists to consume. It is also what makes
// the "path injection" shape legible to static analysis: the taint
// path from a caller-supplied key to os.WriteFile now runs through a
// constructor that only the validator can call.
//
// Nil: never returned — the zero ValidatedPath is an empty path, which
// every os call rejects.
type ValidatedPath struct {
	p string
}

// String returns the absolute filesystem path.
func (v ValidatedPath) String() string { return v.p }

// validatedFS adapts a raw [FS] to ValidatedPath-keyed methods. It is
// the only thing RootedFS calls: every RootedFS method resolves its key
// to a ValidatedPath first, so the raw FS underneath is never reachable
// with an unvalidated path.
//
// The adapter is deliberately thin — it exists for the type barrier,
// not for behaviour. Path values are unwrapped exactly here and nowhere
// else, which keeps the number of places that turn a validated path
// back into a bare string down to one reviewable file.
type validatedFS struct {
	fs FS
}

func (v validatedFS) ReadFile(path ValidatedPath) ([]byte, error) {
	return v.fs.ReadFile(path.p)
}

func (v validatedFS) WriteFile(path ValidatedPath, data []byte, perm os.FileMode) error {
	return v.fs.WriteFile(path.p, data, perm)
}

func (v validatedFS) Remove(path ValidatedPath) error {
	return v.fs.Remove(path.p)
}

func (v validatedFS) Rename(oldPath, newPath ValidatedPath) error {
	return v.fs.Rename(oldPath.p, newPath.p)
}

func (v validatedFS) Stat(path ValidatedPath) (os.FileInfo, error) {
	return v.fs.Stat(path.p)
}

func (v validatedFS) MkdirAll(path ValidatedPath, perm os.FileMode) error {
	return v.fs.MkdirAll(path.p, perm)
}

func (v validatedFS) ReadDir(path ValidatedPath) ([]os.DirEntry, error) {
	return v.fs.ReadDir(path.p)
}

func (v validatedFS) Walk(root ValidatedPath, fn fs.WalkDirFunc) error {
	return v.fs.Walk(root.p, fn)
}

func (v validatedFS) Open(path ValidatedPath) (io.ReadCloser, error) {
	return v.fs.Open(path.p)
}

// OpenForWrite opens path for streaming writes. Unlike the other
// methods this bypasses the wrapped FS and calls os.OpenFile directly:
// the FS interface has no streaming-write method, and adding one is a
// larger design change. The bypass is guarded by
// [RootedFS.SupportsStreaming], which returns true only for an
// OsFS-backed stack — a MemFS-backed RootedFS never reaches here.
func (v validatedFS) OpenForWrite(path ValidatedPath, perm os.FileMode) (io.WriteCloser, error) {
	return os.OpenFile(path.p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
}
