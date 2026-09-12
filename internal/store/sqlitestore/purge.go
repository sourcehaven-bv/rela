package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Version purge (the SQLite half of TKT-BW6UUL) — the audited, operator-only
// exception to the append-only history model. It HARD-DELETES version snapshot
// rows for compliance redaction.
//
// Every guardrail here is load-bearing, and each came out of design review on
// the pgstore implementation. Do not relax one without re-reading that ticket:
//
//   - The whole operation is mutually exclusive with a reconciliation sweep
//     tick. A purge racing a capture-insert is a lost-erasure hazard: the sweep
//     re-writes the very content the purge just removed.
//   - REFUSES (deleting nothing) if the target set contains a `rename` row —
//     purging one orphans or forks the lineage walk. v1 is non-rename-only.
//   - REFUSES if a LIVE row still holds the content, unless ForceLive:
//     otherwise the sweep re-captures it within one interval and the "erasure"
//     is a lie. A ForceLive purge writes a no-content `purge` tombstone whose
//     content_hash IS the live hash, so the sweep's existing dedup suppresses
//     re-capture until the live value genuinely changes again.
//   - `--all` purges the FENCED lineage (exactly the rows ListVersions shows),
//     never a naive `WHERE entity_id = ?`, which would both miss pre-rename
//     segments and destroy a reused id's unrelated history.
//
// Attribution and Reason arrive in the request, populated from ctx at the
// boundary: the store never learns the principal another way, and never echoes
// purged content.
//
// # Where this diverges from pgstore, and why it is still sound
//
// pgstore serializes purge against the sweep with a session-scoped
// `pg_try_advisory_lock`, because several server processes may share one
// database. sqlitestore holds an exclusive sidecar lock at Open and REFUSES a
// second process, so the only writers that can race are goroutines
// inside THIS process — and an in-process mutex is the exact tool for that. The
// mutex is owned by the Store and shared with the sweep (see sweep.go), so
// "purge excludes a sweep tick" holds for the same reason it does on pgstore,
// with a cheaper mechanism rather than a weaker guarantee.

// PurgeVersions implements [store.VersionPurger].
func (v *VersionStore) PurgeVersions(
	ctx context.Context, req store.VersionPurgeRequest,
) (*store.PurgeResult, error) {
	// Mutual exclusion with the sweep. Held for the WHOLE operation — resolve,
	// decide, delete — because a target set resolved under the lock and deleted
	// outside it is exactly the race the lock exists to prevent.
	v.store.versionMu.Lock()
	defer v.store.versionMu.Unlock()

	ids, err := v.entityLineageIDsForPurge(ctx, req.EntityID, req.Face)
	if err != nil {
		return nil, err
	}
	liveHash, liveExists, err := v.liveEntityHash(ctx, req.EntityID, req.Face)
	if err != nil {
		return nil, err
	}

	idPH, idArgs := idPlaceholders(ids)
	baseQ := `
		SELECT vseq, op, content_hash, created_at
		FROM entity_versions
		WHERE entity_id IN (` + idPH + `) AND face = ?`
	baseArgs := append(idArgs, string(req.Face))

	targets, err := selectPurgeTargets(ctx, v.db, baseQ, baseArgs, req.Selector)
	if err != nil {
		return nil, err
	}
	res := &store.PurgeResult{Targets: targets, LiveRowExists: liveExists}
	res.RenameInTargets = anyRename(targets)

	if req.DryRun {
		return res, nil
	}
	if res.RenameInTargets {
		return res, nil // refuse: the caller renders the reason from the flag
	}
	if liveExists && !req.ForceLive {
		return res, nil // refuse: the sweep would re-capture it
	}

	// The delete and its tombstone commit TOGETHER or not at all.
	//
	// Two autocommit statements would leave a window — a crash, a cancelled
	// context, a failed insert — in which the history is gone and no tombstone
	// exists. The sweep then re-captures the live content within one interval
	// and the irreversible erasure quietly un-does itself. versionMu gives
	// mutual exclusion against a concurrent sweep, which is a different
	// property and survives neither a crash nor an early return.
	n, tombstoned, err := v.purgeTx(ctx, func(tx querier) (int, bool, error) {
		deleted, derr := deletePurgeTargets(ctx, tx, "entity_versions", "entity_id", idArgs, targets)
		if derr != nil {
			return 0, false, derr
		}
		if !liveExists || !req.ForceLive {
			return deleted, false, nil
		}
		if terr := writeEntityPurgeTombstone(ctx, tx, req.EntityID, req.Face, liveHash); terr != nil {
			return 0, false, terr
		}
		return deleted, true, nil
	})
	if err != nil {
		return nil, err
	}
	res.Purged = n
	res.TombstoneWritten = tombstoned
	return res, nil
}

