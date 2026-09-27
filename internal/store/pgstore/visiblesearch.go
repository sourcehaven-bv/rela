package pgstore

import (
	"context"
	"fmt"
	"iter"
	"slices"
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

		sqlText, args, anyVisible := buildVisibleSearchSQL(q, scope, s.searchTitles)
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
// The per-field match is computed in Go with [search.MatchTextFields] — the
// same ground-truth matcher the generic backend uses — so pgstore and the
// simple backends agree. It runs in TWO passes (TKT-U9DYW4), because the
// candidate rows no longer carry bodies:
//
//  1. Every row is judged from its id and properties. A row with no hidden
//     fields, or whose match a VISIBLE property or the id already explains,
//     is kept here.
//  2. The rows left over are those only a body can decide. Their bodies are
//     fetched in one statement, for exactly the (id, face) pairs the gated
//     query resolved, and the verdict is recomputed with the body in place.
//
// Pass 1 can only over-approximate "needs a body": a blank Content can remove
// the content field from the matched set, never add a field to it. So no row
// is kept in pass 1 that the single-pass code would have dropped.
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

		sqlText, args, anyVisible := buildVisibleSearchSQL(q, scope, s.searchTitles)
		if !anyVisible {
			return
		}

		rows, err := s.db.Query(ctx, sqlText, args...)
		if err != nil {
			yield(search.Hit{}, fmt.Errorf("%w: pgstore visible search: %w", search.ErrScope, err))
			return
		}
		defer rows.Close()

		emitFieldVisibleRows(ctx, s.db, rows, q, hidden, yield)
	}
}

// emitFieldVisibleRows scans the visible-search rows, applies the Go-side
// property filter and the property-level (hidden-field) drop, and yields the
// survivors up to q.Limit. Any scan/row/hidden-func error is yielded and stops
// iteration. Extracted from SearchVisibleFields to keep that closure's
// branching within the complexity budget.
func emitFieldVisibleRows(
	ctx context.Context, db DBTX, rows pgx.Rows, q search.Query, hidden search.HiddenFieldsFunc,
	yield func(search.Hit, error) bool,
) {
	// Pass 1 decides every row it can from id and properties alone, and
	// remembers the rows whose verdict depends on the body.
	var (
		cands    []searchCandidate
		bodyless []stateKey
		decided  int
	)
	for rows.Next() {
		e, scanErr := scanEntity(rows)
		if scanErr != nil {
			yield(search.Hit{}, fmt.Errorf("%w: pgstore visible search scan: %w", search.ErrScope, scanErr))
			return
		}
		if !search.MatchFilters(e, q.Filters) {
			continue
		}
		c, err := judgeWithoutBody(ctx, q, e, hidden)
		if err != nil {
			yield(search.Hit{}, err)
			return
		}
		if c.needBody {
			bodyless = append(bodyless, stateKey{id: e.ID, face: e.Face})
		}
		cands = append(cands, c)
		if !c.needBody {
			decided++
		}
		// Rows are emitted in order, up to q.Limit. Once that many rows are
		// already KEPT, nothing further can be emitted — the undecided rows
		// ahead of them can only add to the kept set — so stop buffering.
		// This is what bounds memory when Go-side filters moved the LIMIT
		// out of the SQL.
		if q.Limit > 0 && decided >= q.Limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		yield(search.Hit{}, fmt.Errorf("%w: pgstore visible search: %w", search.ErrScope, err))
		return
	}
	rows.Close()

	bodies, err := searchBodies(ctx, db, bodyless)
	if err != nil {
		yield(search.Hit{}, fmt.Errorf("%w: pgstore visible search bodies: %w", search.ErrScope, err))
		return
	}

	emitted := 0
	for _, c := range cands {
		if c.needBody {
			c.e.Content = bodies[stateKey{id: c.e.ID, face: c.e.Face}]
			if !search.MatchHasVisibleField(search.MatchTextFields(c.e, q.Text), c.hidden) {
				continue
			}
		}
		if q.Limit > 0 && emitted >= q.Limit {
			return
		}
		if !yield(c.hit, nil) {
			return
		}
		emitted++
	}
}

