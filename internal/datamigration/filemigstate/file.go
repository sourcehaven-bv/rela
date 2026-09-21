// Package filemigstate stores migration state as JSON at
// `migrations/applied.json` inside the project directory.
//
// # Why this file is committed
//
// It is deliberately NOT under `.rela/`, which is gitignored. On the
// filesystem tier the entities, relations and `migrations/` are all tracked by
// git, so keeping the record of which migrations have run outside git would
// put the data and its position pointer in different trust domains: a fresh
// clone would carry every migration file and no idea which had already run.
// Committing it is what Rails does with `db/schema.rb` and EF Core with its
// model snapshot, for the same reason and at the same price — a generated file
// that conflicts on merge.
//
// That conflict is a feature rather than a cost. Two branches that each add a
// migration both append here, so git raises the collision and a human resolves
// the order, instead of both merging cleanly into an undefined order. Atlas
// makes the same argument for its `atlas.sum`.
//
// It is also deliberately not a dotfile: it is a reviewed artifact like
// `schema.yaml`, and a hidden file is one a reviewer skims past.
//
// # Durability
//
// Writes go through [storage.SafeFS] (temp file, fsync, rename, fsync parent),
// so a crash mid-write leaves the previous state rather than a truncated file.
// A torn write here would lose the whole applied list, which on the next run
// means re-applying every migration.
package filemigstate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// StateFile is the committed applied-list, relative to the project root.
const StateFile = datamigration.MigrationsDir + "/applied.json"

// filePerm matches the 0644 fsstore writes for entity markdown: this file is
// no more sensitive than the migrations it indexes, and both are committed.
const filePerm = 0o644

// Store is a file-backed [datamigration.StateStore].
type Store struct {
	fs *storage.RootedFS
	// mu serializes read-modify-write against this handle. Cross-process
	// serialization is the migration lock's job (TKT-CPCBR7), which every
	// writer already takes; this only prevents two goroutines in one process
	// from interleaving.
	mu sync.Mutex
}

// New returns a store rooted at the project directory.
//
// Nil: rejected — a nil filesystem or empty root is a wiring error, and
// substituting a no-op would silently discard every migration record.
func New(base storage.FS, root string) (*Store, error) {
	if base == nil {
		return nil, errors.New("filemigstate: New requires a filesystem")
	}
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("filemigstate: New requires a root directory")
	}
	// SafeFS first, then root it: RootedFS delegates writes to the FS it
	// wraps, so this order gives containment OVER atomic writes. The reverse
	// would give atomic writes over an uncontained filesystem.
	rooted, err := storage.NewRootedFS(storage.NewSafeFS(base), root)
	if err != nil {
		return nil, fmt.Errorf("filemigstate: rooting at %q: %w", root, err)
	}
	return &Store{fs: rooted}, nil
}

// Load reads the applied-list, or returns (nil, nil) when the file is absent.
//
// A file that exists but does not parse is an ERROR, not an absence. The
// distinction is load-bearing: treating corruption as "no state" would
// re-baseline the store against the live schema and strand every pending
// migration, converting a recoverable bad edit into silent data drift. An
// operator who genuinely wants to start over deletes the file.
func (s *Store) Load(_ context.Context) (*datamigration.State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.fs.ReadFile(StateFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil //nolint:nilnil // (nil, nil) = un-bootstrapped, the documented StateStore contract
		}
		return nil, fmt.Errorf("filemigstate: read %s: %w", StateFile, err)
	}

	dec := json.NewDecoder(strings.NewReader(string(data)))
	// Unknown fields are refused so a typo in a hand-edit is reported rather
	// than dropped — silently ignoring `appliedd` would look like a store that
	// had run nothing.
	dec.DisallowUnknownFields()
	var st datamigration.State
	if err := dec.Decode(&st); err != nil {
		return nil, fmt.Errorf("filemigstate: %s is not valid migration state: %w", StateFile, err)
	}
	if err := datamigration.ValidateState(&st); err != nil {
		return nil, fmt.Errorf("filemigstate: %s: %w", StateFile, err)
	}
	return &st, nil
}

// Save writes the applied-list, creating `migrations/` if needed.
//
// The JSON is indented because this file is committed and read in diffs: a
// one-line blob would make every change an unreviewable single-line diff. The
// trailing newline is for the same reason.
func (s *Store) Save(_ context.Context, st *datamigration.State) error {
	if st == nil {
		return errors.New("filemigstate: Save requires a state")
	}
	if err := datamigration.ValidateState(st); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("filemigstate: marshal state: %w", err)
	}
	data = append(data, '\n')

	if err := s.fs.MkdirAll(path.Dir(StateFile), 0o755); err != nil {
		return fmt.Errorf("filemigstate: create %s: %w", path.Dir(StateFile), err)
	}
	if err := s.fs.WriteFile(StateFile, data, filePerm); err != nil {
		return fmt.Errorf("filemigstate: write %s: %w", StateFile, err)
	}
	return nil
}

// Interface check.
var _ datamigration.StateStore = (*Store)(nil)
