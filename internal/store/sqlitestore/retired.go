package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// successorsBatch caps the ids per Successors query, well under SQLite's
// bound-variable limit. A body's mention list has no cap of its own.
const successorsBatch = 500

// Successors implements [store.RetiredIDs].
func (s *Store) Successors(ctx context.Context, ids []string) (map[string]string, error) {
	out := map[string]string{}
	for batch := range slices.Chunk(ids, successorsBatch) {
		if err := successorsInto(ctx, s, batch, out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func successorsInto(ctx context.Context, s *Store, ids []string, out map[string]string) error {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.q().QueryContext(ctx, `SELECT id, successor FROM retired_ids WHERE id IN (`+
		strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")+`)`, args...)
	if err != nil {
		return fmt.Errorf("sqlitestore: successors: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id, successor string
		if err := rows.Scan(&id, &successor); err != nil {
			return fmt.Errorf("sqlitestore: successors: %w", err)
		}
		out[id] = successor
	}
	return rows.Err()
}

// retireID records that a rename moved the entity at oldID to newID, inside
// the rename's transaction. It refuses newID when another entity retired it
// or when ids retired into newID point at a deleted entity, drops the row
// when the entity is moving back onto an id it retired itself, and re-points
// the ids already retired into oldID so no chain forms.
//
// A rename that changes only the case of the id retires nothing: the old and
// new id are one identity, and a row for the old spelling would collide with
// the next rename's row under the case-folded key. It still re-points, so a
// successor always carries the current spelling.
func retireID(ctx context.Context, s *Store, oldID, newID string) error {
	var successor string
	err := s.q().QueryRowContext(ctx,
		`SELECT successor FROM retired_ids WHERE lower(id) = lower(?)`, newID).Scan(&successor)
	switch {
	case err == nil && !strings.EqualFold(successor, oldID):
		return fmt.Errorf("sqlitestore: rename %s to %s: retired id: %w", oldID, newID, store.ErrConflict)
	case err == nil:
		if _, err = s.write(ctx, `DELETE FROM retired_ids WHERE lower(id) = lower(?)`, newID); err != nil {
			return fmt.Errorf("sqlitestore: rename %s: reclaim %s: %w", oldID, newID, err)
		}
	case !errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("sqlitestore: rename %s: %w", oldID, err)
	}
	if !strings.EqualFold(oldID, newID) {
		held, err := successorTaken(ctx, s, newID)
		if err != nil {
			return fmt.Errorf("sqlitestore: rename %s: %w", oldID, err)
		}
		if held {
			return fmt.Errorf("sqlitestore: rename %s to %s: retired id successor: %w",
				oldID, newID, store.ErrConflict)
		}
	}
	if _, err := s.write(ctx, `UPDATE retired_ids SET successor = ? WHERE successor = ?`, newID, oldID); err != nil {
		return fmt.Errorf("sqlitestore: rename %s: re-point retired ids: %w", oldID, err)
	}
	if strings.EqualFold(oldID, newID) {
		return nil
	}
	if _, err := s.write(ctx, `INSERT INTO retired_ids (id, successor, retired_at) VALUES (?, ?, ?)`,
		oldID, newID, sqlitedb.FormatTime(time.Now())); err != nil {
		return fmt.Errorf("sqlitestore: rename %s: retire: %w", oldID, err)
	}
	return nil
}

// successorTaken reports whether a retired id points at id, case-folded. Such
// an id stays reserved after its entity is deleted, so an unrelated entity
// cannot take over the retired id's mentions.
func successorTaken(ctx context.Context, s *Store, id string) (bool, error) {
	var held bool
	err := s.q().QueryRowContext(ctx,
		`SELECT true FROM retired_ids WHERE lower(successor) = lower(?) LIMIT 1`, id).Scan(&held)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return held, err
}
