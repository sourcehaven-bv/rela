package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// --- EntityReader ---------------------------------------------------------

func (s *Store) GetEntity(ctx context.Context, id string) (*entity.Entity, error) {
	row := s.q().QueryRowContext(ctx,
		`SELECT id, type, properties, content, updated_at FROM entities WHERE id = ?`, id)
	e, err := scanEntity(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("sqlitestore: get %s: %w", id, store.ErrNotFound)
	}
	return e, err
}

func (s *Store) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		var (
			conds []string
			args  []any
		)
		if q.Type != "" {
			conds = append(conds, "type = ?")
			args = append(args, q.Type)
		}
		if len(q.IDs) > 0 {
			conds = append(conds, "id IN ("+placeholders(len(q.IDs))+")")
			for _, id := range q.IDs {
				args = append(args, id)
			}
		}
		sqlText := `SELECT id, type, properties, content, updated_at FROM entities`
		if len(conds) > 0 {
			sqlText += ` WHERE ` + strings.Join(conds, " AND ")
		}
		// Stable ordering is part of the contract — cursors depend on it.
		sqlText += ` ORDER BY id`

		rows, err := s.q().QueryContext(ctx, sqlText, args...)
		if err != nil {
			yield(nil, fmt.Errorf("sqlitestore: list entities: %w", err))
			return
		}
		defer rows.Close()

		for rows.Next() {
			e, err := scanEntity(rows)
			if !yield(e, err) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(nil, fmt.Errorf("sqlitestore: list entities: %w", err))
		}
	}
}

func (s *Store) CountEntities(ctx context.Context, q store.EntityQuery) (int, error) {
	var (
		conds []string
		args  []any
	)
	if q.Type != "" {
		conds = append(conds, "type = ?")
		args = append(args, q.Type)
	}
	if len(q.IDs) > 0 {
		conds = append(conds, "id IN ("+placeholders(len(q.IDs))+")")
		for _, id := range q.IDs {
			args = append(args, id)
		}
	}
	sqlText := `SELECT count(*) FROM entities`
	if len(conds) > 0 {
		sqlText += ` WHERE ` + strings.Join(conds, " AND ")
	}
	var n int
	if err := s.q().QueryRowContext(ctx, sqlText, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("sqlitestore: count entities: %w", err)
	}
	return n, nil
}

// --- EntityWriter ---------------------------------------------------------

// CreateEntity returns store.ErrConflict when the ID is taken. The stress
// test's plain writer tolerates ONLY ErrConflict here, so the mapping from
// SQLite's UNIQUE violation must be exact.
func (s *Store) CreateEntity(ctx context.Context, e *entity.Entity) error {
	if err := storeutil.ValidateID(e.ID); err != nil {
		return fmt.Errorf("sqlitestore: create: %w", err)
	}
	props, err := marshalProps(e.Properties)
	if err != nil {
		return err
	}
	updated := e.UpdatedAt
	if updated.IsZero() {
		updated = time.Now().UTC()
	}

	_, err = s.write(ctx, `INSERT INTO entities (id, type, properties, content, updated_at)
		VALUES (?, ?, ?, ?, ?)`, e.ID, e.Type, props, e.Content, updated.Format(timeFmt))
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("sqlitestore: create %s: %w", e.ID, store.ErrConflict)
		}
		return fmt.Errorf("sqlitestore: create %s: %w", e.ID, err)
	}

	s.notifyPut(e)
	s.emit(store.Event{Op: store.EventEntityCreated, EntityID: e.ID, EntityType: e.Type})
	return nil
}