// purgeTx runs the mutating half of a purge in one transaction.
//
// It exists so the delete and the sweep-suppressing tombstone cannot land
// separately. The callback returns the row count and whether it wrote a
// tombstone; neither is applied to the result until the commit succeeds, so a
// rolled-back purge reports nothing purged rather than a count for rows that
// are still there.
func (v *VersionStore) purgeTx(
	ctx context.Context, fn func(tx querier) (int, bool, error),
) (purged int, tombstoned bool, err error) {
	tx, err := v.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, fmt.Errorf("sqlitestore: begin purge: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	purged, tombstoned, err = fn(tx)
	if err != nil {
		return 0, false, err
	}
	if err := tx.Commit(); err != nil {
		return 0, false, fmt.Errorf("sqlitestore: commit purge: %w", err)
	}
	return purged, tombstoned, nil
}

// PurgeRelationVersions implements [store.RelationVersionPurger].
func (v *VersionStore) PurgeRelationVersions(
	ctx context.Context, req store.RelationVersionPurgeRequest,
) (*store.PurgeResult, error) {
	v.store.versionMu.Lock()
	defer v.store.versionMu.Unlock()

	ids, refused, err := v.resolvePurgeLineage(ctx, req)
	if err != nil {
		return nil, err
	}
	if refused != nil {
		return refused, nil // MultiLifetimeRefused, nothing purged
	}
	if len(ids) == 0 {
		return &store.PurgeResult{}, nil // unknown key
	}

	liveHash, liveExists, err := v.liveRelationHash(ctx, req.From, req.Type, req.To)
	if err != nil {
		return nil, err
	}
	// Resolved BEFORE the delete, not after. The tombstone names the lineage
	// the live row belongs to, and reading that after the delete has already
	// committed would be answering a question about a world the delete has
	// changed — as well as leaving the erasure un-tombstoned if the read fails.
	liveID, err := v.liveRecordID(ctx, req.From, req.Type, req.To)
	if err != nil {
		return nil, err
	}

	idPH, idArgs := idPlaceholders(ids)
	baseQ := `
		SELECT vseq, op, content_hash, created_at
		FROM relation_versions
		WHERE rel_record_id IN (` + idPH + `)`

	targets, err := selectPurgeTargets(ctx, v.db, baseQ, idArgs, req.Selector)
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

	// One transaction, for the reason PurgeVersions documents: a committed
	// delete with no tombstone is an erasure the sweep undoes.
	n, tombstoned, err := v.purgeTx(ctx, func(tx querier) (int, bool, error) {
		deleted, derr := deletePurgeTargets(ctx, tx, "relation_versions", "rel_record_id", idArgs, targets)
		if derr != nil {
			return 0, false, derr
		}
		// The live row's lineage is the newest lifetime; tombstone it so the
		// sweep does not re-capture the purged content.
		if !liveExists || !req.ForceLive || liveID == 0 {
			return deleted, false, nil
		}
		if terr := writeRelationPurgeTombstone(
			ctx, tx, req.From, req.Type, req.To, liveID, liveHash); terr != nil {
			return 0, false, terr
		}
		return deleted, true, nil
	})
	if err != nil {
		return nil, err
	}
	res.Purged = n
	res.TombstoneWritten = tombstoned
	return res, nil
}

// resolvePurgeLineage resolves a relation purge request to the rel_record_ids to
// purge, enforcing the multi-lifetime guardrail.
//
// Returns (ids, nil, nil) to proceed; (nil, refusal, nil) when a multi-lifetime
// key lacks a selector; (nil, nil, nil) for an unknown key.
func (v *VersionStore) resolvePurgeLineage(
	ctx context.Context, req store.RelationVersionPurgeRequest,
) (ids []int64, refused *store.PurgeResult, err error) {
	// The store is the trust boundary, so the contradictory selector is rejected
	// HERE and not only in the CLI — the request struct is public API.
	if req.AllLifetimes && req.RecordID != 0 {
		return nil, nil, errors.New("sqlitestore: RecordID and AllLifetimes are mutually exclusive")
	}
	lifetimes, err := v.ListRelationLifetimes(ctx, req.From, req.Type, req.To)
	if err != nil {
		return nil, nil, err
	}
	if len(lifetimes) == 0 {
		return nil, nil, nil // unknown key
	}

	switch {
	case req.AllLifetimes:
		// ListRelationLifetimes emits heads with disjoint stitched id-sets, so
		// there are no duplicates today. Dedup defensively anyway: a future
		// change to the claimed-set logic must not feed duplicate ids into the
		// delete.
		seen := make(map[int64]struct{})
		for _, lt := range lifetimes {
			lineage, lerr := v.relationLineageIDs(ctx, lt.RecordID)
			if lerr != nil {
				return nil, nil, lerr
			}
			for _, id := range lineage {
				if _, dup := seen[id]; dup {
					continue
				}
				seen[id] = struct{}{}
				ids = append(ids, id)
			}
		}
		return ids, nil, nil

	case req.RecordID != 0:
		ok, verr := v.recordIDIsHeadOfKey(ctx, req.RecordID, req.From, req.Type, req.To)
		if verr != nil {
			return nil, nil, verr
		}
		if !ok {
			return nil, nil, store.ErrNotFound
		}
		lineage, lerr := v.relationLineageIDs(ctx, req.RecordID)
		if lerr != nil {
			return nil, nil, lerr
		}
		return lineage, nil, nil

	case len(lifetimes) > 1:
		// Multi-lifetime key with no selector. Purging the newest only would
		// leave older lifetimes' content behind while reporting success — a
		// false compliance guarantee, so refuse and make the caller choose.
		return nil, &store.PurgeResult{
			MultiLifetimeRefused: true,
			LifetimeCount:        len(lifetimes),
		}, nil

	default:
		lineage, lerr := v.relationLineageIDs(ctx, lifetimes[0].RecordID)
		if lerr != nil {
			return nil, nil, lerr
		}
		return lineage, nil, nil
	}
}

// --- shared helpers -------------------------------------------------------

// anyRename reports whether the target set contains a rename row.
func anyRename(targets []store.PurgeTarget) bool {
	for _, t := range targets {
		if t.IsRename {
			return true
		}
	}
	return false
}

// entityLineageIDsForPurge returns the fenced set of entity_id segments of a
// lineage. It reuses lineageCTE so `--all` matches exactly what ListVersions
// shows and never spills into a reused id's rows.
//
// Falls back to the queried id alone when it has no rename ancestry.
func (v *VersionStore) entityLineageIDsForPurge(
	ctx context.Context, id string, p entity.Face,
) ([]string, error) {
	sel := lineageCTE + ` SELECT entity_id FROM lin`
	rows, err := v.db.QueryContext(ctx, sel, lineageArgs(id, p)...)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: resolve purge lineage: %w", err)
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

// liveEntityHash returns the content hash of the live row for (id, face), and
// whether such a row exists.
func (v *VersionStore) liveEntityHash(
	ctx context.Context, id string, p entity.Face,
) (hash string, exists bool, err error) {
	e, gErr := scanEntity(v.db.QueryRowContext(ctx,
		`SELECT id, face, type, properties, content, updated_at
		 FROM entities WHERE id = ? AND face = ?`, id, string(p)))
	if errors.Is(gErr, sql.ErrNoRows) {
		return "", false, nil
	}
	if gErr != nil {
		return "", false, gErr
	}
	// The face MUST travel into the hash: contentHashOf folds it in, so omitting
	// it here would make a ForceLive tombstone on one face carry a hash matching
	// a SIBLING face holding identical bytes — suppressing that sibling's
	// legitimate sweep capture.
	return contentHashOf(store.VersionInput{
		EntityID: e.ID, Face: e.Face, Type: e.Type,
		Content: e.Content, Properties: e.Properties,
	}), true, nil
}

// liveRelationHash returns the content hash of the live default-tail relation
// row for this key, and whether it exists.
func (v *VersionStore) liveRelationHash(
	ctx context.Context, from, relType, to string,
) (hash string, exists bool, err error) {
	r, gErr := scanRelation(v.db.QueryRowContext(ctx,
		`SELECT from_id, from_face, rel_type, to_id, properties, content, updated_at
		 FROM relations WHERE from_id = ? AND rel_type = ? AND to_id = ? AND from_face = ''`,
		from, relType, to))
	if errors.Is(gErr, sql.ErrNoRows) {
		return "", false, nil
	}
	if gErr != nil {
		return "", false, gErr
	}
	// FromFace is carried from the row rather than left zero, mirroring the
	// entity hash above. The query pins from_face = '' so the zero value would
	// be right today — which is exactly the problem: it would be right by
	// coincidence, and contentHashOfRelation folds the tail into the hash, so
	// relaxing that predicate later would silently produce a hash that
	// suppresses a DIFFERENT lineage's sweep capture.
	return contentHashOfRelation(store.RelationVersionInput{
		From: r.From, FromFace: r.FromFace, Type: r.Type, To: r.To,
		Content: r.Content, Properties: r.Properties,
	}), true, nil
}

// selectPurgeTargets resolves which rows the selector picks, ordered by vseq. It
// never selects content — only the metadata a dry-run and the audit record need.
func selectPurgeTargets(
	ctx context.Context, db querier, baseQ string, baseArgs []any, sel store.PurgeSelector,
) ([]store.PurgeTarget, error) {
	query := baseQ
	args := append([]any(nil), baseArgs...)
	switch {
	case sel.All:
		// no extra predicate — the whole fenced lineage
	case sel.Vseq != 0:
		query += ` AND vseq = ?`
		args = append(args, sel.Vseq)
	case sel.ContentHash != "":
		query += ` AND content_hash = ?`
		args = append(args, sel.ContentHash)
	default:
		return nil, errors.New("sqlitestore: purge selector must set one of Vseq / ContentHash / All")
	}
	query += ` ORDER BY vseq ASC`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: select purge targets: %w", err)
	}
	defer rows.Close()

	var out []store.PurgeTarget
	for rows.Next() {
		var (
			t       store.PurgeTarget
			op      string
			created string
		)
		if err := rows.Scan(&t.Vseq, &op, &t.ContentHash, &created); err != nil {
			return nil, err
		}
		t.Op = store.VersionOp(op)
		t.IsRename = t.Op == store.VersionOpRename
		if t.CreatedAt, err = time.Parse(timeFmt, created); err != nil {
			return nil, fmt.Errorf("sqlitestore: parse purge target created_at: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// deletePurgeTargets deletes exactly the resolved target vseqs within the fenced
// lineage id-set.
//
// Deleting by (id-set, vseq IN targets) rather than re-running the selector is
// what guarantees we delete precisely what the dry-run and the audit record
// reported. Nothing can land in between (the version mutex is held), but the
// invariant is expressed in the statement rather than left to that fact.
func deletePurgeTargets(
	ctx context.Context, db querier, table, idCol string, idArgs []any, targets []store.PurgeTarget,
) (int, error) {
	if len(targets) == 0 {
		return 0, nil
	}
	vseqs := make([]int64, len(targets))
	for i, t := range targets {
		vseqs[i] = t.Vseq
	}
	idPH := strings.TrimSuffix(strings.Repeat("?,", len(idArgs)), ",")
	vseqPH, vseqArgs := idPlaceholders(vseqs)

	del := fmt.Sprintf(`DELETE FROM %s WHERE %s IN (%s) AND vseq IN (%s)`,
		table, idCol, idPH, vseqPH)
	args := append(append([]any(nil), idArgs...), vseqArgs...)

	res, err := db.ExecContext(ctx, del, args...)
	if err != nil {
		return 0, fmt.Errorf("sqlitestore: delete purge targets: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// writeEntityPurgeTombstone appends a no-content `purge` row whose content_hash
// equals the live hash, so the sweep's dedup suppresses re-capture until the
// live value genuinely changes.
func writeEntityPurgeTombstone(
	ctx context.Context, db querier, id string, p entity.Face, liveHash string,
) error {
	now := timestampNow()
	if err := ensureSchemaVersion(ctx, db, purgeSchemaHash, purgeSchemaProjection, now); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO entity_versions
		    (entity_id, face, op, type, content, properties, content_hash,
		     schema_hash, principal_user, principal_tool, triggered_by, created_at)
		VALUES (?, ?, 'purge', '', '', '{}', ?, ?, '', 'version-purge', '', ?)`,
		id, string(p), liveHash, purgeSchemaHash, now)
	if err != nil {
		return fmt.Errorf("sqlitestore: write purge tombstone: %w", err)
	}
	return nil
}

// writeRelationPurgeTombstone is [writeEntityPurgeTombstone] for a relation
// lineage.
func writeRelationPurgeTombstone(
	ctx context.Context, db querier, from, relType, to string, recordID int64, liveHash string,
) error {
	now := timestampNow()
	if err := ensureSchemaVersion(ctx, db, purgeSchemaHash, purgeSchemaProjection, now); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO relation_versions
		    (rel_record_id, op, from_id, rel_type, to_id, content, properties,
		     content_hash, schema_hash, principal_user, principal_tool, triggered_by, created_at)
		VALUES (?, 'purge', ?, ?, ?, '', '{}', ?, ?, '', 'version-purge', '', ?)`,
		recordID, from, relType, to, liveHash, purgeSchemaHash, now)
	if err != nil {
		return fmt.Errorf("sqlitestore: write relation purge tombstone: %w", err)
	}
	return nil
}

// purgeSchemaHash / purgeSchemaProjection are a stable sentinel schema
// projection for tombstone rows. They render nothing; the foreign key just needs
// a resolvable hash. Deduped into schema_versions once.
const purgeSchemaHash = "purge-tombstone"

var purgeSchemaProjection = []byte(`{"purge":true}`)
