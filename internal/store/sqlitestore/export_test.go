package sqlitestore

import (
	"context"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Test-only accessors. They live here so the sweep can be driven a tick at a
// time without widening the package's real API for it, and without a test
// having to wait out a ticker interval.

// SweepNow runs exactly one reconciliation tick synchronously.
func (s *Store) SweepNow(ctx context.Context, p store.ProjectionProvider, cfg store.SweepConfig) error {
	return sweepNow(ctx, s, p, cfg)
}

// ExplainGraphQuery returns SQLite's EXPLAIN QUERY PLAN for the statement
// GraphQuery would run for q, one detail line per plan node.
func (s *Store) ExplainGraphQuery(ctx context.Context, q store.GraphQuery) (string, error) {
	sqlText, args, _ := buildGraphQuerySQL(q, graphSelectRows)
	return s.explain(ctx, sqlText, args)
}

// ExplainMatchingFaces is ExplainGraphQuery for MatchingFaces over ids.
func (s *Store) ExplainMatchingFaces(ctx context.Context, q store.GraphQuery, ids []string) (string, error) {
	sqlText, args, _ := buildMatchingFacesSQL(q, ids)
	return s.explain(ctx, sqlText, args)
}

// ExplainEntityPage is ExplainGraphQuery for the page ListEntitiesPage reads
// for q, cursor and LIMIT included.
func (s *Store) ExplainEntityPage(ctx context.Context, q store.EntityQuery) (string, error) {
	sqlText, args, err := entityPageSQL(q)
	if err != nil {
		return "", err
	}
	return s.explain(ctx, sqlText, args)
}

// ExplainGetEntity is ExplainGraphQuery for GetEntity(ref).
func (s *Store) ExplainGetEntity(ctx context.Context, ref entity.Ref) (string, error) {
	return s.explain(ctx, getEntitySQL, []any{ref.ID, string(ref.Face)})
}

// BuildHighestIDSQLForTest exposes the SQL [Store.HighestID] issues for
// prefix. Test-only.
func BuildHighestIDSQLForTest(prefix string) (sqlText string, args []any) {
	return buildHighestIDSQL(prefix)
}

// ExplainHighestID is ExplainGraphQuery for HighestID(prefix).
func (s *Store) ExplainHighestID(ctx context.Context, prefix string) (string, error) {
	sqlText, args := buildHighestIDSQL(prefix)
	return s.explain(ctx, sqlText, args)
}

func (s *Store) explain(ctx context.Context, sqlText string, args []any) (string, error) {
	lines, err := collectRows(ctx, s.q(), "EXPLAIN QUERY PLAN "+sqlText, args, func(sc scanner) (string, error) {
		var id, parent, unused int
		var detail string
		return detail, sc.Scan(&id, &parent, &unused, &detail)
	})
	return strings.Join(lines, "\n"), err
}

// Indexes returns every index on entities, name to stored CREATE text.
func (s *Store) Indexes(ctx context.Context) (map[string]string, error) {
	rows, err := collectRows(ctx, s.q(),
		`SELECT name, coalesce(sql, '') FROM sqlite_schema WHERE type = 'index' AND tbl_name = 'entities'`, nil,
		func(sc scanner) ([2]string, error) {
			var row [2]string
			return row, sc.Scan(&row[0], &row[1])
		})
	out := map[string]string{}
	for _, r := range rows {
		out[r[0]] = r[1]
	}
	return out, err
}

// ExecRaw runs a statement outside the store's API, to set up schema drift.
func (s *Store) ExecRaw(ctx context.Context, sqlText string) error {
	_, err := s.db.ExecContext(ctx, sqlText)
	return err
}
