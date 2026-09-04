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

// GetRelation returns a relation by its three-part key, or
// store.ErrNotFound. It addresses the DEFAULT-tail edge of the triple
// (TKT-DOFYR1) — see store.RelationData.FromFace.
func (s *Store) GetRelation(ctx context.Context, from, relType, to string) (*entity.Relation, error) {
	const q = `SELECT from_id, from_face, rel_type, to_id, properties, content, updated_at
	           FROM relations WHERE from_id = $1 AND rel_type = $2 AND to_id = $3 AND from_face = ''`
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
	if err := storeutil.ValidateRelationType(relType); err != nil {
		return nil, err
	}
	var fp entity.Face
	var props map[string]any
	content := ""
	if data != nil {
		fp = data.FromFace
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

	editorUser, editorTool := attributionValues(ctx)
	const q = `
		INSERT INTO relations (from_id, from_face, rel_type, to_id, properties, content, updated_at,
		                       last_edited_by_user, last_edited_by_tool)
		VALUES ($1, $2, $3, $4, $5, $6, now(), $7, $8)
		ON CONFLICT (from_id, from_face, rel_type, to_id) DO NOTHING
		RETURNING from_id, from_face, rel_type, to_id, properties, content, updated_at`
	r, err := scanRelation(tx.QueryRow(ctx, q, from, fp, relType, to, rawProps, content,
		editorUser, editorTool))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, store.ErrConflict
	}
	if err != nil {
		return nil, err
	}

	ev := store.Event{Op: store.EventRelationCreated, RelationType: relType, From: from, To: to, Face: fp}
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

	editorUser, editorTool := attributionValues(ctx)
	// Default-tail addressing in Step 1 (TKT-DOFYR1) — see
	// store.RelationData.FromFace.
	const q = `
		UPDATE relations
		SET properties = $4, content = $5, updated_at = now(), seq = nextval('rela_seq'),
		    last_edited_by_user = $6, last_edited_by_tool = $7
		WHERE from_id = $1 AND rel_type = $2 AND to_id = $3 AND from_face = ''
		RETURNING from_id, from_face, rel_type, to_id, properties, content, updated_at`
	r, err := scanRelation(tx.QueryRow(ctx, q, from, relType, to, rawProps, data.Content,
		editorUser, editorTool))
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
//
// Addresses the DEFAULT-tail edge; DeleteRelationState is the general form.
func (s *Store) DeleteRelation(ctx context.Context, from, relType, to string) error {
	return s.DeleteRelationState(ctx, from, "", relType, to)
}

// DeleteRelationState removes the edge with EXACTLY this tail. The tail is
// part of a relation's identity, so addressing the wrong one deletes a
// different edge rather than failing (TKT-C1XUA8).
func (s *Store) DeleteRelationState(
	ctx context.Context, from string, p entity.Face, relType, to string,
) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	const q = `DELETE FROM relations
	           WHERE from_id = $1 AND rel_type = $2 AND to_id = $3 AND from_face = $4`
	tag, err := tx.Exec(ctx, q, from, relType, to, string(p))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}

	// Record a tombstone in the same tx so the durable manifest can report the
	// removal after the live row is gone (FEAT-NJ9FEN).
	if err := s.writeRelationTombstone(ctx, tx, from, p, relType, to); err != nil {
		return err
	}

	ev := store.Event{
		Op: store.EventRelationDeleted, RelationType: relType,
		From: from, To: to, Face: p,
	}
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
		from, fp, relType, to, content string
		props                          []byte
		updatedAt                      time.Time
	)
	if err := row.Scan(&from, &fp, &relType, &to, &props, &content, &updatedAt); err != nil {
		return nil, err
	}
	r := entity.NewRelation(from, relType, to)
	r.FromFace = entity.Face(fp)
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
	sql = `SELECT from_id, from_face, rel_type, to_id, properties, content, updated_at FROM relations` +
		where + ` ORDER BY from_id ASC, from_face ASC, rel_type ASC, to_id ASC`
	return sql, args
}

func relationWhere(q store.RelationQuery, keysetAfter string) (where string, args []any) {
	var conds []string
	add := func(cond string, val any) {
		args = append(args, val)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	// nil = unfiltered (all tails — today's behavior for faceless
	// projects); non-nil matches the tail by equality, the zero face
	// selecting default-tail edges only. Mirrors storeutil.MatchRelation
	// — the storetest States suite keeps the two implementations from
	// drifting (TKT-DOFYR1).
	if q.FromFace != nil {
		add("from_face = $%d", string(*q.FromFace))
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
	if q.EntityIDs != nil {
		// The explicit ::text[] cast keeps an EMPTY slice a valid, never-
		// matching predicate rather than an "unknown array type" error, so
		// nil-vs-empty keeps its documented meaning.
		switch q.Direction {
		case store.DirectionOutgoing:
			add("from_id = ANY($%d::text[])", q.EntityIDs)
		case store.DirectionIncoming:
			add("to_id = ANY($%d::text[])", q.EntityIDs)
		default: // DirectionBoth
			args = append(args, q.EntityIDs)
			conds = append(conds, fmt.Sprintf("(from_id = ANY($%d::text[]) OR to_id = ANY($%d::text[]))",
				len(args), len(args)))
		}
	}
	if keysetAfter != "" {
		// An unparseable cursor genuinely RESTARTS (condition omitted):
		// comparing against garbage would silently skip rows — see the
		// matching note in entityWhere.
		if from, fp, relType, to, ok := splitRelationKey(keysetAfter); ok {
			args = append(args, from, string(fp), relType, to)
			n := len(args)
			// Row-value comparison gives a correct keyset over the composite key.
			conds = append(conds, fmt.Sprintf("(from_id, from_face, rel_type, to_id) > ($%d, $%d, $%d, $%d)",
				n-3, n-2, n-1, n))
		}
	}
	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// splitRelationKey reverses entity.Relation.Key()
// ("from[@face]--type--to"). The store rejects IDs, faces, and
// relation types containing "--", so the split is unambiguous; the FROM
// slot parses through the codec to recover the tail face
// (TKT-DOFYR1). ok=false for a key whose FROM slot the codec rejects —
// the caller omits the keyset and restarts the page.
func splitRelationKey(key string) (from string, fp entity.Face, relType, to string, ok bool) {
	parts := strings.SplitN(key, "--", 3)
	id, p, err := entity.ParseStateRef(parts[0])
	if err != nil {
		return "", "", "", "", false
	}
	from, fp = id, p
	switch len(parts) {
	case 3:
		return from, fp, parts[1], parts[2], true
	case 2:
		return from, fp, parts[1], "", true
	default:
		return from, fp, "", "", true
	}
}
