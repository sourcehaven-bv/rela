package pgstore

import (
	"context"
	"fmt"
	"iter"
	"slices"
	"strings"
	"time"

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
// per-type MatchingFaces round trip.
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
// Go-side residue: q.Filters cannot be pushed down, so the limit is
// enforced after filtering. The result is read in keyset pages (see
// visiblePages) until that many rows passed, so a page LIMIT never
// starves the filtered result the way a single SQL LIMIT would.
func (s *Store) SearchVisible(
	ctx context.Context, q search.Query, scope map[string]search.TypeScope,
) iter.Seq2[search.Hit, error] {
	return func(yield func(search.Hit, error) bool) {
		if err := search.ValidateQuery(q); err != nil {
			yield(search.Hit{}, err)
			return
		}
		if err := search.ValidateScope(scope); err != nil {
			yield(search.Hit{}, err)
			return
		}

		emitted := 0
		for page, err := range visiblePages(ctx, s.db, s.searchTitles, q, scope) {
			if err != nil {
				yield(search.Hit{}, err)
				return
			}
			for _, e := range page {
				if q.Limit > 0 && emitted >= q.Limit {
					return
				}
				if !yield(search.Hit{ID: e.ID, Type: e.Type, Title: e.Title(), Face: e.Face}, nil) {
					return
				}
				emitted++
			}
		}
	}
}

// visibleKey is a visible-search row's keyset key: the rank the result is
// ordered by (descending; zero when the query has no text) and the id.
type visibleKey struct {
	rank float32
	id   string
}

// visiblePages reads a visible search in keyset pages and yields each page's
// rows that pass q.Filters, with the page's rows already closed, so no
// connection is held while the caller yields or runs a hidden-fields callback
// (see defaultIteratorPageSize). The rows carry no bodies. Nothing is yielded
// when the scope admits nothing. A page is at most q.Limit rows when no
// Go-side filter can drop any, so a limited search usually costs one
// statement.
func visiblePages(
	ctx context.Context, db DBTX, titles SearchTitles, q search.Query, scope map[string]search.TypeScope,
) iter.Seq2[[]*entity.Entity, error] {
	return func(yield func([]*entity.Entity, error) bool) {
		size := int(iteratorPageSize.Load())
		if q.Limit > 0 && q.Limit < size && len(q.Filters) == 0 {
			size = q.Limit
		}
		var after *visibleKey
		for {
			rows, anyVisible, err := readVisiblePage(ctx, db, titles, q, scope, after, size)
			if err != nil {
				yield(nil, fmt.Errorf("%w: pgstore visible search: %w", search.ErrScope, err))
				return
			}
			if !anyVisible {
				return // empty effective scope: deny everything, skip the query
			}
			page := make([]*entity.Entity, 0, len(rows))
			for _, r := range rows {
				if search.MatchFilters(r.e, q.Filters) {
					page = append(page, r.e)
				}
			}
			if len(page) > 0 && !yield(page, nil) {
				return
			}
			if len(rows) < size {
				return
			}
			last := rows[len(rows)-1].key
			if after != nil && last == *after {
				yield(nil, fmt.Errorf("%w: pgstore visible search: keyset paging made no progress", search.ErrScope))
				return
			}
			after = &last
		}
	}
}

// readVisiblePage builds and runs one keyset page of a visible search.
// anyVisible is false, with no query run, when the scope admits nothing.
func readVisiblePage(
	ctx context.Context, db DBTX, titles SearchTitles, q search.Query, scope map[string]search.TypeScope,
	after *visibleKey, size int,
) (rows []visibleRow, anyVisible bool, err error) {
	sqlText, args, anyVisible, err := buildVisibleSearchSQL(q, scope, titles, after, size)
	if err != nil || !anyVisible {
		return nil, false, err
	}
	rows, err = queryAll(ctx, db, sqlText, args, scanVisibleRow)
	return rows, err == nil, err
}

// visibleRow is one visible-search row and its keyset key.
type visibleRow struct {
	e   *entity.Entity
	key visibleKey
}

func scanVisibleRow(row scanner) (visibleRow, error) {
	var (
		id, typ, content, ptr string
		props                 []byte
		updatedAt             time.Time
		rank                  float32
	)
	if err := row.Scan(&id, &typ, &ptr, &props, &content, &updatedAt, &rank); err != nil {
		return visibleRow{}, err
	}
	e := entity.New(id, typ)
	e.Face = entity.Face(ptr)
	e.UpdatedAt = updatedAt
	var err error
	if e.Properties, err = unmarshalProps(props); err != nil {
		return visibleRow{}, err
	}
	return visibleRow{e: e, key: visibleKey{rank: rank, id: id}}, nil
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
		if err := search.ValidateQuery(q); err != nil {
			yield(search.Hit{}, err)
			return
		}
		if err := search.ValidateScope(scope); err != nil {
			yield(search.Hit{}, err)
			return
		}

		emitted := 0
		for page, err := range visiblePages(ctx, s.db, s.searchTitles, q, scope) {
			if err != nil {
				yield(search.Hit{}, err)
				return
			}
			remaining := 0
			if q.Limit > 0 {
				remaining = q.Limit - emitted
			}
			n, more := emitFieldVisibleRows(ctx, s.db, page, q, hidden, remaining, yield)
			emitted += n
			if !more || (q.Limit > 0 && emitted >= q.Limit) {
				return
			}
		}
	}
}

