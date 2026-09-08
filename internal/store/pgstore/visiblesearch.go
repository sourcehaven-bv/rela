package pgstore

import (
	"context"
	"fmt"
	"iter"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/Sourcehaven-BV/rela/internal/entity"
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

// compile-time check: the postgres store also filters at the property level.
var _ search.FieldVisibleSearcher = (*Store)(nil)

// SearchVisibleFields is [Store.SearchVisible] plus property-level redaction of the
// match-on-hidden-field oracle. A hit whose text matched only fields the
// principal may not see (per hidden) is dropped, so a search cannot confirm a
// redacted property's value by returning its entity.
//
// The per-field match is computed in Go over the already-scanned entity with
// [search.MatchTextFields] — the same ground-truth matcher the generic backend
// uses — so pgstore and the simple backends agree. This does NOT push the
// field projection into SQL; the entity is already in hand from the visibility
// scan, so the filter is a cheap in-memory pass over the candidate rows. A SQL
// pushdown (matching per-column server-side) is a tracked performance
// follow-up, not a correctness gap.
func (s *Store) SearchVisibleFields(
	ctx context.Context, q search.Query, scope map[string]search.TypeScope, hidden search.HiddenFieldsFunc,
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
			return
		}

		rows, err := s.db.Query(ctx, sqlText, args...)
		if err != nil {
			yield(search.Hit{}, fmt.Errorf("%w: pgstore visible search: %w", search.ErrScope, err))
			return
		}
		defer rows.Close()

		emitFieldVisibleRows(ctx, rows, q, hidden, yield)
	}
}

// emitFieldVisibleRows scans the visible-search rows, applies the Go-side
// property filter and the property-level (hidden-field) drop, and yields the
// survivors up to q.Limit. Any scan/row/hidden-func error is yielded and stops
// iteration. Extracted from SearchVisibleFields to keep that closure's
// branching within the complexity budget.
func emitFieldVisibleRows(
	ctx context.Context, rows pgx.Rows, q search.Query, hidden search.HiddenFieldsFunc,
	yield func(search.Hit, error) bool,
) {
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
		hit := search.Hit{ID: e.ID, Type: e.Type, Title: e.Title()}
		keep, ferr := fieldVisibleForEntity(ctx, q, hit, e, hidden)
		if ferr != nil {
			yield(search.Hit{}, ferr)
			return
		}
		if !keep {
			continue
		}
		if q.Limit > 0 && emitted >= q.Limit {
			return
		}
		if !yield(hit, nil) {
			return
		}
		emitted++
	}
	if err := rows.Err(); err != nil {
		yield(search.Hit{}, fmt.Errorf("%w: pgstore visible search: %w", search.ErrScope, err))
	}
}

// fieldVisibleForEntity decides whether a hit survives the property-level
// filter, given the already-loaded entity. Mirrors search.Visible.fieldVisible
// but computes provenance directly with [search.MatchTextFields] (the entity is
// in hand, no reader round-trip). Keeps the hit when field filtering does not
// apply (no hidden func, no text) or an empty hidden set; otherwise keeps only
// if a non-hidden field matched. A hidden-func error fails closed.
func fieldVisibleForEntity(
	ctx context.Context, q search.Query, h search.Hit, e *entity.Entity, hidden search.HiddenFieldsFunc,
) (bool, error) {
	if hidden == nil || q.Text == "" {
		return true, nil
	}
	hiddenFields, err := hidden(ctx, h, e)
	if err != nil {
		return false, fmt.Errorf("%w: hidden-fields for %q: %w", search.ErrScope, h.ID, err)
	}
	if len(hiddenFields) == 0 {
		return true, nil
	}
	matched := search.MatchTextFields(e, q.Text)
	return search.MatchHasVisibleField(matched, hiddenFields), nil
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
	// The world scope rides q.World (TKT-9KZGJO). Two shapes, exactly as in
	// SearchBackend.Search:
	//
	//   - DEFAULT world: `e.face = ''`, the historical query verbatim, so
	//     a project with no faces pays nothing.
	//   - non-default: resolve per FAMILY with DISTINCT ON, because a world
	//     is a ranked preference and `face IN (...)` would return two rows
	//     for an entity holding two coordinates.
	//
	// LOCKSTEP with pgstore/search.go's SearchBackend.Search: the gated and
	// ungated streams are held to an ordered-subsequence conformance
	// contract, so these two scopes must change together. Both now build
	// their candidate/rank expressions from the SAME worldSQL helper, which
	// is what makes "together" structural rather than a promise.
	//
	// Note the ACL row gate cannot cover for a mistake here: guard rule 1
	// makes the row gate world-INDEPENDENT, so a draft leaking through this
	// scope would not be caught downstream.
	//
	// The ACL visibility clause is applied to the RESOLVED row, after the
	// prime is chosen — world first, gate second, the same order
	// internal/worldreader fixes for the read path. Gating first would let
	// what the ACL denied change WHICH face the world resolves to, which is
	// the existence oracle that ordering exists to close.
	if q.World.IsDefaultWorld() {
		sb.WriteString("SELECT e.id, e.type, e.face, e.properties, e.content, e.updated_at " +
			"FROM entities e WHERE e.face = ''")
	} else {
		rank, candidate := worldSQL(q.World, "e", &b.args)
		sb.WriteString("SELECT e.id, e.type, e.face, e.properties, e.content, e.updated_at FROM (" +
			"SELECT DISTINCT ON (id) * FROM entities e WHERE " + candidate +
			" ORDER BY id ASC, (" + rank + ") ASC, face ASC) e WHERE true")
	}

	// Text match + ordering mirror SearchBackend.Search exactly:
	// escaped needle for LIKE, raw lowercased needle for similarity,
	// id ASC ties — the parity baseline orders by the same expressions.
	orderBy := " ORDER BY e.id ASC"
	if q.Text != "" {
		needle := strings.ToLower(q.Text)
		sb.WriteString(" AND e.search_text LIKE '%' || " + b.arg(escapeLike(needle)) + ` || '%' ESCAPE '\'`)
		orderBy = " ORDER BY similarity(left(e.search_text, " + strconv.Itoa(searchRankPrefix) + "), " +
			b.arg(needle) + ") DESC, e.id ASC"
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
			if len(ts.Query.Any) > 0 {
				w, cond := buildAnySQL(b, fmt.Sprintf("v%d_any", i), ts.Query.Any, typeArg)
				withParts = append(withParts, w...)
				part.WriteString(" AND " + cond)
			}
			part.WriteByte(')')
			visParts = append(visParts, part.String())
		default:
			// zero-value entry: explicit deny, same as absence.
		}
	}
	return withParts, visParts
}
