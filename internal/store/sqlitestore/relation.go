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

// --- RelationReader -------------------------------------------------------

func (s *Store) GetRelation(ctx context.Context, from, relType, to string) (*entity.Relation, error) {
	row := s.q().QueryRowContext(ctx,
		`SELECT from_id, rel_type, to_id, properties, content, updated_at
		 FROM relations WHERE from_id = ? AND rel_type = ? AND to_id = ?`, from, relType, to)
	r, err := scanRelation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("sqlitestore: get relation %s--%s->%s: %w", from, relType, to, store.ErrNotFound)
	}
	return r, err
}

func (s *Store) ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error] {
	return func(yield func(*entity.Relation, error) bool) {
		sqlText, args := buildRelationQuery(q)
		rows, err := s.q().QueryContext(ctx, sqlText, args...)
		if err != nil {
			yield(nil, fmt.Errorf("sqlitestore: list relations: %w", err))
			return
		}
		defer rows.Close()

		for rows.Next() {
			r, err := scanRelation(rows)
			if !yield(r, err) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(nil, fmt.Errorf("sqlitestore: list relations: %w", err))
		}
	}
}

func (s *Store) CountRelations(ctx context.Context, q store.RelationQuery) (int, error) {
	sqlText, args := buildRelationQuery(q)
	// Wrap rather than rebuild, so filter semantics cannot drift between the
	// two paths.
	var n int
	if err := s.q().QueryRowContext(ctx,
		`SELECT count(*) FROM (`+sqlText+`)`, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("sqlitestore: count relations: %w", err)
	}
	return n, nil
}

// buildRelationQuery builds the shared SELECT for list and count.
func buildRelationQuery(q store.RelationQuery) (sqlText string, args []any) {
	return buildRelationQueryFrom(q, "")
}

// buildRelationQueryFrom builds the shared SELECT, optionally resuming after a
// cursor key. The cursor is folded in as a normal predicate rather than spliced
// into finished SQL, so there is one code path that decides WHERE-vs-AND.
func buildRelationQueryFrom(q store.RelationQuery, cursorKey string) (sqlText string, args []any) {
	var conds []string
	if cursorKey != "" {
		from, relType, to, ok := splitRelationKey(cursorKey)
		if ok {
			// Row-value comparison matches the multi-column ORDER BY exactly,
			// without hand-expanding it into a three-way OR.
			conds = append(conds, "(from_id, rel_type, to_id) > (?, ?, ?)")
			args = append(args, from, relType, to)
		}
	}
	if q.From != "" {
		conds = append(conds, "from_id = ?")
		args = append(args, q.From)
	}
	if q.To != "" {
		conds = append(conds, "to_id = ?")
		args = append(args, q.To)
	}
	if q.Type != "" {
		conds = append(conds, "rel_type = ?")
		args = append(args, q.Type)
	}
	if q.EntityID != "" {
		switch q.Direction {
		case store.DirectionOutgoing:
			conds = append(conds, "from_id = ?")
			args = append(args, q.EntityID)
		case store.DirectionIncoming:
			conds = append(conds, "to_id = ?")
			args = append(args, q.EntityID)
		default:
			conds = append(conds, "(from_id = ? OR to_id = ?)")
			args = append(args, q.EntityID, q.EntityID)
		}
	}

	sqlText = `SELECT from_id, rel_type, to_id, properties, content, updated_at FROM relations`
	if len(conds) > 0 {
		sqlText += ` WHERE ` + strings.Join(conds, " AND ")
	}
	sqlText += ` ORDER BY from_id, rel_type, to_id`
	return sqlText, args
}

// --- RelationWriter -------------------------------------------------------

