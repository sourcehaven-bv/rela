package sqlitestore

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/graphquerynaive"
)

// The siblings roll-up has pgstore's shape. A climb is the along walk of
// [store.SiblingClimb], run level by level; its arrivals and the sources
// are the nodes, passed to one statement as JSON. Each edge's order is its
// stored number as a REAL when that is finite, else NULL, which sorts
// last; ties fall to the source id under BINARY, which is byte order.
// SQLite parses a JSON number as Go's strconv.ParseFloat does, so this is
// the order [store.SiblingWalk] names. ne and se read at most hop cap + 1
// edges per source. A query whose names the builder cannot render is
// answered by [store.RollupGeneric].

// siblingNode is one node of a siblings walk: a source or an ancestor it
// climbed to.
type siblingNode struct {
	Src string `json:"s"`
	ID  string `json:"i"`
}

// rollupSiblings is [Store.Rollup] for a siblings query.
func (s *Store) rollupSiblings(ctx context.Context, q store.RollupQuery) (map[string]store.RollupResult, error) {
	w := q.Siblings
	for _, nodes := range []map[string]store.AlongNode{w.Parents, w.Siblings} {
		for _, node := range nodes {
			if err := graphquerynaive.CheckEndpointShape(node.Gate); err != nil {
				return nil, err
			}
		}
	}
	sqlText, args, ok := buildSiblingsSQL(q)
	climb, stmts, climbOK := siblingClimbSQL(q)
	if !ok || !climbOK {
		return store.RollupGeneric(ctx, s, q)
	}
	nodes, climbed, err := s.siblingNodes(ctx, q, climb, stmts)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(nodes)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: rollup: siblings nodes: %w", err)
	}
	args[0] = string(raw)
	type row struct {
		src     string
		n, read int
		parents string
		over    bool
	}
	rows, err := collectRows(ctx, s.q(), sqlText, args, func(sc scanner) (row, error) {
		var r row
		scanErr := sc.Scan(&r.src, &r.n, &r.read, &r.parents, &r.over)
		return r, scanErr
	})
	if err != nil {
		return nil, err
	}
	limit := w.RowCap
	if limit == 0 {
		limit = store.AlongRowCap
	}
	out := make(map[string]store.RollupResult, len(rows)+len(climbed.over))
	for src := range climbed.over {
		out[src] = store.RollupResult{OverCap: true}
	}
	for _, r := range rows {
		if r.over || climbed.held[r.src]+r.read > limit {
			out[r.src] = store.RollupResult{OverCap: true}
			continue
		}
		var parents []string
		if err := json.Unmarshal([]byte(r.parents), &parents); err != nil {
			return nil, fmt.Errorf("sqlitestore: rollup: siblings parents: %w", err)
		}
		slices.Sort(parents)
		out[r.src] = store.RollupResult{Count: r.n, Parents: parents}
	}
	return out, nil
}

// siblingClimbSQL is the along query of q's climb and its level
// statements, none without a climb. It reports !ok when a name cannot be
// rendered.
func siblingClimbSQL(q store.RollupQuery) (store.RollupQuery, []alongLevel, bool) {
	climb := store.RollupQuery{Sources: q.Sources, Func: store.RollupCount, Along: store.SiblingClimb(q.Siblings)}
	if q.Siblings.Climb == 0 {
		return climb, nil, true
	}
	levels := store.AlongLevels(climb.Along)
	stmts := make([]alongLevel, len(levels))
	for i, hops := range levels {
		lvl, ok := buildAlongLevelSQL(climb, hops)
		if !ok {
			return climb, nil, false
		}
		stmts[i] = lvl
	}
	return climb, stmts, true
}

// siblingNodes is q's sources within the climb's cap, each its own node,
// and every gated ancestor the climb's stmts reached, with what the climb
// held per source.
func (s *Store) siblingNodes(
	ctx context.Context, q, climb store.RollupQuery, stmts []alongLevel,
) ([]siblingNode, alongWalked, error) {
	w := alongWalked{held: map[string]int{}, over: map[string]bool{}}
	if len(stmts) > 0 {
		var err error
		if w, err = s.walkAlong(ctx, climb, stmts); err != nil {
			return nil, alongWalked{}, err
		}
	}
	srcs := slices.Compact(slices.Sorted(slices.Values(q.Sources)))
	nodes := make([]siblingNode, 0, len(srcs)+len(w.visits))
	for _, src := range srcs {
		if !w.over[src] {
			nodes = append(nodes, siblingNode{Src: src, ID: src})
		}
	}
	for _, v := range w.visits {
		nodes = append(nodes, siblingNode{Src: v.Src, ID: v.ID})
	}
	slices.SortFunc(nodes, func(a, b siblingNode) int { return strings.Compare(a.Src+"\x00"+a.ID, b.Src+"\x00"+b.ID) })
	return slices.Compact(nodes), w, nil
}

