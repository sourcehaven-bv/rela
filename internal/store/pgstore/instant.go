package pgstore

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// Point-in-time reads (TKT-6KLUT3U). The contract is on store.BindReadInstant;
// this file is the PostgreSQL mechanism.
//
// Every read statement names the live tables `entities` and `relations`. Under
// a read instant, [instantDB] prefixes the statement with two common table
// expressions of those names that compute the state at the instant from the
// live rows and the version history. A CTE name takes precedence over a table
// name, so every read (point reads, lists, header pages, graph queries, roll-
// ups, search) answers from the snapshot without a second copy of its SQL.
// The CTEs read the live tables through schema-qualified names, so they never
// refer to themselves, which a statement that is already WITH RECURSIVE would
// otherwise turn into a recursive reference.
//
// The ACL read gate is the exception (DEC-TDEF3GD: history is read with
// today's ACL, so a principal who lost access cannot read the past). A
// HasInbound / HasOutbound predicate or an Any branch of a graph query or a
// visible search renders its tables through [storeutil.LiveTable], and this
// seam qualifies those with the schema, which no CTE shadows. The whole gate
// reads live: the role-conferring edge from the principal, the endpoint rows
// it matches, and the inherit-through containment edges both closures climb.
// Reading containment at the instant instead would let an edge removed since
// grant access to the past; reading it live fails closed for an entity that
// has since moved out of reach, been renamed or been deleted. What stays at
// the instant is the candidate row itself (its properties in a `when:`) and
// every Related traversal, which is data, not the gate.
//
// Writes are refused: Exec and Begin return store.ErrReadOnlyInstant, and so
// does a data-modifying statement sent through Query or QueryRow.
//
// The instant is rendered into the statement as a literal produced by Go
// from a time.Time, never from caller text, so it needs no placeholder and
// the statement's own placeholder numbering is untouched.

// instantDB is the store's execution seam. Without a read instant on ctx it
// passes every call through to the pool unchanged.
type instantDB struct {
	inner DBTX

	// schema is the store's schema, resolved the first time an instant read
	// needs it. The pool's search_path decides it, so it is fixed for the
	// pool's lifetime. Only a success is kept: a failure (a cancelled
	// request ctx, a dropped connection) is retried by the next read.
	schemaMu sync.Mutex
	schema   string
}

func newInstantDB(inner DBTX) *instantDB { return &instantDB{inner: inner} }

// unwrapDB returns the handle a store was built on, below the instant seam.
// Code that needs the concrete pool (advisory locks, the version service, the
// state and user-state stores) asserts on this, never on the seam.
func unwrapDB(db DBTX) DBTX {
	if i, ok := db.(*instantDB); ok {
		return i.inner
	}
	return db
}

func (d *instantDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if _, ok := store.ReadInstant(ctx); ok {
		return pgconn.CommandTag{}, store.ErrReadOnlyInstant
	}
	return d.inner.Exec(ctx, sql, args...)
}

func (d *instantDB) Begin(ctx context.Context) (pgx.Tx, error) {
	if _, ok := store.ReadInstant(ctx); ok {
		return nil, store.ErrReadOnlyInstant
	}
	return d.inner.Begin(ctx)
}

func (d *instantDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	sql, err := d.rewrite(ctx, sql)
	if err != nil {
		return nil, err
	}
	return d.inner.Query(ctx, sql, args...)
}

func (d *instantDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	sql, err := d.rewrite(ctx, sql)
	if err != nil {
		return errRow{err: err}
	}
	return d.inner.QueryRow(ctx, sql, args...)
}

// errRow is a pgx.Row whose Scan reports err.
type errRow struct{ err error }

func (r errRow) Scan(...any) error { return r.err }

