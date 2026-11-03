package pgstore

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Successors implements [store.RetiredIDs].
func (s *Store) Successors(ctx context.Context, ids []string) (map[string]string, error) {
	out := map[string]string{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.db.Query(ctx, `SELECT id, successor FROM retired_ids WHERE id = ANY($1::text[])`, ids)
	if err != nil {
		return nil, fmt.Errorf("pgstore: successors: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, successor string
		if err := rows.Scan(&id, &successor); err != nil {
			return nil, fmt.Errorf("pgstore: successors: %w", err)
		}
		out[id] = successor
	}
	return out, rows.Err()
}

// retiredIDTaken reports whether a rename retired id, or whether a retired id
// points at id, case-folded. A successor stays reserved after its entity is
// deleted, so an unrelated entity cannot take over the retired id's mentions.
func retiredIDTaken(ctx context.Context, q DBTX, id string) (bool, error) {
	var held bool
	err := q.QueryRow(ctx, `SELECT true FROM retired_ids
		WHERE lower(id) = lower($1) OR lower(successor) = lower($1) LIMIT 1`, id).Scan(&held)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return held, err
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
//
// The caller holds the family locks of both ids, which CreateEntity takes
// too, so a create cannot slip onto oldID between the re-key and this row.
func retireID(ctx context.Context, tx DBTX, oldID, newID string) error {
	var successor string
	err := tx.QueryRow(ctx,
		`SELECT successor FROM retired_ids WHERE lower(id) = lower($1)`, newID).Scan(&successor)
	switch {
	case err == nil && !strings.EqualFold(successor, oldID):
		return fmt.Errorf("pgstore: rename %s to %s: retired id: %w", oldID, newID, store.ErrConflict)
	case err == nil:
		if _, err = tx.Exec(ctx, `DELETE FROM retired_ids WHERE lower(id) = lower($1)`, newID); err != nil {
			return fmt.Errorf("pgstore: rename %s: reclaim %s: %w", oldID, newID, err)
		}
	case !errors.Is(err, pgx.ErrNoRows):
		return fmt.Errorf("pgstore: rename %s: %w", oldID, err)
	}
	if !strings.EqualFold(oldID, newID) {
		var held bool
		err := tx.QueryRow(ctx,
			`SELECT true FROM retired_ids WHERE lower(successor) = lower($1) LIMIT 1`, newID).Scan(&held)
		switch {
		case err == nil:
			return fmt.Errorf("pgstore: rename %s to %s: retired id successor: %w", oldID, newID, store.ErrConflict)
		case !errors.Is(err, pgx.ErrNoRows):
			return fmt.Errorf("pgstore: rename %s: %w", oldID, err)
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE retired_ids SET successor = $2 WHERE successor = $1`, oldID, newID); err != nil {
		return fmt.Errorf("pgstore: rename %s: re-point retired ids: %w", oldID, err)
	}
	if strings.EqualFold(oldID, newID) {
		return nil
	}
	if _, err := tx.Exec(ctx, `INSERT INTO retired_ids (id, successor) VALUES ($1, $2)`, oldID, newID); err != nil {
		return fmt.Errorf("pgstore: rename %s: retire: %w", oldID, err)
	}
	return nil
}

// lockRenamePair takes the family locks of both ids of a rename, in a fixed
// order so two renames over the same pair cannot deadlock.
func lockRenamePair(ctx context.Context, tx DBTX, oldID, newID string) error {
	first, second := oldID, newID
	if second < first {
		first, second = second, first
	}
	if err := lockFamily(ctx, tx, first); err != nil {
		return err
	}
	if first == second {
		return nil
	}
	return lockFamily(ctx, tx, second)
}