// buildSiblingsSQL renders q's siblings walk as one statement returning,
// per source, its count, its gated rows (its node edges into a qualifying
// parent and the qualifying siblings' edges into those parents), its
// qualifying parents as a JSON array, and whether it read more than the
// hop cap of edges. Its first argument is the nodes, set
// per run. It reports !ok when a name cannot be rendered.
func buildSiblingsSQL(q store.RollupQuery) (sqlText string, args []any, ok bool) {
	w := q.Siblings
	b := &sqlBuilder{}
	nodesArg := b.arg("")
	relArg := b.arg(w.Relation)
	ord := siblingOrderSQL(b.jsonPath(store.SiblingOrderProperty))
	hopCap := w.HopCap
	if hopCap == 0 {
		hopCap = store.AlongHopCap
	}
	hopArg, readArg := b.arg(hopCap), b.arg(hopCap+1)
	with := []string{
		"sn AS (SELECT json_extract(value, '$.s') AS src, json_extract(value, '$.i') AS id FROM json_each(" +
			nodesArg + "))",
		"ne AS MATERIALIZED (" + perSourceEdgesSQL("SELECT DISTINCT src FROM sn",
			"SELECT s.id FROM sn s WHERE s.src = p.src",
			"r.from_id AS id, r.to_id AS parent, "+ord+" AS ord", "from_id", relArg, readArg) + ")",
		"neo AS (SELECT src FROM ne GROUP BY src HAVING count(*) > " + hopArg + ")",
	}
	with = append(with, siblingGatesSQL(b, "sp", w.Parents,
		"SELECT DISTINCT parent FROM ne")...)
	with = append(with,
		"qp AS ("+siblingAdmittedSQL("sp", w.Parents)+")",
		"sj AS MATERIALIZED (SELECT ne.src, ne.id, ne.parent, ne.ord FROM ne WHERE ne.parent IN (SELECT id FROM qp) "+
			"AND ne.src NOT IN (SELECT src FROM neo))",
		"se AS MATERIALIZED ("+perSourceEdgesSQL("SELECT DISTINCT src FROM sj",
			"SELECT j.parent FROM sj j WHERE j.src = p.src",
			"r.to_id AS parent, r.from_id AS peer, "+ord+" AS ord", "to_id", relArg, readArg)+")",
		"seo AS (SELECT src FROM se GROUP BY src HAVING count(*) > "+hopArg+")",
		"so AS (SELECT src FROM neo UNION SELECT src FROM seo)")
	with = append(with, siblingGatesSQL(b, "ss", w.Siblings,
		"SELECT DISTINCT peer FROM se")...)
	with = append(with,
		"sm AS (SELECT se.* FROM se WHERE se.peer IN ("+siblingAdmittedSQL("ss", w.Siblings)+"))")
	before := "(m.ord IS NULL, COALESCE(m.ord, 0), m.peer) < (j.ord IS NULL, COALESCE(j.ord, 0), j.id)"
	if w.CountSiblings {
		with = append(with, "sb AS (SELECT DISTINCT j.src, j.id, m.peer FROM sj j JOIN sm m ON m.src = j.src "+
			"AND m.parent = j.parent WHERE "+before+")")
	} else {
		with = append(with,
			"sf AS (SELECT parent, peer, ord FROM (SELECT parent, peer, ord, row_number() OVER "+
				"(PARTITION BY parent ORDER BY ord IS NULL, ord, peer) AS rn FROM sm) WHERE rn = 1)",
			"sb AS (SELECT DISTINCT j.src, j.id FROM sj j JOIN sf m ON m.parent = j.parent WHERE "+before+")")
	}
	read := "count(*) + (SELECT count(*) FROM sm m WHERE m.src = g.src)"
	sqlText = withClause(with) + "SELECT g.src, (SELECT count(*) FROM sb WHERE sb.src = g.src), " + read +
		", json_group_array(DISTINCT g.parent), 0 FROM sj g WHERE g.src NOT IN (SELECT src FROM so) GROUP BY g.src " +
		"UNION ALL SELECT src, 0, 0, '[]', 1 FROM so"
	return sqlText, b.args, !b.unsafe
}

// siblingOrderSQL is an edge's order over r: the stored number as a REAL
// when it is finite, NULL otherwise. The cast makes an integer the double
// Go converts it to.
func siblingOrderSQL(path string) string {
	v := "CAST(json_extract(r.properties, " + path + ") AS REAL)"
	return "CASE WHEN json_type(r.properties, " + path + ") IN ('integer', 'real') AND abs(" + v +
		") <= 1.7976931348623157e308 THEN " + v + " END"
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
	arms = append(arms, "SELECT NULL AS id WHERE 0")
	for ti, typ := range slices.Sorted(maps.Keys(nodes)) {
		arm := fmt.Sprintf("SELECT id FROM %s_t%d", prefix, ti)
		if nodes[typ].Counts != nil {
			arm += " WHERE hit"
		}
		arms = append(arms, arm)
	}
	return strings.Join(arms, " UNION ")
}
