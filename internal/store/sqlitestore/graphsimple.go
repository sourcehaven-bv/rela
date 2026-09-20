package sqlitestore

import (
	"context"
	"database/sql"
	"fmt"
	"iter"
	"reflect"
	"regexp"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// SQL evaluation of the SIMPLE graph-query shape (TKT-U9DYW4).
//
// graphquerynaive answers every GraphQuery by listing the whole type with
// bodies and judging each row in Go. That is the only honest answer for a
// relation predicate, which SQLite has no pushdown for yet. But the query a
// list page sends an unrestricted principal has no relation predicate at all:
// it is "this type, maybe `status = open`, ordered by one property, rows 26
// to 50". For that shape the database can do all of it, and the page costs a
// bounded read instead of a scan of the type.
//
// Exactness comes from a NARROW gate rather than from a clever translation.
// A query is pushed only when every part of it has an obviously identical
// reading in SQL; anything else — and, deliberately, any GraphQuery field
// this file does not know about — declines to graphquerynaive. The
// differential test in graphsimple_test.go holds the two paths to the same
// rows in the same order.

// headerColumns is entityColumns with the body projected away, in the order
// scanEntity expects.
const headerColumns = "id, type, face, properties, '' AS content, updated_at"

var (
	_ store.HeaderReader       = (*Store)(nil)
	_ store.GraphHeaderQueryer = (*Store)(nil)
	_ store.MatchedCounter     = (*Store)(nil)
)

// ListEntityHeaders implements store.HeaderReader: ListEntities without the
// markdown body crossing the database boundary.
func (s *Store) ListEntityHeaders(ctx context.Context, q store.EntityQuery) iter.Seq2[store.EntityHeader, error] {
	if err := storeutil.ValidateEntityQuery(q); err != nil {
		return func(yield func(store.EntityHeader, error) bool) { yield(store.EntityHeader{}, err) }
	}
	sqlText, args := buildEntitySelectSQL(q, "", headerColumns)
	return s.headerRows(ctx, "list entity headers", sqlText, args)
}

// GraphQueryHeaders implements store.GraphHeaderQueryer.
func (s *Store) GraphQueryHeaders(ctx context.Context, q store.GraphQuery) iter.Seq2[store.EntityHeader, error] {
	sqlText, args, ok := simpleGraphSQL(q, headerColumns, false)
	if !ok {
		return func(yield func(store.EntityHeader, error) bool) {
			for e, err := range s.GraphQuery(ctx, q) {
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
	return s.headerRows(ctx, "graph query headers", sqlText, args)
}

// CountMatched implements store.MatchedCounter.
func (s *Store) CountMatched(ctx context.Context, q store.GraphQuery) (int, error) {
	sqlText, args, ok := simpleGraphSQL(q, headerColumns, true)
	if !ok {
		matched, _, err := s.GraphCount(ctx, q)
		return matched, err
	}
	var n int
	if err := s.q().QueryRowContext(ctx, sqlText, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("sqlitestore: count matched: %w", err)
	}
	return n, nil
}

// simpleGraphRows runs the SQL path for GraphQuery; ok=false means the shape
// is not simple and the caller must use graphquerynaive.
func (s *Store) simpleGraphRows(ctx context.Context, q store.GraphQuery) (iter.Seq2[*entity.Entity, error], bool) {
	sqlText, args, ok := simpleGraphSQL(q, entityColumns, false)
	if !ok {
		return nil, false
	}
	return func(yield func(*entity.Entity, error) bool) {
		rows, err := s.q().QueryContext(ctx, sqlText, args...)
		if err != nil {
			yield(nil, fmt.Errorf("sqlitestore: graph query: %w", err))
			return
		}
		defer rows.Close()
		for rows.Next() {
			e, err := scanEntity(rows)
			if !yield(e, err) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(nil, fmt.Errorf("sqlitestore: graph query: %w", err))
		}
	}, true
}

func (s *Store) headerRows(ctx context.Context, what, sqlText string, args []any) iter.Seq2[store.EntityHeader, error] {
	return func(yield func(store.EntityHeader, error) bool) {
		var rows *sql.Rows
		rows, err := s.q().QueryContext(ctx, sqlText, args...)
		if err != nil {
			yield(store.EntityHeader{}, fmt.Errorf("sqlitestore: %s: %w", what, err))
			return
		}
		defer rows.Close()
		for rows.Next() {
			e, err := scanEntity(rows)
			if err != nil {
				yield(store.EntityHeader{}, err)
				return
			}
			if !yield(store.HeaderOf(e), nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(store.EntityHeader{}, fmt.Errorf("sqlitestore: %s: %w", what, err))
		}
	}
}

// pushablePropertyName bounds the names that reach a JSON path. The path is
// text inside the statement (SQLite cannot bind one), so the name is held to
// a charset that needs no escaping inside the quoted-label form `$."name"`.
// A name outside it simply declines.
var pushablePropertyName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

func jsonPath(property string) string { return `'$."` + property + `"'` }

// simpleGraphSQL builds the statement for a simple-shaped q, or reports
// ok=false. count selects count(*) and ignores ordering and paging, as
// GraphCount does.
func simpleGraphSQL(q store.GraphQuery, columns string, count bool) (sqlText string, args []any, ok bool) {
	// The gate is an allowlist over the ZERO VALUE: clear the fields this
	// function translates, and whatever is left must be empty. A field added
	// to GraphQuery later therefore declines here until someone teaches this
	// function about it — it can never be silently ignored, which for a
	// narrowing field would widen the answer.
	rest := q
	rest.EntityType, rest.Props, rest.World, rest.FaceIn = "", nil, store.WorldScope{}, nil
	rest.OrderBy, rest.Limit, rest.Offset = nil, 0, 0
	if !reflect.DeepEqual(rest, store.GraphQuery{}) || q.EntityType == "" {
		return "", nil, false
	}
	eq := store.EntityQuery{Type: q.EntityType, World: q.World, FaceIn: q.FaceIn}
	if storeutil.ValidateEntityQuery(eq) != nil {
		return "", nil, false // let the naive path report the error its way
	}

	var conds []string
	var condArgs []any
	for _, p := range q.Props {
		// Only the scalar-string equality has a reading SQL shares exactly:
		// the stored value is JSON text and equals Value. Emptiness tests,
		// negation, list membership and ordered comparison all go through
		// propmatch's rules for non-string values, which stay in Go.
		if !p.Scalar || p.Op != store.PropEqual || p.Value == "" || !pushablePropertyName.MatchString(p.Property) {
			return "", nil, false
		}
		path := jsonPath(p.Property)
		conds = append(conds, "json_type(properties, "+path+") = 'text' AND json_extract(properties, "+path+") = ?")
		condArgs = append(condArgs, p.Value)
	}

	// World resolution happens INSIDE, property predicates OUTSIDE: a
	// predicate judges the entity's prime, exactly as graphquerynaive judges
	// the rows ListEntities resolved.
	inner, args := buildEntitySelectSQL(eq, "", columns)
	inner = strings.TrimSuffix(inner, " ORDER BY id, face")
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
		args = append(args, condArgs...)
	}
	if count {
		return "SELECT count(*) FROM (" + inner + ")" + where, args, true
	}

	var order strings.Builder
	for _, spec := range q.OrderBy {
		if !pushablePropertyName.MatchString(spec.Property) {
			return "", nil, false
		}
		// The sort key is the value's TEXT form, compared byte-wise, with a
		// missing or JSON-null value as the largest — GraphQuery.OrderBy's
		// contract. SQLite's own NULL placement is the opposite of that, so
		// the null test leads each key explicitly.
		path := jsonPath(spec.Property)
		key := "(CASE json_type(properties, " + path + ")" +
			" WHEN 'true' THEN 'true' WHEN 'false' THEN 'false' WHEN 'null' THEN NULL" +
			" ELSE CAST(json_extract(properties, " + path + ") AS TEXT) END)"
		if spec.Descending {
			order.WriteString(key + " IS NULL DESC, " + key + " COLLATE BINARY DESC, ")
		} else {
			order.WriteString(key + " IS NULL ASC, " + key + " COLLATE BINARY ASC, ")
		}
	}
	order.WriteString("id ASC, face ASC")

	sqlText = "SELECT " + columns + " FROM (" + inner + ")" + where + " ORDER BY " + order.String()
	if q.Limit > 0 || q.Offset > 0 {
		limit := -1 // SQLite's spelling of "no limit", needed to carry an OFFSET
		if q.Limit > 0 {
			limit = q.Limit
		}
		sqlText += " LIMIT ? OFFSET ?"
		args = append(args, limit, max(q.Offset, 0))
	}
	return sqlText, args, true
}
