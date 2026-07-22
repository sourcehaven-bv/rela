package pgstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// WriteRelationVersion implements store.RelationVersionWriter: it persists one
// synchronously captured relation version (delete/rename). The schema-dedup
// INSERT and the relation_versions INSERT run in one transaction so the row is
// all-or-nothing. The content hash is computed here from the snapshot.
//
// RecordID is the surrogate lineage id. When it is 0 the store resolves it from
// the composite key via recordIDForKey — which is correct for a SYNCHRONOUS
// capture taken BEFORE a delete (the live row still carries the id) or during a
// rename (resolved from the pre-rename key). Callers that capture after the row
// is gone must supply a non-zero RecordID read earlier.
func (v *VersionStore) WriteRelationVersion(ctx context.Context, in store.RelationVersionInput) error {
	if in.RecordID == 0 {
		// Resolve from the (still-live, pre-delete) row or the most-recent lineage.
		id, err := v.recordIDForKey(ctx, in.From, in.Type, in.To)
		if err != nil {
			return fmt.Errorf("pgstore: resolve rel_record_id for %s--%s--%s: %w",
				in.From, in.Type, in.To, err)
		}
		in.RecordID = id
	}
	tx, err := v.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op
	if err := insertRelationVersion(ctx, tx, in, contentHashOfRelation(in)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// insertRelationVersion writes one relation_versions row within q (a pool conn
// or tx). The caller supplies the content hash; vseq/created_at default in SQL.
func insertRelationVersion(ctx context.Context, q DBTX, in store.RelationVersionInput, contentHash string) error {
	props, err := marshalProps(in.Properties)
	if err != nil {
		return err
	}
	if err = ensureSchemaVersion(ctx, q, in.SchemaHash, in.Projection); err != nil {
		return fmt.Errorf("pgstore: ensure schema_version: %w", err)
	}
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
		    (rel_record_id, op, from_id, rel_type, to_id, prev_from, prev_to,
		     content, properties, content_hash, schema_hash,
		     principal_user, principal_tool, triggered_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`
	_, err = q.Exec(ctx, ins,
		in.RecordID, string(in.Op), in.From, in.Type, in.To, prevFrom, prevTo,
		in.Content, props, contentHash, in.SchemaHash,
		in.PrincipalUser, in.PrincipalTool, in.TriggeredBy)
	return err
}

// contentHashOfRelation computes the canonical content hash of a relation
// snapshot, matching the live relation hash so a sweep can compare against the
// current relation's hash. HashRelation folds in the (from,type,to) triple, so
// two distinct relations with identical props/content do not collide.
func contentHashOfRelation(in store.RelationVersionInput) string {
	return canonical.HashRelation(entity.Relation{
		From:       in.From,
		Type:       in.Type,
		To:         in.To,
		Properties: in.Properties,
		Content:    in.Content,
	})
}

// --- store.RelationHistoryReader ---
//
// Unlike entity history, relation lineage needs NO recursive rename-fencing CTE:
// rel_record_id is a stable surrogate carried across delete+recreate (a fresh id
// is minted), so it fences those lifecycles apart for free.
//
// Since #1127 an endpoint RENAME is an ATOMIC in-place `UPDATE relations SET
// from_id/to_id=...` (see entity.go RenameEntity), so the relation KEEPS its
// rel_record_id across the rename — the lineage is already continuous on one id
// and the `rename` version row (carrying prev_from/prev_to = the old triple) is
// just a marker on that same lineage, not a fork. The predecessor-lineage walk
// below (relationLineageIDs) therefore normally finds no fork on the current
// live path; it remains as belt-and-braces for any historical or future path
// that DOES mint a fresh id on rename (the pre-#1127 delete-old + create-new
// decomposition, whose rows may still exist in older databases). A relation's
// full history is therefore the current rel_record_id PLUS every predecessor
// lineage reachable by following those rename links back — see
// relationLineageIDs.

// relationLineageIDs returns every rel_record_id that makes up a relation's
// history, starting from headID and walking `rename` rows backward: a rename row
// (rel_record_id=N, op='rename', prev_from/prev_to=old triple) links N's lineage
// to the predecessor lineage that ended at that old triple. The predecessor is
// the lineage whose latest row BEFORE the rename carried (prev_from, type,
// prev_to). Cycles are impossible (each hop strictly decreases the rel_record_id
// frontier's max vseq), but a visited-set guards regardless.
func (v *VersionStore) relationLineageIDs(ctx context.Context, headID int64) ([]int64, error) {
	ids := []int64{headID}
	seen := map[int64]struct{}{headID: {}}
	frontier := []int64{headID}
	for len(frontier) > 0 {
		id := frontier[0]
		frontier = frontier[1:]
		// Predecessor lineages linked from id's rename rows.
		const q = `
			SELECT DISTINCT pred.rel_record_id
			FROM relation_versions ren
			JOIN LATERAL (
			    -- the predecessor lineage: rows carrying the old triple, whose OWN
			    -- latest row precedes this rename. Pick the newest such lineage.
			    SELECT rv.rel_record_id
			    FROM relation_versions rv
			    WHERE rv.from_id = ren.prev_from
			      AND rv.rel_type = ren.rel_type
			      AND rv.to_id   = ren.prev_to
			      AND rv.vseq < ren.vseq
			    ORDER BY rv.vseq DESC
			    LIMIT 1
			) pred ON true
			WHERE ren.rel_record_id = $1
			  AND ren.op = 'rename'
			  AND ren.prev_from IS NOT NULL
			  AND ren.prev_to IS NOT NULL`
		rows, err := v.db.Query(ctx, q, id)
		if err != nil {
			return nil, err
		}
		var preds []int64
		for rows.Next() {
			var p int64
			if err := rows.Scan(&p); err != nil {
				rows.Close()
				return nil, err
			}
			preds = append(preds, p)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
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

// recordIDForKey resolves a relation's composite key to the rel_record_id of its
// current (or most-recent) lineage. For a LIVE relation it reads the id off the
// relations row. For a DELETED relation (row gone) it falls back to the most
// recent relation_versions lineage that ended at this key — i.e. the largest
// rel_record_id whose latest row still carries this (from,type,to). Returns
// (0, ErrNotFound) when the key has no live row and no history.
func (v *VersionStore) recordIDForKey(ctx context.Context, from, relType, to string) (int64, error) {
	// Live row first.
	const live = `SELECT rel_record_id FROM relations
	              WHERE from_id = $1 AND rel_type = $2 AND to_id = $3`
	var id int64
	err := v.db.QueryRow(ctx, live, from, relType, to).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	// Deleted: the most recent lineage whose FINAL row matches this key. Ordering
	// by vseq DESC picks the newest lineage; matching the final row's endpoints
	// (not any historical row) avoids attaching to a lineage that was renamed
	// AWAY from this key.
	const dead = `
		SELECT rv.rel_record_id
		FROM relation_versions rv
		JOIN (
		    SELECT rel_record_id, max(vseq) AS vseq
		    FROM relation_versions GROUP BY rel_record_id
		) latest ON latest.rel_record_id = rv.rel_record_id AND latest.vseq = rv.vseq
		WHERE rv.from_id = $1 AND rv.rel_type = $2 AND rv.to_id = $3
		ORDER BY rv.vseq DESC
		LIMIT 1`
	err = v.db.QueryRow(ctx, dead, from, relType, to).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, store.ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	return id, nil
}

// ListRelationVersions implements store.RelationHistoryReader.
func (v *VersionStore) ListRelationVersions(
	ctx context.Context, from, relType, to string,
) ([]store.RelationVersionMeta, error) {
	id, err := v.recordIDForKey(ctx, from, relType, to)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil // no history is an empty slice, not an error
	}
	if err != nil {
		return nil, err
	}
	ids, err := v.relationLineageIDs(ctx, id)
	if err != nil {
		return nil, err
	}

	const sel = `
		SELECT vseq, op, from_id, rel_type, to_id, prev_from, prev_to,
		       content_hash, schema_hash, principal_user, principal_tool,
		       triggered_by, created_at
		FROM relation_versions
		WHERE rel_record_id = ANY($1)
		ORDER BY vseq ASC`
	rows, err := v.db.Query(ctx, sel, ids)
	if err != nil {
		return nil, err
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

// GetRelationVersion implements store.RelationHistoryReader. version is a 1-based
// ordinal over the lineage ordered by vseq.
func (v *VersionStore) GetRelationVersion(
	ctx context.Context, from, relType, to string, version int,
) (*store.RelationVersionSnapshot, error) {
	if version < 1 {
		return nil, store.ErrNotFound
	}
	id, err := v.recordIDForKey(ctx, from, relType, to)
	if err != nil {
		return nil, err // ErrNotFound propagates
	}
	ids, err := v.relationLineageIDs(ctx, id)
	if err != nil {
		return nil, err
	}

	const sel = `
		SELECT rv.op, rv.from_id, rv.rel_type, rv.to_id, rv.prev_from, rv.prev_to,
		       rv.content_hash, rv.schema_hash, rv.principal_user, rv.principal_tool,
		       rv.triggered_by, rv.created_at, rv.content, rv.properties, sv.projection
		FROM relation_versions rv
		JOIN schema_versions sv ON sv.hash = rv.schema_hash
		WHERE rv.rel_record_id = ANY($1)
		ORDER BY rv.vseq ASC
		OFFSET $2 LIMIT 1`
	row := v.db.QueryRow(ctx, sel, ids, version-1)

	var (
		snap     store.RelationVersionSnapshot
		op       string
		prevFrom *string
		prevTo   *string
		props    []byte
	)
	err = row.Scan(&op, &snap.From, &snap.Type, &snap.To, &prevFrom, &prevTo,
		&snap.ContentHash, &snap.SchemaHash, &snap.PrincipalUser, &snap.PrincipalTool,
		&snap.TriggeredBy, &snap.CreatedAt, &snap.Content, &props, &snap.Projection)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	snap.Version = version
	snap.Op = store.VersionOp(op)
	if prevFrom != nil {
		snap.PrevFrom = *prevFrom
	}
	if prevTo != nil {
		snap.PrevTo = *prevTo
	}
	if snap.Properties, err = unmarshalProps(props); err != nil {
		return nil, err
	}
	return &snap, nil
}

// scanRelationVersionMeta scans a relation-version-metadata row. The leading
// column is vseq, scanned into a throwaway (the read-time Version ordinal
// replaces it).
func scanRelationVersionMeta(row scanner) (store.RelationVersionMeta, error) {
	var (
		m        store.RelationVersionMeta
		vseq     int64
		op       string
		prevFrom *string
		prevTo   *string
		created  time.Time
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
	m.CreatedAt = created
	return m, nil
}
