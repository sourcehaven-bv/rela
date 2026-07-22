package pgstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Version purge (TKT-BW6UUL) — the audited, operator-only exception to the
// append-only history model. HARD-DELETES version snapshot rows for compliance
// redaction. Every guardrail here is load-bearing (design-review RR-SH28E /
// RR-EQQP1 / RR-ECUWV); do not relax one without re-reading the ticket:
//
//   - Runs the whole operation on ONE connection under sweepAdvisoryLockKey, so
//     it is mutually exclusive with a reconciliation sweep tick (a purge racing
//     a capture-insert is a lost-erasure hazard). Same session-scoped-lock
//     discipline as the sweep itself.
//   - REFUSES (deletes nothing) if the target set contains a `rename` row —
//     purging one orphans/forks the lineage walk. v1 is non-rename-only.
//   - REFUSES if a LIVE row still holds the content, unless ForceLive: otherwise
//     the sweep re-captures it within one interval and the "erasure" is a lie. A
//     ForceLive purge writes a no-content `purge` tombstone whose content_hash is
//     the live hash, so the sweep's existing dedup suppresses re-capture until
//     the live value genuinely changes again.
//   - --all purges the FENCED lineage (the same rows ListVersions shows), never
//     a naive WHERE id=$1 (which both misses pre-rename segments and destroys a
//     reused id's unrelated history).
//
// Attribution + Reason arrive in the request from ctx at the boundary; the store
// never learns the principal another way and never echoes purged content.

// PurgeVersions implements store.VersionPurger.
func (v *VersionStore) PurgeVersions(ctx context.Context, req store.VersionPurgeRequest) (*store.PurgeResult, error) {
	pool, ok := v.db.(*pgxpool.Pool)
	if !ok {
		return nil, errors.New("pgstore: PurgeVersions requires a *pgxpool.Pool (session-scoped advisory lock)")
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	locked, err := tryAdvisoryLock(ctx, conn, sweepAdvisoryLockKey)
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, errors.New("pgstore: purge could not acquire the version lock (a sweep is running); retry shortly")
	}
	defer advisoryUnlock(context.WithoutCancel(ctx), conn, sweepAdvisoryLockKey)

	// Resolve the target lineage ids (fenced) and the current live content hash.
	ids, err := v.entityLineageIDsForPurge(ctx, conn, req.EntityID)
	if err != nil {
		return nil, err
	}
	liveHash, liveExists, err := v.liveEntityHash(ctx, conn, req.EntityID)
	if err != nil {
		return nil, err
	}

	targets, err := selectPurgeTargets(ctx, conn, entityPurgeQ, ids, req.Selector)
	if err != nil {
		return nil, err
	}
	res := &store.PurgeResult{Targets: targets, LiveRowExists: liveExists}
	res.RenameInTargets = anyRename(targets)

	if req.DryRun {
		return res, nil
	}
	if res.RenameInTargets {
		return res, nil // refuse: caller renders the reason from the flag
	}
	if liveExists && !req.ForceLive {
		return res, nil // refuse: sweep would re-capture; caller renders the reason
	}

	n, err := deletePurgeTargets(ctx, conn, "entity_versions", "entity_id", ids, targets)
	if err != nil {
		return nil, err
	}
	res.Purged = n

	// ForceLive over a live row: write the no-content tombstone so the sweep
	// does not re-capture the live content (its content_hash = the live hash
	// dedups against the sweep's lvc probe).
	if liveExists && req.ForceLive {
		if err := writeEntityPurgeTombstone(ctx, conn, req.EntityID, liveHash); err != nil {
			return nil, err
		}
		res.TombstoneWritten = true
	}
	return res, nil
}

