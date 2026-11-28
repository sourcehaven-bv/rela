package pgstore

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The along walk (design gap 1, section 3.4) is one statement per query:
// [store.AlongLevels] unrolled into CTEs, because a recursive CTE cannot
// carry a gate rendered as a subquery. Level K is, per hop, the distinct
// (src, id) pairs the identity-scoped edges reach from level K-1, joined to
// one gate per type the hop's segment reaches. Each gate is the reader's
// graph source seeded by the hop's ids, so it never scans its type, and it
// judges the node's Counts on the prime row it selects. Level K keeps the
// (src, id, state) triples no earlier level holds (a cumulative CTE), at
// most cap + 1 of them; a sentinel row reports a walk over the cap. The
// reference is [store.RollupGeneric].

// rollupAlong is [Store.Rollup] for an along query.
func (s *Store) rollupAlong(ctx context.Context, q store.RollupQuery) (map[string]store.RollupResult, error) {
	for _, seg := range q.Along.Segments {
		for _, node := range seg.Through {
			if err := checkGraphQueryScope(node.Gate); err != nil {
				return nil, err
			}
		}
	}
	out := map[string]store.RollupResult{}
	levels := store.AlongLevels(q.Along)
	if len(q.Sources) == 0 || len(levels) == 0 {
		return out, nil
	}
	sqlText, args, err := buildAlongSQL(q, levels)
	if err != nil {
		return nil, err
	}
	visits := map[string][]store.EntityHeader{}
	err = s.queryBounded(ctx, sqlText, args, func(rows pgx.Rows) error {
		var r alongRow
		if serr := rows.Scan(&r.overCap, &r.src, &r.id, &r.typ, &r.face, &r.count, &r.value); serr != nil {
			return fmt.Errorf("pgstore: rollup scan: %w", serr)
		}
		return r.collect(q, out, visits)
	})
	if err != nil {
		return nil, err
	}
	for src, rows := range visits {
		out[src] = store.AlongVisitsResult(q, rows)
	}
	return out, nil
}

// alongRow is one row of the along statement: the sentinel, a folded
// source, or a counted visit when the query asks for visits.
type alongRow struct {
	overCap      bool
	src, id, typ *string
	face, value  *string
	count        *int
}

