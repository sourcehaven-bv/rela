package config

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"time"
)

// FSView adapts a [Loader] to the standard [io/fs.FS] interface, so consumers
// written against fs.FS — [datamigration.LoadDir] and the `lua:` migration
// step, today — read through the config seam instead of the filesystem
// directly.
//
// It supports exactly what those consumers use: [io/fs.ReadFileFS] and
// [io/fs.ReadDirFS]. Open returns a reader over the whole file, which is what
// fs.ReadFile falls back to anyway; there is no seeking and no streaming,
// because a config file is read whole or not at all.
//
// # Not walkable from the root
//
// The root ("." ) opens, and lists EMPTY. [Loader] has no enumerate-everything
// capability — List takes a directory and rejects both "" and "." — so a root
// listing could only be invented, and an invented one would be wrong the
// moment a backend held a path this view had not been told about.
//
// So fs.WalkDir and fs.Glob over an FSView find nothing, and fstest.TestFS
// fails it for that reason. Both are working as intended: address the
// directory you want (ReadDir("migrations")) rather than discovering it. The
// root is openable only because WalkDir and Glob open it before anything else,
// and an fs.FS whose root cannot be opened is malformed in a way that breaks
// callers for no benefit.
//
// Nil: rejected — [NewFSView] returns an error rather than deferring to a nil
// dereference at the first read.
type FSView struct {
	loader Loader
	// ctx is carried on the value because [fs.FS] has no context in any of
	// its method signatures, so there is nowhere else to put one. The usual
	// objection to a stored context — that it hides the cancellation seam from
	// the caller — is answered by [FSView.WithContext] being the only way to
	// set it: a view is explicitly bound, never implicitly.
	//
	//nolint:containedctx // fs.FS's signatures take no ctx; see above.
	ctx context.Context
}

var (
	_ fs.FS         = (*FSView)(nil)
	_ fs.ReadFileFS = (*FSView)(nil)
	_ fs.ReadDirFS  = (*FSView)(nil)
)

// NewFSView wraps a Loader as an fs.FS. Its reads carry
// context.Background(); bind a real one with [FSView.WithContext].
func NewFSView(loader Loader) (*FSView, error) {
	if loader == nil {
		return nil, errors.New("config: NewFSView requires a non-nil Loader")
	}
	return &FSView{loader: loader}, nil
}

// WithContext returns a view whose reads carry ctx.
//
// [fs.FS] has no context in its signature, so a ctx cannot be threaded per
// call and lives on the value instead. It matters because a Loader's reads can
// genuinely block — a database-backed one waiting on a locked file — and
// internal/datamigration deliberately binds a `lua:` step's VM to the run
// context so a runaway migration is interruptible. Reading that step's script
// through an uncancellable FS would defeat half of that.
//
// Nil: rejected, rather than silently replaced with Background, so a caller
// who meant to pass a context finds out here.
func (v *FSView) WithContext(ctx context.Context) (*FSView, error) {
	if ctx == nil {
		return nil, errors.New("config: WithContext requires a non-nil context")
	}
	return &FSView{loader: v.loader, ctx: ctx}, nil
}

// context returns the view's context, defaulting to Background.
func (v *FSView) context() context.Context {
	if v.ctx == nil {
		return context.Background()
	}
	return v.ctx
}

// Open returns the named file's contents as a read-only fs.File.
//
// The ROOT (".") is the one name that opens as a directory, because
// [fs.WalkDir] and [fs.Glob] open it before doing anything else and an fs.FS
// whose root cannot be opened is malformed.
//
// Every other name must be a file. A directory fallback for other names is
// deliberately absent: List reports an absent directory as an empty list, so
// "list it and see" cannot tell a directory that is empty from one that does
// not exist. Falling back on that would make Open("typo.yaml") return a
// successful empty directory instead of an error — an fs.FS contract violation
// that surfaces as a bizarre failure in whatever wraps this next, rather than
// as the plain "file not found" the caller asked for. Use [FSView.ReadDir] to
// list a directory.
func (v *FSView) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if name == "." {
		return &memDir{name: "."}, nil
	}
	data, err := v.loader.Load(v.context(), name)
	if err != nil {
		return nil, err
	}
	return &memFile{name: path.Base(name), data: data}, nil
}

// ReadFile returns the named file's bytes.
func (v *FSView) ReadFile(name string) ([]byte, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	return v.loader.Load(v.context(), name)
}