// PurgeRelationVersions implements store.RelationVersionPurger.
func (v *VersionStore) PurgeRelationVersions(
	ctx context.Context, req store.RelationVersionPurgeRequest,
) (*store.PurgeResult, error) {
	pool, ok := v.db.(*pgxpool.Pool)
	if !ok {
		return nil, errors.New("pgstore: PurgeRelationVersions requires a *pgxpool.Pool")
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	locked, err := tryAdvisoryLock(ctx, conn, sweepAdvisoryLockKey)
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, errors.New("pgstore: purge could not acquire the version lock (a sweep is running); retry shortly")
	}
	defer advisoryUnlock(context.WithoutCancel(ctx), conn, sweepAdvisoryLockKey)

	id, err := v.recordIDForKey(ctx, req.From, req.Type, req.To)
	if errors.Is(err, store.ErrNotFound) {
		return &store.PurgeResult{}, nil // nothing to purge
	}
	if err != nil {
		return nil, err
	}
	ids, err := v.relationLineageIDs(ctx, id)
	if err != nil {
		return nil, err
	}
	liveHash, liveExists, err := v.liveRelationHash(ctx, conn, req.From, req.Type, req.To)
	if err != nil {
		return nil, err
	}

	targets, err := selectPurgeTargets(ctx, conn, relationPurgeQ, ids, req.Selector)
	if err != nil {
		return nil, err
	}
	res := &store.PurgeResult{Targets: targets, LiveRowExists: liveExists}
	res.RenameInTargets = anyRename(targets)

	if req.DryRun {
		return res, nil
	}
	if res.RenameInTargets {
		return res, nil
	}
	if liveExists && !req.ForceLive {
		return res, nil
	}

	n, err := deletePurgeTargets(ctx, conn, "relation_versions", "rel_record_id", ids, targets)
	if err != nil {
		return nil, err
	}
	res.Purged = n

	if liveExists && req.ForceLive {
		if err := writeRelationPurgeTombstone(ctx, conn, req.From, req.Type, req.To, id, liveHash); err != nil {
			return nil, err
		}
		res.TombstoneWritten = true
	}
	return res, nil
}

// --- shared helpers ---

func anyRename(targets []store.PurgeTarget) bool {
	for _, t := range targets {
		if t.IsRename {
			return true
		}
	}
	return false
}

// entityLineageIDsForPurge returns the fenced set of (entity_id) segments of a
// lineage as a slice usable in `entity_id = ANY($1)`. It reuses lineageCTE so
// --all matches exactly what ListVersions shows and never spills into a reused
// id's rows. Returns just the queried id if it has no rename ancestry.
func (v *VersionStore) entityLineageIDsForPurge(ctx context.Context, q DBTX, id string) ([]string, error) {
	sel := lineageCTE + ` SELECT entity_id FROM lin`
	rows, err := q.Query(ctx, sel, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			return nil, err
		}
		ids = append(ids, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		ids = []string{id}
	}
	return ids, nil
}

func (v *VersionStore) liveEntityHash(ctx context.Context, q DBTX, id string) (hash string, exists bool, err error) {
	e, gErr := scanEntity(q.QueryRow(ctx,
		`SELECT id, type, properties, content, updated_at FROM entities WHERE id = $1`, id))
	if errors.Is(gErr, pgx.ErrNoRows) {
		return "", false, nil
	}
	if gErr != nil {
		return "", false, gErr
	}
	return contentHashOf(store.VersionInput{
		EntityID: e.ID, Type: e.Type, Content: e.Content, Properties: e.Properties,
	}), true, nil
}

func (v *VersionStore) liveRelationHash(
	ctx context.Context, q DBTX, from, relType, to string,
) (hash string, exists bool, err error) {
	r, gErr := scanRelation(q.QueryRow(ctx,
		`SELECT from_id, rel_type, to_id, properties, content, updated_at
		 FROM relations WHERE from_id=$1 AND rel_type=$2 AND to_id=$3`, from, relType, to))
	if errors.Is(gErr, pgx.ErrNoRows) {
		return "", false, nil
	}
	if gErr != nil {
		return "", false, gErr
	}
	return contentHashOfRelation(store.RelationVersionInput{
		From: r.From, Type: r.Type, To: r.To, Content: r.Content, Properties: r.Properties,
	}), true, nil
}

// entityPurgeQ / relationPurgeQ select the resolvable target metadata (never the
// snapshot content) for a lineage id-set, applying the selector. `$1` is the
// id-set (ANY), the selector's vseq/content-hash are appended as $2.
const entityPurgeQ = `
	SELECT vseq, op, content_hash, created_at
	FROM entity_versions
	WHERE entity_id = ANY($1)`
