// Package config provides read-only access to project-root configuration
// files — the YAML/JSON files users check into their repo alongside
// schema.yaml (data-entry.yaml, schedules.yaml, and so on).
//
// The Loader interface is the swap boundary. FSLoader is the default
// backend; remote or embedded deployments plug in by implementing Loader.
package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// Loader reads a named project-root configuration file.
type Loader interface {
	// Load returns the raw bytes for the named file. Implementations
	// return an os.IsNotExist-compatible error when the file is absent.
	Load(ctx context.Context, name string) ([]byte, error)

	// List returns the file names directly under dir, sorted, without the
	// dir prefix and without subdirectory entries. It is how the
	// directory-shaped parts of a project — scripts/, actions/,
	// validations/, migrations/, templates/, custom/, apps/ — are read
	// through this seam rather than through the filesystem directly.
	//
	// An ABSENT directory lists empty with a nil error; every other
	// failure surfaces, a path that exists but is not a directory
	// included. That asymmetry is deliberate and matches
	// datamigration.LoadDir: a project legitimately has no scripts/, but
	// an unreadable one reported as "nothing here" would silently drop
	// operator-authored config.
	//
	// Only regular files are listed.
	//
	// Non-recursive by design. Every consumer wants one directory's worth
	// of files, and a recursive walk would make the two backends differ:
	// a filesystem has real subdirectories, while a database has flat
	// keys that merely contain slashes.
	List(ctx context.Context, dir string) ([]string, error)
}

// Subscriber is the optional change-notification interface on a Loader.
// Backends that can detect external changes to a named file satisfy
// Subscriber; consumers type-assert to subscribe. Backends with no
// change-detection capability (embedded sources, remote APIs with no
// polling) simply don't implement it.
type Subscriber interface {
	// Subscribe starts a watcher that invokes onChange whenever the named
	// file changes on the underlying source. The returned stop function
	// releases all watcher resources.
	Subscribe(ctx context.Context, name string, onChange func()) (stop func(), err error)
}

// FSLoader serves project config files from a directory on a filesystem.
type FSLoader struct {
	fs   storage.FS
	root string
}

var (
	_ Loader     = (*FSLoader)(nil)
	_ Subscriber = (*FSLoader)(nil)
)

// NewFSLoader constructs a filesystem-backed project-config loader rooted
// at dir (typically the project root).
func NewFSLoader(fs storage.FS, dir string) *FSLoader {
	return &FSLoader{fs: fs, root: dir}
}

// Load reads the bytes of the named file. The name must be a simple
// filename or a relative subdirectory path; traversal is rejected.
func (l *FSLoader) Load(_ context.Context, name string) ([]byte, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	return l.fs.ReadFile(filepath.Join(l.root, name))
}

// List returns the sorted names of the regular files directly under dir.
//
// A missing directory is an empty list, not an error — see the interface
// doc for why only that case is forgiven. The absent check is an explicit
// Stat rather than an errors.Is on ReadDir's error, because the two
// storage.FS implementations disagree there: OsFS reports ENOTDIR for a
// path that exists but is a file, while MemFS reports os.ErrNotExist for
// anything absent from its directory map. Reading the distinction off
// ReadDir would therefore forgive a not-a-directory on MemFS and surface it
// on OsFS — the exact "silently drop operator-authored config" case the
// asymmetry exists to prevent, made dependent on which FS happens to be
// installed.
//
// Entries that are not regular files are skipped, symlinks included. A
// symlink is not a directory, so it would otherwise be listed as an
// ordinary config file while pointing anywhere the process can read —
// .rela/secrets.yaml among them. Containment for the dir argument is
// validateName's job; this is the containment the ENTRIES need, and it is
// the duty this seam inherits from the os.OpenRoot call sites it replaces.
func (l *FSLoader) List(_ context.Context, dir string) ([]string, error) {
	if err := validateName(dir); err != nil {
		return nil, err
	}
	full := filepath.Join(l.root, dir)
	switch info, err := l.fs.Stat(full); {
	case errors.Is(err, os.ErrNotExist):
		return nil, nil
	case err != nil:
		return nil, err
	case !info.IsDir():
		return nil, fmt.Errorf("config: %s is not a directory", dir)
	}

	entries, err := l.fs.ReadDir(full)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		names = append(names, e.Name())
	}
	// storage.FS.ReadDir is documented as sorted, but sort here anyway: the
	// order is part of this method's contract (a script chain's execution
	// order depends on it), so it must not rest on a promise made by
	// whichever FS implementation happens to be installed.
	slices.Sort(names)
	return names, nil
}

// Subscribe watches the named file for changes and invokes onChange for
// each event (after a short debounce).
func (l *FSLoader) Subscribe(_ context.Context, name string, onChange func()) (func(), error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	watcher, err := storage.NewWatcher(storage.WatchConfig{
		Files:      []string{filepath.Join(l.root, name)},
		Debounce:   200 * time.Millisecond,
		SkipHidden: true,
		OnChange: func(_ []storage.ChangeEvent) {
			onChange()
		},
	})
	if err != nil {
		return nil, err
	}
	go watcher.Start()
	return watcher.Stop, nil
}

// validateName applies the same safety checks as state.validateKey —
// project files are attacker-controllable names in some call paths
// (e.g. config filenames passed via flags) and must stay inside root.
func validateName(name string) error {
	if name == "" {
		return errors.New("config: name must not be empty")
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return errors.New("config: control character (including NUL) not allowed")
		}
	}
	if strings.ContainsRune(name, '\\') {
		return errors.New("config: backslash not allowed (use forward slash)")
	}
	if strings.HasPrefix(name, "/") {
		return errors.New("config: name must be relative")
	}
	for seg := range strings.SplitSeq(name, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return errors.New("config: traversal or empty segment not allowed")
		}
	}
	if len(name) >= 2 && name[1] == ':' {
		return errors.New("config: drive letter not allowed")
	}
	return nil
}