func (s *Store) UpdateEntity(ctx context.Context, e *entity.Entity) error {
	props, err := marshalProps(e.Properties)
	if err != nil {
		return err
	}
	updated := e.UpdatedAt
	if updated.IsZero() {
		updated = time.Now().UTC()
	}

	res, err := s.write(ctx, `UPDATE entities SET type = ?, properties = ?, content = ?, updated_at = ?
		WHERE id = ?`, e.Type, props, e.Content, updated.Format(timeFmt), e.ID)
	if err != nil {
		return fmt.Errorf("sqlitestore: update %s: %w", e.ID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlitestore: update %s: %w", e.ID, err)
	}
	if n == 0 {
		return fmt.Errorf("sqlitestore: update %s: %w", e.ID, store.ErrNotFound)
	}

	s.notifyPut(e)
	s.emit(store.Event{Op: store.EventEntityUpdated, EntityID: e.ID, EntityType: e.Type})
	return nil
}

// DeleteEntity refuses an entity with relations unless cascade is set. The
// stress test tolerates ErrNotFound and ErrHasRelations here and nothing
// else, so both must be reported precisely.
func (s *Store) DeleteEntity(ctx context.Context, id string, cascade bool) (*store.DeleteResult, error) {
	existing, err := s.GetEntity(ctx, id)
	if err != nil {
		return nil, err
	}

	var relCount int
	if err := s.q().QueryRowContext(ctx,
		`SELECT count(*) FROM relations WHERE from_id = ? OR to_id = ?`, id, id).Scan(&relCount); err != nil {
		return nil, fmt.Errorf("sqlitestore: delete %s: %w", id, err)
	}
	if relCount > 0 && !cascade {
		return nil, fmt.Errorf("sqlitestore: delete %s: %w", id, store.ErrHasRelations)
	}

	// The deleted entity is part of the result contract: callers use it to
	// report what went away, and a version-capturing backend needs the
	// pre-delete state, which no longer exists once the row is gone.
	result := &store.DeleteResult{DeletedEntities: []*entity.Entity{existing}}
	if cascade && relCount > 0 {
		// Collect BEFORE deleting: the result reports which relations went, and
		// after the DELETE there is nothing left to enumerate.
		incident, err := s.incidentRelations(ctx, id)
		if err != nil {
			return nil, err
		}
		result.DeletedRelations = incident
		if _, err := s.write(ctx, `DELETE FROM relations WHERE from_id = ? OR to_id = ?`, id, id); err != nil {
			return nil, fmt.Errorf("sqlitestore: delete %s relations: %w", id, err)
		}
	}

	// Attachments are owned by the entity, so they go with it regardless of
	// cascade — that flag governs RELATIONS, which have another endpoint and so
	// need the caller's consent to remove.
	if _, err := s.write(ctx, `DELETE FROM attachments WHERE entity_id = ?`, id); err != nil {
		return nil, fmt.Errorf("sqlitestore: delete %s attachments: %w", id, err)
	}
	if _, err := s.write(ctx, `DELETE FROM entities WHERE id = ?`, id); err != nil {
		return nil, fmt.Errorf("sqlitestore: delete %s: %w", id, err)
	}

	for _, r := range result.DeletedRelations {
		s.emit(store.Event{
			Op: store.EventRelationDeleted, RelationType: r.Type, From: r.From, To: r.To,
		})
	}
	s.notifyDelete(id)
	s.emit(store.Event{Op: store.EventEntityDeleted, EntityID: id, EntityType: existing.Type})
	return result, nil
}

// incidentRelations lists every relation with id at either endpoint. Its own
// function so the rows handle can be closed with defer rather than by hand on
// each return path.
func (s *Store) incidentRelations(ctx context.Context, id string) ([]*entity.Relation, error) {
	rows, err := s.q().QueryContext(ctx,
		`SELECT from_id, rel_type, to_id FROM relations WHERE from_id = ? OR to_id = ?`, id, id)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: incident relations for %s: %w", id, err)
	}
	defer rows.Close()

	var out []*entity.Relation
	for rows.Next() {
		var r entity.Relation
		if err := rows.Scan(&r.From, &r.Type, &r.To); err != nil {
			return nil, fmt.Errorf("sqlitestore: incident relations for %s: %w", id, err)
		}
		out = append(out, &r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlitestore: incident relations for %s: %w", id, err)
	}
	return out, nil
}

// --- Freshness / Lifecycle ------------------------------------------------

func (s *Store) LastModified(ctx context.Context) (time.Time, error) {
	var raw sql.NullString
	if err := s.q().QueryRowContext(ctx,
		`SELECT max(updated_at) FROM (
			SELECT updated_at FROM entities UNION ALL SELECT updated_at FROM relations)`).Scan(&raw); err != nil {
		return time.Time{}, fmt.Errorf("sqlitestore: last modified: %w", err)
	}
	if !raw.Valid || raw.String == "" {
		return time.Time{}, nil // empty store: zero time, per contract
	}
	t, err := time.Parse(timeFmt, raw.String)
	if err != nil {
		return time.Time{}, fmt.Errorf("sqlitestore: last modified: %w", err)
	}
	return t, nil
}

// --- helpers --------------------------------------------------------------

type scanner interface {
	Scan(dest ...any) error
}

func scanEntity(sc scanner) (*entity.Entity, error) {
	var (
		e       entity.Entity
		props   string
		updated string
	)
	if err := sc.Scan(&e.ID, &e.Type, &props, &e.Content, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("sqlitestore: scan entity: %w", err)
	}
	props2, err := unmarshalProps(props)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: entity %s: %w", e.ID, err)
	}
	e.Properties = props2
	t, err := time.Parse(timeFmt, updated)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: parse updated_at for %s: %w", e.ID, err)
	}
	e.UpdatedAt = t
	return &e, nil
}
