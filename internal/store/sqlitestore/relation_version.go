package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Relation versioning (the SQLite half of TKT-92JL8P / TKT-C1XUA8).
//
// Relations have no stable id — the composite (from_id, rel_type, to_id) IS the
// key, and it mutates when an endpoint is renamed. Lineage is therefore keyed on
// the surrogate rel_record_id carried ON the relations row, so a delete followed
// by a re-create of the same triple mints a FRESH lineage rather than
// resurrecting the old one.
//
// Two Postgres constructs from the reference implementation have no SQLite
// equivalent and are rewritten here rather than emulated:
//
//   - `JOIN LATERAL`: the predecessor lookup becomes a correlated subquery in
//     the WHERE clause, which SQLite optimizes the same way and which expresses
//     the identical "newest lineage carrying the old triple before this rename"
//     selection.
//   - `= ANY($1)`: SQLite has no array binding, so id sets expand to an IN
//     clause with one placeholder per id (idPlaceholders). The sets are lineage
//     sizes — a handful of ids after a rename chain, not user-controlled bulk —
//     so expansion is bounded.

// --- Write ----------------------------------------------------------------

// WriteRelationVersion implements [store.RelationVersionWriter]: it persists one
// synchronously captured relation version (rename or delete).
//
// Delete is the ONLY path that captures a relation deletion, and it covers both
// an explicit DeleteRelation and an entity CASCADE delete — the store bulk-
// deletes relations below the entitymanager, so cascade edges would otherwise
// lose their history entirely.
func (v *VersionStore) WriteRelationVersion(ctx context.Context, in store.RelationVersionInput) error {
	tx, err := v.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlitestore: begin relation version write: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after commit is a no-op

	if err := insertRelationVersion(ctx, tx, in, contentHashOfRelation(in)); err != nil {
		return err
	}
	return tx.Commit()
}

// contentHashOfRelation hashes a captured relation snapshot. The triple is
// folded in, so two distinct relations holding identical content do not dedup
// against each other.
func contentHashOfRelation(in store.RelationVersionInput) string {
	r := entity.Relation{
		From:       in.From,
		FromFace:   in.FromFace,
		Type:       in.Type,
		To:         in.To,
		Properties: in.Properties,
		Content:    in.Content,
	}
	return canonical.HashRelation(r)
}

