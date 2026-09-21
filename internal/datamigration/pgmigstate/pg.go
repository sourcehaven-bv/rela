// Package pgmigstate stores the data-migration record in the tenant's
// PostgreSQL schema.
//
// The record says which migrations have run against a store's content and what
// schema shape that content conforms to, so it has to live with the content.
// On this backend that means the tenant's schema: docs/postgres-backend.md
// documents that each tenant tracks its own shape and that tenants at
// different points migrate independently, which a single shared file could not
// express.
//
// Like the other database-backed subsystems here, this takes an INJECTED
// handle — the same pool the store and search backend run on — and declares
// its own narrow DBTX rather than importing internal/store. A migration record
// is not graph content, and this package has no reason to know a store exists.
package pgmigstate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Sourcehaven-BV/rela/internal/datamigration"
)

// DBTX is the slice of a pgx pool this package uses.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Store is a PostgreSQL-backed [datamigration.StateStore].
type Store struct {
	db DBTX
}

// New returns a Store over db.
//
// Nil: rejected. A no-op store would report every migration un-run and replay
// the chain on each start.
func New(db DBTX) (*Store, error) {
	if db == nil {
		return nil, errors.New("pgmigstate: a non-nil database handle is required")
	}
	return &Store{db: db}, nil
}

// Load returns the recorded state, or (nil, nil) when none is stored.
//
// Malformed JSON is an ERROR rather than an absence: absence means
// "un-bootstrapped", which baselines the store against the live schema and
// marks every pending migration applied.
func (s *Store) Load(ctx context.Context) (*datamigration.State, error) {
	var raw []byte
	err := s.db.QueryRow(ctx, `SELECT state FROM migration_state WHERE id = 1`).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // (nil, nil) = un-bootstrapped, the documented StateStore contract
	}
	if err != nil {
		return nil, fmt.Errorf("pgmigstate: read migration state: %w", err)
	}

	var st datamigration.State
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, fmt.Errorf("pgmigstate: stored migration state is not valid JSON: %w", err)
	}
	if err := datamigration.ValidateState(&st); err != nil {
		return nil, fmt.Errorf("pgmigstate: %w", err)
	}
	return &st, nil
}

// Save replaces the recorded state.
func (s *Store) Save(ctx context.Context, st *datamigration.State) error {
	if st == nil {
		return errors.New("pgmigstate: Save requires a state")
	}
	if err := datamigration.ValidateState(st); err != nil {
		return err
	}
	data, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("pgmigstate: marshal state: %w", err)
	}
	_, err = s.db.Exec(ctx, `
		INSERT INTO migration_state (id, state) VALUES (1, $1)
		ON CONFLICT (id) DO UPDATE SET state = excluded.state`, data)
	if err != nil {
		return fmt.Errorf("pgmigstate: write migration state: %w", err)
	}
	return nil
}

// Interface check.
var _ datamigration.StateStore = (*Store)(nil)
