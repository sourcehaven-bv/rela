package sqlitestore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/graphquerynaive"
)

// The along walk has pgstore's unrolled shape, one level of
// [store.AlongLevels] at a time: SQLite expands a common table expression
// at every reference, so a chain of levels that each read the one before
// twice (the hop and the anti-join) grows exponentially and is refused
// past about nine levels. Each level is one statement over the previous
// level's new triples and every triple seen so far, both passed as JSON;
// its hops, gates and node conditions are pgstore's. Go keeps the seen
// set, applies the row cap to each source alone and folds the counted
// visits. A walk whose
// names the builder cannot render is answered by [store.RollupGeneric].

// alongTriple is one (source, entity, state) arrival of the walk.
type alongTriple struct {
	Src   string `json:"s"`
	ID    string `json:"i"`
	State int    `json:"t"`
}

// alongLevel is one level's statement; its first three arguments are the
// frontier, the seen triples and the row limit, set per run.
type alongLevel struct {
	sql  string
	args []any
}

// alongVisit is a gated arrival as the level statement returns it.
type alongVisit struct {
	alongTriple
	typ, face string
	hit       bool
	value     sql.NullString
}

// rollupAlong is [Store.Rollup] for an along query.
func (s *Store) rollupAlong(ctx context.Context, q store.RollupQuery) (map[string]store.RollupResult, error) {
	for _, seg := range q.Along.Segments {
		for _, node := range seg.Through {
			if err := graphquerynaive.CheckEndpointShape(node.Gate); err != nil {
				return nil, err
			}
		}
	}
	levels := store.AlongLevels(q.Along)
	if len(q.Sources) == 0 || len(levels) == 0 {
		return map[string]store.RollupResult{}, nil
	}
	stmts := make([]alongLevel, len(levels))
	for i, hops := range levels {
		lvl, ok := buildAlongLevelSQL(q, hops)
		if !ok {
			return store.RollupGeneric(ctx, s, q)
		}
		stmts[i] = lvl
	}
	w, err := s.walkAlong(ctx, q, stmts)
	if err != nil {
		return nil, err
	}
	out, err := foldAlong(q, w.visits)
	if err != nil {
		return nil, err
	}
	for src := range w.over {
		out[src] = store.RollupResult{OverCap: true}
	}
	return out, nil
}

// alongWalked is a walk's gated arrivals for the sources within their cap,
// the triples each of them holds, and the sources over the cap.
type alongWalked struct {
	visits []alongVisit
	held   map[string]int
	over   map[string]bool
}

// walkAlong runs the levels in order. A source holding more triples than
// the cap is over it: its arrivals are dropped and it walks no further.
// Each level returns at most one row past the cap per source.
func (s *Store) walkAlong(ctx context.Context, q store.RollupQuery, stmts []alongLevel) (alongWalked, error) {
	limit := q.Along.RowCap
	if limit == 0 {
		limit = store.AlongRowCap
	}
	srcs := slices.Compact(slices.Sorted(slices.Values(q.Sources)))
	frontier := make([]alongTriple, 0, len(srcs))
	for _, src := range srcs {
		frontier = append(frontier, alongTriple{Src: src, ID: src, State: -1})
	}
	w := alongWalked{held: map[string]int{}, over: map[string]bool{}}
	for _, lvl := range stmts {
		if len(frontier) == 0 {
			break
		}
		rows, err := s.runAlongLevel(ctx, lvl, frontier, w.visits, limit+1)
		if err != nil {
			return alongWalked{}, err
		}
		for _, r := range rows {
			w.held[r.Src]++
			if w.held[r.Src] > limit {
				w.over[r.Src] = true
			}
		}
		frontier = frontier[:0]
		for _, r := range rows {
			if !w.over[r.Src] {
				frontier = append(frontier, r.alongTriple)
				w.visits = append(w.visits, r)
			}
		}
	}
	w.visits = slices.DeleteFunc(w.visits, func(v alongVisit) bool { return w.over[v.Src] })
	for src := range w.over {
		delete(w.held, src)
	}
	return w, nil
}

// runAlongLevel runs one level from frontier, excluding the triples seen
// already, and returns at most limit gated arrivals per source.
func (s *Store) runAlongLevel(
	ctx context.Context, lvl alongLevel, frontier []alongTriple, seen []alongVisit, limit int,
) ([]alongVisit, error) {
	front, err := json.Marshal(frontier)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: rollup: along frontier: %w", err)
	}
	prior := make([]alongTriple, len(seen))
	for i, v := range seen {
		prior[i] = v.alongTriple
	}
	old, err := json.Marshal(prior)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: rollup: along seen: %w", err)
	}
	args := slices.Clone(lvl.args)
	args[0], args[1], args[2] = string(front), string(old), limit
	return collectRows(ctx, s.q(), lvl.sql, args, func(sc scanner) (alongVisit, error) {
		var v alongVisit
		var face sql.NullString
		var hit sql.NullInt64
		err := sc.Scan(&v.Src, &v.ID, &v.State, &v.typ, &face, &hit, &v.value)
		v.face, v.hit = face.String, hit.Int64 != 0
		return v, err
	})
}

