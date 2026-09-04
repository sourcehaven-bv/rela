package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"

	"modernc.org/sqlite"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// --- RelationReader -------------------------------------------------------

// GetRelation returns the DEFAULT-tail edge of the triple (TKT-DOFYR1) — see
// store.RelationData.FromFace. A triple can carry one edge per tail face, so
// this is an address, not a wildcard.
func (s *Store) GetRelation(ctx context.Context, from, relType, to string) (*entity.Relation, error) {
	row := s.q().QueryRowContext(ctx,
		`SELECT `+relationColumns+`
		 FROM relations WHERE from_id = ? AND rel_type = ? AND to_id = ? AND from_face = ''`,
		from, relType, to)
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
		from, face, relType, to, ok := splitRelationKey(cursorKey)
		if ok {
			// Row-value comparison matches the multi-column ORDER BY exactly,
			// without hand-expanding it into a four-way OR.
			conds = append(conds, "(from_id, from_face, rel_type, to_id) > (?, ?, ?, ?)")
			args = append(args, from, face, relType, to)
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
	// The tail-face filter is nil-PERMISSIVE (TKT-DOFYR1): a nil FromFace
	// matches every tail — default-tail edges and all state-tailed ones —
	// which is today's behavior for faceless projects and the compat story for
	// every existing query site. Non-nil matches by equality only; the store
	// compares, never inspects (see entity.Face).
	if q.FromFace != nil {
		conds = append(conds, "from_face = ?")
		args = append(args, string(*q.FromFace))
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

	sqlText = `SELECT ` + relationColumns + ` FROM relations`
	if len(conds) > 0 {
		sqlText += ` WHERE ` + strings.Join(conds, " AND ")
	}
	sqlText += ` ORDER BY from_id, from_face, rel_type, to_id`
	return sqlText, args
}

// relationColumns is the column list every relation read selects, in the order
// scanRelation expects.
const relationColumns = "from_id, from_face, rel_type, to_id, properties, content, updated_at"

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
		face    entity.Face
		err     error
	)
	if data != nil {
		if props, err = marshalProps(data.Properties); err != nil {
			return nil, err
		}
		content = data.Content
		// The tail is part of the edge's IDENTITY, not a property of it: two
		// edges on one triple with different tails are two relations.
		face = data.FromFace
	}
	now := time.Now().UTC()

	_, err = s.write(ctx, `INSERT INTO relations
		(from_id, from_face, rel_type, to_id, properties, content, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		from, string(face), relType, to, props, content, now.Format(timeFmt))
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("sqlitestore: create relation: %w", store.ErrConflict)
		}
		return nil, fmt.Errorf("sqlitestore: create relation: %w", err)
	}

	s.emit(store.Event{
		Op: store.EventRelationCreated, RelationType: relType, From: from, To: to, Face: face,
	})
	return s.getRelationState(ctx, from, face, relType, to)
}

// getRelationState reads the edge of a triple carrying EXACTLY tail p. Unlike
// GetRelation (default-tail only, per the store.RelationReader contract) this
// is internal, so CreateRelation can echo back the state-tailed edge it just
// wrote rather than a different edge that happens to share the triple.
func (s *Store) getRelationState(
	ctx context.Context, from string, p entity.Face, relType, to string,
) (*entity.Relation, error) {
	row := s.q().QueryRowContext(ctx,
		`SELECT `+relationColumns+`
		 FROM relations WHERE from_id = ? AND from_face = ? AND rel_type = ? AND to_id = ?`,
		from, string(p), relType, to)
	r, err := scanRelation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("sqlitestore: get relation %s--%s->%s: %w",
			entity.FormatStateRef(from, p), relType, to, store.ErrNotFound)
	}
	return r, err
}

func (s *Store) UpdateRelation(
	ctx context.Context, from, relType, to string, data store.RelationData,
) (*entity.Relation, error) {
	props, err := marshalProps(data.Properties)
	if err != nil {
		return nil, err
	}
	// Addresses the DEFAULT-tail edge, matching GetRelation and the
	// store.RelationWriter contract (TKT-DOFYR1).
	res, err := s.write(ctx, `UPDATE relations SET properties = ?, content = ?, updated_at = ?
		WHERE from_id = ? AND rel_type = ? AND to_id = ? AND from_face = ''`,
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

// DeleteRelation removes the DEFAULT-tail edge of the triple.
// DeleteRelationState is the general form.
func (s *Store) DeleteRelation(ctx context.Context, from, relType, to string) error {
	return s.DeleteRelationState(ctx, from, "", relType, to)
}

// DeleteRelationState removes the edge with EXACTLY this tail (TKT-C1XUA8).
//
// The tail is part of a relation's identity, so addressing the wrong one
// deletes a DIFFERENT edge rather than failing — which is precisely the bug
// this separate method exists to make unavailable.
func (s *Store) DeleteRelationState(
	ctx context.Context, from string, p entity.Face, relType, to string,
) error {
	res, err := s.write(ctx,
		`DELETE FROM relations WHERE from_id = ? AND from_face = ? AND rel_type = ? AND to_id = ?`,
		from, string(p), relType, to)
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

	s.emit(store.Event{
		Op: store.EventRelationDeleted, RelationType: relType, From: from, To: to, Face: p,
	})
	return nil
}

// --- helpers --------------------------------------------------------------

func scanRelation(sc scanner) (*entity.Relation, error) {
	var (
		r       entity.Relation
		face    string
		props   string
		updated string
	)
	if err := sc.Scan(&r.From, &face, &r.Type, &r.To, &props, &r.Content, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("sqlitestore: scan relation: %w", err)
	}
	r.FromFace = entity.Face(face)
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
// constraint failure, so it can be mapped to store.ErrConflict.
//
// Checks the driver's extended result code rather than the message text: the
// stress test tolerates ErrConflict and nothing else from a racing create, so
// a reworded message upstream would turn a normal conflict into a hard failure.
// The string check remains as a fallback for any path that surfaces an error
// the type assertion cannot see.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var se *sqlite.Error
	if errors.As(err, &se) {
		switch se.Code() {
		case sqliteConstraintPrimaryKey, sqliteConstraintUnique:
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint failed") ||
		strings.Contains(msg, "primary key must be unique")
}

// SQLite extended result codes for the two constraint failures that mean
// "this key is taken". Not exported by the driver as named constants.
const (
	sqliteConstraintPrimaryKey = 1555
	sqliteConstraintUnique     = 2067
)