const relationPurgeQ = `
	SELECT vseq, op, content_hash, created_at
	FROM relation_versions
	WHERE rel_record_id = ANY($1)`

// selectPurgeTargets resolves which rows the selector picks, ordered by vseq.
// It never selects content. idSet is []string for entities, []int64 for
// relations — passed through as `ANY`.
func selectPurgeTargets(
	ctx context.Context, q DBTX, baseQ string, idSet any, sel store.PurgeSelector,
) ([]store.PurgeTarget, error) {
	query := baseQ
	args := []any{idSet}
	switch {
	case sel.All:
		// no extra predicate — whole fenced lineage
	case sel.Vseq != 0:
		query += ` AND vseq = $2`
		args = append(args, sel.Vseq)
	case sel.ContentHash != "":
		query += ` AND content_hash = $2`
		args = append(args, sel.ContentHash)
	default:
		return nil, errors.New("pgstore: purge selector must set one of Vseq / ContentHash / All")
	}
	query += ` ORDER BY vseq ASC`

	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.PurgeTarget
	for rows.Next() {
		var t store.PurgeTarget
		var op string
		if err := rows.Scan(&t.Vseq, &op, &t.ContentHash, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.Op = store.VersionOp(op)
		t.IsRename = t.Op == store.VersionOpRename
		out = append(out, t)
	}
	return out, rows.Err()
}

// deletePurgeTargets deletes exactly the resolved target vseqs within the fenced
// lineage id-set. Deleting by (id-set, vseq IN targets) rather than re-running
// the selector guarantees we delete precisely what the dry-run/audit reported,
// even if a concurrent write landed (it can't — we hold the advisory lock — but
// the invariant is explicit). idCol is entity_id | rel_record_id.
func deletePurgeTargets(
	ctx context.Context, q DBTX, table, idCol string, idSet any, targets []store.PurgeTarget,
) (int, error) {
	if len(targets) == 0 {
		return 0, nil
	}
	vseqs := make([]int64, len(targets))
	for i, t := range targets {
		vseqs[i] = t.Vseq
	}
	del := fmt.Sprintf(
		`DELETE FROM %s WHERE %s = ANY($1) AND vseq = ANY($2)`, table, idCol)
	tag, err := q.Exec(ctx, del, idSet, vseqs)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// writeEntityPurgeTombstone appends a no-content `purge` row whose content_hash
// equals the live hash, so the sweep's dedup (lvc.content_hash == live hash)
// suppresses re-capture until the live value genuinely changes. Needs a
// schema_hash for the FK; reuse a stable sentinel projection row.
func writeEntityPurgeTombstone(ctx context.Context, q DBTX, id, liveHash string) error {
	if err := ensureSchemaVersion(ctx, q, purgeSchemaHash, purgeSchemaProjection); err != nil {
		return err
	}
	_, err := q.Exec(ctx, `
		INSERT INTO entity_versions
		    (entity_id, op, type, content, properties, content_hash, schema_hash,
		     principal_user, principal_tool, triggered_by)
		VALUES ($1, 'purge', '', '', '{}'::jsonb, $2, $3, '', 'version-purge', '')`,
		id, liveHash, purgeSchemaHash)
	return err
}

func writeRelationPurgeTombstone(
	ctx context.Context, q DBTX, from, relType, to string, recordID int64, liveHash string,
) error {
	if err := ensureSchemaVersion(ctx, q, purgeSchemaHash, purgeSchemaProjection); err != nil {
		return err
	}
	_, err := q.Exec(ctx, `
		INSERT INTO relation_versions
		    (rel_record_id, op, from_id, rel_type, to_id, content, properties,
		     content_hash, schema_hash, principal_user, principal_tool, triggered_by)
		VALUES ($1, 'purge', $2, $3, $4, '', '{}'::jsonb, $5, $6, '', 'version-purge', '')`,
		recordID, from, relType, to, liveHash, purgeSchemaHash)
	return err
}

// A stable sentinel schema projection for tombstone rows (they render nothing;
// the FK just needs a resolvable hash). Deduped into schema_versions once.
const purgeSchemaHash = "purge-tombstone"

var purgeSchemaProjection = []byte(`{"purge":true}`)
