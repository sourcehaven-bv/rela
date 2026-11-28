package visibility

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/rollup"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// An along roll-up (design gap 1) walks the reader's world, gate before
// fold:
//
//   - every entity the walk reaches is gated by the reader's read query for
//     its type ([Rollups.gate]), so one the reader cannot read is not
//     visited and breaks every path through it;
//   - a node condition is judged on the visited row as the reader sees it:
//     a property a `visible:` grant hides there reads as unset, and every
//     other property the row leaves empty holds its declared default.
//
// A column whose node conditions all lower is judged in the store, each
// condition rewritten as `(V and leaf) or (not V and leaf_at_unset)` per
// hidden property (see fieldcond.go). Otherwise, or when a grant has no
// store form, the store returns the gated visits and Go judges them: one
// header batch per visited type, the hidden properties decided once per
// grant, then [rollup.Node.Holds] and the fold.

// alongGates is the reader's gate per visited type; a type absent from it
// is one the reader reads none of.
type alongGates map[string]store.GraphQuery

// alongColumn runs one along roll-up for ids, each walk capped at rowCap
// rows. withheld are the sources of a nested roll-up whose value is absent
// because an inner value on one of their visits is.
func (r *Rollups) alongColumn(
	ctx context.Context, col rollup.Column, ids []string, world store.WorldScope, rowCap int,
) (results map[string]store.RollupResult, withheld map[string]bool, err error) {
	a := col.Along
	gates, err := r.alongGates(ctx, a, world)
	if err != nil {
		return nil, nil, err
	}
	if a.Lowers() {
		counts, err := r.alongCounts(ctx, col, gates)
		switch {
		case err == nil:
			results, err = store.Rollup(ctx, r.st, alongQuery(col, ids, gates, counts, false, rowCap))
			return results, nil, err
		case !errors.Is(err, errFieldCondUnsupported):
			return nil, nil, err
		}
	}
	return r.alongGoFold(ctx, col, ids, gates, world, rowCap)
}

// alongGates is the reader's gate per type a reaches in world.
func (r *Rollups) alongGates(ctx context.Context, a *rollup.Along, world store.WorldScope) (alongGates, error) {
	gates := alongGates{}
	for _, seg := range a.Segments {
		for _, typ := range seg.Reaches {
			if _, done := gates[typ]; done {
				continue
			}
			g, denied, err := r.gate(ctx, typ, world)
			switch {
			case err != nil:
				return nil, err
			case !denied:
				gates[typ] = g
			}
		}
	}
	return gates, nil
}

// alongQuery is the store query of col: each reachable type the reader
// reads gated, and each node type judged by counts.
func alongQuery(
	col rollup.Column, ids []string, gates alongGates, counts map[string]store.Cond, visits bool, rowCap int,
) store.RollupQuery {
	a := col.Along
	walk := &store.AlongWalk{MaxDepth: a.MaxDepth, Visits: visits, RowCap: rowCap}
	for _, seg := range a.Segments {
		through := map[string]store.AlongNode{}
		for _, typ := range seg.Reaches {
			g, ok := gates[typ]
			if !ok {
				continue
			}
			node := store.AlongNode{Gate: g}
			if c, counting := counts[typ]; counting {
				node.Counts = &c
			}
			through[typ] = node
		}
		walk.Segments = append(walk.Segments, store.AlongSegment{
			Relation: seg.Relation, SourceIsTo: seg.SourceIsTo, Repeat: seg.Repeat, Through: through,
		})
	}
	return store.RollupQuery{
		Along: walk, Sources: ids, Func: col.StoreFunc(), Value: col.Value, Numeric: col.Numeric,
	}
}