// collect adds r to out, or to visits for a visits query. The sentinel
// sorts first, so a walk over its cap is refused before any row is kept.
func (r alongRow) collect(
	q store.RollupQuery, out map[string]store.RollupResult, visits map[string][]store.EntityHeader,
) error {
	if r.overCap {
		return store.ErrAlongRowCap
	}
	if r.src == nil {
		return errors.New("pgstore: rollup: along row without a source")
	}
	if q.Along.Visits {
		h := store.EntityHeader{ID: deref(r.id), Type: deref(r.typ), Face: entity.Face(deref(r.face))}
		if r.value != nil {
			v, err := rollupValue(q, *r.value)
			if err != nil {
				return err
			}
			h.Properties = map[string]any{q.Value: v}
		}
		visits[*r.src] = append(visits[*r.src], h)
		return nil
	}
	res := store.RollupResult{}
	if r.count != nil {
		res.Count = *r.count
	}
	if r.value != nil {
		v, err := rollupValue(q, *r.value)
		if err != nil {
			return err
		}
		res.Value = v
	}
	out[*r.src] = res
	return nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// buildAlongSQL renders q's walk as one statement returning the columns
// alongRow scans. Every name and id is a bind argument; the states and
// level numbers are rendered, and come from Go, not from the caller.
func buildAlongSQL(q store.RollupQuery, levels [][]store.AlongHop) (sqlText string, args []any, err error) {
	b := &sqlBuilder{}
	limit := q.Along.RowCap
	if limit == 0 {
		limit = store.AlongRowCap
	}
	with, overCap := alongCountedSQL(b, q, levels, sourcesSeed(b, q.Sources), limit)
	sentinel := "SELECT TRUE AS over_cap, NULL::text AS src, NULL::text AS id, NULL::text AS type, " +
		"NULL::text AS face, NULL::int AS n, NULL::text AS value WHERE " + overCap
	result := "SELECT FALSE, c.src, NULL, NULL, NULL, count(*)::int, " + alongFoldSQL(q) +
		" FROM counted c GROUP BY c.src"
	if q.Along.Visits {
		result = "SELECT FALSE, c.src, c.id, c.type, c.face, NULL, c.val::text FROM counted c"
	}
	sqlText = withClause(with) + "SELECT * FROM (" + sentinel + " UNION ALL " + result + ") o ORDER BY o.over_cap DESC"
	if b.err != nil {
		return "", nil, b.err
	}
	return sqlText, b.args, nil
}

// sourcesSeed selects the ids of sources, one column per row.
func sourcesSeed(b *sqlBuilder, sources []string) string {
	return "SELECT unnest(" + b.arg(sources) + "::text[])"
}

// alongCountedSQL renders the walk of q from the ids seed selects and
// counted, its counted visits per (src, id), with the condition under which
// the walk is over limit rows.
func alongCountedSQL(
	b *sqlBuilder, q store.RollupQuery, levels [][]store.AlongHop, seed string, limit int,
) (with []string, overCap string) {
	with = append(alongWalkSQL(b, q, levels, seed, b.arg(limit+1)),
		`counted AS (SELECT w.src, w.id, min(w.type) AS type, min(w.face COLLATE "C") AS face, `+
			`min(w.val) AS val FROM walk w WHERE w.id <> w.src GROUP BY w.src, w.id HAVING bool_or(w.hit))`)
	return with, "(SELECT count(*) FROM walk) > " + b.arg(limit)
}

// alongWalkSQL renders the CTEs l0 to lK of q's walk from the ids seed
// selects and walk, the union of l1 to lK: every gated (src, id, state)
// triple, at most limitArg per level.
func alongWalkSQL(b *sqlBuilder, q store.RollupQuery, levels [][]store.AlongHop, seed, limitArg string) []string {
	with := []string{fmt.Sprintf(
		`l0 AS (SELECT DISTINCT s COLLATE "C" AS src, s COLLATE "C" AS id, -1 AS state FROM (%s) z(s))`, seed)}
	walk := make([]string, 0, len(levels))
	for i, hops := range levels {
		k := i + 1
		with = append(with, alongLevelSQL(b, q, k, hops, limitArg)...)
		if k > 1 && k < len(levels) {
			with = append(with, fmt.Sprintf(
				"%s AS (SELECT src, id, state FROM %s UNION ALL SELECT src, id, state FROM l%d)",
				alongSeen(k), alongSeen(k-1), k))
		}
		walk = append(walk, fmt.Sprintf("SELECT src, id, type, face, hit, val FROM l%d", k))
	}
	return append(with, "walk AS ("+strings.Join(walk, " UNION ALL ")+")")
}

// alongSeen names the CTE of every triple levels 1 to k hold.
func alongSeen(k int) string {
	if k == 1 {
		return "l1"
	}
	return "a" + strconv.Itoa(k)
}

// alongLevelSQL renders level k: per hop its pairs and one gate per type,
// then the level itself, the gated triples no earlier level holds.
func alongLevelSQL(b *sqlBuilder, q store.RollupQuery, k int, hops []store.AlongHop, limitArg string) []string {
	var with, arms []string
	for _, hop := range hops {
		seg := q.Along.Segments[hop.Seg]
		pairs := fmt.Sprintf("l%d_s%d", k, hop.Seg)
		with = append(with, pairs+" AS ("+alongHopSQL(b, k, hop, seg)+")")
		for ti, typ := range slices.Sorted(maps.Keys(seg.Through)) {
			gate := fmt.Sprintf("%s_t%d", pairs, ti)
			with = append(with, alongGateSQL(b, gate, seg.Through[typ], q, "SELECT id FROM "+pairs)...)
			arms = append(arms, fmt.Sprintf(
				"SELECT h.src, h.id, %d AS state, %s::text AS type, g.face, g.hit, g.val FROM %s h JOIN %s g ON g.id = h.id",
				hop.Seg, b.arg(typ), pairs, gate))
		}
	}
	level := fmt.Sprintf("l%d AS (SELECT u.* FROM (%s) u", k, strings.Join(arms, " UNION ALL "))
	if k > 1 {
		level += fmt.Sprintf(" WHERE NOT EXISTS (SELECT 1 FROM %s x WHERE x.src = u.src AND x.id = u.id AND x.state = u.state)",
			alongSeen(k-1))
	}
	return append(with, level+" LIMIT "+limitArg+")")
}

// alongHopSQL selects the distinct (src, id) pairs the identity-scoped
// edges of seg reach from the level k-1 triples in hop's states.
func alongHopSQL(b *sqlBuilder, k int, hop store.AlongHop, seg store.AlongSegment) string {
	nearCol, farCol := "from_id", "to_id"
	if seg.SourceIsTo {
		nearCol, farCol = farCol, nearCol
	}
	states := make([]string, len(hop.From))
	for i, st := range hop.From {
		states[i] = strconv.Itoa(st)
	}
	return fmt.Sprintf("SELECT DISTINCT p.src, r.%s AS id FROM l%d p JOIN relations r ON r.%s = p.id "+
		"WHERE r.rel_type = %s AND r.from_face = '' AND p.state IN (%s)",
		farCol, k-1, nearCol, b.arg(seg.Relation), strings.Join(states, ", "))
}

// alongGateSQL renders the gate name over the ids subquery: the prime row
// node.Gate selects for each id, with whether node.Counts holds on it and
// the value q folds. The candidates and every closure are seeded by ids.
func alongGateSQL(b *sqlBuilder, name string, node store.AlongNode, q store.RollupQuery, ids string) []string {
	typeArg := b.arg(node.Gate.EntityType)
	with, source := graphSourceSeeded(b, name+"_", node.Gate, typeArg, ids)
	hit := "FALSE"
	if node.Counts != nil {
		prev := b.closureSeed
		b.closureSeed = ids
		cb := condBuilder{b: b, prefix: name + "_ct", typeArg: typeArg, sel: node.Gate.Faces}
		hit = "COALESCE(" + cb.render(*node.Counts) + ", FALSE)"
		b.closureSeed = prev
		with = append(with, cb.with...)
	}
	return append(with, fmt.Sprintf(
		"%s AS (SELECT x.id, x.face, x.hit, %s AS val FROM (SELECT e.id, e.face, e.properties, %s AS hit FROM %s) x)",
		name, alongValueSQL(b, q), hit, source))
}

// alongValueSQL is the value a visit contributes to q's fold, over x.
func alongValueSQL(b *sqlBuilder, q store.RollupQuery) string {
	switch {
	case q.Func == store.RollupCount:
		return "NULL::text"
	case q.Numeric:
		return integralSQL(b, q.Value)
	default:
		return nonEmptyStringSQL(b, q.Value)
	}
}

// alongFoldSQL folds the counted visits' values per source as text.
func alongFoldSQL(q store.RollupQuery) string {
	switch {
	case q.Func == store.RollupMin && q.Numeric:
		return "min(c.val)::text"
	case q.Func == store.RollupMax && q.Numeric:
		return "max(c.val)::text"
	case q.Func == store.RollupMin:
		return `min(c.val COLLATE "C")`
	case q.Func == store.RollupMax:
		return `max(c.val COLLATE "C")`
	default:
		return "NULL::text"
	}
}