// rewrite returns sql unchanged without a read instant on ctx, and with the
// snapshot CTEs prefixed under one.
func (d *instantDB) rewrite(ctx context.Context, sql string) (string, error) {
	at, ok := store.ReadInstant(ctx)
	if !ok {
		return sql, nil
	}
	if storeutil.IsWriteStatement(sql) {
		return "", store.ErrReadOnlyInstant
	}
	if store.ReadsToday(ctx) {
		return sql, nil
	}
	schema, err := d.currentSchema(ctx)
	if err != nil {
		return "", err
	}
	quoted := pgx.Identifier{schema}.Sanitize()
	return storeutil.PrefixCTEs(storeutil.ResolveLive(sql, quoted), snapshotCTEs(quoted, at)), nil
}

// currentSchema returns the store's schema, resolving it on first use.
func (d *instantDB) currentSchema(ctx context.Context) (string, error) {
	d.schemaMu.Lock()
	defer d.schemaMu.Unlock()
	if d.schema != "" {
		return d.schema, nil
	}
	schema, err := resolveCurrentSchema(ctx, d.inner)
	if err != nil {
		return "", err
	}
	d.schema = schema
	return schema, nil
}

// resolveCurrentSchema is the schema the pool's unqualified names resolve to.
func resolveCurrentSchema(ctx context.Context, db DBTX) (string, error) {
	var schema string
	if err := db.QueryRow(ctx, `SELECT current_schema()`).Scan(&schema); err != nil {
		return "", err
	}
	if schema == "" {
		return "", errors.New("pgstore: no current schema for a point-in-time read")
	}
	return schema, nil
}

// instantLiteral renders at as a timestamptz literal.
func instantLiteral(at time.Time) string {
	return "'" + at.UTC().Format("2006-01-02T15:04:05.999999Z07:00") + "'::timestamptz"
}

// snapshotCTEs is the `entities` and `relations` CTE pair for the state at
// at, reading the live tables in schema (already quoted). The rule is the one
// documented on store.BindReadInstant.
//
// Columns are listed, not starred, so a column a later migration adds is
// absent from the snapshot: a read that needs it fails loudly instead of
// reading a guess. A version carries no search title, mention index or
// reconciliation element, so those read NULL on a row served from history;
// its search_title is recomputed the way the live write path computes it
// (searchtitle.go), because search ranks and pages on it.
func snapshotCTEs(schema string, at time.Time) string {
	t := instantLiteral(at)
	r := strings.NewReplacer("{S}", schema, "{T}", t)
	return r.Replace(snapshotTemplate)
}

