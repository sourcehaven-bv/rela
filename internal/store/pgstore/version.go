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

// WriteVersion implements store.VersionWriter: it persists one synchronously
// captured version (rename/delete). The schema-dedup INSERT and the
// entity_versions INSERT run in one transaction so the version row is
// all-or-nothing (no orphaned schema_versions row, no half-write across a
// crash). The content hash is computed here from the snapshot.
func (s *Store) WriteVersion(ctx context.Context, in store.VersionInput) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op
	if err := insertVersion(ctx, tx, in, contentHashOf(in)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ensureSchemaVersion inserts the render-schema projection if its hash is not
// already present. Deduped by the schema_versions PK, so an unchanged
// (render-relevant) metamodel across many captures stores exactly one row.
func ensureSchemaVersion(ctx context.Context, q DBTX, hash string, projection []byte) error {
	const ins = `INSERT INTO schema_versions (hash, projection) VALUES ($1, $2)
	             ON CONFLICT (hash) DO NOTHING`
	_, err := q.Exec(ctx, ins, hash, projection)
	return err
}

// insertVersion writes one entity_versions row within q (a pool conn or tx).
// The caller supplies the content hash; vseq/created_at default in SQL.
func insertVersion(ctx context.Context, q DBTX, in store.VersionInput, contentHash string) error {
	props, err := marshalProps(in.Properties)
	if err != nil {
		return err
	}
	if err = ensureSchemaVersion(ctx, q, in.SchemaHash, in.Projection); err != nil {
		return fmt.Errorf("pgstore: ensure schema_version: %w", err)
	}
	var prev *string
	if in.Op == store.VersionOpRename && in.PrevID != "" {
		prev = &in.PrevID
	}
	const ins = `
		INSERT INTO entity_versions
		    (entity_id, op, prev_id, type, content, properties, content_hash,
		     schema_hash, principal_user, principal_tool, triggered_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	_, err = q.Exec(ctx, ins,
		in.EntityID, string(in.Op), prev, in.Type, in.Content, props, contentHash,
		in.SchemaHash, in.PrincipalUser, in.PrincipalTool, in.TriggeredBy)
	return err
}

// contentHashOf computes the canonical content hash of a snapshot, matching the
// live entity hash so a sweep can compare against the current entity's hash.
func contentHashOf(in store.VersionInput) string {
	return canonical.HashEntity(entity.Entity{
		ID:         in.EntityID,
		Type:       in.Type,
		Properties: in.Properties,
		Content:    in.Content,
	})
}

// --- store.HistoryReader ---

// lineage returns the ordered list of entity ids that make up an id's history,
// oldest first, by walking op=rename rows backward. The result always ends with
// the queried id. A rename records a row for the NEW id carrying prev_id=oldID,
// so we follow prev_id links from the newest known id back to the origin.
//
// # ID reuse and vseq fencing
//
// rela permits id reuse: a rename A→B frees id A, which a later, unrelated
// entity may reclaim. So "all rows with entity_id = A" is NOT a lineage — it can
// mix the pre-rename A (which became B) with a brand-new A. Every segment is
// therefore fenced by a vseq upper bound: a rename row (entity_id=B, prev_id=A,
// vseq=V) means A belonged to THIS lineage only for vseq < V (strictly: A's rows
// up to but not including the rename that renamed a value INTO B; the rename row
// itself lives under B). The recursive walk carries that bound down each hop, so
// a reclaimed id contributes only its in-window rows.
//
// lineageCTE is the shared recursive term producing (entity_id, hi) rows where
// hi is the exclusive vseq upper bound for that id in this lineage (NULL = no
// upper bound, i.e. the head id). $1 is the queried id.
const lineageCTE = `
	WITH RECURSIVE lin(entity_id, lo, hi) AS (
	    -- Head: the queried id. Its CURRENT lifecycle starts strictly after the
	    -- most recent rename that renamed this id AWAY (prev_id = the id) — before
	    -- that, the id belonged to an earlier, unrelated entity that has since
	    -- been renamed off it. lo is that boundary (0 if the id was never renamed
	    -- away). Unbounded above (hi = NULL). COLLATE "C" matches
	    -- entity_versions.entity_id (both recursive terms must agree, else 42P21).
	    SELECT CAST($1 AS text) COLLATE "C",
	           COALESCE((SELECT max(vseq) FROM entity_versions
	                     WHERE prev_id = $1 AND op = 'rename'), 0),
	           CAST(NULL AS bigint)
	    UNION
	    -- Each hop: the rename row that renamed some predecessor INTO the current
	    -- id. The predecessor's rows belong to this lineage in [its own
	    -- rename-away boundary, this rename's vseq). hi = r.vseq (exclusive upper);
	    -- lo = the predecessor's most-recent earlier rename-away, so a
	    -- doubly-reused predecessor id is also fenced. Guard cycles via hi.
	    SELECT r.prev_id,
	           COALESCE((SELECT max(vseq) FROM entity_versions
	                     WHERE prev_id = r.prev_id AND op = 'rename' AND vseq < r.vseq), 0),
	           r.vseq
	    FROM lin
	    JOIN entity_versions r
	      ON r.entity_id = lin.entity_id
	     AND r.op = 'rename'
	     AND r.prev_id IS NOT NULL
	     AND (lin.hi IS NULL OR r.vseq < lin.hi)
	)`

// lineageWhere joins entity_versions rows to their fenced lineage segment: a row
// belongs if its entity_id is a segment and its vseq is within that segment's
// [lo, hi) window (lo exclusive lower, hi exclusive upper / NULL = unbounded).
// DISTINCT at the call site because a rename diamond could match a row twice.
func lineageWhere() string {
	return `
		JOIN lin ON lin.entity_id = ev.entity_id
		         AND ev.vseq > lin.lo
		         AND (lin.hi IS NULL OR ev.vseq < lin.hi)`
}

// ListVersions implements store.HistoryReader.
func (s *Store) ListVersions(ctx context.Context, id string) ([]store.VersionMeta, error) {
	sel := lineageCTE + `
		SELECT DISTINCT ev.vseq, ev.op, ev.prev_id, ev.type, ev.content_hash, ev.schema_hash,
		       ev.principal_user, ev.principal_tool, ev.triggered_by, ev.created_at
		FROM entity_versions ev` + lineageWhere() + `
		ORDER BY ev.vseq ASC`
	rows, err := s.db.Query(ctx, sel, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metas []store.VersionMeta
	for rows.Next() {
		m, err := scanVersionMeta(rows)
		if err != nil {
			return nil, err
		}
		metas = append(metas, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Assign 1-based ordinals in lineage order (metas already ordered by vseq).
	for i := range metas {
		metas[i].Version = i + 1
	}
	return metas, nil
}

// GetVersion implements store.HistoryReader. version is a 1-based ordinal over
// the fenced lineage ordered by vseq. NOTE: the ordinal is only meaningful
// relative to a ListVersions read taken at the same time — the lineage is
// append-only, so an ordinal a caller already holds stays valid, but callers
// should treat it as a cursor into a specific ListVersions result.
func (s *Store) GetVersion(ctx context.Context, id string, version int) (*store.VersionSnapshot, error) {
	if version < 1 {
		return nil, store.ErrNotFound
	}
	sel := lineageCTE + `
		SELECT ev.op, ev.prev_id, ev.type, ev.content_hash, ev.schema_hash,
		       ev.principal_user, ev.principal_tool, ev.triggered_by, ev.created_at,
		       ev.content, ev.properties, sv.projection
		FROM entity_versions ev` + lineageWhere() + `
		JOIN schema_versions sv ON sv.hash = ev.schema_hash
		ORDER BY ev.vseq ASC
		OFFSET $2 LIMIT 1`
	row := s.db.QueryRow(ctx, sel, id, version-1)

	var (
		snap  store.VersionSnapshot
		op    string
		prev  *string
		props []byte
	)
	err := row.Scan(&op, &prev, &snap.Type, &snap.ContentHash, &snap.SchemaHash,
		&snap.PrincipalUser, &snap.PrincipalTool, &snap.TriggeredBy, &snap.CreatedAt,
		&snap.Content, &props, &snap.Projection)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	snap.Version = version
	snap.Op = store.VersionOp(op)
	if prev != nil {
		snap.PrevID = *prev
	}
	if snap.Properties, err = unmarshalProps(props); err != nil {
		return nil, err
	}
	return &snap, nil
}

// scanVersionMeta scans a version-metadata row. The leading column is vseq,
// which is not surfaced (the read-time Version ordinal replaces it); it is
// scanned into a throwaway.
func scanVersionMeta(row scanner) (store.VersionMeta, error) {
	var (
		m       store.VersionMeta
		vseq    int64
		op      string
		prev    *string
		created time.Time
	)
	if err := row.Scan(&vseq, &op, &prev, &m.Type, &m.ContentHash, &m.SchemaHash,
		&m.PrincipalUser, &m.PrincipalTool, &m.TriggeredBy, &created); err != nil {
		return store.VersionMeta{}, err
	}
	m.Op = store.VersionOp(op)
	if prev != nil {
		m.PrevID = *prev
	}
	m.CreatedAt = created
	return m, nil
}

// Static assertion that Store satisfies the optional capability.
var _ store.HistoryReader = (*Store)(nil)