// searchCandidate is one gated search row between the two passes.
type searchCandidate struct {
	hit      search.Hit
	e        *entity.Entity
	hidden   map[string]struct{}
	needBody bool
}

// judgeWithoutBody is pass 1 for one row: needBody is set only when the row
// has hidden fields and neither its id nor a visible property explains the
// match, so only its body can.
func judgeWithoutBody(
	ctx context.Context, q search.Query, e *entity.Entity, hidden search.HiddenFieldsFunc,
) (searchCandidate, error) {
	c := searchCandidate{hit: search.Hit{ID: e.ID, Type: e.Type, Title: e.Title()}, e: e}
	if hidden == nil || q.Text == "" {
		return c, nil
	}
	hf, err := hidden(ctx, c.hit, e)
	if err != nil {
		return c, fmt.Errorf("%w: hidden-fields for %q: %w", search.ErrScope, c.hit.ID, err)
	}
	if len(hf) > 0 && !search.MatchHasVisibleField(search.MatchTextFields(e, q.Text), hf) {
		c.hidden, c.needBody = hf, true
	}
	return c, nil
}

// stateKey addresses one stored state of an entity.
type stateKey struct {
	id   string
	face entity.Face
}

// searchBodies loads the bodies of exactly the given states. The keys are the
// (id, face) pairs the GATED query resolved, so this statement reads no row
// the gate did not already admit — it must never widen to "every state of
// these ids", which would pull a face the world or the ACL withheld.
func searchBodies(ctx context.Context, db DBTX, keys []stateKey) (map[stateKey]string, error) {
	if len(keys) == 0 {
		return map[stateKey]string{}, nil
	}
	ids, faces := make([]string, len(keys)), make([]string, len(keys))
	for i, k := range keys {
		ids[i], faces[i] = k.id, string(k.face)
	}
	rows, err := db.Query(ctx,
		"SELECT e.id, e.face, e.content FROM entities e"+
			" JOIN unnest($1::text[], $2::text[]) AS k(id, face) ON e.id = k.id AND e.face = k.face",
		ids, faces)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[stateKey]string, len(keys))
	for rows.Next() {
		var id, face, content string
		if err := rows.Scan(&id, &face, &content); err != nil {
			return nil, err
		}
		out[stateKey{id: id, face: entity.Face(face)}] = content
	}
	return out, rows.Err()
}

// visibleSearchColumns is scanEntity's column list with the body projected
// away (TKT-U9DYW4). A hit is an id, a type and a title; the property filters
// read properties only. The one consumer of a body is the hidden-field check,
// and only for a row whose match is not already explained by a visible
// property — those bodies are fetched afterwards, for exactly those rows
// (see emitFieldVisibleRows). Shipping every candidate's body was 280 ms of a
// 290 ms search at 1000 hits.
const visibleSearchColumns = "e.id, e.type, e.face, e.properties, ''::text AS content, e.updated_at"

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
	q search.Query, scope map[string]search.TypeScope, titles SearchTitles,
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
		sb.WriteString("SELECT " + visibleSearchColumns + " FROM entities e WHERE e.face = ''")
	} else {
		rank, candidate := worldSQL(q.World, "e", &b.args)
		sb.WriteString("SELECT " + visibleSearchColumns + " FROM (" +
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
		orderBy = " ORDER BY " + titles.rankSQL("e", func(v any) string { return b.arg(v) }, needle) + " DESC, e.id ASC"
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
			// The ACL never fills Related today. It is rendered anyway,
			// because skipping a field of the gate's query would widen it.
			for j, rel := range ts.Query.Related {
				w, ex := buildPredicateSQL(b, fmt.Sprintf("v%d_%s", i, relatedPrefix(j)), rel.Pred, typeArg,
					relatedDirection(rel))
				withParts = append(withParts, w...)
				part.WriteString(" AND " + existsCond(ex, rel.Pred.Negate))
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