// versionAt is the edit time of a version row aliased x (see migration 0052).
const snapshotTemplate = `entities AS NOT MATERIALIZED (
  SELECT l.id, l.type, l.properties, l.content, l.search_text, l.created_at, l.updated_at, l.seq,
         l.last_edited_by_user, l.last_edited_by_tool, l.face,
         l.origin_kind, l.origin_source, l.origin_source_face, l.origin_source_type, l.origin_definition,
         l.content_hash, l.mention_targets, l.mentions_hash,
         l.element_hash, l.element_fp0, l.element_fp1, l.element_fp2, l.element_fp3, l.search_title
  FROM {S}.entities l
  WHERE l.updated_at <= {T}
    AND NOT EXISTS (SELECT 1 FROM {S}.entity_versions x
                    WHERE x.entity_id = l.id AND x.face = l.face
                      AND COALESCE(x.content_at, x.created_at) > {T})
  UNION ALL
  SELECT v.entity_id, v.type, v.properties, v.content,
         lower(v.entity_id) || E'\n' ||
           COALESCE((SELECT string_agg(lower(p.value #>> '{}'), E'\n' ORDER BY p.key COLLATE "C")
                     FROM jsonb_each(v.properties) p WHERE jsonb_typeof(p.value) = 'string'), '') ||
           E'\n' || lower(v.content),
         v.at, v.at, 0::bigint,
         NULLIF(v.principal_user, ''), NULLIF(v.principal_tool, ''), v.face,
         v.origin_kind, v.origin_source, v.origin_source_face, v.origin_source_type, v.origin_definition,
         NULL::text, NULL::text[], NULL::text,
         NULL::text, NULL::bigint, NULL::bigint, NULL::bigint, NULL::bigint,
         lower(COALESCE(NULLIF(v.properties ->>
                          (SELECT p.prop FROM {S}.search_title_props p WHERE p.type = v.type), ''),
                        v.entity_id))
  FROM (SELECT DISTINCT ON (x.entity_id, x.face) x.*, COALESCE(x.content_at, x.created_at) AS at
        FROM {S}.entity_versions x
        WHERE COALESCE(x.content_at, x.created_at) <= {T}
        ORDER BY x.entity_id, x.face, COALESCE(x.content_at, x.created_at) DESC, x.vseq DESC) v
  WHERE v.op NOT IN ('delete', 'purge')
    AND NOT EXISTS (SELECT 1 FROM {S}.entities l
                    WHERE l.id = v.entity_id AND l.face = v.face AND l.updated_at <= {T}
                      AND NOT EXISTS (SELECT 1 FROM {S}.entity_versions x
                                      WHERE x.entity_id = l.id AND x.face = l.face
                                        AND COALESCE(x.content_at, x.created_at) > {T}))
    AND NOT EXISTS (SELECT 1 FROM {S}.entity_versions y
                    WHERE y.prev_id = v.entity_id AND COALESCE(y.prev_face, y.face) = v.face
                      AND (y.entity_id <> v.entity_id OR y.face <> v.face)
                      AND y.vseq > v.vseq AND COALESCE(y.content_at, y.created_at) <= {T})
    AND NOT EXISTS (SELECT 1 FROM {S}.marked_entities m
                    WHERE m.id = v.entity_id AND m.face = v.face AND m.deleted_at <= {T})
),
relations AS NOT MATERIALIZED (
    SELECT l.from_id, l.rel_type, l.to_id, l.properties, l.content, l.created_at, l.updated_at, l.seq,
           l.rel_record_id, l.last_edited_by_user, l.last_edited_by_tool, l.from_face, l.content_hash,
           l.element_hash, l.element_fp0, l.element_fp1, l.element_fp2, l.element_fp3
    FROM {S}.relations l
    WHERE l.updated_at <= {T}
      AND NOT EXISTS (SELECT 1 FROM {S}.relation_versions x
                      WHERE x.rel_record_id = l.rel_record_id
                        AND COALESCE(x.content_at, x.created_at) > {T})
    UNION ALL
    SELECT v.from_id, v.rel_type, v.to_id, v.properties, v.content, v.at, v.at, 0::bigint,
           v.rel_record_id, NULLIF(v.principal_user, ''), NULLIF(v.principal_tool, ''), v.from_face, NULL::text,
           NULL::text, NULL::bigint, NULL::bigint, NULL::bigint, NULL::bigint
    FROM (SELECT DISTINCT ON (x.rel_record_id) x.*, COALESCE(x.content_at, x.created_at) AS at
          FROM {S}.relation_versions x
          WHERE COALESCE(x.content_at, x.created_at) <= {T}
          ORDER BY x.rel_record_id, COALESCE(x.content_at, x.created_at) DESC, x.vseq DESC) v
    WHERE v.op NOT IN ('delete', 'purge')
      AND NOT EXISTS (SELECT 1 FROM {S}.relations l
                      WHERE l.rel_record_id = v.rel_record_id AND l.updated_at <= {T}
                        AND NOT EXISTS (SELECT 1 FROM {S}.relation_versions x
                                        WHERE x.rel_record_id = l.rel_record_id
                                          AND COALESCE(x.content_at, x.created_at) > {T}))
)`

// ReadsAtInstant implements [store.InstantReader]: every read honors a read
// instant on ctx through the instant seam.
func (s *Store) ReadsAtInstant() bool { return true }
