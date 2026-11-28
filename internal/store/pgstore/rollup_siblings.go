package pgstore

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The siblings roll-up (design gap 2, section 4.4) is one statement per
// query. With a climb, the along CTEs of [store.SiblingClimb] give the
// ancestors. ne holds the nodes' identity-scoped edges, one gate per parent
// type seeded by their targets judges the parents, se holds the edges into
// the qualifying parents, and one gate per sibling type seeded by their
// sources judges the siblings. Each edge's order is the guarded float8 of
// [store.SiblingOrderProperty]; NULL sorts last and ties fall to the source
// id in byte order, which is the order [store.SiblingWalk] names. The reference is
// [store.RollupGeneric].

// orderUnderflow is half the smallest float64 subnormal: a number at or
// below it in magnitude rounds to zero, and PostgreSQL refuses to cast it.
const orderUnderflow = "2.4703282292062327208828439643411068618252990130716238221279284125e-324"

// orderOverflow is the magnitude from which a number rounds to an infinity
// in float64 (2^1024 - 2^970), and PostgreSQL refuses to cast it.
const orderOverflow = "179769313486231580793728971405303415079934132710037826936173778980444968292764750946649017977587" +
	"207096330286416692887910946555547851940402630657488671505820681908902000708383676273854845817711531764475730270069" +
	"855571366959622842914819860834936475292719074168444365510704342711559699508093042880177904174497792"

