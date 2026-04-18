// Package config provides read-only access to project-root configuration
// files — the YAML/JSON files users check into their repo alongside
// metamodel.yaml (data-entry.yaml, schedules.yaml, and so on).
//
// The Loader interface is the swap boundary. FSLoader is the default
// backend; remote or embedded deployments plug in by implementing Loader.
package config

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// Loader reads a named project-root configuration file.
type Loader interface {
	// Load returns the raw bytes for the named file. Implementations
	// return an os.IsNotExist-compatible error when the file is absent.
	Load(ctx context.Context, name string) ([]byte, error)
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
	for _, seg := range strings.Split(name, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return errors.New("config: traversal or empty segment not allowed")
		}
	}
	if len(name) >= 2 && name[1] == ':' {
		return errors.New("config: drive letter not allowed")
	}
	return nil
}