// insertRelationVersion writes one relation_versions row through q.
func insertRelationVersion(
	ctx context.Context, q querier, in store.RelationVersionInput, contentHash string,
) error {
	props, err := marshalProps(in.Properties)
	if err != nil {
		return err
	}
	// One timestamp for both inserts, as insertVersion does and for the same
	// reason: the projection and the version describe one capture.
	now := timestampNow()
	if schemaErr := ensureSchemaVersion(ctx, q, in.SchemaHash, in.Projection, now); schemaErr != nil {
		return schemaErr
	}
	// prev_from/prev_to are the rename stitch links; NULL on every other op.
	var prevFrom, prevTo *string
	if in.Op == store.VersionOpRename {
		if in.PrevFrom != "" {
			prevFrom = &in.PrevFrom
		}
		if in.PrevTo != "" {
			prevTo = &in.PrevTo
		}
	}
	const ins = `
		INSERT INTO relation_versions
		    (rel_record_id, op, from_id, from_face, rel_type, to_id, prev_from, prev_to,
		     content, properties, content_hash, schema_hash,
		     principal_user, principal_tool, triggered_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = q.ExecContext(ctx, ins,
		in.RecordID, string(in.Op), in.From, string(in.FromFace), in.Type, in.To,
		prevFrom, prevTo, in.Content, props, contentHash, in.SchemaHash,
		in.PrincipalUser, in.PrincipalTool, in.TriggeredBy, now)
	if err != nil {
		return fmt.Errorf("sqlitestore: insert relation version: %w", err)
	}
	return nil
}

// --- Lineage --------------------------------------------------------------

// idPlaceholders renders "?, ?, ?" for n ids and returns them as bind args,
// standing in for Postgres's `= ANY($1)`.
//
// Generic over the id type because relation lineages are keyed by int64
// (rel_record_id) and entity lineages by string (entity_id), and two copies of
// four lines differing only in that parameter is how the two drift.
//
// An EMPTY slice yields the placeholder "NULL", not "", because the latter
// renders `IN ()` — a syntax error. Callers guard on len today, so this is a
// guard rail rather than a live path: NULL matches nothing, which is the
// correct reading of "none of these ids".
func idPlaceholders[T any](ids []T) (placeholders string, args []any) {
	if len(ids) == 0 {
		return "NULL", nil
	}
	args = make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return strings.TrimSuffix(strings.Repeat("?,", len(ids)), ","), args
}

// relationLineageIDs returns every rel_record_id making up a relation's history,
// starting at headID and walking `rename` rows backward.
//
// A rename row (rel_record_id=N, op='rename', prev_from/prev_to = the old
// triple) links N's lineage to the predecessor that ended at that old triple.
//
// Since the store renames endpoints ATOMICALLY (a bulk in-place UPDATE of
// from_id/to_id), a relation KEEPS its rel_record_id across a rename, so the
// lineage is already continuous on one id and the rename row is just a marker.
// This walk therefore normally finds no fork; it stays as belt-and-braces for
// any historical or future path that DOES mint a fresh id on rename.
//
// Cycles are impossible (each hop strictly decreases the frontier's max vseq),
// but a visited set guards regardless.
func (v *VersionStore) relationLineageIDs(ctx context.Context, headID int64) ([]int64, error) {
	ids := []int64{headID}
	seen := map[int64]struct{}{headID: {}}
	frontier := []int64{headID}

	// The correlated subquery replaces pgstore's JOIN LATERAL: for each rename
	// row of `id`, find the newest lineage that carried the old triple before
	// that rename. Matching from_face too is what keeps a state-tailed edge from
	// stitching onto the default face's lineage.
	const q = `
		SELECT DISTINCT (
		    SELECT rv.rel_record_id
		    FROM relation_versions rv
		    WHERE rv.from_id   = ren.prev_from
		      AND rv.from_face = ren.from_face
		      AND rv.rel_type  = ren.rel_type
		      AND rv.to_id     = ren.prev_to
		      AND rv.vseq      < ren.vseq
		    ORDER BY rv.vseq DESC
		    LIMIT 1
		) AS pred
		FROM relation_versions ren
		WHERE ren.rel_record_id = ?
		  AND ren.op = 'rename'
		  AND ren.prev_from IS NOT NULL
		  AND ren.prev_to IS NOT NULL`

	for len(frontier) > 0 {
		id := frontier[0]
		frontier = frontier[1:]

		// NULLs are dropped: a rename with no resolvable predecessor (the
		// stitch target was purged, or this is the head of the chain) simply
		// contributes nothing to the frontier.
		preds, err := scanNullableIDs(ctx, v.db, q, id)
		if err != nil {
			return nil, fmt.Errorf("sqlitestore: walk relation lineage: %w", err)
		}
		for _, p := range preds {
			if _, dup := seen[p]; dup {
				continue
			}
			seen[p] = struct{}{}
			ids = append(ids, p)
			frontier = append(frontier, p)
		}
	}
	return ids, nil
}

// recordIDForKey resolves a composite key to the rel_record_id of its current
// (or most recent) lineage.
//
// Live rows are matched on the DEFAULT TAIL only, and that is deliberate: this
// resolves a (from, type, to) KEY, which is what a caller who did not name a
// face is asking about. A state-tailed sibling of the same triple is a DIFFERENT
// edge with its own lineage; answering with it would silently redirect a
// default-tail question to another face. A face-aware caller resolves its own
// record id and passes it directly.
//
// Nil: returns (0, store.ErrNotFound) when the key has neither a live row nor
// any history.
func (v *VersionStore) recordIDForKey(ctx context.Context, from, relType, to string) (int64, error) {
	const live = `SELECT rel_record_id FROM relations
	              WHERE from_id = ? AND rel_type = ? AND to_id = ? AND from_face = ''`
	var id int64
	err := v.db.QueryRowContext(ctx, live, from, relType, to).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	// Deleted: the most recent lineage whose FINAL row carries this key.
	// Matching the final row (not any historical one) is what avoids attaching
	// to a lineage that was renamed AWAY from this key.
	const dead = `
		SELECT rv.rel_record_id
		FROM relation_versions rv
		JOIN (SELECT rel_record_id, max(vseq) AS vseq
		      FROM relation_versions GROUP BY rel_record_id) latest
		  ON latest.rel_record_id = rv.rel_record_id AND latest.vseq = rv.vseq
		WHERE rv.from_id = ? AND rv.rel_type = ? AND rv.to_id = ? AND rv.from_face = ''
		ORDER BY rv.vseq DESC
		LIMIT 1`
	err = v.db.QueryRowContext(ctx, dead, from, relType, to).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, store.ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	return id, nil
}

// resolveLineageIDs turns a history query into the set of rel_record_ids making
// up the selected lifetime's stitched history.
//
// With RecordID == 0 it resolves the newest lifetime. With a non-zero RecordID
// it REQUIRES that id to be a head of this key — else [store.ErrNotFound] — so
// the composite key stays the authorization boundary and a caller cannot read an
// arbitrary lineage by guessing an id.
func (v *VersionStore) resolveLineageIDs(
	ctx context.Context, q store.RelationHistoryQuery,
) ([]int64, error) {
	head := q.RecordID
	if head == 0 {
		id, err := v.recordIDForKey(ctx, q.From, q.Type, q.To)
		if err != nil {
			return nil, err // ErrNotFound propagates
		}
		head = id
	} else {
		ok, err := v.recordIDIsHeadOfKey(ctx, head, q.From, q.Type, q.To)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, store.ErrNotFound
		}
	}
	return v.relationLineageIDs(ctx, head)
}

// recordIDIsHeadOfKey reports whether recordID is a lineage whose FINAL version
// row carries (from,type,to) — a valid lifetime handle for this key. This is the
// membership check that keeps a caller-supplied RecordID bounded to the key.
func (v *VersionStore) recordIDIsHeadOfKey(
	ctx context.Context, recordID int64, from, relType, to string,
) (bool, error) {
	const q = `
		SELECT EXISTS (
		    SELECT 1
		    FROM relation_versions rv
		    WHERE rv.rel_record_id = ?
		      AND rv.vseq = (SELECT max(vseq) FROM relation_versions WHERE rel_record_id = ?)
		      AND rv.from_id = ? AND rv.rel_type = ? AND rv.to_id = ?
		)`
	var ok bool
	if err := v.db.QueryRowContext(ctx, q, recordID, recordID, from, relType, to).Scan(&ok); err != nil {
		return false, err
	}
	return ok, nil
}

// --- Read -----------------------------------------------------------------

// ListRelationVersions implements [store.RelationHistoryReader].
func (v *VersionStore) ListRelationVersions(
	ctx context.Context, q store.RelationHistoryQuery,
) ([]store.RelationVersionMeta, error) {
	ids, err := v.resolveLineageIDs(ctx, q)
	if errors.Is(err, store.ErrNotFound) {
		// An unknown key (newest lifetime) is empty-not-error — the established
		// contract. A non-zero RecordID that is NOT a lifetime of this key is a
		// misuse of the handle: surface it rather than returning an empty
		// timeline that reads as "no history."
		if q.RecordID == 0 {
			return nil, nil
		}
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	ph, args := idPlaceholders(ids)
	// G202: the interpolated `ph` is idPlaceholders' output — a run of "?"
	// separated by commas, or the literal NULL. It never carries a bind value,
	// let alone caller input; the ids travel as args below. SQLite has no array
	// parameter, so expanding an IN list this way is the only option.
	//nolint:gosec // G202: placeholders only, ids are bound as arguments
	sel := `
		SELECT vseq, op, from_id, rel_type, to_id, prev_from, prev_to,
		       content_hash, schema_hash, principal_user, principal_tool,
		       triggered_by, created_at
		FROM relation_versions
		WHERE rel_record_id IN (` + ph + `)
		ORDER BY vseq ASC`
	rows, err := v.db.QueryContext(ctx, sel, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: list relation versions: %w", err)
	}
	defer rows.Close()

	var metas []store.RelationVersionMeta
	for rows.Next() {
		m, err := scanRelationVersionMeta(rows)
		if err != nil {
			return nil, err
		}
		metas = append(metas, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range metas {
		metas[i].Version = i + 1
	}
	return metas, nil
}

// GetRelationVersion implements [store.RelationHistoryReader]. version is a
// 1-based ordinal over the selected lifetime ordered by vseq.
func (v *VersionStore) GetRelationVersion(
	ctx context.Context, q store.RelationHistoryQuery, version int,
) (*store.RelationVersionSnapshot, error) {
	if version < 1 {
		return nil, store.ErrNotFound
	}
	ids, err := v.resolveLineageIDs(ctx, q)
	if err != nil {
		return nil, err // ErrNotFound propagates
	}

	ph, args := idPlaceholders(ids)
	//nolint:gosec // G202: placeholders only, ids are bound as arguments (see above)
	sel := `
		SELECT rv.op, rv.from_id, rv.rel_type, rv.to_id, rv.prev_from, rv.prev_to,
		       rv.content_hash, rv.schema_hash, rv.principal_user, rv.principal_tool,
		       rv.triggered_by, rv.created_at, rv.content, rv.properties, sv.projection
		FROM relation_versions rv
		JOIN schema_versions sv ON sv.hash = rv.schema_hash
		WHERE rv.rel_record_id IN (` + ph + `)
		ORDER BY rv.vseq ASC
		LIMIT 1 OFFSET ?`
	args = append(args, version-1)
	row := v.db.QueryRowContext(ctx, sel, args...)

	var (
		snap     store.RelationVersionSnapshot
		op       string
		prevFrom *string
		prevTo   *string
		props    string
		created  string
	)
	err = row.Scan(&op, &snap.From, &snap.Type, &snap.To, &prevFrom, &prevTo,
		&snap.ContentHash, &snap.SchemaHash, &snap.PrincipalUser, &snap.PrincipalTool,
		&snap.TriggeredBy, &created, &snap.Content, &props, &snap.Projection)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: get relation version %d: %w", version, err)
	}

	snap.Version = version
	snap.Op = store.VersionOp(op)
	if prevFrom != nil {
		snap.PrevFrom = *prevFrom
	}
	if prevTo != nil {
		snap.PrevTo = *prevTo
	}
	if snap.CreatedAt, err = time.Parse(timeFmt, created); err != nil {
		return nil, fmt.Errorf("sqlitestore: parse relation version created_at: %w", err)
	}
	if snap.Properties, err = unmarshalProps(props); err != nil {
		return nil, err
	}
	return &snap, nil
}

// ListRelationLifetimes implements [store.RelationHistoryReader]: it enumerates
// every past lifetime of a relation key, newest first, so a caller can discover
// that a reused triple has older deleted lifetimes and obtain a handle to read
// one.
//
// A "lifetime" is a stitched history head — a rel_record_id whose final version
// row still carries this key — with its rename predecessors folded in. Heads are
// walked newest-first and each head's whole stitched id-set is marked CLAIMED,
// so a lineage already folded into a newer lifetime is not also listed as its
// own (this guards a rename that cycles a triple back onto an earlier head).
//
// A lifetime is bounded by its id-set's count and timestamps, NOT by the
// presence of a create row: create/update rows come only from the async sweep,
// so a short-lived relation's lineage may hold only a delete.
func (v *VersionStore) ListRelationLifetimes(
	ctx context.Context, from, relType, to string,
) ([]store.RelationLifetime, error) {
	// Heads: every lineage whose FINAL row carries this key, newest-first.
	const headsQ = `
		SELECT rv.rel_record_id
		FROM relation_versions rv
		JOIN (SELECT rel_record_id, max(vseq) AS vseq
		      FROM relation_versions GROUP BY rel_record_id) latest
		  ON latest.rel_record_id = rv.rel_record_id AND latest.vseq = rv.vseq
		WHERE rv.from_id = ? AND rv.rel_type = ? AND rv.to_id = ?
		ORDER BY latest.vseq DESC`
	heads, err := scanIDs(ctx, v.db, headsQ, from, relType, to)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: list relation lifetimes: %w", err)
	}
	if len(heads) == 0 {
		return nil, nil
	}

	liveID, err := v.liveRecordID(ctx, from, relType, to)
	if err != nil {
		return nil, err
	}

	claimed := make(map[int64]struct{})
	lifetimes := make([]store.RelationLifetime, 0, len(heads))
	for _, head := range heads {
		if _, dup := claimed[head]; dup {
			continue // already folded into a newer lifetime's stitched set
		}
		ids, err := v.relationLineageIDs(ctx, head)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			claimed[id] = struct{}{}
		}
		lt, err := v.aggregateLifetime(ctx, ids)
		if err != nil {
			return nil, err
		}
		lt.RecordID = head
		lt.Live = liveID != 0 && liveID == head
		lifetimes = append(lifetimes, lt)
	}
	for i := range lifetimes {
		lifetimes[i].Lifetime = i + 1 // newest-first (heads were ordered DESC)
	}
	return lifetimes, nil
}

// liveRecordID returns the rel_record_id of the live relations row for this key,
// or 0 when the relation is not currently live.
func (v *VersionStore) liveRecordID(ctx context.Context, from, relType, to string) (int64, error) {
	const q = `SELECT rel_record_id FROM relations
	           WHERE from_id = ? AND rel_type = ? AND to_id = ? AND from_face = ''`
	var id int64
	err := v.db.QueryRowContext(ctx, q, from, relType, to).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return id, nil
}

// aggregateLifetime rolls a stitched id-set up into count, timestamps and final
// op (the op of the newest row across the set).
func (v *VersionStore) aggregateLifetime(ctx context.Context, ids []int64) (store.RelationLifetime, error) {
	ph, args := idPlaceholders(ids)
	q := `
		SELECT count(*), min(created_at), max(created_at),
		       (SELECT op FROM relation_versions
		        WHERE rel_record_id IN (` + ph + `) ORDER BY vseq DESC LIMIT 1)
		FROM relation_versions
		WHERE rel_record_id IN (` + ph + `)`

	var (
		lt          store.RelationLifetime
		finalOp     *string
		first, last *string
	)
	// Nullable scans, because the aggregate has an EMPTY-SET case that is
	// reachable rather than theoretical: purging a whole lineage leaves a head
	// whose rows are gone while a newer lifetime on the same key survives.
	// count(*) is then 0 and min/max are NULL, which a scan into string rejects
	// with a conversion error — failing the entire lifetime listing over a
	// lineage that simply has nothing left in it.
	//
	// The id set is bound twice: once for the correlated final-op subquery and
	// once for the outer aggregate.
	both := append(append([]any{}, args...), args...)
	if err := v.db.QueryRowContext(ctx, q, both...).Scan(
		&lt.VersionCount, &first, &last, &finalOp,
	); err != nil {
		return store.RelationLifetime{}, fmt.Errorf("sqlitestore: aggregate lifetime: %w", err)
	}
	if first == nil || last == nil {
		// No rows: a zero lifetime, with the count the query actually reported.
		return lt, nil
	}
	var err error
	if lt.FirstSeen, err = time.Parse(timeFmt, *first); err != nil {
		return store.RelationLifetime{}, fmt.Errorf("sqlitestore: parse lifetime first_seen: %w", err)
	}
	if lt.LastSeen, err = time.Parse(timeFmt, *last); err != nil {
		return store.RelationLifetime{}, fmt.Errorf("sqlitestore: parse lifetime last_seen: %w", err)
	}
	if finalOp != nil {
		lt.FinalOp = store.VersionOp(*finalOp)
	}
	return lt, nil
}

// scanRelationVersionMeta scans one relation-version metadata row. The leading
// column is vseq, scanned into a throwaway (the read-time ordinal replaces it).
func scanRelationVersionMeta(row scanner) (store.RelationVersionMeta, error) {
	var (
		m        store.RelationVersionMeta
		vseq     int64
		op       string
		prevFrom *string
		prevTo   *string
		created  string
	)
	if err := row.Scan(&vseq, &op, &m.From, &m.Type, &m.To, &prevFrom, &prevTo,
		&m.ContentHash, &m.SchemaHash, &m.PrincipalUser, &m.PrincipalTool,
		&m.TriggeredBy, &created); err != nil {
		return store.RelationVersionMeta{}, err
	}
	m.Op = store.VersionOp(op)
	if prevFrom != nil {
		m.PrevFrom = *prevFrom
	}
	if prevTo != nil {
		m.PrevTo = *prevTo
	}
	var err error
	if m.CreatedAt, err = time.Parse(timeFmt, created); err != nil {
		return store.RelationVersionMeta{}, fmt.Errorf("sqlitestore: parse relation version created_at: %w", err)
	}
	return m, nil
}

// RelationRecordID returns the surrogate lineage id of the live relation row for
// this triple's DEFAULT tail, or 0 when no such row exists.
//
// This is not part of [store.Store] — it is an accessor for the opaque handle
// callers otherwise obtain from [store.RelationLifetime]. It exists so a caller
// holding a live relation can attribute a synchronous capture to the right
// lineage without first reading history that may not exist yet (a relation
// deleted before the sweep ever ran has no version rows to resolve against).
//
// On VersionStore rather than Store, matching pgstore: Store carries a pinned
// plimsoll line whose doc says an added capability accessor should have to
// argue for itself, and this one does not need to be there — lineage is this
// type's concern. Keeping the two backends' accessor in the same place also
// lets storetest discover them with one lookup.
func (v *VersionStore) RelationRecordID(ctx context.Context, from, relType, to string) (int64, error) {
	return v.liveRecordID(ctx, from, relType, to)
}

// bumpRelRecordSeq consumes the relation-lineage id the caller just used.
//
// A counter table rather than AUTOINCREMENT because rel_record_id is a plain
// column on relations, not its rowid alias, and SQLite's AUTOINCREMENT applies
// only to the latter.
//
// Split from the READ deliberately. CreateRelation reads the counter inline in
// its INSERT and calls this only once that INSERT has succeeded, so a rejected
// duplicate triple consumes no id — see the comment there. A combined
// read-and-bump would have to run first, and its UPDATE autocommits outside a
// Tx, so the failed insert could not give the id back.
//
// The bump runs through q — the pinned transaction connection inside a Tx, the
// pool otherwise — so it takes the same single-writer discipline as every
// other write here.
func bumpRelRecordSeq(ctx context.Context, q querier) error {
	if _, err := q.ExecContext(ctx,
		`UPDATE rel_record_seq SET next = next + 1 WHERE id = 1`); err != nil {
		return fmt.Errorf("advance relation lineage counter: %w", err)
	}
	return nil
}

// scanIDs runs a single-column int64 query to completion.
//
// Extracted so the rows handle is closed by defer rather than by a Close call
// on each exit path — the shape sqlclosecheck flags, and the one that leaks a
// cursor the first time somebody adds a return.
func scanIDs(ctx context.Context, q querier, sql string, args ...any) ([]int64, error) {
	rows, err := q.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []int64
	for rows.Next() {
		var id int64
		if scanErr := rows.Scan(&id); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// scanNullableIDs is [scanIDs] for a column that may be NULL, dropping the
// NULLs. The caller decides what an absent value means; here it is always
// "no predecessor", which contributes nothing.
func scanNullableIDs(ctx context.Context, q querier, sql string, args ...any) ([]int64, error) {
	rows, err := q.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []int64
	for rows.Next() {
		var id *int64
		if scanErr := rows.Scan(&id); scanErr != nil {
			return nil, scanErr
		}
		if id != nil {
			out = append(out, *id)
		}
	}
	return out, rows.Err()
}
