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
// captured version (rename/delete) in a single statement pair (schema dedup +
// version insert). The content hash is computed here from the snapshot.
func (s *Store) WriteVersion(ctx context.Context, in store.VersionInput) error {
	return insertVersion(ctx, s.db, in, contentHashOf(in))
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
	if err := ensureSchemaVersion(ctx, q, in.SchemaHash, in.Projection); err != nil {
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

// latestContentHash returns the content_hash of an entity's newest version, or
// ("", false, nil) when it has none. Used by the sweep to dedup: skip capture
// when the live content hash already matches the latest version.
func latestContentHash(ctx context.Context, q DBTX, entityID string) (string, bool, error) {
	const sel = `SELECT content_hash FROM entity_versions
	             WHERE entity_id = $1 ORDER BY vseq DESC LIMIT 1`
	var h string
	err := q.QueryRow(ctx, sel, entityID).Scan(&h)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return h, true, nil
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
// The walk is bounded by a visited set (defensive against a pathological
// prev_id cycle from hand-edited data) and returns ids oldest-first.
func lineage(ctx context.Context, q DBTX, id string) ([]string, error) {
	// Collect the chain newest-first, then reverse.
	chain := []string{id}
	visited := map[string]bool{id: true}
	cur := id
	for {
		const sel = `SELECT prev_id FROM entity_versions
		             WHERE entity_id = $1 AND op = 'rename' AND prev_id IS NOT NULL
		             ORDER BY vseq ASC LIMIT 1`
		var prev string
		err := q.QueryRow(ctx, sel, cur).Scan(&prev)
		if errors.Is(err, pgx.ErrNoRows) {
			break
		}
		if err != nil {
			return nil, err
		}
		if prev == "" || visited[prev] {
			break
		}
		chain = append(chain, prev)
		visited[prev] = true
		cur = prev
	}
	// Reverse to oldest-first.
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}

// ListVersions implements store.HistoryReader.
func (s *Store) ListVersions(ctx context.Context, id string) ([]store.VersionMeta, error) {
	ids, err := lineage(ctx, s.db, id)
	if err != nil {
		return nil, err
	}
	const sel = `
		SELECT entity_id, op, prev_id, type, content_hash, schema_hash,
		       principal_user, principal_tool, triggered_by, created_at
		FROM entity_versions
		WHERE entity_id = ANY($1)
		ORDER BY vseq ASC`
	rows, err := s.db.Query(ctx, sel, ids)
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

// GetVersion implements store.HistoryReader.
func (s *Store) GetVersion(ctx context.Context, id string, version int) (*store.VersionSnapshot, error) {
	if version < 1 {
		return nil, store.ErrNotFound
	}
	ids, err := lineage(ctx, s.db, id)
	if err != nil {
		return nil, err
	}
	// version is a 1-based ordinal over the lineage ordered by vseq; use OFFSET.
	const sel = `
		SELECT ev.entity_id, ev.op, ev.prev_id, ev.type, ev.content_hash, ev.schema_hash,
		       ev.principal_user, ev.principal_tool, ev.triggered_by, ev.created_at,
		       ev.content, ev.properties, sv.projection
		FROM entity_versions ev
		JOIN schema_versions sv ON sv.hash = ev.schema_hash
		WHERE ev.entity_id = ANY($1)
		ORDER BY ev.vseq ASC
		OFFSET $2 LIMIT 1`
	row := s.db.QueryRow(ctx, sel, ids, version-1)

	var (
		snap     store.VersionSnapshot
		entityID string
		op       string
		prev     *string
		props    []byte
	)
	err = row.Scan(&entityID, &op, &prev, &snap.Type, &snap.ContentHash, &snap.SchemaHash,
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

// EntityID is a scan target field on VersionMeta only via the snapshot; ListVersions
// does not expose entity_id per row, so scanVersionMeta discards it.
func scanVersionMeta(row scanner) (store.VersionMeta, error) {
	var (
		m        store.VersionMeta
		entityID string
		op       string
		prev     *string
		created  time.Time
	)
	if err := row.Scan(&entityID, &op, &prev, &m.Type, &m.ContentHash, &m.SchemaHash,
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