// rollupSiblings is [Store.Rollup] for a siblings query.
func (s *Store) rollupSiblings(ctx context.Context, q store.RollupQuery) (map[string]store.RollupResult, error) {
	for _, nodes := range []map[string]store.AlongNode{q.Siblings.Parents, q.Siblings.Siblings} {
		for _, node := range nodes {
			if err := checkGraphQueryScope(node.Gate); err != nil {
				return nil, err
			}
		}
	}
	sqlText, args, err := buildSiblingsSQL(q)
	if err != nil {
		return nil, err
	}
	out := map[string]store.RollupResult{}
	err = s.queryBounded(ctx, sqlText, args, func(rows pgx.Rows) error {
		var (
			overCap bool
			src     *string
			n       *int
			parents []string
		)
		if scanErr := rows.Scan(&overCap, &src, &n, &parents); scanErr != nil {
			return fmt.Errorf("pgstore: rollup scan: %w", scanErr)
		}
		if overCap {
			return store.ErrAlongRowCap
		}
		slices.Sort(parents)
		out[deref(src)] = store.RollupResult{Count: *n, Parents: parents}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// buildSiblingsSQL renders q's siblings walk as one statement returning
// the sentinel, then per source its count and its qualifying parents.
func buildSiblingsSQL(q store.RollupQuery) (sqlText string, args []any, err error) {
	b := &sqlBuilder{}
	with, overCap := siblingsWalkSQL(b, q, sourcesSeed(b, q.Sources))
	count := "(SELECT count(DISTINCT j.id) FROM sj j WHERE j.src = g.src AND EXISTS " +
		"(SELECT 1 FROM sf m WHERE m.parent = j.parent AND " + siblingBefore + "))::int"
	if q.Siblings.CountSiblings {
		count = "(SELECT count(DISTINCT (j.id, m.peer)) FROM sj j JOIN sm m ON m.parent = j.parent " +
			"WHERE j.src = g.src AND " + siblingBefore + ")::int"
	}
	sentinel := "SELECT TRUE AS over_cap, NULL::text AS src, NULL::int AS n, NULL::text[] AS parents WHERE " + overCap
	result := "SELECT FALSE, g.src, " + count + ", array_agg(DISTINCT g.parent) FROM sj g GROUP BY g.src"
	sqlText = withClause(with) + "SELECT * FROM (" + sentinel + " UNION ALL " + result + ") o ORDER BY o.over_cap DESC"
	if b.err != nil {
		return "", nil, b.err
	}
	return sqlText, b.args, nil
}

// siblingBefore holds when the sibling edge m sorts before the node edge j.
const siblingBefore = `(m.ord IS NULL, COALESCE(m.ord, 0), m.peer COLLATE "C") < ` +
	`(j.ord IS NULL, COALESCE(j.ord, 0), j.id COLLATE "C")`

// siblingsWalkSQL renders the CTEs of q's siblings walk from the ids seed
// selects: sj, each source's node edges into a qualifying parent; sm, the
// qualifying siblings' edges; sf, each parent's first of them. overCap is
// the condition under which the walk is over its cap.
func siblingsWalkSQL(b *sqlBuilder, q store.RollupQuery, seed string) (with []string, overCap string) {
	w := q.Siblings
	limit := w.RowCap
	if limit == 0 {
		limit = store.AlongRowCap
	}
	limitArg := b.arg(limit + 1)
	relArg := b.arg(w.Relation)
	nodes := `SELECT DISTINCT s COLLATE "C" AS src, s COLLATE "C" AS id FROM (` + seed + `) z(s)`
	read := "0"
	if w.Climb > 0 {
		climb := store.RollupQuery{Func: store.RollupCount, Along: store.SiblingClimb(w)}
		with = alongWalkSQL(b, climb, store.AlongLevels(climb.Along), seed, limitArg)
		nodes += " UNION SELECT src, id FROM walk"
		read = "(SELECT count(*) FROM walk)"
	}
	ord := siblingOrderSQL(b.arg(store.SiblingOrderProperty))
	with = append(with,
		"sn AS ("+nodes+")",
		fmt.Sprintf("ne AS (SELECT r.from_id AS id, r.to_id AS parent, %s AS ord FROM relations r "+
			"WHERE r.from_id IN (SELECT id FROM sn) AND r.rel_type = %s AND r.from_face = '' LIMIT %s)",
			ord, relArg, limitArg))
	with = append(with, siblingGatesSQL(b, "sp", w.Parents, "SELECT DISTINCT parent FROM ne")...)
	with = append(with,
		"qp AS ("+siblingAdmittedSQL("sp", w.Parents)+")",
		fmt.Sprintf("se AS (SELECT r.to_id AS parent, r.from_id AS peer, %s AS ord FROM relations r "+
			"WHERE r.to_id IN (SELECT id FROM qp) AND r.rel_type = %s AND r.from_face = '' LIMIT %s)",
			ord, relArg, limitArg))
	with = append(with, siblingGatesSQL(b, "ss", w.Siblings, "SELECT DISTINCT peer FROM se")...)
	with = append(with,
		"sm AS (SELECT se.* FROM se WHERE se.peer IN ("+siblingAdmittedSQL("ss", w.Siblings)+"))",
		"sj AS (SELECT n.src, ne.id, ne.parent, ne.ord FROM sn n JOIN ne ON ne.id = n.id "+
			"WHERE ne.parent IN (SELECT id FROM qp))",
		`sf AS (SELECT DISTINCT ON (parent) parent, peer, ord FROM sm `+
			`ORDER BY parent, ord IS NULL, ord, peer COLLATE "C")`)
	return with, read + " + (SELECT count(*) FROM ne) + (SELECT count(*) FROM se) > " + b.arg(limit)
}

// siblingOrderSQL is an edge's order over r: the stored number as float8
// when float64 holds it, zero when it rounds to zero, and NULL otherwise.
func siblingOrderSQL(propArg string) string {
	t := "(r.properties ->> " + propArg + ")::numeric"
	return "CASE WHEN jsonb_typeof(r.properties -> " + propArg + ") = 'number' THEN CASE " +
		"WHEN abs(" + t + ") <= " + orderUnderflow + " THEN 0::float8 " +
		"WHEN abs(" + t + ") < " + orderOverflow + " THEN " + t + "::float8 END END"
}

// siblingGatesSQL renders one gate per type of nodes, named prefix_tN and
// seeded by ids.
func siblingGatesSQL(b *sqlBuilder, prefix string, nodes map[string]store.AlongNode, ids string) []string {
	with := make([]string, 0, len(nodes))
	count := store.RollupQuery{Func: store.RollupCount}
	for ti, typ := range slices.Sorted(maps.Keys(nodes)) {
		with = append(with, alongGateSQL(b, fmt.Sprintf("%s_t%d", prefix, ti), nodes[typ], count, ids)...)
	}
	return with
}

// siblingAdmittedSQL selects the ids the gates siblingGatesSQL named
// admit and whose Counts holds; a nil Counts holds on every gated row.
func siblingAdmittedSQL(prefix string, nodes map[string]store.AlongNode) string {
	arms := make([]string, 0, len(nodes)+1)
	arms = append(arms, `SELECT NULL::text COLLATE "C" AS id WHERE FALSE`)
	for ti, typ := range slices.Sorted(maps.Keys(nodes)) {
		arm := fmt.Sprintf("SELECT id FROM %s_t%d", prefix, ti)
		if nodes[typ].Counts != nil {
			arm += " WHERE hit"
		}
		arms = append(arms, arm)
	}
	return strings.Join(arms, " UNION ")
}