// emitFieldVisibleRows applies the property-level (hidden-field) drop to one
// page of filtered visible-search rows and yields the survivors, at most
// limit of them when limit > 0. It reports how many it yielded, and more is
// false once iteration must stop: the consumer broke off, or a hidden-func
// or body-read error was yielded. Extracted from SearchVisibleFields to keep
// that closure's branching within the complexity budget.
func emitFieldVisibleRows(
	ctx context.Context, db DBTX, ents []*entity.Entity, q search.Query, hidden search.HiddenFieldsFunc,
	limit int, yield func(search.Hit, error) bool,
) (emitted int, more bool) {
	// Pass 1 decides every row it can from id and properties alone, and
	// remembers the rows whose verdict depends on the body.
	var (
		cands    []searchCandidate
		bodyless []entity.Ref
		decided  int
	)
	for _, e := range ents {
		c, err := judgeWithoutBody(ctx, q, e, hidden)
		if err != nil {
			yield(search.Hit{}, err)
			return 0, false
		}
		if c.needBody {
			bodyless = append(bodyless, e.Ref())
		}
		cands = append(cands, c)
		if !c.needBody {
			decided++
		}
		// Rows are emitted in order, up to limit. Once that many rows are
		// already KEPT, nothing further can be emitted — the undecided rows
		// ahead of them can only add to the kept set — so stop judging.
		if limit > 0 && decided >= limit {
			break
		}
	}

	bodies, err := searchBodies(ctx, db, bodyless)
	if err != nil {
		yield(search.Hit{}, fmt.Errorf("%w: pgstore visible search bodies: %w", search.ErrScope, err))
		return 0, false
	}

	for _, c := range cands {
		if c.needBody {
			c.e.Content = bodies[c.e.Ref()]
			if !search.MatchHasVisibleField(search.MatchTextFields(c.e, q.Text), c.hidden) {
				continue
			}
		}
		if limit > 0 && emitted >= limit {
			return emitted, true
		}
		if !yield(c.hit, nil) {
			return emitted, false
		}
		emitted++
	}
	return emitted, true
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
	c := searchCandidate{hit: search.Hit{ID: e.ID, Type: e.Type, Title: e.Title(), Face: e.Face}, e: e}
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

// searchBodies loads the bodies of exactly the given states. The keys are the
// (id, face) pairs the GATED query resolved, so this statement reads no row
// the gate did not already admit — it must never widen to "every state of
// these ids", which would pull a face the world or the ACL withheld.
func searchBodies(ctx context.Context, db DBTX, keys []entity.Ref) (map[entity.Ref]string, error) {
	if len(keys) == 0 {
		return map[entity.Ref]string{}, nil
	}
	ids, faces := make([]string, len(keys)), make([]string, len(keys))
	for i, k := range keys {
		ids[i], faces[i] = k.ID, string(k.Face)
	}
	rows, err := db.Query(ctx,
		"SELECT e.id, e.face, e.content FROM entities e"+
			" JOIN unnest($1::text[], $2::text[]) AS k(id, face) ON e.id = k.id AND e.face = k.face",
		ids, faces)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[entity.Ref]string, len(keys))
	for rows.Next() {
		var id, face, content string
		if err := rows.Scan(&id, &face, &content); err != nil {
			return nil, err
		}
		out[entity.Ref{ID: id, Face: entity.Face(face)}] = content
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
//
// It reads one keyset page of at most pageLimit rows after the key after (nil
// for the first page). Each row carries its rank as a seventh column, which
// is the key's first part; see visiblePages.
func buildVisibleSearchSQL(
	q search.Query, scope map[string]search.TypeScope, titles SearchTitles, after *visibleKey, pageLimit int,
) (sqlText string, args []any, anyVisible bool, err error) {
	b := &sqlBuilder{}

	wildcardAllow := false
	if ws, ok := scope[search.WildcardType]; ok && ws.AllowAll {
		wildcardAllow = true
	}

	var withParts, visParts []string
	if !wildcardAllow {
		withParts, visParts = buildVisibilityDisjunction(b, scope, store.InWorld(q.World))
		if len(visParts) == 0 {
			return "", nil, false, nil
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
	// The ACL visibility clause trims the CANDIDATE faces before the world
	// ranks them (TKT-7IZHP0), as on lists and the single-entity read: a
	// principal whose grant denies the prime is served the next readable
	// face rather than losing the entity. The verdict is evaluated per face
	// row, and only the resolved rows come out, so a face the world ranked
	// out is never counted or matched.
	visCond := ""
	if !wildcardAllow {
		visCond = " AND (" + strings.Join(visParts, " OR ") + ")"
	}
	// Text match + ordering mirror SearchBackend.Search exactly:
	// escaped needle for LIKE, raw lowercased needle for similarity,
	// id ASC ties — the parity baseline orders by the same expressions.
	// The rank is rendered twice, as a column and in the keyset condition,
	// because WHERE cannot name a SELECT alias.
	rankCol, orderBy := "0::real", " ORDER BY e.id ASC"
	var needle string
	if q.Text != "" {
		needle = strings.ToLower(q.Text)
		rankCol = titles.rankSQL("e", func(v any) string { return b.arg(v) }, needle)
		orderBy = " ORDER BY search_rank DESC, e.id ASC"
	}
	columns := visibleSearchColumns + ", " + rankCol + " AS search_rank"
	if q.World.IsTrivial() {
		sb.WriteString("SELECT " + columns + " FROM entities e WHERE e.face = ''" + visCond)
	} else {
		rank, candidate := worldSQL(q.World, "e", &b.args)
		sb.WriteString("SELECT " + columns + " FROM (" +
			"SELECT DISTINCT ON (id) * FROM entities e WHERE " + candidate + visCond +
			" ORDER BY id ASC, (" + rank + ") ASC, face ASC) e WHERE true")
	}
	if q.Text != "" {
		sb.WriteString(" AND e.search_text LIKE '%' || " + b.arg(escapeLike(needle)) + ` || '%' ESCAPE '\'`)
	}
	if after != nil {
		if q.Text == "" {
			sb.WriteString(" AND e.id > " + b.arg(after.id))
		} else {
			rank := titles.rankSQL("e", func(v any) string { return b.arg(v) }, needle)
			r, id := b.arg(after.rank), b.arg(after.id)
			sb.WriteString(" AND (" + rank + " < " + r + " OR (" + rank + " = " + r + " AND e.id > " + id + "))")
		}
	}
	if len(q.Types) > 0 {
		sb.WriteString(" AND e.type = ANY(" + b.arg(q.Types) + ")")
	}
	sb.WriteString(orderBy)
	sb.WriteString(" LIMIT " + b.arg(pageLimit))
	if b.err != nil {
		return "", nil, false, b.err
	}
	return sb.String(), b.args, true, nil
}

// buildVisibilityDisjunction emits the per-type OR-parts of the
// visibility clause: a bare type test for AllowAll entries, type test
// + EXISTS chain for Query entries, nothing for deny entries. Scope
// keys are visited in sorted order; CTE names get per-type prefixes
// ("v<i>_in"/"v<i>_out") so two Query verdicts can't collide. sel is the
// search's world: an EndpointMatch in a gate query that names no selection
// of its own reads its endpoint there.
func buildVisibilityDisjunction(
	b *sqlBuilder, scope map[string]search.TypeScope, sel store.FaceSelection,
) (withParts, visParts []string) {
	types := make([]string, 0, len(scope))
	for typ := range scope {
		if typ != search.WildcardType {
			types = append(types, typ)
		}
	}
	slices.Sort(types)

	for i, typ := range types {
		ts := scope[typ]
		faceCond := ""
		if len(ts.Faces) > 0 {
			vals := make([]string, len(ts.Faces))
			for j, f := range ts.Faces {
				vals[j] = f.String()
			}
			faceCond = " AND e.face = ANY(" + b.arg(vals) + ")"
		}
		switch {
		case ts.AllowAll:
			visParts = append(visParts, "(e.type = "+b.arg(typ)+faceCond+")")
		case ts.Query != nil:
			// The scope-map key, not ts.Query.EntityType, drives the
			// type test: the seam contract makes the consumer keep
			// them equal, and keying on the map entry means a
			// mismatched Query can only ever narrow its own type.
			typeArg := b.arg(typ)
			var part strings.Builder
			part.WriteString("(e.type = " + typeArg + faceCond)
			if ts.Query.HasInbound != nil {
				w, ex := buildPredicateSQL(b, fmt.Sprintf("v%d_in", i), *ts.Query.HasInbound, typeArg,
					store.DirectionIncoming, sel)
				withParts = append(withParts, w...)
				part.WriteString(" AND EXISTS (" + ex + ")")
			}
			if ts.Query.HasOutbound != nil {
				w, ex := buildPredicateSQL(b, fmt.Sprintf("v%d_out", i), *ts.Query.HasOutbound, typeArg,
					store.DirectionOutgoing, sel)
				withParts = append(withParts, w...)
				part.WriteString(" AND EXISTS (" + ex + ")")
			}
			// The ACL never fills Related today. It is rendered anyway,
			// because skipping a field of the gate's query would widen it.
			for j, rel := range ts.Query.Related {
				w, ex := buildPredicateSQL(b, fmt.Sprintf("v%d_%s", i, relatedPrefix(j)), rel.Pred, typeArg,
					relatedDirection(rel), sel)
				withParts = append(withParts, w...)
				part.WriteString(" AND " + existsCond(ex, rel.Pred.Negate))
			}
			if len(ts.Query.Any) > 0 {
				w, cond := buildAnySQL(b, fmt.Sprintf("v%d_any", i), ts.Query.Any, typeArg, sel)
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
