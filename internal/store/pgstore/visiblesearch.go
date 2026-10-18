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
// invariant. A [search.TypeScope.Fields] entry adds the field check (see
// [fieldCheck]): the statement reports which field grants hold on each row,
// and Go judges which fields matched.
//
// Because search and visibility execute as one statement, any query
// failure is wrapped in [search.ErrScope]: the statement IS the gate
// here, and consumers route ErrScope through their ACL-error path
// (cancel-silent / deadline-504 mapping included — ctx is threaded
// into the query, unlike the legacy ctx-less Backend.Search).
//
// Go-side residue: q.Filters and the field check run in Go, so the limit is
// enforced after them. The result is read in keyset pages (see
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
// rows that pass q.Filters and the field check, with the page's rows already
// closed, so no connection is held while the caller yields (see
// defaultIteratorPageSize). The rows carry no bodies. Nothing is yielded when
// the scope admits nothing.
//
// A page is at most q.Limit rows when no Go-side check can drop any, so a
// limited search usually costs one statement. Otherwise a page costs the
// search statement plus at most one body read for the field check.
func visiblePages(
	ctx context.Context, db DBTX, titles SearchTitles, q search.Query, scope map[string]search.TypeScope,
) iter.Seq2[[]*entity.Entity, error] {
	return func(yield func([]*entity.Entity, error) bool) {
		gated := q.Text != "" && fieldGated(scope)
		size := int(iteratorPageSize.Load())
		if q.Limit > 0 && q.Limit < size && len(q.Filters) == 0 && !gated {
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
			page, err := keepRows(ctx, db, q, scope, gated, rows)
			if err != nil {
				yield(nil, err)
				return
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

// keepRows returns the entities of rows that pass the field check (when
// gated) and q.Filters, in order.
func keepRows(
	ctx context.Context, db DBTX, q search.Query, scope map[string]search.TypeScope, gated bool, rows []visibleRow,
) ([]*entity.Entity, error) {
	kept := rows
	if gated {
		var err error
		if kept, err = fieldCheck(ctx, db, q.Text, scope, rows); err != nil {
			return nil, err
		}
	}
	page := make([]*entity.Entity, 0, len(kept))
	for _, r := range kept {
		if search.MatchFilters(r.e, q.Filters) {
			page = append(page, r.e)
		}
	}
	return page, nil
}

// visibleRow is one visible-search row and its keyset key. grants is the
// row's field-grant verdicts (see [fieldGrantColumn]): grants[i] reports
// whether grant i of its type's field scope holds on the row. It is nil for
// a type without a field scope.
type visibleRow struct {
	e      *entity.Entity
	key    visibleKey
	grants []bool
}

func scanVisibleRow(row scanner) (visibleRow, error) {
	var (
		id, typ, content, ptr string
		props                 []byte
		updatedAt             time.Time
		rank                  float32
		grants                []bool
	)
	if err := row.Scan(&id, &typ, &ptr, &props, &content, &updatedAt, &rank, &grants); err != nil {
		return visibleRow{}, err
	}
	e := entity.New(id, typ)
	e.Face = entity.Face(ptr)
	e.UpdatedAt = updatedAt
	var err error
	if e.Properties, err = unmarshalProps(props); err != nil {
		return visibleRow{}, err
	}
	return visibleRow{e: e, key: visibleKey{rank: rank, id: id}, grants: grants}, nil
}

// visibleSearchColumns is scanEntity's column list with the body projected
// away (TKT-U9DYW4). A hit is an id, a type and a title; the property filters
// read properties only. The field check reads a body only for a row that
// no visible property admits (see [fieldCheck]). Shipping every candidate's
// body was 280 ms of a 290 ms search at 1000 hits.
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
// is the key's first part, and its field-grant verdicts as an eighth; see
// visiblePages.
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
	grantsCol := "NULL::boolean[]"
	if q.Text != "" {
		var w []string
		w, grantsCol = fieldGrantColumn(b, scope)
		withParts = append(withParts, w...)
	}
	columns := visibleSearchColumns + ", " + rankCol + " AS search_rank, " + grantsCol + " AS field_grants"
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
	return withClause(withParts) + sb.String(), b.args, true, nil
}

// fieldGated reports whether some entry of scope gates properties.
func fieldGated(scope map[string]search.TypeScope) bool {
	for typ, ts := range scope {
		if typ != search.WildcardType && ts.Fields != nil {
			return true
		}
	}
	return false
}

// fieldGrantColumn renders the field-grant verdicts of a row as one
// boolean[] column: for a row of a type whose entry carries
// [search.TypeScope.Fields], element i+1 says whether grant i's When holds on
// the row (true for a nil When); for any other row, NULL. The When
// predicates render through the graph query builder with a per-grant CTE
// prefix, against every face of their endpoints, as MatchingFaces would
// evaluate them.
//
// The statement decides only which grants hold on which rows. Which fields
// the text matched is judged in Go ([fieldCheck]), because only Go folds
// case the way search_text was folded.
func fieldGrantColumn(b *sqlBuilder, scope map[string]search.TypeScope) (with []string, column string) {
	types := make([]string, 0, len(scope))
	for typ, ts := range scope {
		if typ != search.WildcardType && ts.Fields != nil {
			types = append(types, typ)
		}
	}
	if len(types) == 0 {
		return nil, "NULL::boolean[]"
	}
	slices.Sort(types)
	var cases strings.Builder
	cases.WriteString("CASE")
	for i, typ := range types {
		typeArg := b.arg(typ)
		elems := make([]string, 0, len(scope[typ].Fields.Grants))
		for j, g := range scope[typ].Fields.Grants {
			if g.When == nil {
				elems = append(elems, "TRUE")
				continue
			}
			w, conds := fieldGrantCond(b, fmt.Sprintf("f%d_%d_", i, j), *g.When, typeArg)
			with = append(with, w...)
			cond := "TRUE"
			if len(conds) > 0 {
				cond = strings.Join(conds, " AND ")
			}
			elems = append(elems, "COALESCE(("+cond+"), FALSE)")
		}
		cases.WriteString(" WHEN e.type = " + typeArg + " THEN ARRAY[" + strings.Join(elems, ", ") + "]::boolean[]")
	}
	cases.WriteString(" ELSE NULL END")
	return with, cases.String()
}

// fieldCheck keeps the rows whose text match the field scope admits, in
// order, so a hit whose only match is a property the reader may not see is
// never returned (the match-on-hidden-field oracle).
//
// The matched fields come from [search.MatchTextFields], the matcher the
// generic searcher uses. It folds case with Go's strings.ToLower, as the
// store does when it writes search_text, so the trigram pre-filter and this
// check agree. PostgreSQL's lower() folds some characters differently (for
// example 'İ'); judging in SQL with it could admit a row whose real match
// is in a hidden field.
//
// A row is first judged on its id and properties. Only a row they do not
// admit has its body read, in one batch per page, because the body is never
// property-gated and may still admit it.
func fieldCheck(
	ctx context.Context, db DBTX, text string, scope map[string]search.TypeScope, rows []visibleRow,
) ([]visibleRow, error) {
	verdict := make([]bool, len(rows))
	var pending []entity.Ref
	for i, r := range rows {
		fs := scope[r.e.Type].Fields
		if fs == nil {
			verdict[i] = true
			continue
		}
		if admits(fs, r, text) {
			verdict[i] = true
			continue
		}
		pending = append(pending, r.e.Ref())
	}
	if len(pending) > 0 {
		bodies, err := readBodies(ctx, db, pending)
		if err != nil {
			return nil, fmt.Errorf("%w: pgstore visible search: field check: %w", search.ErrScope, err)
		}
		for i, r := range rows {
			if verdict[i] {
				continue
			}
			body, ok := bodies[r.e.Ref()]
			if !ok || body == "" {
				continue // gone, or no body to match: nothing visible matched
			}
			withBody := *r.e
			withBody.Content = body
			verdict[i] = admits(scope[r.e.Type].Fields, visibleRow{e: &withBody, grants: r.grants}, text)
		}
	}
	out := rows[:0:0]
	for i, r := range rows {
		if verdict[i] {
			out = append(out, r)
		}
	}
	return out, nil
}

// admits reports whether fs keeps r for text. A grant the statement did not
// report as holding is treated as not holding, which can only hide more.
func admits(fs *search.FieldScope, r visibleRow, text string) bool {
	matched := search.MatchTextFields(r.e, text)
	return fs.AdmitsMatch(matched, func(i int) bool { return i < len(r.grants) && r.grants[i] })
}

// readBodies reads the bodies of the face rows refs in one statement.
func readBodies(ctx context.Context, db DBTX, refs []entity.Ref) (map[entity.Ref]string, error) {
	ids := make([]string, len(refs))
	faces := make([]string, len(refs))
	for i, r := range refs {
		ids[i], faces[i] = r.ID, r.Face.String()
	}
	type body struct {
		ref     entity.Ref
		content string
	}
	got, err := queryAll(ctx, db,
		"SELECT e.id, e.face, e.content FROM entities e"+
			" JOIN unnest($1::text[], $2::text[]) AS k(id, face) ON e.id = k.id AND e.face = k.face",
		[]any{ids, faces},
		func(row scanner) (body, error) {
			var id, face, content string
			if err := row.Scan(&id, &face, &content); err != nil {
				return body{}, err
			}
			return body{ref: entity.Ref{ID: id, Face: entity.Face(face)}, content: content}, nil
		})
	if err != nil {
		return nil, err
	}
	out := make(map[entity.Ref]string, len(got))
	for _, g := range got {
		out[g.ref] = g.content
	}
	return out, nil
}

// fieldGrantCond renders a field grant's When as conjuncts on e. Its
// relation predicates read every face of their endpoints, and its FaceIn
// restricts the row's own face.
func fieldGrantCond(b *sqlBuilder, prefix string, when store.GraphQuery, typeArg string) (with, conds []string) {
	when.Faces = store.AllFaces()
	with, pre, post := buildPredicatePartsPrefixed(b, prefix, when, typeArg)
	conds = slices.Concat(pre, post)
	if len(when.FaceIn) > 0 {
		faces := make([]string, len(when.FaceIn))
		for i, f := range when.FaceIn {
			faces[i] = f.String()
		}
		conds = append(conds, "e.face = ANY("+b.arg(faces)+")")
	}
	return with, conds
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
