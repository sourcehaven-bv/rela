package sqlitestore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
// level's new triples, passed as JSON, and every triple seen so far, kept
// in a temporary table keyed by (src, id, state) on one connection; its
// hops, gates and node conditions are pgstore's. Go applies the row cap
// and the hop cap to each source alone and folds the counted visits. A
// walk whose names the builder cannot render is answered by
// [store.RollupGeneric].

// alongTriple is one (source, entity, state) arrival of the walk.
type alongTriple struct {
	Src   string `json:"s"`
	ID    string `json:"i"`
	State int    `json:"t"`
}

// alongLevel is one level's statement; its first two arguments are the
// frontier and the row limit, set per run.
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

// walkAlong runs the levels in order on one connection, which holds the
// seen triples. A source holding more triples than the cap, or reading
// more edges than the hop cap in one hop, is over it: its arrivals are
// dropped and it walks no further. Each level returns at most one row past
// the cap per source.
func (s *Store) walkAlong(ctx context.Context, q store.RollupQuery, stmts []alongLevel) (w alongWalked, err error) {
	limit := q.Along.RowCap
	if limit == 0 {
		limit = store.AlongRowCap
	}
	seen, err := s.openAlongSeen(ctx)
	if err != nil {
		return alongWalked{}, err
	}
	defer func() { err = errors.Join(err, seen.close(ctx)) }()
	srcs := slices.Compact(slices.Sorted(slices.Values(q.Sources)))
	frontier := make([]alongTriple, 0, len(srcs))
	for _, src := range srcs {
		frontier = append(frontier, alongTriple{Src: src, ID: src, State: -1})
	}
	w = alongWalked{held: map[string]int{}, over: map[string]bool{}}
	for _, lvl := range stmts {
		if len(frontier) == 0 {
			break
		}
		rows, err := seen.runLevel(ctx, lvl, frontier, limit+1)
		if err != nil {
			return alongWalked{}, err
		}
		for _, r := range rows {
			if r.State == alongHopOver {
				w.over[r.Src] = true
				continue
			}
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
		if err := seen.add(ctx, frontier); err != nil {
			return alongWalked{}, err
		}
	}
	w.visits = slices.DeleteFunc(w.visits, func(v alongVisit) bool { return w.over[v.Src] })
	for src := range w.over {
		delete(w.held, src)
	}
	return w, nil
}

// alongHopOver is the state of the row a level statement returns for a
// source over the hop cap.
const alongHopOver = -2

// alongSeenTable keeps a walk's seen triples, keyed, for the anti-join.
const alongSeenTable = "temp.rela_along_seen"

// alongSeen is the connection a walk runs on, holding its seen triples.
type alongSeen struct {
	raw   querier
	close func(context.Context) error
}

// openAlongSeen pins a connection, the transaction's inside a Tx, and
// empties the seen table on it. The table is temporary, so it is never
// part of the database file; close empties it again and releases the
// connection.
func (s *Store) openAlongSeen(ctx context.Context) (alongSeen, error) {
	var raw querier = s.conn
	release := func() error { return nil }
	if s.conn == nil {
		c, err := s.db.Conn(ctx)
		if err != nil {
			return alongSeen{}, fmt.Errorf("sqlitestore: rollup: along connection: %w", err)
		}
		raw, release = c, c.Close
	}
	wipe := func(ctx context.Context) error {
		_, err := raw.ExecContext(ctx, "DELETE FROM "+alongSeenTable)
		return err
	}
	_, err := raw.ExecContext(ctx, "CREATE TEMP TABLE IF NOT EXISTS rela_along_seen (src TEXT NOT NULL, "+
		"id TEXT NOT NULL, state INTEGER NOT NULL, PRIMARY KEY (src, id, state)) WITHOUT ROWID")
	if err == nil {
		err = wipe(ctx)
	}
	if err != nil {
		return alongSeen{}, errors.Join(fmt.Errorf("sqlitestore: rollup: along seen: %w", err), release())
	}
	return alongSeen{raw: raw, close: func(ctx context.Context) error {
		return errors.Join(wipe(context.WithoutCancel(ctx)), release())
	}}, nil
}

// add records triples as seen.
func (a alongSeen) add(ctx context.Context, triples []alongTriple) error {
	if len(triples) == 0 {
		return nil
	}
	data, err := json.Marshal(triples)
	if err != nil {
		return fmt.Errorf("sqlitestore: rollup: along seen: %w", err)
	}
	_, err = a.raw.ExecContext(ctx, "INSERT OR IGNORE INTO "+alongSeenTable+" (src, id, state) "+
		"SELECT json_extract(value, '$.s'), json_extract(value, '$.i'), json_extract(value, '$.t') "+
		"FROM json_each(?)", string(data))
	if err != nil {
		return fmt.Errorf("sqlitestore: rollup: along seen: %w", err)
	}
	return nil
}

// runLevel runs one level from frontier, excluding the triples seen
// already, and returns at most limit gated arrivals per source, and one
// [alongHopOver] row per source over the hop cap.
func (a alongSeen) runLevel(
	ctx context.Context, lvl alongLevel, frontier []alongTriple, limit int,
) ([]alongVisit, error) {
	front, err := json.Marshal(frontier)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: rollup: along frontier: %w", err)
	}
	args := slices.Clone(lvl.args)
	args[0], args[1] = string(front), limit
	return collectRows(ctx, instantQuerier{inner: a.raw}, lvl.sql, args, func(sc scanner) (alongVisit, error) {
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
	frontArg, limitArg := b.arg(""), b.arg(0)
	hopCap := q.Along.HopCap
	if hopCap == 0 {
		hopCap = store.AlongHopCap
	}
	hopArg := b.arg(hopCap)
	with := []string{
		"f AS (SELECT json_extract(value, '$.s') AS src, json_extract(value, '$.i') AS id, " +
			"json_extract(value, '$.t') AS state FROM json_each(" + frontArg + "))",
	}
	arms, over := make([]string, 0, len(hops)), make([]string, 0, len(hops))
	for _, hop := range hops {
		seg := q.Along.Segments[hop.Seg]
		pairs := fmt.Sprintf("s%d", hop.Seg)
		with = append(with, alongHopSQL(b, pairs, hop, seg, hopArg)...)
		over = append(over, "SELECT src FROM "+pairs+"_o")
		for ti, typ := range slices.Sorted(maps.Keys(seg.Through)) {
			gate := fmt.Sprintf("%s_t%d", pairs, ti)
			with = append(with, alongGateSQL(b, gate, seg.Through[typ], q, "SELECT id FROM "+pairs)...)
			arms = append(arms, fmt.Sprintf(
				"SELECT h.src, h.id, %d AS state, %s AS type, g.face, g.hit, g.val FROM %s h JOIN %s g ON g.id = h.id",
				hop.Seg, b.arg(typ), pairs, gate))
		}
	}
	sqlText := withClause(with) + "SELECT * FROM (SELECT v.src, v.id, v.state, v.type, v.face, v.hit, " +
		"CAST(v.val AS TEXT) FROM (SELECT u.*, row_number() OVER (PARTITION BY u.src ORDER BY u.id, u.state) AS rn " +
		"FROM (" + strings.Join(arms, " UNION ALL ") + ") u WHERE NOT EXISTS (SELECT 1 FROM " + alongSeenTable +
		" x WHERE x.src = u.src AND x.id = u.id AND x.state = u.state)) v WHERE v.rn <= " + limitArg +
		" UNION ALL SELECT o.src, '', " + strconv.Itoa(alongHopOver) + ", '', NULL, 0, NULL FROM (" +
		strings.Join(over, " UNION ") + ") o) ORDER BY 1, 2, 3"
	return alongLevel{sql: sqlText, args: b.args}, !b.unsafe
}

// alongHopSQL renders hop's edges as name_e, the edges of seg from each
// distinct (src, entity) of the frontier triples in hop's states; name_o,
// the sources reading more than hopArg of them; and name, the distinct
// (src, id) pairs of the other sources.
func alongHopSQL(b *sqlBuilder, name string, hop store.AlongHop, seg store.AlongSegment, hopArg string) []string {
	nearCol, farCol := "from_id", "to_id"
	if seg.SourceIsTo {
		nearCol, farCol = farCol, nearCol
	}
	states := make([]string, len(hop.From))
	for i, st := range hop.From {
		states[i] = strconv.Itoa(st)
	}
	return []string{
		fmt.Sprintf("%s_e AS MATERIALIZED (SELECT p.src, r.%s AS id FROM (SELECT DISTINCT src, id FROM f "+
			"WHERE state IN (%s)) p JOIN relations r ON r.%s = p.id WHERE r.rel_type = %s AND r.from_face = '')",
			name, farCol, strings.Join(states, ", "), nearCol, b.arg(seg.Relation)),
		fmt.Sprintf("%s_o AS (SELECT src FROM %s_e GROUP BY src HAVING count(*) > %s)", name, name, hopArg),
		fmt.Sprintf("%s AS (SELECT DISTINCT src, id FROM %s_e WHERE src NOT IN (SELECT src FROM %s_o))",
			name, name, name),
	}
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
