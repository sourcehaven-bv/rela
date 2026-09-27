package sqlitestore

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/graphquerynaive"
)

// This file renders a store.GraphQuery as one SQLite statement. It mirrors
// pgstore's graphquery.go function by function, under the same names, so a
// change to one builder has an obvious counterpart in the other. The two
// dialects differ in the leaves only:
//
//   - Placeholders are numbered (?N) and a list binds as ONE JSON array read
//     back through json_each, where pgstore binds a text[] and unnests it.
//   - JSON paths are SQL LITERALS, never bound, so a query expression is
//     textually the expression a derived index was built on. SQLite matches an
//     index expression structurally, and a bound path is a different
//     expression. See [jsonPath] for why that is safe.
//   - A property's text form is [txtExpr], because `->>` yields SQLite's own
//     types (1 for true, a REAL for 1.5) where pgstore's `->>` yields text.
//   - A world picks its prime with ROW_NUMBER, as the entity listings do,
//     because SQLite has no DISTINCT ON.
//
// Every value is bound. The only caller data rendered into the text is a
// property name inside a JSON path and a declared enum value inside an ORDER
// BY rank, both through [quoteLiteral] after [safeLiteral] admitted them. A
// name or value it refuses marks the builder unsafe, and the caller answers
// the WHOLE query through graphquerynaive. Refusing one arm instead would be
// unsound: inside a NOT, a skipped arm inverts.