// alongCounts is the store form of every node condition the reader can
// reach: defaults filled, each hidden property guarded, and for min or max
// the value property required visible. An error wrapping
// errFieldCondUnsupported sends the column to the Go fold.
func (r *Rollups) alongCounts(
	ctx context.Context, col rollup.Column, gates alongGates,
) (map[string]store.Cond, error) {
	out := map[string]store.Cond{}
	for _, typ := range slices.Sorted(maps.Keys(col.Along.Nodes)) {
		g, ok := gates[typ]
		if !ok {
			continue
		}
		n := col.Along.Nodes[typ]
		fs, err := r.fields.SearchFieldScope(ctx, typ)
		if err != nil {
			return nil, fmt.Errorf("field scope: %w", err)
		}
		faces := returnableFaces(g.FaceIn, g.Faces, typ)
		var hidden []string
		for _, p := range n.Reads {
			if fs.Gates(p) {
				hidden = append(hidden, p)
			}
		}
		conds, err := hiddenPropConds(fs, typ, hidden, faces)
		if err != nil {
			return nil, err
		}
		c, err := guardFilledTree(*n.Lowered, conds, n.FillDefaults)
		if err != nil {
			return nil, err
		}
		if col.Value != "" {
			v, err := visibleCond(fs, typ, col.Value, faces)
			if err != nil {
				return nil, err
			}
			c = store.AndCond(c, v)
		}
		if err := store.ValidateCond(c); err != nil {
			return nil, fmt.Errorf("%w: %w", errFieldCondUnsupported, err)
		}
		out[typ] = c
	}
	return out, nil
}

// alongGoFold judges the node conditions in Go: the store returns every
// gated visit of a node type, one header read per type loads their rows,
// and each row is judged as the reader sees it before the fold. For a
// nested roll-up the visits are the frontier: each inner roll-up is
// computed for them first, one batched call per inner column, and a
// source with a visit whose inner value is absent is withheld.
//
// A nested source has its own budget of rowCap rows: charged (1 + its
// gated visits) x segments, it is withheld when over it. No other row of
// the page changes it; innerValues caps the inner walks.
func (r *Rollups) alongGoFold(
	ctx context.Context, col rollup.Column, ids []string, gates alongGates, world store.WorldScope, rowCap int,
) (results map[string]store.RollupResult, withheld map[string]bool, err error) {
	nested := len(col.Inner()) > 0
	if nested && rowCap == 0 {
		rowCap = store.AlongRowCap
	}
	all := map[string]store.Cond{}
	for typ := range col.Along.Nodes {
		all[typ] = store.AndCond()
	}
	if nested {
		for typ := range gates {
			all[typ] = store.AndCond()
		}
	}
	q := alongQuery(col, ids, gates, all, true, rowCap)
	res, err := store.Rollup(ctx, r.st, q)
	if err != nil {
		return nil, nil, err
	}
	refs := map[string][]entity.Ref{}
	over := map[string]bool{}
	for _, src := range slices.Sorted(maps.Keys(res)) {
		if res[src].OverCap || nested && (1+len(res[src].Visits))*len(col.Along.Segments) > rowCap {
			over[src] = true
			continue
		}
		for _, v := range res[src].Visits {
			ref := entity.Ref{ID: v.ID, Face: v.Face}
			if _, node := col.Along.Nodes[v.Type]; node && !slices.Contains(refs[v.Type], ref) {
				refs[v.Type] = append(refs[v.Type], ref)
			}
		}
	}
	var inner map[string]map[string]map[string]any
	if nested {
		if inner, err = r.innerValues(ctx, col, refs, world, rowCap); err != nil {
			return nil, nil, err
		}
	}
	holds := map[string]map[entity.Ref]store.EntityHeader{}
	unknown := map[entity.Ref]bool{}
	for _, typ := range slices.Sorted(maps.Keys(refs)) {
		rows, err := r.judgeVisits(ctx, col, typ, refs[typ], inner[typ], unknown)
		if err != nil {
			return nil, nil, err
		}
		holds[typ] = rows
	}
	results, withheld = foldVisits(q, res, holds, unknown, over)
	return results, withheld, nil
}

// foldVisits folds, per source, the visited rows that hold, and withholds a
// source over its budget or with a visit whose inner value is unknown.
func foldVisits(
	q store.RollupQuery, res map[string]store.RollupResult,
	holds map[string]map[entity.Ref]store.EntityHeader, unknown map[entity.Ref]bool, over map[string]bool,
) (results map[string]store.RollupResult, withheld map[string]bool) {
	results, withheld = map[string]store.RollupResult{}, map[string]bool{}
	for src, sr := range res {
		if over[src] {
			withheld[src] = true
			continue
		}
		var rows []store.EntityHeader
		for _, v := range sr.Visits {
			ref := entity.Ref{ID: v.ID, Face: v.Face}
			if unknown[ref] {
				withheld[src] = true
			}
			if h, ok := holds[v.Type][ref]; ok {
				rows = append(rows, h)
			}
		}
		if len(rows) > 0 {
			results[src] = store.FoldHeaders(q, rows)
		}
	}
	return results, withheld
}