func (s *Store) CreateRelation(
	ctx context.Context, from, relType, to string, data *store.RelationData,
) (*entity.Relation, error) {
	// storeutil is the validity ORACLE the fuzz suite enforces directionally:
	// anything it rejects the store MUST reject. Skipping this let an empty
	// relation type through — caught by FuzzRelationKeyCollision seed #4.
	if err := storeutil.ValidateRelationType(relType); err != nil {
		return nil, fmt.Errorf("sqlitestore: create relation: %w", err)
	}
	if err := storeutil.ValidateID(from); err != nil {
		return nil, fmt.Errorf("sqlitestore: create relation from: %w", err)
	}
	if err := storeutil.ValidateID(to); err != nil {
		return nil, fmt.Errorf("sqlitestore: create relation to: %w", err)
	}

	var (
		props   = "{}"
		content string
		err     error
	)
	if data != nil {
		if props, err = marshalProps(data.Properties); err != nil {
			return nil, err
		}
		content = data.Content
	}
	now := time.Now().UTC()

	_, err = s.write(ctx, `INSERT INTO relations
		(from_id, rel_type, to_id, properties, content, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		from, relType, to, props, content, now.Format(timeFmt))
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("sqlitestore: create relation: %w", store.ErrConflict)
		}
		return nil, fmt.Errorf("sqlitestore: create relation: %w", err)
	}

	s.emit(store.Event{Op: store.EventRelationCreated, RelationType: relType, From: from, To: to})
	return s.GetRelation(ctx, from, relType, to)
}

func (s *Store) UpdateRelation(
	ctx context.Context, from, relType, to string, data store.RelationData,
) (*entity.Relation, error) {
	props, err := marshalProps(data.Properties)
	if err != nil {
		return nil, err
	}
	res, err := s.write(ctx, `UPDATE relations SET properties = ?, content = ?, updated_at = ?
		WHERE from_id = ? AND rel_type = ? AND to_id = ?`,
		props, data.Content, time.Now().UTC().Format(timeFmt), from, relType, to)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: update relation: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: update relation: %w", err)
	}
	if n == 0 {
		return nil, fmt.Errorf("sqlitestore: update relation: %w", store.ErrNotFound)
	}

	s.emit(store.Event{Op: store.EventRelationUpdated, RelationType: relType, From: from, To: to})
	return s.GetRelation(ctx, from, relType, to)
}

func (s *Store) DeleteRelation(ctx context.Context, from, relType, to string) error {
	res, err := s.write(ctx,
		`DELETE FROM relations WHERE from_id = ? AND rel_type = ? AND to_id = ?`, from, relType, to)
	if err != nil {
		return fmt.Errorf("sqlitestore: delete relation: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlitestore: delete relation: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("sqlitestore: delete relation: %w", store.ErrNotFound)
	}

	s.emit(store.Event{Op: store.EventRelationDeleted, RelationType: relType, From: from, To: to})
	return nil
}

// --- helpers --------------------------------------------------------------

func scanRelation(sc scanner) (*entity.Relation, error) {
	var (
		r       entity.Relation
		props   string
		updated string
	)
	if err := sc.Scan(&r.From, &r.Type, &r.To, &props, &r.Content, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("sqlitestore: scan relation: %w", err)
	}
	var err error
	if r.Properties, err = unmarshalProps(props); err != nil {
		return nil, fmt.Errorf("sqlitestore: relation %s--%s->%s: %w", r.From, r.Type, r.To, err)
	}
	t, err := time.Parse(timeFmt, updated)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: parse relation updated_at: %w", err)
	}
	r.UpdatedAt = t
	return &r, nil
}

// isUniqueViolation reports whether err is a SQLite PRIMARY KEY / UNIQUE
// constraint failure.
//
// Matching on the message rather than the driver's error struct keeps the
// spike from depending on modernc's internal error types — a real backend
// would type-assert *sqlite.Error and check the extended result code
// (SQLITE_CONSTRAINT_PRIMARYKEY = 1555, SQLITE_CONSTRAINT_UNIQUE = 2067).
// Noted as an implementation-ticket refinement, not a spike concern.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint failed") ||
		strings.Contains(msg, "primary key must be unique")
}