// foldAlong folds each source's counted visits: an entity other than the
// source that some arrival counted, at the type, face and value of its
// first arrival. A source with no counted visit has no result.
func foldAlong(q store.RollupQuery, visits []alongVisit) (map[string]store.RollupResult, error) {
	type key struct{ src, id string }
	first := map[key]alongVisit{}
	counted := map[key]bool{}
	for _, v := range visits {
		if v.ID == v.Src {
			continue
		}
		k := key{v.Src, v.ID}
		if _, ok := first[k]; !ok {
			first[k] = v
		}
		counted[k] = counted[k] || v.hit
	}
	rows := map[string][]store.EntityHeader{}
	for k, v := range first {
		if !counted[k] {
			continue
		}
		h := store.EntityHeader{ID: v.ID, Type: v.typ, Face: entity.Face(v.face)}
		if v.value.Valid {
			val, err := rollupValue(q, v.value.String)
			if err != nil {
				return nil, err
			}
			h.Properties = map[string]any{q.Value: val}
		}
		rows[k.src] = append(rows[k.src], h)
	}
	out := make(map[string]store.RollupResult, len(rows))
	for src, hs := range rows {
		res := store.AlongVisitsResult(q, hs)
		if !q.Along.Visits {
			res.Visits = nil
		}
		out[src] = res
	}
	return out, nil
}

// buildAlongLevelSQL renders one level: per hop the distinct (src, id)
// pairs the identity-scoped edges reach from the frontier, one gate per
// type the hop's segment reaches, and the gated triples not yet seen.
// It reports !ok when a name cannot be rendered.
func buildAlongLevelSQL(q store.RollupQuery, hops []store.AlongHop) (alongLevel, bool) {
	b := &sqlBuilder{}
	frontArg, seenArg, limitArg := b.arg(""), b.arg(""), b.arg(0)
	with := []string{
		"f AS (SELECT json_extract(value, '$.s') AS src, json_extract(value, '$.i') AS id, " +
			"json_extract(value, '$.t') AS state FROM json_each(" + frontArg + "))",
		"seen AS (SELECT json_extract(value, '$.s') AS src, json_extract(value, '$.i') AS id, " +
			"json_extract(value, '$.t') AS state FROM json_each(" + seenArg + "))",
	}
	var arms []string
	for _, hop := range hops {
		seg := q.Along.Segments[hop.Seg]
		pairs := fmt.Sprintf("s%d", hop.Seg)
		with = append(with, pairs+" AS ("+alongHopSQL(b, hop, seg)+")")
		for ti, typ := range slices.Sorted(maps.Keys(seg.Through)) {
			gate := fmt.Sprintf("%s_t%d", pairs, ti)
			with = append(with, alongGateSQL(b, gate, seg.Through[typ], q, "SELECT id FROM "+pairs)...)
			arms = append(arms, fmt.Sprintf(
				"SELECT h.src, h.id, %d AS state, %s AS type, g.face, g.hit, g.val FROM %s h JOIN %s g ON g.id = h.id",
				hop.Seg, b.arg(typ), pairs, gate))
		}
	}
	sqlText := withClause(with) + "SELECT v.src, v.id, v.state, v.type, v.face, v.hit, CAST(v.val AS TEXT) FROM " +
		"(SELECT u.*, row_number() OVER (PARTITION BY u.src ORDER BY u.id, u.state) AS rn FROM (" +
		strings.Join(arms, " UNION ALL ") + ") u WHERE NOT EXISTS (SELECT 1 FROM seen x " +
		"WHERE x.src = u.src AND x.id = u.id AND x.state = u.state)) v WHERE v.rn <= " + limitArg +
		" ORDER BY v.src, v.id, v.state"
	return alongLevel{sql: sqlText, args: b.args}, !b.unsafe
}

// alongHopSQL selects the distinct (src, id) pairs the identity-scoped
// edges of seg reach from the frontier triples in hop's states.
func alongHopSQL(b *sqlBuilder, hop store.AlongHop, seg store.AlongSegment) string {
	nearCol, farCol := "from_id", "to_id"
	if seg.SourceIsTo {
		nearCol, farCol = farCol, nearCol
	}
	states := make([]string, len(hop.From))
	for i, st := range hop.From {
		states[i] = strconv.Itoa(st)
	}
	return fmt.Sprintf("SELECT DISTINCT p.src, r.%s AS id FROM f p JOIN relations r ON r.%s = p.id "+
		"WHERE r.rel_type = %s AND r.from_face = '' AND p.state IN (%s)",
		farCol, nearCol, b.arg(seg.Relation), strings.Join(states, ", "))
}

// alongGateSQL renders the gate name over the ids subquery: the prime row
// node.Gate selects for each id, with whether node.Counts holds on it and
// the value q folds. The candidates and every closure are seeded by ids.
func alongGateSQL(b *sqlBuilder, name string, node store.AlongNode, q store.RollupQuery, ids string) []string {
	typeArg := b.arg(node.Gate.EntityType)
	with, source := graphSourcePrefixed(b, name+"_", node.Gate, typeArg, ids)
	hit := "0"
	if node.Counts != nil {
		seed := "SELECT e0.id FROM entities e0 WHERE e0.id IN (" + ids + ") AND +e0.type = " + typeArg
		cb := condBuilder{b: b, prefix: name + "_ct", seed: seed, sel: node.Gate.Faces}
		hit = "COALESCE(" + cb.render(*node.Counts) + ", 0)"
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
		return "NULL"
	case q.Numeric:
		return integralSQL(b, q.Value)
	default:
		return nonEmptyStringSQL(b, q.Value)
	}
}
