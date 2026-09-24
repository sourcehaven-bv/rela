package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/graphquerynaive"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// The graph queries run as one SQL statement each, built in graphsql.go
// (TKT-B51CYD). graphquerynaive stays the behavioral reference: the
// differential harness compares the two on the same store, and a query whose
// names cannot be rendered safely is answered by graphquerynaive directly.
// The fallback calls the naive package rather than the store.* helpers, which
// would type-assert this store and recurse.
//
// Rows are read to the end before the first one is yielded. Inside a Tx every
// statement shares one pinned connection, so a consumer that queried the store
// while a result set was still open would wait on itself.

func (s *Store) GraphQuery(ctx context.Context, q store.GraphQuery) iter.Seq2[*entity.Entity, error] {
	if err := graphquerynaive.CheckEndpointShape(q); err != nil {
		return func(yield func(*entity.Entity, error) bool) { yield(nil, err) }
	}
	sqlText, args, ok := buildGraphQuerySQL(q, graphSelectRows)
	if !ok || orderIsInexact(ctx, s.q(), q) {
		return graphquerynaive.Run(ctx, s, q)
	}
	return yieldAll(func() ([]*entity.Entity, error) {
		return collectRows(ctx, s.q(), sqlText, args, scanEntity)
	})
}

// orderIsInexact reports whether a sort key of q holds, anywhere in the type,
// a value whose SQL text form is not the one graphquerynaive sorts by: Go's
// fmt gives `map[k:v]` and `[a b]` where SQL gives JSON text, and a float's
// token need not match either. Sort keys are scalar strings by contract; a
// type that breaks it is ordered in Go, so a list does not reorder depending
// on which path served it. The probe scans the type, a superset of the
// matched rows, so it can only decline a query that would have been exact.
// A probe failure also declines.
func orderIsInexact(ctx context.Context, db querier, q store.GraphQuery) bool {
	if len(q.OrderBy) == 0 {
		return false
	}
	b := &sqlBuilder{}
	conds := make([]string, 0, len(q.OrderBy))
	for _, spec := range q.OrderBy {
		conds = append(conds, typeExpr("", b.jsonPath(spec.Property))+" IN ('real', 'array', 'object')")
	}
	sqlText := "SELECT 1 FROM entities WHERE (" + strings.Join(conds, " OR ") + ")"
	if q.EntityType != "" {
		sqlText += " AND type = " + b.arg(q.EntityType)
	}
	var hit int
	err := db.QueryRowContext(ctx, sqlText+" LIMIT 1", b.args...).Scan(&hit)
	return b.unsafe || !errors.Is(err, sql.ErrNoRows)
}

// GraphQueryHeaders implements store.GraphHeaderQueryer: GraphQuery with the
// content column left out of the projection.
func (s *Store) GraphQueryHeaders(ctx context.Context, q store.GraphQuery) iter.Seq2[store.EntityHeader, error] {
	if err := graphquerynaive.CheckEndpointShape(q); err != nil {
		return func(yield func(store.EntityHeader, error) bool) { yield(store.EntityHeader{}, err) }
	}
	sqlText, args, ok := buildGraphQuerySQL(q, graphSelectHeaders)
	if !ok || orderIsInexact(ctx, s.q(), q) {
		return headersOf(graphquerynaive.Run(ctx, s, q))
	}
	return yieldAll(func() ([]store.EntityHeader, error) {
		return collectRows(ctx, s.q(), sqlText, args, scanEntityHeader)
	})
}

// CountMatched implements store.MatchedCounter: GraphCount without the
// type-wide total a list page does not need.
func (s *Store) CountMatched(ctx context.Context, q store.GraphQuery) (int, error) {
	if err := graphquerynaive.CheckEndpointShape(q); err != nil {
		return 0, err
	}
	sqlText, args, ok := buildGraphQuerySQL(q, graphSelectCount)
	if !ok {
		matched, _, err := graphquerynaive.Count(ctx, s, q)
		return matched, err
	}
	var matched int
	if err := s.q().QueryRowContext(ctx, sqlText, args...).Scan(&matched); err != nil {
		return 0, fmt.Errorf("sqlitestore: graph count (matched): %w", err)
	}
	return matched, nil
}