// jsonPath renders the SQLite JSON path selecting the top-level key prop, as a
// SQL string literal: `'$."prop"'`.
//
// The key is double-quoted inside the path so `.`, `[` and spaces are part of
// the key rather than path syntax. A key containing `"` or `\` cannot be
// quoted that way, and a control character has no business in a property
// name, so those mark the builder unsafe and the query falls back.
func (b *sqlBuilder) jsonPath(prop string) string {
	if prop == "" || strings.ContainsAny(prop, `"\`) || !safeLiteral(prop) {
		b.unsafe = true
		return "'$'"
	}
	return quoteLiteral(`$."` + prop + `"`)
}

// literal renders v as a SQL string literal, or marks the builder unsafe.
func (b *sqlBuilder) literal(v string) string {
	if !safeLiteral(v) {
		b.unsafe = true
		return "''"
	}
	return quoteLiteral(v)
}

// safeLiteral reports whether s may be rendered with [quoteLiteral]: no
// control characters, so no NUL can end the statement early.
func safeLiteral(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// quoteLiteral renders s as a SQL string literal. Doubling the quote is the
// whole escaping rule in SQLite; there are no backslash escapes.
func quoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// jsonArg binds vals as ONE JSON array and returns its placeholder.
func (b *sqlBuilder) jsonArg(vals []string) string {
	raw, err := json.Marshal(vals)
	if err != nil {
		// A []string always marshals; keep the query honest if it ever did not.
		b.unsafe = true
		return "'[]'"
	}
	return b.arg(string(raw))
}

// jsonList is the subquery listing vals' elements, for `x IN (...)`.
func (b *sqlBuilder) jsonList(vals []string) string {
	return "SELECT value FROM json_each(" + b.jsonArg(vals) + ")"
}

// propsCol is the properties column of alias, or the bare column for DDL.
func propsCol(alias string) string {
	if alias == "" {
		return "properties"
	}
	return alias + ".properties"
}

// rawExpr is `->>`: SQLite's value for the key, NULL when absent or JSON null.
func rawExpr(alias, path string) string {
	return "(" + propsCol(alias) + " ->> " + path + ")"
}

// typeExpr is the key's JSON type name, NULL when absent.
func typeExpr(alias, path string) string {
	return "json_type(" + propsCol(alias) + ", " + path + ")"
}

// txtExpr is a property's TEXT form, matching propmatch.Stringify and the
// graphquerynaive sort key: a string as is, an integer in decimal, a boolean
// as `true`/`false`, NULL when absent or JSON null. `->>` alone would give 1
// for true. A real renders as its JSON token, which is what the stored
// document holds.
//
// One spelling for queries and index DDL alike: an ORDER BY reaches a derived
// list index only when its key is structurally the indexed expression.
func txtExpr(alias, path string) string {
	col := propsCol(alias)
	return "(CASE json_type(" + col + ", " + path + ")" +
		" WHEN 'true' THEN 'true' WHEN 'false' THEN 'false'" +
		" WHEN 'real' THEN (" + col + " -> " + path + ")" +
		" ELSE CAST(" + col + " ->> " + path + " AS TEXT) END)"
}

// scalarEqualCond is a scalar string equality. It is the shape a derived
// query index serves: the index is partial on the same json_type guard and
// keyed on the same `->>` expression.
func scalarEqualCond(alias, path, valArg string) string {
	return "(" + typeExpr(alias, path) + " = 'text' AND " + rawExpr(alias, path) + " = " + valArg + ")"
}

// propCond renders one property predicate against the candidate row.
func propCond(b *sqlBuilder, p store.PropPredicate) string {
	return propCondOn(b, "e", p)
}

// propCondOn renders p against alias. It must agree with graphquerynaive's
// matchesProps on every value shape, as pgstore's propCondOn must; see there
// for the reasoning behind each arm. isEmpty covers a missing key, JSON null,
// the empty string and the empty array, and is never NULL itself, so a NOT
// around it behaves.
func propCondOn(b *sqlBuilder, alias string, p store.PropPredicate) string {
	path := b.jsonPath(p.Property)
	if p.Scalar && p.Op == store.PropEqual && p.Value != "" {
		return scalarEqualCond(alias, path, b.arg(p.Value))
	}
	raw := rawExpr(alias, path)
	isEmpty := "(" + raw + " IS NULL OR " + raw + " = '' OR (" + typeExpr(alias, path) +
		" = 'array' AND json_array_length(" + propsCol(alias) + ", " + path + ") = 0))"

	switch {
	case p.Value == "" && p.Op == store.PropEqual:
		return isEmpty
	case p.Value == "" && p.Op == store.PropNotEqual:
		return "NOT " + isEmpty
	case p.Op == store.PropNotEqualOrEmpty:
		if p.Value == "" {
			return "1"
		}
		return "(" + isEmpty + " OR NOT " + equalsCond(b, alias, path, p.Value) + ")"
	case p.Op == store.PropGreaterEqual, p.Op == store.PropLessEqual:
		return orderedCond(b, alias, path, p.Op, p.Value)
	case p.Op == store.PropNotEqual:
		return "(NOT " + isEmpty + " AND NOT " + equalsCond(b, alias, path, p.Value) + ")"
	default:
		return equalsCond(b, alias, path, p.Value)
	}
}

// orderedCond is a byte-wise range test on the text form. Lists never match,
// because Go and SQL render a list differently (see pgstore's orderedCond).
func orderedCond(b *sqlBuilder, alias, path string, op store.PropOp, value string) string {
	cmp := ">="
	if op == store.PropLessEqual {
		cmp = "<="
	}
	txt := txtExpr(alias, path)
	return "(" + typeExpr(alias, path) + " <> 'array' AND " + txt + " <> '' AND " +
		txt + " " + cmp + " " + b.arg(value) + ")"
}

// equalsCond is value equality: a scalar by its text form, a list when a
// STRING element equals the target, as pgstore's `?` operator decides it.
// The array arm is an EXISTS, which is never NULL; the scalar arm is NULL for
// an absent key, which every caller guards with isEmpty or a plain AND.
func equalsCond(b *sqlBuilder, alias, path, value string) string {
	valArg := b.arg(value)
	return "(CASE WHEN " + typeExpr(alias, path) + " = 'array' THEN EXISTS (SELECT 1 FROM json_each(" +
		propsCol(alias) + ", " + path + ") je WHERE je.type = 'text' AND je.value = " + valArg + ")" +
		" ELSE " + txtExpr(alias, path) + " = " + valArg + " END)"
}

// buildPredicateParts emits the CTE definitions and the conjuncts shared by
// every graph statement, in pgstore's two groups: pre trims the candidate
// faces before a world ranks them, post tests the prime (BUG-2SKLD3). seed
// selects the ids an entity-inheritance closure starts from.
func buildPredicateParts(b *sqlBuilder, q store.GraphQuery, seed string) (with, pre, post []string) {
	if q.HasInbound != nil {
		w, ex := buildPredicateSQL(b, "in", *q.HasInbound, seed, store.DirectionIncoming)
		with = append(with, w...)
		pre = append(pre, existsCond(ex, q.HasInbound.Negate))
	}
	if q.HasOutbound != nil {
		w, ex := buildPredicateSQL(b, "out", *q.HasOutbound, seed, store.DirectionOutgoing)
		with = append(with, w...)
		pre = append(pre, existsCond(ex, q.HasOutbound.Negate))
	}
	for i, rel := range q.Related {
		dir := store.DirectionOutgoing
		if rel.Incoming {
			dir = store.DirectionIncoming
		}
		w, ex := buildPredicateSQL(b, "rel"+strconv.Itoa(i), rel.Pred, seed, dir)
		with = append(with, w...)
		pre = append(pre, existsCond(ex, rel.Pred.Negate))
	}
	if len(q.Any) > 0 {
		w, cond := buildAnySQL(b, "any", q.Any, seed)
		with = append(with, w...)
		pre = append(pre, cond)
	}
	for _, p := range q.Props {
		post = append(post, propCond(b, p))
	}
	if len(q.Narrowing) > 0 {
		post = append(post, buildNarrowingSQL(b, q.Narrowing))
	}
	return with, pre, post
}

// graphSource renders what follows `SELECT <list> FROM ` in every graph
// statement: the in-scope rows satisfying q, one per id, aliased e. See
// pgstore's graphSource. ids, when set, is a subquery of the only ids asked
// about; it restricts the candidates and the closure seeds alike, so a page's
// statement costs the page, not the type.
func graphSource(b *sqlBuilder, q store.GraphQuery, typeArg, ids string) (with []string, source string) {
	if q.EntityType == "" {
		// graphquerynaive reads every type for an empty EntityType, which
		// `e.type = ''` would silently turn into no rows.
		b.unsafe = true
	}
	seed := "SELECT e0.id FROM entities e0 WHERE e0.type = " + typeArg
	if ids != "" {
		// Unary + as for the candidates below: the id list must drive.
		seed = "SELECT e0.id FROM entities e0 WHERE e0.id IN (" + ids + ") AND +e0.type = " + typeArg
	}
	with, pre, post := buildPredicateParts(b, q, seed)
	w := effectiveWorld(q.World, q.EntityType)
	var scope, rank string
	if w.IsDefaultWorld() {
		scope = "e.face = ''"
	} else {
		rank, scope = worldSQL(b, w, "e")
	}
	where := []string{scope}
	if len(q.FaceIn) > 0 {
		faces := make([]string, len(q.FaceIn))
		for i, f := range q.FaceIn {
			faces[i] = f.String()
		}
		// Before the rank; see [store.GraphQuery.FaceIn].
		where = append(where, "e.face IN ("+b.jsonList(faces)+")")
	}
	if ids == "" {
		where = append(where, "e.type = "+typeArg)
	} else {
		// Unary + keeps e.type out of index selection. Without statistics the
		// planner rates type=? no worse than the id list and walks the whole
		// type; the id list is one page, so the primary key must drive.
		where = append(where, "e.id IN ("+ids+")", "+e.type = "+typeArg)
	}
	where = append(where, pre...)
	if rank == "" {
		where = append(where, post...)
		return with, "entities e WHERE " + strings.Join(where, " AND ")
	}
	source = "(SELECT e.*, ROW_NUMBER() OVER (PARTITION BY e.id ORDER BY (" + rank + "), e.face) AS rn" +
		" FROM entities e WHERE " + strings.Join(where, " AND ") + ") e WHERE " +
		strings.Join(append([]string{"e.rn = 1"}, post...), " AND ")
	return with, source
}

// withClause renders the recursive CTE prefix, or nothing.
func withClause(with []string) string {
	if len(with) == 0 {
		return ""
	}
	return "WITH RECURSIVE " + strings.Join(with, ",\n") + "\n"
}

// buildNarrowingSQL renders [store.GraphQuery.Narrowing] as one conjunct, a
// disjunction of per-branch conjunctions, separate from Any's (see pgstore).
func buildNarrowingSQL(b *sqlBuilder, branches []store.NarrowBranch) string {
	parts := make([]string, 0, len(branches))
	for _, br := range branches {
		if len(br.Props) == 0 {
			parts = append(parts, "1")
			continue
		}
		conj := make([]string, 0, len(br.Props))
		for _, p := range br.Props {
			conj = append(conj, propCond(b, p))
		}
		parts = append(parts, "("+strings.Join(conj, " AND ")+")")
	}
	return "(" + strings.Join(parts, " OR ") + ")"
}

// buildAnySQL renders [store.GraphQuery.Any] as one conjunct: a disjunction
// of per-branch `(EXISTS(relation) AND face IN faces)` arms, which trims the
// candidates before a world ranks them.
func buildAnySQL(
	b *sqlBuilder, prefix string, branches []store.GraphBranch, seed string,
) (with []string, cond string) {
	parts := make([]string, 0, len(branches))
	for i, br := range branches {
		var conj []string
		if br.HasInbound != nil {
			w, ex := buildPredicateSQL(b, fmt.Sprintf("%s%d_in", prefix, i), *br.HasInbound,
				seed, store.DirectionIncoming)
			with = append(with, w...)
			conj = append(conj, existsCond(ex, br.HasInbound.Negate))
		}
		if len(br.FaceIn) > 0 {
			vals := make([]string, len(br.FaceIn))
			for j, f := range br.FaceIn {
				vals[j] = f.String()
			}
			conj = append(conj, "e.face IN ("+b.jsonList(vals)+")")
		}
		if len(conj) == 0 {
			conj = []string{"1"}
		}
		parts = append(parts, "("+strings.Join(conj, " AND ")+")")
	}
	return with, "(" + strings.Join(parts, " OR ") + ")"
}

// existsCond wraps an EXISTS body, negating it for an absence query.
func existsCond(body string, negate bool) string {
	if negate {
		return "NOT EXISTS (" + body + ")"
	}
	return "EXISTS (" + body + ")"
}

// graphSelect chooses what a graph statement projects.
type graphSelect int

const (
	graphSelectRows graphSelect = iota
	graphSelectHeaders
	graphSelectCount
)

// graphSelectLists are the projections; scanEntity and scanEntityHeader expect
// exactly these columns in this order.
var graphSelectLists = map[graphSelect]string{
	graphSelectRows:    "e.id, e.type, e.face, e.properties, e.content, e.updated_at",
	graphSelectHeaders: "e.id, e.type, e.face, e.properties, e.updated_at",
}

// buildGraphQuerySQL renders q projected by sel. ok is false when a name could
// not be rendered safely and the query must go to graphquerynaive instead.
func buildGraphQuerySQL(q store.GraphQuery, sel graphSelect) (sqlText string, args []any, ok bool) {
	b := &sqlBuilder{}
	typeArg := b.arg(q.EntityType)
	with, source := graphSource(b, q, typeArg, "")
	if sel == graphSelectCount {
		return withClause(with) + "SELECT count(*) FROM " + source, b.args, !b.unsafe
	}

	var sb strings.Builder
	sb.WriteString(withClause(with))
	sb.WriteString("SELECT " + graphSelectLists[sel] + " FROM " + source)
	sb.WriteString(" ORDER BY ")
	for _, spec := range q.OrderBy {
		sb.WriteString(orderKeySQL(b, "e", spec) + ", ")
	}
	sb.WriteString("e.id ASC")
	switch {
	case q.Limit > 0:
		sb.WriteString(" LIMIT " + b.arg(q.Limit))
	case q.Offset > 0:
		// SQLite has no OFFSET without LIMIT; a negative limit means none.
		sb.WriteString(" LIMIT -1")
	}
	if q.Offset > 0 {
		sb.WriteString(" OFFSET " + b.arg(q.Offset))
	}
	return sb.String(), b.args, !b.unsafe
}

// orderKeySQL renders one sort key as `(key IS NULL), [rank,] key`, all in
// the spec's direction. SQLite sorts NULL first ascending; the leading test
// puts an absent value last ascending and first descending, as pgstore and
// graphquerynaive do. The default BINARY collation is Go's byte order.
//
// alias is "e" in a query and "" in index DDL, and nothing else differs, so
// the two stay structurally equal.
func orderKeySQL(b *sqlBuilder, alias string, spec store.OrderSpec) string {
	dir := " ASC"
	if spec.Descending {
		dir = " DESC"
	}
	key := txtExpr(alias, b.jsonPath(spec.Property))
	out := "(" + key + " IS NULL)" + dir
	if rank := orderRankSQL(b, key, spec.Values); rank != "" {
		out += ", " + rank + dir
	}
	return out + ", " + key + dir
}

// orderRankSQL ranks key by its position in values, every undeclared value
// sharing one rank past the end. Values are literals for the reason pgstore's
// orderKeySQL gives: a bound CASE arm is a different expression from the
// indexed one.
func orderRankSQL(b *sqlBuilder, key string, values []string) string {
	if len(values) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("(CASE " + key)
	for i, v := range values {
		sb.WriteString(" WHEN " + b.literal(v) + " THEN " + strconv.Itoa(i))
	}
	sb.WriteString(" ELSE " + strconv.Itoa(len(values)) + " END)")
	return sb.String()
}

// buildGraphTotalSQL counts the in-scope entities of the type: GraphCount's
// denominator, world-scoped for the reason pgstore's is (RR-EHER1V).
func buildGraphTotalSQL(q store.GraphQuery) (sqlText string, args []any) {
	b := &sqlBuilder{}
	typeArg := b.arg(q.EntityType)
	w := effectiveWorld(q.World, q.EntityType)
	scope, agg := "e.face = ''", "count(*)"
	if !w.IsDefaultWorld() {
		_, scope = worldSQL(b, w, "e")
		agg = "count(DISTINCT e.id)"
	}
	if len(q.FaceIn) > 0 {
		faces := make([]string, len(q.FaceIn))
		for i, f := range q.FaceIn {
			faces[i] = f.String()
		}
		scope += " AND e.face IN (" + b.jsonList(faces) + ")"
	}
	return "SELECT " + agg + " FROM entities e WHERE e.type = " + typeArg + " AND " + scope, b.args
}

// buildMatchingIDsSQL is the graph statement restricted to ids, selecting
// only the id.
func buildMatchingIDsSQL(q store.GraphQuery, ids []string) (sqlText string, args []any, ok bool) {
	b := &sqlBuilder{}
	typeArg := b.arg(q.EntityType)
	with, source := graphSource(b, q, typeArg, b.jsonList(ids))
	return withClause(with) + "SELECT e.id FROM " + source, b.args, !b.unsafe
}

// buildPredicateSQL emits (CTE definitions, EXISTS body) for one relation
// predicate, as pgstore's does. The closures are bounded by
// graphquerynaive.DepthCap through [cappedDepth].
//
// The entity closure seeds from every id seed selects, whatever its face:
// graphquerynaive starts from the candidate's id, and a relation walk does not
// depend on the reader's world.
func buildPredicateSQL(
	b *sqlBuilder, prefix string,
	p store.RelationPredicate, seed string, dir store.Direction,
) (with []string, exists string) {
	var endpointSrc string
	if len(p.Endpoints) > 0 {
		endpointsArg := b.jsonArg(p.Endpoints)
		endpointSrc = "SELECT value FROM json_each(" + endpointsArg + ")"
		if len(p.InheritThrough) > 0 && p.Depth > 0 {
			cteName := prefix + "_endpoint_closure"
			with = append(with, cteName+`(id, depth) AS (
    SELECT value, 0 FROM json_each(`+endpointsArg+`)
    UNION
    SELECT r.to_id, c.depth + 1
    FROM relations r
    JOIN `+cteName+` c ON r.from_id = c.id
    WHERE r.rel_type IN (`+b.jsonList(p.InheritThrough)+`)
      AND c.depth < `+b.arg(cappedDepth(p.Depth))+`
)`)
			endpointSrc = "SELECT id FROM " + cteName
		}
	}

	entityJoin := "e.id"
	if len(p.EntityInheritThrough) > 0 && p.EntityDepth > 0 {
		cteName := prefix + "_entity_closure"
		with = append(with, cteName+`(id, root, depth) AS (
    SELECT DISTINCT s.id, s.id, 0 FROM (`+seed+`) s
    UNION
    SELECT r.to_id, c.root, c.depth + 1
    FROM relations r
    JOIN `+cteName+` c ON r.from_id = c.id
    WHERE r.rel_type IN (`+b.jsonList(p.EntityInheritThrough)+`)
      AND c.depth < `+b.arg(cappedDepth(p.EntityDepth))+`
)`)
		entityJoin = "SELECT id FROM " + cteName + " WHERE root = e.id"
	}

	endpointCol, entityCol := "r.from_id", "r.to_id"
	if dir == store.DirectionOutgoing {
		endpointCol, entityCol = "r.to_id", "r.from_id"
	}

	alias := prefix + "_ep"
	var sb strings.Builder
	sb.WriteString("SELECT 1 FROM relations r ")
	if p.EndpointMatch != nil {
		sb.WriteString("JOIN entities " + alias + " ON " + alias + ".id = " + endpointCol + " ")
	}
	sb.WriteString("WHERE ")
	if p.EndpointMatch != nil {
		sb.WriteString(defaultStateCond(alias))
	}
	if len(p.OfTypes) > 0 {
		sb.WriteString("r.rel_type IN (" + b.jsonList(p.OfTypes) + ") AND ")
	}
	// An empty Endpoints list means "any endpoint" (see pgstore).
	if endpointSrc != "" {
		sb.WriteString(endpointCol + " IN (" + endpointSrc + ") AND ")
	}
	if entityJoin == "e.id" {
		sb.WriteString(entityCol + " = e.id")
	} else {
		sb.WriteString(entityCol + " IN (" + entityJoin + ")")
	}
	with = append(with, endpointMatchSQL(b, &sb, alias, p.EndpointMatch)...)
	return with, sb.String()
}

// endpointMatchSQL appends an EndpointMatch's type, property and chained-hop
// conditions on the joined endpoint alias.
func endpointMatchSQL(b *sqlBuilder, sb *strings.Builder, alias string, m *store.EndpointPredicate) []string {
	if m == nil {
		return nil
	}
	var with []string
	if m.EntityType != "" {
		sb.WriteString(" AND " + alias + ".type = " + b.arg(m.EntityType))
	}
	for _, pp := range m.Props {
		sb.WriteString(" AND " + propCondOn(b, alias, pp))
	}
	for _, chain := range []struct {
		pred *store.RelationPredicate
		dir  store.Direction
	}{
		{m.HasInbound, store.DirectionIncoming},
		{m.HasOutbound, store.DirectionOutgoing},
	} {
		if chain.pred == nil {
			continue
		}
		w, ex := nestedPredicateSQL(b, alias, *chain.pred, chain.dir)
		with = append(with, w...)
		sb.WriteString(" AND " + existsCond(ex, chain.pred.Negate))
	}
	return with
}

// defaultStateCond pins an endpoint-match hop to the default state: the
// endpoint's default face and a default-tailed edge.
func defaultStateCond(endpointAlias string) string {
	return "r.from_face = '' AND " + endpointAlias + ".face = '' AND "
}

// nestedPredicateSQL is [buildPredicateSQL] for a hop whose candidate row is
// the endpoint alias candidate rather than e. It supports no inheritance
// expansion; CheckEndpointShape refuses one up front, as in pgstore.
func nestedPredicateSQL(
	b *sqlBuilder, candidate string, p store.RelationPredicate, dir store.Direction,
) (with []string, exists string) {
	endpointCol, entityCol := "r.from_id", "r.to_id"
	if dir == store.DirectionOutgoing {
		endpointCol, entityCol = "r.to_id", "r.from_id"
	}
	alias := candidate + "_ep"
	var sb strings.Builder
	sb.WriteString("SELECT 1 FROM relations r ")
	if p.EndpointMatch != nil {
		sb.WriteString("JOIN entities " + alias + " ON " + alias + ".id = " + endpointCol + " ")
	}
	sb.WriteString("WHERE ")
	if p.EndpointMatch != nil {
		sb.WriteString(defaultStateCond(alias))
	}
	if len(p.OfTypes) > 0 {
		sb.WriteString("r.rel_type IN (" + b.jsonList(p.OfTypes) + ") AND ")
	}
	if len(p.Endpoints) > 0 {
		sb.WriteString(endpointCol + " IN (" + b.jsonList(p.Endpoints) + ") AND ")
	}
	sb.WriteString(entityCol + " = " + candidate + ".id")
	with = endpointMatchSQL(b, &sb, alias, p.EndpointMatch)
	return with, sb.String()
}

// cappedDepth bounds depth at graphquerynaive's cap, treating a negative
// depth as no expansion.
func cappedDepth(d int) int {
	if d < 0 {
		return 0
	}
	if d > graphquerynaive.DepthCap {
		return graphquerynaive.DepthCap
	}
	return d
}
