// Package sqlitemigstate stores the data-migration record in the SQLite
// database rather than in a file beside it.
//
// The reasoning is content versioning's (TKT-4NU9ZD), not state.KV's. SQLite
// is single-process, so the multi-node argument that moved runtime state into
// postgres does not apply here. What applies is that `rela.db` is meant to be
// shippable on its own: a record sitting beside the file would be left behind,
// and the receiving copy would claim no migration had ever run — so the next
// run would replay every one of them against already-migrated content.
//
// "Is this about the machine or about the content?" is the question, and the
// migration record is squarely about the content.
package sqlitemigstate

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/datamigration"
)

// Store is a SQLite-backed [datamigration.StateStore].
//
// One row, pinned by the table's id CHECK: there is exactly one store per
// database file, so the record has no key to vary on.
type Store struct {
	db *sql.DB
}

// New returns a Store over db.
//
// The caller owns db: this borrows the handle and never closes it.
//
// Nil: rejected. A no-op store would report every migration un-run and replay
// the chain on each start.
func New(db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, errors.New("sqlitemigstate: a non-nil database handle is required")
	}
	return &Store{db: db}, nil
}

// Load returns the recorded state, or (nil, nil) when none is stored.
//
// Malformed JSON is an ERROR rather than an absence: absence means
// "un-bootstrapped", which baselines the store against the live schema and
// marks every pending migration applied. A row that exists but cannot be read
// is a condition an operator must see, not one to paper over.
func (s *Store) Load(ctx context.Context) (*datamigration.State, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT state FROM migration_state WHERE id = 1`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // (nil, nil) = un-bootstrapped, the documented StateStore contract
	}
	if err != nil {
		return nil, fmt.Errorf("sqlitemigstate: read migration state: %w", err)
	}

	var st datamigration.State
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		return nil, fmt.Errorf("sqlitemigstate: stored migration state is not valid JSON: %w", err)
	}
	if err := datamigration.ValidateState(&st); err != nil {
		return nil, fmt.Errorf("sqlitemigstate: %w", err)
	}
	return &st, nil
}

// Save replaces the recorded state.
func (s *Store) Save(ctx context.Context, st *datamigration.State) error {
	if st == nil {
		return errors.New("sqlitemigstate: Save requires a state")
	}
	if err := datamigration.ValidateState(st); err != nil {
		return err
	}
	data, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("sqlitemigstate: marshal state: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO migration_state (id, state) VALUES (1, ?)
		ON CONFLICT(id) DO UPDATE SET state = excluded.state`, string(data))
	if err != nil {
		return fmt.Errorf("sqlitemigstate: write migration state: %w", err)
	}
	return nil
}

// Interface check.
var _ datamigration.StateStore = (*Store)(nil)