func (s *Store) GraphCount(ctx context.Context, q store.GraphQuery) (matched, total int, err error) {
	if _, _, ok := buildGraphQuerySQL(q, graphSelectCount); !ok {
		if shapeErr := graphquerynaive.CheckEndpointShape(q); shapeErr != nil {
			return 0, 0, shapeErr
		}
		return graphquerynaive.Count(ctx, s, q)
	}
	if matched, err = s.CountMatched(ctx, q); err != nil {
		return 0, 0, err
	}
	totalSQL, totalArgs := buildGraphTotalSQL(q)
	if err = s.q().QueryRowContext(ctx, totalSQL, totalArgs...).Scan(&total); err != nil {
		return 0, 0, fmt.Errorf("sqlitestore: graph count (total): %w", err)
	}
	return matched, total, nil
}

func (s *Store) MatchingIDs(
	ctx context.Context, q store.GraphQuery, ids []string,
) (map[string]bool, error) {
	if err := graphquerynaive.CheckEndpointShape(q); err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = false
	}
	if len(out) == 0 {
		return out, nil
	}
	sqlText, args, ok := buildMatchingIDsSQL(q, ids)
	if !ok {
		return graphquerynaive.MatchingIDs(ctx, s, q, ids)
	}
	matched, err := collectRows(ctx, s.q(), sqlText, args, func(sc scanner) (string, error) {
		var id string
		return id, sc.Scan(&id)
	})
	if err != nil {
		return nil, err
	}
	for _, id := range matched {
		out[id] = true
	}
	return out, nil
}

// ListEntityHeaders implements store.HeaderReader: ListEntities without the
// content column, so a list page never reads the bodies it will not render.
func (s *Store) ListEntityHeaders(ctx context.Context, q store.EntityQuery) iter.Seq2[store.EntityHeader, error] {
	if err := storeutil.ValidateEntityQuery(q); err != nil {
		return func(yield func(store.EntityHeader, error) bool) { yield(store.EntityHeader{}, err) }
	}
	sqlText, args := buildEntitySelectSQL(q, "", entityHeaderColumns)
	return yieldAll(func() ([]store.EntityHeader, error) {
		return collectRows(ctx, s.q(), sqlText, args, scanEntityHeader)
	})
}

// entityHeaderColumns is entityColumns without content, in the order
// scanEntityHeader expects.
const entityHeaderColumns = "id, type, face, properties, updated_at"

func scanEntityHeader(sc scanner) (store.EntityHeader, error) {
	var h store.EntityHeader
	var face, props, updated string
	if err := sc.Scan(&h.ID, &h.Type, &face, &props, &updated); err != nil {
		return store.EntityHeader{}, fmt.Errorf("sqlitestore: scan entity header: %w", err)
	}
	h.Face = entity.Face(face)
	var err error
	if h.Properties, err = unmarshalProps(props); err != nil {
		return store.EntityHeader{}, fmt.Errorf("sqlitestore: entity %s: %w", h.ID, err)
	}
	if h.UpdatedAt, err = time.Parse(timeFmt, updated); err != nil {
		return store.EntityHeader{}, fmt.Errorf("sqlitestore: parse updated_at for %s: %w", h.ID, err)
	}
	return h, nil
}

// collectRows runs a query and scans every row before returning.
func collectRows[T any](
	ctx context.Context, qr querier, sqlText string, args []any, scan func(scanner) (T, error),
) ([]T, error) {
	rows, err := qr.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: query: %w", err)
	}
	defer rows.Close()
	var out []T
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlitestore: query: %w", err)
	}
	return out, nil
}

// yieldAll runs load when the iterator is ranged over and yields its rows, or
// its error.
func yieldAll[T any](load func() ([]T, error)) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		rows, err := load()
		if err != nil {
			var zero T
			yield(zero, err)
			return
		}
		for _, r := range rows {
			if !yield(r, nil) {
				return
			}
		}
	}
}

// headersOf projects full entities to headers, for the naive fallback.
func headersOf(seq iter.Seq2[*entity.Entity, error]) iter.Seq2[store.EntityHeader, error] {
	return func(yield func(store.EntityHeader, error) bool) {
		for e, err := range seq {
			if err != nil {
				yield(store.EntityHeader{}, err)
				return
			}
			if !yield(store.HeaderOf(e), nil) {
				return
			}
		}
	}
}
