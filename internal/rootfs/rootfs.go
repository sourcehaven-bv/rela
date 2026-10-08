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
	"io"
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
// A file below the top level is read through an [os.Root] opened at its
// AREA: the top-level directory holding it (scripts/, custom/, ...), or the
// app's own directory for apps/<id>/. Each directory on the way to that root
// is itself opened as a nested root, so a symlinked scripts/ cannot point
// outside the project. Within an area, a symlink may point anywhere inside
// that area and nowhere else: a symlink in custom/ to ../schema.yaml is not
// served, and one in apps/a/ cannot read apps/b/. A symlink that escapes is
// an error, never [fs.ErrNotExist], so a layered loader does not fall
// through to another source and hide it.
//
// A top-level file (schema.yaml, data-entry.yaml, ...) is read as the
// filesystem loader always read it, following symlinks: an operator may
// share one data-entry.yaml between projects.
//
// Only regular files are read, so a FIFO cannot block a request.
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
	f, err := d.openFile(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}

// Open opens name for reading, with the same containment as [Dir.Load]. The
// file is an [*os.File], so it seeks and reads at offsets: a server can
// stream it rather than buffer it.
func (d *Dir) Open(_ context.Context, name string) (fs.File, error) {
	return d.openFile(name)
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
	parts := strings.Split(dir, "/")
	root, closeRoots, err := d.openArea(parts[:areaDepth(parts, true)])
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer closeRoots()
	rest := path.Join(parts[areaDepth(parts, true):]...)
	if rest == "" {
		rest = "."
	}
	entries, err := fs.ReadDir(root.FS(), rest)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
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
	f, err := d.openFile(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return f.Stat()
}

// Subscribe watches the named file and calls onChange after each change,
// debounced. It satisfies config.Subscriber, which is how data-entry.yaml is
// reloaded live.
//
// It watches the file's directory and filters on the name, because a watch
// on the file itself cannot be placed while the file is absent. That is the
// normal state of a project whose config lives in its database: a file
// created beside it later must still be noticed, and so must a file that is
// removed (the layered loader then falls back to the stored copy).
func (d *Dir) Subscribe(_ context.Context, name string, onChange func()) (func(), error) {
	if !fs.ValidPath(name) || name == "." {
		return nil, fmt.Errorf("rootfs: invalid file name %q", name)
	}
	target := filepath.Join(d.path, filepath.FromSlash(name))
	watcher, err := storage.NewWatcher(storage.WatchConfig{
		Files:      []string{filepath.Dir(target)},
		Debounce:   watchDebounce,
		SkipHidden: true,
		OnChange: func(events []storage.ChangeEvent) {
			for _, ev := range events {
				if filepath.Clean(ev.Path) == target {
					onChange()
					return
				}
			}
		},
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

// appsDir is the one area whose containment is a level deeper: each app
// is its own root, so one app cannot read another's files.
const appsDir = "apps"

// areaDepth is how many leading components of parts form the root that
// contains them; see [Dir]. For a file the last component is never part of
// it, so a top-level file has depth 0.
func areaDepth(parts []string, isDir bool) int {
	depth := 1
	if parts[0] == appsDir {
		depth = 2
	}
	most := len(parts)
	if !isDir {
		most--
	}
	return min(depth, most)
}

// openFile validates name and opens it for reading, refusing anything but
// a regular file. The roots opened on the way are closed before it returns:
// an open file stays readable without them.
func (d *Dir) openFile(name string) (*os.File, error) {
	if !fs.ValidPath(name) || name == "." {
		return nil, fmt.Errorf("rootfs: invalid file name %q", name)
	}
	parts := strings.Split(name, "/")
	depth := areaDepth(parts, false)
	if depth == 0 {
		// One path component, so fs.ValidPath already rules out "..". The
		// check states it where the path is built.
		if strings.Contains(name, "..") {
			return nil, fmt.Errorf("rootfs: invalid file name %q", name)
		}
		full := filepath.Join(d.path, name)
		info, err := os.Stat(full)
		if err != nil {
			return nil, err
		}
		if err := requireRegular(name, info); err != nil {
			return nil, err
		}
		return os.Open(full)
	}
	root, closeRoots, err := d.openArea(parts[:depth])
	if err != nil {
		return nil, err
	}
	defer closeRoots()
	rest := path.Join(parts[depth:]...)
	info, err := root.Stat(rest)
	if err != nil {
		return nil, err
	}
	if err := requireRegular(name, info); err != nil {
		return nil, err
	}
	return root.Open(rest)
}

// requireRegular refuses anything but a regular file. Checked before
// opening: opening a FIFO blocks until a writer appears.
func requireRegular(name string, info fs.FileInfo) error {
	if !info.Mode().IsRegular() {
		return fmt.Errorf("rootfs: %s is not a regular file", name)
	}
	return nil
}

// openArea opens the directory made of parts as a chain of nested roots,
// one per component, starting at the project directory. With no parts it
// returns the project directory's own root.
func (d *Dir) openArea(parts []string) (*os.Root, func(), error) {
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
	for _, part := range parts {
		next, err := roots[len(roots)-1].OpenRoot(part)
		if err != nil {
			closeAll()
			return nil, nil, err
		}
		roots = append(roots, next)
	}
	return roots[len(roots)-1], closeAll, nil
}