// ReadDir lists the regular files directly under name.
//
// Entries carry a name and a "regular file" mode and nothing else. That is
// everything [io/fs.ReadDirFS]'s consumers here need — LoadDir filters on
// IsDir and the extension — and inventing a size or mtime for a row in a
// database would be fabricating detail the backend does not have.
func (v *FSView) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}
	// The ROOT lists empty rather than enumerating the project.
	//
	// Loader has no "list everything" capability — List takes a directory, and
	// validateName rejects both "" and "." — so a root listing could only be
	// fabricated. Empty is the honest answer, and it is enough: every consumer
	// here addresses a named directory (migrations/, scripts/), and the reason
	// the root is openable at all is that fs.WalkDir and fs.Glob start there.
	//
	// The cost is that a WalkDir over this view finds nothing. That is worth
	// stating rather than discovering: use ReadDir on the directory you want.
	if name == "." {
		return nil, nil
	}
	names, err := v.loader.List(v.context(), name)
	if err != nil {
		// A Loader may report an absent directory either way: FSLoader
		// returns an empty list, while returning os.ErrNotExist is the
		// obvious alternative and matches what Load is documented to do.
		// Both must mean "no such directory", because datamigration.LoadDir
		// treats exactly that as an empty chain and fails on anything else —
		// so a backend choosing the other spelling would otherwise fail every
		// project that simply has no migrations. Every OTHER error surfaces.
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	entries := make([]fs.DirEntry, 0, len(names))
	for _, n := range names {
		entries = append(entries, &memDirEntry{name: n})
	}
	return entries, nil
}

// readOnlyMode is the mode a config file reports through this view.
//
// It describes the HANDLE, not the underlying resource: an FSLoader-backed
// migrations/001.yaml is a perfectly writable file that the operator edits in
// an editor. fs.FS has no write surface at all, so no consumer could act on a
// wider mode anyway — 0444 is simply the honest description of what this view
// offers, and inventing permission bits from a backend that may not have any
// would be fabricating detail.
const readOnlyMode fs.FileMode = 0o444

// memFile is a read-only fs.File over an in-memory byte slice.
type memFile struct {
	name string
	data []byte
	off  int
}

func (f *memFile) Stat() (fs.FileInfo, error) {
	return &memFileInfo{name: f.name, size: int64(len(f.data))}, nil
}
func (f *memFile) Close() error { return nil }

func (f *memFile) Read(p []byte) (int, error) {
	if f.off >= len(f.data) {
		return 0, io.EOF
	}
	n := copy(p, f.data[f.off:])
	f.off += n
	return n, nil
}

// memFileInfo is the minimal fs.FileInfo a read-through file can honestly
// report: a name and a size. ModTime is the zero time rather than time.Now(),
// because a made-up timestamp is worse than an obviously absent one — a
// consumer comparing it would silently treat every read as freshly modified.
type memFileInfo struct {
	name string
	size int64
	dir  bool
}

func (i *memFileInfo) Name() string { return i.name }
func (i *memFileInfo) Size() int64  { return i.size }
func (i *memFileInfo) Mode() fs.FileMode {
	if i.dir {
		return fs.ModeDir | readOnlyMode
	}
	return readOnlyMode
}
func (i *memFileInfo) ModTime() time.Time { return time.Time{} }
func (i *memFileInfo) IsDir() bool        { return i.dir }
func (i *memFileInfo) Sys() any           { return nil }

// memDir is a read-only fs.ReadDirFile over an already-listed directory.
type memDir struct {
	name    string
	entries []fs.DirEntry
	off     int
}

func (d *memDir) Stat() (fs.FileInfo, error) {
	return &memFileInfo{name: d.name, dir: true}, nil
}
func (d *memDir) Close() error { return nil }

// Read on a directory is an error, matching os.File.
func (d *memDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.name, Err: fs.ErrInvalid}
}

// ReadDir implements fs.ReadDirFile: n <= 0 returns everything remaining,
// n > 0 returns at most n and reports io.EOF once exhausted.
func (d *memDir) ReadDir(n int) ([]fs.DirEntry, error) {
	remaining := d.entries[d.off:]
	if n <= 0 {
		d.off = len(d.entries)
		return remaining, nil
	}
	if len(remaining) == 0 {
		return nil, io.EOF
	}
	if n > len(remaining) {
		n = len(remaining)
	}
	d.off += n
	return remaining[:n], nil
}

// memDirEntry is a directory entry for a file the Loader listed.
type memDirEntry struct{ name string }

func (e *memDirEntry) Name() string      { return e.name }
func (e *memDirEntry) IsDir() bool       { return false }
func (e *memDirEntry) Type() fs.FileMode { return 0 }

// Info reports the entry's name and mode. It does NOT report a size.
//
// [Loader.List] returns names only, so a size could not be known without
// reading the file. Returning 0 would be a wrong number rather than an absent
// one — the same trap the zero ModTime avoids — so this reports the size as
// unavailable and leaves a caller that needs one to Load the file.
func (e *memDirEntry) Info() (fs.FileInfo, error) {
	return nil, &fs.PathError{Op: "stat", Path: e.name, Err: errSizeUnavailable}
}

// errSizeUnavailable is returned by [memDirEntry.Info]: a listing carries
// names, not sizes.
var errSizeUnavailable = errors.New(
	"config: file metadata is not available from a directory listing; read the file")
