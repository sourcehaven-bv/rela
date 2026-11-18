package sqlitestore

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// Point-in-time reads (TKT-6KLUT3U). The contract is on store.BindReadInstant;
// this file is the SQLite mechanism, the same one pgstore uses.
//
// Every read statement goes through [Store.q] and names the live tables
// `entities` and `relations`. Under a read instant, [instantQuerier] prefixes
// the statement with two common table expressions of those names that compute
// the state at the instant. A CTE name takes precedence over a table name, so
// every read answers from the snapshot. The CTEs read the live tables as
// `main.entities` and `main.relations`, so they never refer to themselves.
//
// The ACL read gate reads the live tables, as in pgstore (see its instant.go
// for the rule and the choice to read containment edges live too): its
// [storeutil.LiveTable] names become `main.relations` and `main.entities`.
//
// Writes are refused: ExecContext returns store.ErrReadOnlyInstant, and so
// does [Store.Tx]. The instant is rendered as a literal by Go from a
// time.Time, in the fixed-width [sqlitedb.TimeFormat] the timestamp columns
// compare in.

// instantQuerier is the store's execution seam. Without a read instant on ctx
// it passes every call through unchanged.
type instantQuerier struct{ inner querier }

func (q instantQuerier) ExecContext(ctx context.Context, sqlText string, args ...any) (sql.Result, error) {
	if _, ok := store.ReadInstant(ctx); ok {
		return nil, store.ErrReadOnlyInstant
	}
	return q.inner.ExecContext(ctx, sqlText, args...)
}

func (q instantQuerier) QueryContext(ctx context.Context, sqlText string, args ...any) (*sql.Rows, error) {
	sqlText, ok := rewriteAtInstant(ctx, sqlText)
	if !ok {
		return nil, store.ErrReadOnlyInstant
	}
	return q.inner.QueryContext(ctx, sqlText, args...)
}

// QueryRowContext cannot return a *sql.Row carrying store.ErrReadOnlyInstant,
// so a write statement sent through it under an instant runs a statement that
// fails with a message naming the refusal. No write path of this store uses
// QueryRowContext; this is the backstop.
func (q instantQuerier) QueryRowContext(ctx context.Context, sqlText string, args ...any) *sql.Row {
	sqlText, ok := rewriteAtInstant(ctx, sqlText)
	if !ok {
		return q.inner.QueryRowContext(ctx, `SELECT * FROM "point-in-time read cannot write"`)
	}
	return q.inner.QueryRowContext(ctx, sqlText, args...)
}

// rewriteAtInstant returns sqlText with the snapshot CTEs under a read
// instant, and unchanged without one. It reports false for a write statement
// under an instant.
func rewriteAtInstant(ctx context.Context, sqlText string) (string, bool) {
	at, ok := store.ReadInstant(ctx)
	if !ok {
		return sqlText, true
	}
	if storeutil.IsWriteStatement(sqlText) {
		return "", false
	}
	if store.ReadsToday(ctx) {
		return sqlText, true
	}
	return storeutil.PrefixCTEs(storeutil.ResolveLive(sqlText, "main"), snapshotCTEs(at)), true
}

// snapshotCTEs is the `entities` and `relations` CTE pair for the state at
// at. The rule is the one documented on store.BindReadInstant.
//
// Columns are listed, not starred, so a column a later migration adds is
// absent from the snapshot and a read that needs it fails loudly. A version
// carries no mention index or reconciliation element, so those read NULL on a
// row served from history. Its search_text is the lower-cased id, string
// property values and body; SQLite's lower() folds ASCII only, so text
// matching under an instant must not rely on it for other letters.
func snapshotCTEs(at time.Time) string {
	t := "'" + sqlitedb.FormatTime(at) + "'"
	return strings.ReplaceAll(snapshotTemplate, "{T}", t)
}

