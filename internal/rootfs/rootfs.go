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
// The first segment of a multi-segment name is opened as a NESTED root, and
// the rest of the name is resolved inside it. That narrower root is
// security-critical: a symlink inside custom/ pointing at ../schema.yaml never
// leaves the project directory, so a single root at the project would follow
// it and serve the file. Only the nested root refuses it. A symlink that
// escapes is an error, never [fs.ErrNotExist], so a layered loader does not
// fall through to another source and hide it.
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
	if !fs.ValidPath(dir) || dir == "." {
		return nil, fmt.Errorf("rootfs: invalid directory name %q", dir)
	}
	root, err := os.OpenRoot(d.path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	sub, err := root.OpenRoot(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = sub.Close() }()
	entries, err := fs.ReadDir(sub.FS(), ".")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Type().IsRegular() {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	return names, nil
}

// Stat returns the file info of name, resolved with the same containment as
// [Dir.Load].
func (d *Dir) Stat(_ context.Context, name string) (fs.FileInfo, error) {
	root, rest, closeRoots, err := d.open(name)
	if err != nil {
		return nil, err
	}
	defer closeRoots()
	return root.Stat(rest)
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

// open validates name and returns the root that contains it, the name
// relative to that root, and a func closing every root it opened.
func (d *Dir) open(name string) (root *os.Root, rest string, closeRoots func(), err error) {
	if !fs.ValidPath(name) || name == "." {
		return nil, "", nil, fmt.Errorf("rootfs: invalid file name %q", name)
	}
	top, err := os.OpenRoot(d.path)
	if err != nil {
		return nil, "", nil, err
	}
	first, rest, nested := strings.Cut(name, "/")
	if !nested {
		return top, name, func() { _ = top.Close() }, nil
	}
	sub, err := top.OpenRoot(first)
	if err != nil {
		_ = top.Close()
		return nil, "", nil, err
	}
	return sub, rest, func() {
		_ = sub.Close()
		_ = top.Close()
	}, nil
}
