package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// HighestID returns the largest numeric suffix among ids in the "PREFIX-N"
// series, for sequential id generation.
//
// The caller passes the bare prefix ("FEAT"); the separator is this function's
// business, matching memstore. The scan runs in SQL but the parse in Go:
// SQLite's CAST yields 0 for a non-numeric suffix, which would make "FEAT-abc"
// indistinguishable from "FEAT-0".
func (s *Store) HighestID(ctx context.Context, prefix string) (int, error) {
	pfx := prefix + "-"
	rows, err := s.q().QueryContext(ctx,
		`SELECT id FROM entities WHERE id LIKE ? ESCAPE '\'`, likePrefix(pfx))
	if err != nil {
		return 0, fmt.Errorf("sqlitestore: highest id for %q: %w", prefix, err)
	}
	defer rows.Close()

	highest := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("sqlitestore: highest id for %q: %w", prefix, err)
		}
		n, err := strconv.Atoi(id[len(pfx):])
		if err != nil {
			continue // not a sequential id in this series
		}
		if n > highest {
			highest = n
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("sqlitestore: highest id for %q: %w", prefix, err)
	}
	return highest, nil
}

// likePrefix escapes LIKE wildcards so a prefix containing % or _ matches
// literally rather than as a pattern.
func likePrefix(prefix string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(prefix) + "%"
}

// PropertyValues returns the distinct values of a property, most frequent
// first. Counting is done here; the ranking is shared via storeutil.TopValues
// so every backend orders identically.
func (s *Store) PropertyValues(ctx context.Context, property string, limit int) ([]string, error) {
	if err := storeutil.ValidateProperty(property); err != nil {
		return nil, fmt.Errorf("sqlitestore: property values: %w", err)
	}

	// Iterate json_each rather than building a '$.<name>' path.
	//
	// A path has to be CONSTRUCTED as a string, and SQLite's JSON path syntax
	// has its own quoting rules — a property name containing a double quote
	// produces `$."` and a "bad JSON path" error, as the fuzz suite found. The
	// property name would have to be escaped for a second grammar nested inside
	// the SQL string, which is the shape that goes wrong quietly later.
	// json_each yields the keys as VALUES instead, so the name is compared as a
	// bound parameter and needs no escaping at all.
	rows, err := s.q().QueryContext(ctx,
		`SELECT j.value, j.type, count(*)
		 FROM entities, json_each(entities.properties) AS j
		 WHERE j.key = ?
		 GROUP BY j.value, j.type`, property)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: property values for %q: %w", property, err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var (
			value    sql.NullString
			jsonType string
			n        int
		)
		if err := rows.Scan(&value, &jsonType, &n); err != nil {
			return nil, fmt.Errorf("sqlitestore: property values for %q: %w", property, err)
		}
		// Skip arrays and objects: a composite has no single scalar value to
		// count, matching what the in-memory backends do. NULL and empty are
		// skipped as "no value set".
		if jsonType == "array" || jsonType == "object" || !value.Valid || value.String == "" {
			continue
		}
		counts[value.String] += n
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlitestore: property values for %q: %w", property, err)
	}
	return storeutil.TopValues(counts, limit), nil
}

// RenameEntity changes an entity's id and re-keys every relation referencing
// it, atomically.
//
// Atomicity is the whole point: a rename that updated the entity and then
// failed mid-way through the relations would leave edges pointing at an id
// that no longer exists. Running inside a transaction — and emitting the
// rename event only after it commits — is what makes the operation safe to
// interrupt.
func (s *Store) RenameEntity(ctx context.Context, oldID, newID string) (*store.RenameResult, error) {
	if err := storeutil.ValidateID(newID); err != nil {
		return nil, fmt.Errorf("sqlitestore: rename to %q: %w", newID, err)
	}

	var result store.RenameResult
	rename := func(tx store.Store) error {
		view, ok := tx.(*Store)
		if !ok { // unreachable: Tx always hands back our own view type
			return errors.New("sqlitestore: unexpected transaction view type")
		}
		return view.renameLocked(ctx, oldID, newID, &result)
	}

	// A nested call joins the caller's transaction rather than opening a
	// second one, so RenameEntity is safe to call from inside a Tx.
	if err := s.Tx(ctx, rename); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *Store) renameLocked(
	ctx context.Context, oldID, newID string, result *store.RenameResult,
) error {
	existing, err := s.GetEntity(ctx, oldID)
	if err != nil {
		return err
	}

	// Check the target explicitly rather than relying on the PRIMARY KEY
	// violation: a UNIQUE error would have to be string-matched to be
	// distinguished from any other constraint failure.
	// Compare folded: IDs are case-insensitive identities, so renaming "other"
	// to "ABC" must conflict when "abc" exists.
	var taken int
	if err := s.q().QueryRowContext(ctx,
		`SELECT count(*) FROM entities WHERE lower(id) = lower(?) AND lower(id) != lower(?)`,
		newID, oldID).Scan(&taken); err != nil {
		return fmt.Errorf("sqlitestore: rename %s: %w", oldID, err)
	}
	if taken > 0 {
		return fmt.Errorf("sqlitestore: rename %s to %s: %w", oldID, newID, store.ErrConflict)
	}

	if _, err := s.write(ctx, `UPDATE entities SET id = ? WHERE id = ?`, newID, oldID); err != nil {
		return fmt.Errorf("sqlitestore: rename %s: %w", oldID, err)
	}

	// Bulk in-place re-key, matching pgstore: one statement per direction
	// rather than delete-and-recreate, so relation identity survives.
	fromRes, ferr := s.write(ctx, `UPDATE relations SET from_id = ? WHERE from_id = ?`, newID, oldID)
	if ferr != nil {
		return fmt.Errorf("sqlitestore: rename %s relations (from): %w", oldID, ferr)
	}
	toRes, terr := s.write(ctx, `UPDATE relations SET to_id = ? WHERE to_id = ?`, newID, oldID)
	if terr != nil {
		return fmt.Errorf("sqlitestore: rename %s relations (to): %w", oldID, terr)
	}
	if _, err := s.write(ctx,
		`UPDATE attachments SET entity_id = ? WHERE entity_id = ?`, newID, oldID); err != nil {
		return fmt.Errorf("sqlitestore: rename %s attachments: %w", oldID, err)
	}

	fromN, _ := fromRes.RowsAffected()
	toN, _ := toRes.RowsAffected()
	result.RelationsUpdated = int(fromN + toN)

	renamed := existing.Clone()
	renamed.ID = newID
	s.notifyRenamed(oldID, renamed)

	// One EventEntityUpdated under the NEW id, matching memstore.
	//
	// There is deliberately no rename op in store.EventOp: the Event stream is
	// a coarse staleness signal, and a consumer re-snapshots by id either way.
	// Precise old→new re-keying is the job of store.EntityObserver.EntityRenamed,
	// a separate callback this backend does not yet implement (no derived index
	// is wired to it here — search runs off bleve via the generic wrapper).
	// Emitting delete+put instead would make a consumer drop and rebuild state
	// it could have re-keyed.
	s.emit(store.Event{
		Op:         store.EventEntityUpdated,
		EntityID:   newID,
		EntityType: existing.Type,
	})
	return nil
}
