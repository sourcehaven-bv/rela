package pgstore

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// --- RelationReader ---

// GetRelation returns a relation by its three-part key, or store.ErrNotFound.
func (s *Store) GetRelation(ctx context.Context, from, relType, to string) (*entity.Relation, error) {
	const q = `SELECT from_id, rel_type, to_id, properties, content, updated_at
	           FROM relations WHERE from_id = $1 AND rel_type = $2 AND to_id = $3`
	r, err := scanRelation(s.db.QueryRow(ctx, q, from, relType, to))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

// ListRelations streams relations matching q in stable key order. Cursor and
// Limit are ignored (per the RelationReader contract).
func (s *Store) ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error] {
	sql, args := buildRelationListSQL(q, "")
	return func(yield func(*entity.Relation, error) bool) {
		rows, err := s.db.Query(ctx, sql, args...)
		if err != nil {
			yield(nil, err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			r, err := scanRelation(rows)
			if err != nil {
				yield(nil, err)
				return
			}
			if !yield(r, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(nil, err)
		}
	}
}

// ListRelationsPage returns a page of relations using a keyset cursor over the
// composite key rendered as "from--type--to".
func (s *Store) ListRelationsPage(ctx context.Context, q store.RelationQuery) (store.Page[*entity.Relation], error) {
	cursorKey, err := storeutil.DecodeCursor(q.Cursor)
	if err != nil {
		return store.Page[*entity.Relation]{}, err
	}

	fetch := q.Limit
	if fetch > 0 {
		fetch++
	}
	sql, args := buildRelationListSQL(q, cursorKey)
	if fetch > 0 {
		sql += fmt.Sprintf(" LIMIT %d", fetch)
	}

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return store.Page[*entity.Relation]{}, err
	}
	defer rows.Close()

	items := make([]*entity.Relation, 0)
	for rows.Next() {
		r, err := scanRelation(rows)
		if err != nil {
			return store.Page[*entity.Relation]{}, err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return store.Page[*entity.Relation]{}, err
	}

	var next string
	if q.Limit > 0 && len(items) > q.Limit {
		last := items[q.Limit-1]
		items = items[:q.Limit]
		next = storeutil.EncodeCursor(last.Key())
	}
	return store.Page[*entity.Relation]{Items: items, NextCursor: next}, nil
}

// CountRelations counts relations matching q.
func (s *Store) CountRelations(ctx context.Context, q store.RelationQuery) (int, error) {
	where, args := relationWhere(q, "")
	sql := "SELECT count(*) FROM relations" + where
	var n int
	if err := s.db.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// --- RelationWriter ---

// CreateRelation inserts a new relation. Returns store.ErrConflict if the
// (from, type, to) key already exists.
func (s *Store) CreateRelation(
	ctx context.Context, from, relType, to string, data *store.RelationData,
) (*entity.Relation, error) {
	for _, id := range []string{from, to} {
		if err := validateID(id); err != nil {
			return nil, err
		}
	}
	if relType == "" {
		return nil, errors.New("store: empty relation type")
	}
	if strings.Contains(relType, "--") {
		return nil, fmt.Errorf("store: relation type %q contains consecutive dashes", relType)
	}

	var props map[string]interface{}
	content := ""
	if data != nil {
		content = data.Content
		props = data.Properties
	}
	rawProps, err := marshalProps(props)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	const q = `
		INSERT INTO relations (from_id, rel_type, to_id, properties, content, updated_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (from_id, rel_type, to_id) DO NOTHING
		RETURNING from_id, rel_type, to_id, properties, content, updated_at`
	r, err := scanRelation(tx.QueryRow(ctx, q, from, relType, to, rawProps, content))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, store.ErrConflict
	}
	if err != nil {
		return nil, err
	}

	ev := store.Event{Op: store.EventRelationCreated, RelationType: relType, From: from, To: to}
	s.notify(ctx, tx, ev)
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.emit(ev)
	return r, nil
}

// UpdateRelation overwrites a relation's data. Returns store.ErrNotFound if it
// does not exist. Nil data.Properties clears the property set.
func (s *Store) UpdateRelation(
	ctx context.Context, from, relType, to string, data store.RelationData,
) (*entity.Relation, error) {
	rawProps, err := marshalProps(data.Properties)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	const q = `
		UPDATE relations
		SET properties = $4, content = $5, updated_at = now(), seq = nextval('rela_seq')
		WHERE from_id = $1 AND rel_type = $2 AND to_id = $3
		RETURNING from_id, rel_type, to_id, properties, content, updated_at`
	r, err := scanRelation(tx.QueryRow(ctx, q, from, relType, to, rawProps, data.Content))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	ev := store.Event{Op: store.EventRelationUpdated, RelationType: relType, From: from, To: to}
	s.notify(ctx, tx, ev)
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.emit(ev)
	return r, nil
}

// DeleteRelation removes a relation. Returns store.ErrNotFound if absent.
func (s *Store) DeleteRelation(ctx context.Context, from, relType, to string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	const q = `DELETE FROM relations WHERE from_id = $1 AND rel_type = $2 AND to_id = $3`
	tag, err := tx.Exec(ctx, q, from, relType, to)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}

	ev := store.Event{Op: store.EventRelationDeleted, RelationType: relType, From: from, To: to}
	s.notify(ctx, tx, ev)
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.emit(ev)
	return nil
}

// --- row scanning + query building ---

func scanRelation(row scanner) (*entity.Relation, error) {
	var (
		from, relType, to, content string
		props                      []byte
		updatedAt                  time.Time
	)
	if err := row.Scan(&from, &relType, &to, &props, &content, &updatedAt); err != nil {
		return nil, err
	}
	r := entity.NewRelation(from, relType, to)
	r.Content = content
	r.UpdatedAt = updatedAt
	var err error
	if r.Properties, err = unmarshalProps(props); err != nil {
		return nil, err
	}
	return r, nil
}

// scanRelations runs a query (within a tx or on the pool) and collects all
// matching relations. Used by cascade delete.
func scanRelations(ctx context.Context, db DBTX, sql string, args ...any) ([]*entity.Relation, error) {
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*entity.Relation
	for rows.Next() {
		r, err := scanRelation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// buildRelationListSQL builds SELECT + WHERE + ORDER BY for relation listings.
// Order is by the composite key (from_id, rel_type, to_id), which equals the
// "from--type--to" Key() ordering used for cursors. keysetAfter resumes after
// a decoded cursor key.
func buildRelationListSQL(q store.RelationQuery, keysetAfter string) (sql string, args []any) {
	where, args := relationWhere(q, keysetAfter)
	sql = `SELECT from_id, rel_type, to_id, properties, content, updated_at FROM relations` +
		where + ` ORDER BY from_id ASC, rel_type ASC, to_id ASC`
	return sql, args
}

func relationWhere(q store.RelationQuery, keysetAfter string) (where string, args []any) {
	var conds []string
	add := func(cond string, val any) {
		args = append(args, val)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if q.Type != "" {
		add("rel_type = $%d", q.Type)
	}
	if q.From != "" {
		add("from_id = $%d", q.From)
	}
	if q.To != "" {
		add("to_id = $%d", q.To)
	}
	if q.EntityID != "" {
		switch q.Direction {
		case store.DirectionOutgoing:
			add("from_id = $%d", q.EntityID)
		case store.DirectionIncoming:
			add("to_id = $%d", q.EntityID)
		default: // DirectionBoth
			args = append(args, q.EntityID)
			conds = append(conds, fmt.Sprintf("(from_id = $%d OR to_id = $%d)", len(args), len(args)))
		}
	}
	if keysetAfter != "" {
		from, relType, to := splitRelationKey(keysetAfter)
		args = append(args, from, relType, to)
		n := len(args)
		// Row-value comparison gives a correct keyset over the composite key.
		conds = append(conds, fmt.Sprintf("(from_id, rel_type, to_id) > ($%d, $%d, $%d)", n-2, n-1, n))
	}
	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// splitRelationKey reverses entity.Relation.Key() ("from--type--to"). The
// store rejects IDs and relation types containing "--", so the split is
// unambiguous: exactly two "--" separators.
func splitRelationKey(key string) (from, relType, to string) {
	parts := strings.SplitN(key, "--", 3)
	switch len(parts) {
	case 3:
		return parts[0], parts[1], parts[2]
	case 2:
		return parts[0], parts[1], ""
	default:
		return parts[0], "", ""
	}
}