// innerValues computes the inner roll-ups of col for the frontier refs,
// keyed by type, id and property, each inner walk capped per inner source
// at rowCap split evenly over the inner columns that walk. An inner column
// that fails fails col: its value would be judged on a guess. An inner
// source over its cap has no value, so the outer sources visiting it are
// withheld.
func (r *Rollups) innerValues(
	ctx context.Context, col rollup.Column, refs map[string][]entity.Ref, world store.WorldScope, rowCap int,
) (map[string]map[string]map[string]any, error) {
	walks := 0
	for _, in := range col.Inner() {
		if len(refs[in.Type]) == 0 {
			continue
		}
		for _, c := range r.set.For(in.Type) {
			if c.Property == in.Property && (c.Along != nil || c.Siblings != nil) {
				walks++
			}
		}
	}
	share := rowCap
	if walks > 0 {
		share = rowCap / walks
		if share <= 0 {
			return nil, store.ErrAlongRowCap
		}
	}
	out := map[string]map[string]map[string]any{}
	for _, typ := range slices.Sorted(maps.Keys(refs)) {
		names := col.Along.Nodes[typ].Rollups
		if len(names) == 0 {
			continue
		}
		want := func(p string) bool { return slices.Contains(names, p) }
		vals, _, err := r.values(ctx, typ, refs[typ], world, want, share)
		if err != nil {
			return nil, fmt.Errorf("inner roll-ups of %s: %w", typ, err)
		}
		out[typ] = vals
	}
	return out, nil
}

// judgeVisits reads the visited rows of typ once and returns those whose
// node condition holds, as the reader sees them: each property the
// condition or the fold reads and a grant hides on the row is removed,
// and each inner roll-up the condition reads is filled from inner. A row
// whose inner value is absent is not judged; it is marked in unknown.
func (r *Rollups) judgeVisits(
	ctx context.Context, col rollup.Column, typ string, refs []entity.Ref, inner map[string]map[string]any,
	unknown map[entity.Ref]bool,
) (map[entity.Ref]store.EntityHeader, error) {
	n := col.Along.Nodes[typ]
	fs, err := r.fields.SearchFieldScope(ctx, typ)
	if err != nil {
		return nil, fmt.Errorf("field scope: %w", err)
	}
	ids := make([]string, 0, len(refs))
	var faces []entity.Face
	for _, ref := range refs {
		ids = append(ids, ref.ID)
		faces = append(faces, ref.Face)
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	slices.Sort(faces)
	faces = slices.Compact(faces)
	reads := n.Reads
	if col.Value != "" && !slices.Contains(reads, col.Value) {
		reads = append(slices.Clone(reads), col.Value)
	}
	shown := map[string]func(store.EntityHeader) bool{}
	for _, p := range reads {
		if c, gated := visibleWhen(fs, p); gated {
			f, err := store.HoldingRows(ctx, r.st, typ, []store.RowCondition{c}, ids)
			if err != nil {
				return nil, err
			}
			shown[p] = f
		}
	}
	out := map[entity.Ref]store.EntityHeader{}
	for h, err := range store.ListEntityHeaders(ctx, r.st, store.EntityQuery{
		Type: typ, IDs: ids, Faces: store.AtFaces(faces...),
	}) {
		if err != nil {
			return nil, err
		}
		ref := entity.Ref{ID: h.ID, Face: h.Face}
		if !slices.Contains(refs, ref) {
			continue
		}
		props := maps.Clone(h.Properties)
		if props == nil {
			props = map[string]any{}
		}
		var hidden []string
		for _, p := range slices.Sorted(maps.Keys(shown)) {
			if !shown[p](h) {
				hidden = append(hidden, p)
				delete(props, p)
			}
		}
		if !fillInner(props, n.Rollups, inner[h.ID]) {
			unknown[ref] = true
			continue
		}
		ok, err := n.Holds(ctx, h.ID, props, hidden)
		if err != nil {
			return nil, err
		}
		if ok {
			h.Type, h.Properties = typ, props
			out[ref] = h
		}
	}
	return out, nil
}

// fillInner writes the inner roll-up values of one row into props, and
// reports false when one of names has no value there.
func fillInner(props map[string]any, names []string, values map[string]any) bool {
	for _, p := range names {
		v, ok := values[p]
		if !ok {
			return false
		}
		props[p] = v
	}
	return true
}
