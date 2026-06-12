package pgstore

import (
	"context"
	"fmt"
	"iter"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// compile-time check: the postgres store is a native VisibleSearcher
// (the appbuild postgres recipe wires the store itself; simple backends
// use search.NewVisible instead).
var _ search.VisibleSearcher = (*Store)(nil)

// SearchVisible is the native postgres implementation of
// [search.VisibleSearcher]: visibility is composed into the search
// statement itself, so hidden rows are never returned, the LIMIT
// applies post-visibility (no cap starvation), and there is no
// per-type MatchingIDs round trip.
//
// Shape: the trgm-accelerated LIKE from [SearchBackend.Search] ANDed
// with a per-type visibility disjunction — a bare type test for
// AllowAll entries, the [buildPredicateSQL] EXISTS chain (with
// per-type CTE prefixes) for Query entries, denied types omitted. A
// wildcard-allow scope drops the disjunction entirely. Ordering
// matches the ungated backend (`similarity DESC, id ASC`; plain id
// order for empty text) so the gated stream is the ungated stream
// minus hidden rows — the conformance suite's ordered-subsequence
// invariant.
//
// Because search and visibility execute as one statement, any query
// failure is wrapped in [search.ErrScope]: the statement IS the gate
// here, and consumers route ErrScope through their ACL-error path
// (cancel-silent / deadline-504 mapping included — ctx is threaded
// into the query, unlike the legacy ctx-less Backend.Search).
//
// Go-side residue: q.Filters cannot be pushed down, so when filters
// are present the SQL LIMIT is omitted and the limit is enforced after
// filtering — a SQL LIMIT before Go-side filters would re-open the
// starvation gap the post-visibility contract closes.
func (s *Store) SearchVisible(
	ctx context.Context, q search.Query, scope map[string]search.TypeScope,
) iter.Seq2[search.Hit, error] {
	return func(yield func(search.Hit, error) bool) {
		if err := search.ValidateFilters(q.Filters); err != nil {
			yield(search.Hit{}, err)
			return
		}
		if ws, ok := scope[search.WildcardType]; ok && ws.Query != nil {
			yield(search.Hit{}, fmt.Errorf("%w: wildcard scope entry cannot carry a GraphQuery", search.ErrScope))
			return
		}

		sqlText, args, anyVisible := buildVisibleSearchSQL(q, scope)
		if !anyVisible {
			return // empty effective scope: deny everything, skip the query
		}

		rows, err := s.db.Query(ctx, sqlText, args...)
		if err != nil {
			yield(search.Hit{}, fmt.Errorf("%w: pgstore visible search: %w", search.ErrScope, err))
			return
		}
		defer rows.Close()

		emitted := 0
		for rows.Next() {
			e, scanErr := scanEntity(rows)
			if scanErr != nil {
				yield(search.Hit{}, fmt.Errorf("%w: pgstore visible search scan: %w", search.ErrScope, scanErr))
				return
			}
			if !search.MatchFilters(e, q.Filters) {
				continue
			}
			if q.Limit > 0 && emitted >= q.Limit {
				return
			}
			if !yield(search.Hit{ID: e.ID, Type: e.Type, Title: e.Title()}, nil) {
				return
			}
			emitted++
		}
		if err := rows.Err(); err != nil {
			yield(search.Hit{}, fmt.Errorf("%w: pgstore visible search: %w", search.ErrScope, err))
		}
	}
}

// buildVisibleSearchSQL emits the combined search+visibility statement.
// The third return is false when the scope admits nothing — the caller
// must not run a query at all in that case.
//
// Scope keys are visited in sorted order so the SQL text and arg list
// are deterministic for a given scope (map iteration order must never
// reach the wire). Every value flows through [sqlBuilder.arg]; the only
// interpolated strings are placeholder names and the compile-time CTE
// prefixes ("v<i>_in"/"v<i>_out") — same injection-safety property as
// buildGraphQuerySQL.
func buildVisibleSearchSQL(
	q search.Query, scope map[string]search.TypeScope,
) (sqlText string, args []any, anyVisible bool) {
	b := &sqlBuilder{}

	wildcardAllow := false
	if ws, ok := scope[search.WildcardType]; ok && ws.AllowAll {
		wildcardAllow = true
	}

	var withParts, visParts []string
	if !wildcardAllow {
		withParts, visParts = buildVisibilityDisjunction(b, scope)
		if len(visParts) == 0 {
			return "", nil, false
		}
	}

	var sb strings.Builder
	if len(withParts) > 0 {
		sb.WriteString("WITH RECURSIVE ")
		sb.WriteString(strings.Join(withParts, ",\n"))
		sb.WriteByte('\n')
	}
	sb.WriteString("SELECT e.id, e.type, e.properties, e.content, e.updated_at FROM entities e WHERE TRUE")

	// Text match + ordering mirror SearchBackend.Search exactly:
	// escaped needle for LIKE, raw lowercased needle for similarity,
	// id ASC ties — the parity baseline orders by the same expressions.
	orderBy := " ORDER BY e.id ASC"
	if q.Text != "" {
		needle := strings.ToLower(q.Text)
		sb.WriteString(" AND e.search_text LIKE '%' || " + b.arg(escapeLike(needle)) + ` || '%' ESCAPE '\'`)
		orderBy = " ORDER BY similarity(e.search_text, " + b.arg(needle) + ") DESC, e.id ASC"
	}
	if len(q.Types) > 0 {
		sb.WriteString(" AND e.type = ANY(" + b.arg(q.Types) + ")")
	}
	if !wildcardAllow {
		sb.WriteString(" AND (" + strings.Join(visParts, " OR ") + ")")
	}
	sb.WriteString(orderBy)
	if q.Limit > 0 && len(q.Filters) == 0 {
		// With Go-side filters pending, the limit moves above them —
		// see the method godoc.
		sb.WriteString(" LIMIT " + b.arg(q.Limit))
	}
	return sb.String(), b.args, true
}

// buildVisibilityDisjunction emits the per-type OR-parts of the
// visibility clause: a bare type test for AllowAll entries, type test
// + EXISTS chain for Query entries, nothing for deny entries. Scope
// keys are visited in sorted order; CTE names get per-type prefixes
// ("v<i>_in"/"v<i>_out") so two Query verdicts can't collide.
func buildVisibilityDisjunction(b *sqlBuilder, scope map[string]search.TypeScope) (withParts, visParts []string) {
	types := make([]string, 0, len(scope))
	for typ := range scope {
		if typ != search.WildcardType {
			types = append(types, typ)
		}
	}
	slices.Sort(types)

	for i, typ := range types {
		ts := scope[typ]
		switch {
		case ts.AllowAll:
			visParts = append(visParts, "e.type = "+b.arg(typ))
		case ts.Query != nil:
			// The scope-map key, not ts.Query.EntityType, drives the
			// type test: the seam contract makes the consumer keep
			// them equal, and keying on the map entry means a
			// mismatched Query can only ever narrow its own type.
			typeArg := b.arg(typ)
			var part strings.Builder
			part.WriteString("(e.type = " + typeArg)
			if ts.Query.HasInbound != nil {
				w, ex := buildPredicateSQL(b, fmt.Sprintf("v%d_in", i), *ts.Query.HasInbound, typeArg, store.DirectionIncoming)
				withParts = append(withParts, w...)
				part.WriteString(" AND EXISTS (" + ex + ")")
			}
			if ts.Query.HasOutbound != nil {
				w, ex := buildPredicateSQL(b, fmt.Sprintf("v%d_out", i), *ts.Query.HasOutbound, typeArg, store.DirectionOutgoing)
				withParts = append(withParts, w...)
				part.WriteString(" AND EXISTS (" + ex + ")")
			}
			part.WriteByte(')')
			visParts = append(visParts, part.String())
		default:
			// zero-value entry: explicit deny, same as absence.
		}
	}
	return withParts, visParts
}
