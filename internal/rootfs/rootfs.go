// Package rootfs reads operator-authored project files — scripts/, actions/,
// validations/, custom/, apps/ — from a directory on disk, contained with
// [os.Root].
//
// It exists so that every reader of those files shares one containment
// implementation instead of repeating the os.OpenRoot dance at each call
// site, and so that the readers can take a loader interface: a project whose
// files live in its database (FEAT-UP14BT) is served by the same consumers,
// layered behind this one.
//
// Its only internal dependency is storage (for the file watcher), so that
// lua, script and validation, which may not import internal/config, can still
// fall back to it. [Dir] satisfies config.Loader and config.Subscriber
// structurally.
package rootfs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// Dir reads files under one directory on disk.
//
// # Containment
//
// Every directory on the way to a file is opened as its own NESTED root, so a
// symlink resolves only within the directory that holds it. That is what
// keeps a symlink in custom/ pointing at ../schema.yaml from being served,
// and a symlink in apps/a/ from reading apps/b/: a single root at the project
// would follow both. A symlink that escapes is an error, never
// [fs.ErrNotExist], so a layered loader does not fall through to another
// source and hide it.
//
// Nil: [New] never returns nil; a nil *Dir is a programming error.
type Dir struct {
	path string
}

// New returns a Dir reading under path.
func New(path string) *Dir { return &Dir{path: path} }

// Load returns the bytes of name, a slash-separated path relative to the
// directory. A missing file or directory is reported as [fs.ErrNotExist].
func (d *Dir) Load(_ context.Context, name string) ([]byte, error) {
	root, rest, closeRoots, err := d.open(name)
	if err != nil {
		return nil, err
	}
	defer closeRoots()
	info, err := root.Stat(rest)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("rootfs: %s is a directory", name)
	}
	return root.ReadFile(rest)
}

// List returns the sorted names of the regular files directly under dir. An
// absent directory lists empty with a nil error, matching config.Loader.
// Symlinks are not listed: whatever they point at is not a regular file of
// this directory.
func (d *Dir) List(_ context.Context, dir string) ([]string, error) {
	return d.entries(dir, func(e fs.DirEntry) bool { return e.Type().IsRegular() })
}

// Dirs returns the sorted names of the subdirectories directly under dir,
// symlinks excluded. An absent directory lists empty with a nil error.
func (d *Dir) Dirs(_ context.Context, dir string) ([]string, error) {
	return d.entries(dir, func(e fs.DirEntry) bool { return e.IsDir() })
}

func (d *Dir) entries(dir string, keep func(fs.DirEntry) bool) ([]string, error) {
	if !fs.ValidPath(dir) || dir == "." {
		return nil, fmt.Errorf("rootfs: invalid directory name %q", dir)
	}
	root, closeRoots, err := d.openDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer closeRoots()
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if keep(e) {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	return names, nil
}

// Stat returns the file info of name, resolved with the same containment as
// [Dir.Load]. It opens the file and stats the handle, so a file Load could
// not read (no read permission) fails here too: a caller that checks with
// Stat before serving must not promise a file that then 404s.
func (d *Dir) Stat(_ context.Context, name string) (fs.FileInfo, error) {
	root, base, closeRoots, err := d.open(name)
	if err != nil {
		return nil, err
	}
	defer closeRoots()
	f, err := root.Open(base)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return f.Stat()
}

// Subscribe watches the named file and calls onChange after each change,
// debounced. It satisfies config.Subscriber, which is how data-entry.yaml is
// reloaded live.
func (d *Dir) Subscribe(_ context.Context, name string, onChange func()) (func(), error) {
	if !fs.ValidPath(name) || name == "." {
		return nil, fmt.Errorf("rootfs: invalid file name %q", name)
	}
	watcher, err := storage.NewWatcher(storage.WatchConfig{
		Files:      []string{filepath.Join(d.path, filepath.FromSlash(name))},
		Debounce:   watchDebounce,
		SkipHidden: true,
		OnChange:   func([]storage.ChangeEvent) { onChange() },
	})
	if err != nil { // coverage-ignore: fsnotify.NewWatcher fails only on OS watch-descriptor exhaustion
		return nil, err
	}
	go watcher.Start()
	return watcher.Stop, nil
}

// watchDebounce matches config.FSLoader, so a reload behaves the same
// whichever loader serves the file.
const watchDebounce = 200 * time.Millisecond

// open validates name and returns the root of the directory holding it,
// the file's base name, and a func closing every root it opened.
func (d *Dir) open(name string) (root *os.Root, base string, closeRoots func(), err error) {
	if !fs.ValidPath(name) || name == "." {
		return nil, "", nil, fmt.Errorf("rootfs: invalid file name %q", name)
	}
	dir, base := path.Split(name)
	root, closeRoots, err = d.openDir(strings.TrimSuffix(dir, "/"))
	if err != nil {
		return nil, "", nil, err
	}
	return root, base, closeRoots, nil
}

// openDir opens dir ("" for the directory itself) one nested root per
// component; see the type comment.
func (d *Dir) openDir(dir string) (*os.Root, func(), error) {
	top, err := os.OpenRoot(d.path)
	if err != nil {
		return nil, nil, err
	}
	roots := []*os.Root{top}
	closeAll := func() {
		for i := len(roots) - 1; i >= 0; i-- {
			_ = roots[i].Close()
		}
	}
	if dir == "" {
		return top, closeAll, nil
	}
	for part := range strings.SplitSeq(dir, "/") {
		next, err := roots[len(roots)-1].OpenRoot(part)
		if err != nil {
			closeAll()
			return nil, nil, err
		}
		roots = append(roots, next)
	}
	return roots[len(roots)-1], closeAll, nil
}
