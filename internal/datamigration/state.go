package datamigration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// StateFormatVersion is the schema version of the persisted [State].
//
// It exists so a reader can tell "no state yet" from "state written by a newer
// rela" — two conditions that look identical to a reader that simply fails to
// parse, and whose right responses are opposites: bootstrap a baseline versus
// refuse and let the operator upgrade. Without it, a downgrade re-baselines a
// store silently and every pending migration becomes unreachable.
const StateFormatVersion = 1

// AppliedEntry records one migration that has run against a store.
type AppliedEntry struct {
	// Name is the migration filename, validated by [ParseMigrationName].
	Name string `json:"name"`
	// AppliedAt is when the migration completed, UTC.
	AppliedAt time.Time `json:"applied_at"`
}

// State is the per-store record of which migrations have run and what shape
// the store's content therefore conforms to.
//
// It replaces the old state.KV marker (TKT-XCJ0Y2). Two fields, two jobs:
//
//   - Applied is the POSITION — the list of migration names already run. It
//     alone decides what [Resolve] returns, which is what makes a data-only
//     migration expressible: a file with no shape change still runs, because
//     running is keyed on the name being absent, not on a shape edge existing.
//   - Projection is the SHAPE the data conforms to. The gate diffs it against
//     the live schema to classify a change, and `rela migrate gen` diffs it to
//     draft one. A hash alone cannot serve either — you cannot recover a
//     property list from a digest — which is why the full projection is stored.
type State struct {
	// FormatVersion is [StateFormatVersion] as written. A reader that finds a
	// higher value must refuse rather than guess.
	FormatVersion int `json:"format_version"`
	// Applied lists the migrations already run, in application order.
	Applied []AppliedEntry `json:"applied,omitempty"`
	// Projection is the ShapeProjection JSON ([metamodel.ShapeProjection.JSON]).
	Projection json.RawMessage `json:"projection"`
	// UpdatedAt is when this state was last written, UTC.
	UpdatedAt time.Time `json:"updated_at"`
}

// StateStore persists a single store's [State].
//
// One implementation per storage tier, selected by the composition root the
// same way `comments.Store` is (TKT-OGTVJW): a file under the project's
// `migrations/` on the filesystem tier, a row in the tenant's schema on
// postgres, a row in `rela.db` on sqlite. The split exists because migration
// state describes the DATA, so it has to travel with the data — a postgres
// tenant's position cannot live in a file shared by every tenant on the box,
// and a sqlite project's position cannot live beside the database file that is
// meant to be shippable on its own.
//
// Implementations must pass migstatetest.RunAll.
//
// Nil: an implementation's constructor rejects a nil handle rather than
// substituting a no-op, so a misconfigured backend fails at wiring instead of
// silently forgetting every migration it runs.
type StateStore interface {
	// Load returns the store's state, or (nil, nil) when none has been
	// recorded yet — an un-bootstrapped store, which callers handle rather
	// than treat as an error.
	Load(ctx context.Context) (*State, error)

	// Save replaces the recorded state.
	Save(ctx context.Context, s *State) error
}

// ShapeProjection parses the state's stored projection.
func (s *State) ShapeProjection() (metamodel.ShapeProjection, error) {
	return metamodel.ShapeProjectionFromJSON(s.Projection)
}

// AppliedNames returns the applied migration names in order.
func (s *State) AppliedNames() []string {
	if s == nil {
		return nil
	}
	out := make([]string, 0, len(s.Applied))
	for _, e := range s.Applied {
		out = append(out, e.Name)
	}
	return out
}

// HasApplied reports whether the named migration has already run.
func (s *State) HasApplied(name string) bool {
	if s == nil {
		return false
	}
	return slices.ContainsFunc(s.Applied, func(e AppliedEntry) bool { return e.Name == name })
}

// WithApplied returns a copy of s recording that name ran at now, and moving
// the conformed shape to proj.
//
// Re-recording a name already present is a no-op on the list (the timestamp of
// the first run is kept): a re-run is the documented crash-recovery path, and
// it should not look like a second migration in the record.
func (s *State) WithApplied(name string, proj metamodel.ShapeProjection, now time.Time) (*State, error) {
	next, err := NewState(proj, nil, now)
	if err != nil {
		return nil, err
	}
	if s != nil {
		next.Applied = slices.Clone(s.Applied)
	}
	if !next.HasApplied(name) {
		next.Applied = append(next.Applied, AppliedEntry{Name: name, AppliedAt: now.UTC()})
	}
	return next, nil
}

// NewState builds a state for the given projection at the current time.
func NewState(proj metamodel.ShapeProjection, applied []AppliedEntry, now time.Time) (*State, error) {
	projJSON, err := proj.JSON()
	if err != nil {
		return nil, fmt.Errorf("datamigration: marshal projection: %w", err)
	}
	return &State{
		FormatVersion: StateFormatVersion,
		Applied:       slices.Clone(applied),
		Projection:    projJSON,
		UpdatedAt:     now.UTC(),
	}, nil
}

// StateFromFutureError reports a state written by a newer rela than this one.
//
// Returned rather than swallowed because the alternatives are both wrong: a
// reader that treats it as absent re-baselines the store and strands every
// pending migration, and one that treats it as corrupt invites the operator to
// delete it. Upgrading is the fix, so the error says so.
type StateFromFutureError struct {
	Found int
	Known int
}

func (e *StateFromFutureError) Error() string {
	return fmt.Sprintf(
		"datamigration: migration state is format version %d but this rela understands %d — upgrade rela",
		e.Found, e.Known)
}

// ValidateState checks a freshly decoded state: the format version must be one
// this build understands, and every recorded name must satisfy the allowlist.
//
// Name validation happens HERE, on the way in from storage, because the
// applied-list is compared against directory entries and its names can reach a
// path join. Validating at the point of use instead would leave each caller to
// remember.
func ValidateState(s *State) error {
	if s == nil {
		return nil
	}
	if s.FormatVersion > StateFormatVersion {
		return &StateFromFutureError{Found: s.FormatVersion, Known: StateFormatVersion}
	}
	for _, e := range s.Applied {
		if _, err := ParseMigrationName(e.Name); err != nil {
			return fmt.Errorf("datamigration: applied-list entry: %w", err)
		}
	}
	if len(s.Projection) == 0 {
		return errors.New("datamigration: migration state has no projection")
	}
	return nil
}