const snapshotTemplate = `entities AS NOT MATERIALIZED (
  SELECT l.id, l.face, l.type, l.properties, l.content, l.updated_at,
         l.last_edited_by_user, l.last_edited_by_tool,
         l.origin_kind, l.origin_source, l.origin_source_face, l.origin_source_type, l.origin_definition,
         l.content_hash, l.mention_targets, l.search_text,
         l.element_hash, l.element_fp0, l.element_fp1, l.element_fp2, l.element_fp3
  FROM main.entities l
  WHERE l.updated_at <= {T}
    AND NOT EXISTS (SELECT 1 FROM main.entity_versions x
                    WHERE x.entity_id = l.id AND x.face = l.face
                      AND COALESCE(x.content_at, x.created_at) > {T})
  UNION ALL
  SELECT v.entity_id, v.face, v.type, v.properties, v.content, v.at,
         NULLIF(v.principal_user, ''), NULLIF(v.principal_tool, ''),
         v.origin_kind, v.origin_source, v.origin_source_face, v.origin_source_type, v.origin_definition,
         NULL, NULL,
         lower(v.entity_id) || char(10) ||
           COALESCE((SELECT group_concat(lower(p.value), char(10))
                     FROM (SELECT j.value FROM json_each(v.properties) j
                           WHERE j.type = 'text' ORDER BY j.key) p), '') ||
           char(10) || lower(v.content),
         NULL, NULL, NULL, NULL, NULL
  FROM (SELECT x.*, COALESCE(x.content_at, x.created_at) AS at,
               ROW_NUMBER() OVER (PARTITION BY x.entity_id, x.face
                                  ORDER BY COALESCE(x.content_at, x.created_at) DESC, x.vseq DESC) AS rn
        FROM main.entity_versions x
        WHERE COALESCE(x.content_at, x.created_at) <= {T}) v
  WHERE v.rn = 1
    AND v.op NOT IN ('delete', 'purge')
    AND NOT EXISTS (SELECT 1 FROM main.entities l
                    WHERE l.id = v.entity_id AND l.face = v.face AND l.updated_at <= {T}
                      AND NOT EXISTS (SELECT 1 FROM main.entity_versions x
                                      WHERE x.entity_id = l.id AND x.face = l.face
                                        AND COALESCE(x.content_at, x.created_at) > {T}))
    AND NOT EXISTS (SELECT 1 FROM main.entity_versions y
                    WHERE y.prev_id = v.entity_id AND COALESCE(y.prev_face, y.face) = v.face
                      AND (y.entity_id <> v.entity_id OR y.face <> v.face)
                      AND y.vseq > v.vseq AND COALESCE(y.content_at, y.created_at) <= {T})
    AND NOT EXISTS (SELECT 1 FROM main.marked_entities m
                    WHERE m.id = v.entity_id AND m.face = v.face AND m.deleted_at <= {T})
),
relations AS NOT MATERIALIZED (
  SELECT l.from_id, l.from_face, l.rel_type, l.to_id, l.properties, l.content, l.updated_at,
         l.rel_record_id, l.last_edited_by_user, l.last_edited_by_tool, l.content_hash,
         l.element_hash, l.element_fp0, l.element_fp1, l.element_fp2, l.element_fp3
  FROM main.relations l
  WHERE l.updated_at <= {T}
    AND NOT EXISTS (SELECT 1 FROM main.relation_versions x
                    WHERE x.rel_record_id = l.rel_record_id
                      AND COALESCE(x.content_at, x.created_at) > {T})
  UNION ALL
  SELECT v.from_id, v.from_face, v.rel_type, v.to_id, v.properties, v.content, v.at,
         v.rel_record_id, NULLIF(v.principal_user, ''), NULLIF(v.principal_tool, ''), NULL,
         NULL, NULL, NULL, NULL, NULL
  FROM (SELECT x.*, COALESCE(x.content_at, x.created_at) AS at,
               ROW_NUMBER() OVER (PARTITION BY x.rel_record_id
                                  ORDER BY COALESCE(x.content_at, x.created_at) DESC, x.vseq DESC) AS rn
        FROM main.relation_versions x
        WHERE COALESCE(x.content_at, x.created_at) <= {T}) v
  WHERE v.rn = 1
    AND v.op NOT IN ('delete', 'purge')
    AND NOT EXISTS (SELECT 1 FROM main.relations l
                    WHERE l.rel_record_id = v.rel_record_id AND l.updated_at <= {T}
                      AND NOT EXISTS (SELECT 1 FROM main.relation_versions x
                                      WHERE x.rel_record_id = l.rel_record_id
                                        AND COALESCE(x.content_at, x.created_at) > {T}))
)`

// ReadsAtInstant implements [store.InstantReader]: every read honors a read
// instant on ctx through [instantQuerier].
func (s *Store) ReadsAtInstant() bool { return true }
