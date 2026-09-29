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

// GetRelation returns the edge at k, tail included. A triple can carry one
// edge per tail face, so this is an address, not a wildcard.
func (s *Store) GetRelation(ctx context.Context, k entity.RelationKey) (*entity.Relation, error) {
	row := s.q().QueryRowContext(ctx,
		`SELECT `+relationColumns+`
		 FROM relations WHERE from_id = ? AND from_face = ? AND rel_type = ? AND to_id = ?`,
		k.From, string(k.FromFace), k.Type, k.To)
	r, err := scanRelation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("sqlitestore: get relation %s: %w", k, store.ErrNotFound)
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
	if q.EntityIDs != nil {
		// SQLite has no array parameters, and a placeholder per id runs into
		// SQLITE_MAX_VARIABLE_NUMBER for a large batch (a gantt subtree is
		// unbounded, TKT-U9DYW4). The ids travel as ONE JSON array instead and
		// json_each unpacks it, so the statement has a fixed parameter count
		// whatever the batch size. An empty slice yields an empty set, which
		// keeps nil-vs-empty's documented meaning (nil = unfiltered, empty =
		// nothing).
		ids := jsonIDs(q.EntityIDs)
		const in = "(SELECT value FROM json_each(?))"
		switch q.Direction {
		case store.DirectionOutgoing:
			conds = append(conds, "from_id IN "+in)
			args = append(args, ids)
		case store.DirectionIncoming:
			conds = append(conds, "to_id IN "+in)
			args = append(args, ids)
		default:
			conds = append(conds, "(from_id IN "+in+" OR to_id IN "+in+")")
			args = append(args, ids, ids)
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
	ctx context.Context, k entity.RelationKey, data *store.RelationData,
) (*entity.Relation, error) {
	// storeutil is the validity ORACLE the fuzz suite enforces directionally:
	// anything it rejects the store MUST reject. Skipping this let an empty
	// relation type through — caught by FuzzRelationKeyCollision seed #4.
	if err := storeutil.ValidateRelationType(k.Type); err != nil {
		return nil, fmt.Errorf("sqlitestore: create relation: %w", err)
	}
	if err := storeutil.ValidateID(k.From); err != nil {
		return nil, fmt.Errorf("sqlitestore: create relation from: %w", err)
	}
	if err := storeutil.ValidateID(k.To); err != nil {
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

	// Mint the surrogate lineage id at CREATE, once. It is what separates two
	// relations' histories: a delete followed by a re-create of the same triple
	// gets a FRESH id and therefore a fresh lineage, while a rename carries the
	// existing id across in place. Reconstructing it later from the composite
	// key would race the sweep and merge lineages that must stay apart.
	//
	// The id is READ INLINE in the INSERT, and the counter is bumped only
	// afterwards, because the two orders differ on the failure path. Allocating
	// first means a duplicate triple — an ordinary ErrConflict an automation
	// retries on, not an exceptional case — permanently consumes an id: outside
	// a Tx the UPDATE autocommits, so the INSERT's rollback cannot take it back.
	// Reading the counter inside the INSERT makes allocation a consequence of a
	// successful insert rather than a precondition for attempting one.
	editorUser, editorTool := store.AttributionColumns(ctx)
	if _, err = s.write(ctx, `INSERT INTO relations
		(from_id, from_face, rel_type, to_id, properties, content, updated_at,
		 last_edited_by_user, last_edited_by_tool, rel_record_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, (SELECT next FROM rel_record_seq WHERE id = 1))`,
		k.From, string(k.FromFace), k.Type, k.To, props, content, now.Format(timeFmt),
		editorUser, editorTool); err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("sqlitestore: create relation: %w", store.ErrConflict)
		}
		return nil, fmt.Errorf("sqlitestore: create relation: %w", err)
	}
	// Consume the id this row just took. A second CreateRelation cannot
	// interleave between the two statements: writes are serialized by writeMu
	// inside a Tx and by the single-writer lock across the process, which is
	// the same discipline every other multi-statement write here relies on.
	if err = bumpRelRecordSeq(ctx, s.q()); err != nil {
		return nil, fmt.Errorf("sqlitestore: create relation: %w", err)
	}

	s.emit(store.Event{
		Op: store.EventRelationCreated, RelationType: k.Type, From: k.From, To: k.To, Face: k.FromFace,
	})
	return s.GetRelation(ctx, k)
}

// UpdateRelation updates the edge at k, tail included (BUG-64MU2Q).
//
// The tail is part of a relation's identity, so addressing the wrong one
// writes the caller's properties onto a DIFFERENT edge rather than failing.
func (s *Store) UpdateRelation(
	ctx context.Context, k entity.RelationKey, data store.RelationData,
) (*entity.Relation, error) {
	props, err := marshalProps(data.Properties)
	if err != nil {
		return nil, err
	}
	editorUser, editorTool := store.AttributionColumns(ctx)
	res, err := s.write(ctx, `UPDATE relations SET properties = ?, content = ?, updated_at = ?,
		    last_edited_by_user = ?, last_edited_by_tool = ?
		WHERE from_id = ? AND rel_type = ? AND to_id = ? AND from_face = ?`,
		props, data.Content, time.Now().UTC().Format(timeFmt), editorUser, editorTool,
		k.From, k.Type, k.To, string(k.FromFace))
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

	s.emit(store.Event{
		Op: store.EventRelationUpdated, RelationType: k.Type, From: k.From, To: k.To, Face: k.FromFace,
	})
	return s.GetRelation(ctx, k)
}

// DeleteRelation removes the edge at k, tail included (TKT-C1XUA8).
//
// The tail is part of a relation's identity, so addressing the wrong one
// deletes a DIFFERENT edge rather than failing.
func (s *Store) DeleteRelation(ctx context.Context, k entity.RelationKey) error {
	res, err := s.write(ctx,
		`DELETE FROM relations WHERE from_id = ? AND from_face = ? AND rel_type = ? AND to_id = ?`,
		k.From, string(k.FromFace), k.Type, k.To)
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
		Op: store.EventRelationDeleted, RelationType: k.Type, From: k.From, To: k.To, Face: k.FromFace,
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
