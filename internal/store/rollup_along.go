package store

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// MaxAlongDepth is the most hops an [AlongWalk] may take. Each level of the
// walk is one more pair of reads (or one more unrolled level of a SQL
// statement), so the cap bounds the cost of a statement.
const MaxAlongDepth = 32

// AlongRowCap is the default most (source, entity, state) triples one
// source's walk may hold. Only triples a gate admits count, and each source
// is capped alone. A source whose walk would hold more has a result with
// [RollupResult.OverCap] rather than a truncated, and so inexact, answer.
const AlongRowCap = 50000

// AlongHopCap is the default most identity-scoped edges one source may
// read in one hop of its walk, gated or not. It bounds what a hub costs:
// a source whose hop reads more is over its cap, as for [AlongRowCap].
// Since the edges are read before any gate, a reader can learn that a hop
// reached more than this many entities, hidden ones included.
const AlongHopCap = 50000

// ErrAlongRowCap is returned when a row budget is too small to hold any
// walk at all, whatever the data. The caller leaves the roll-up absent: it
// fails closed. A source over its cap is reported by
// [RollupResult.OverCap], never by this error.
var ErrAlongRowCap = errors.New("store: rollup: the along walk exceeds its row cap")

// AlongWalk is the walk of an inherited roll-up (design gap 1): from each
// source, follow the segments in order, a repeated segment zero or more
// times, for at most MaxDepth hops. Every entity other than the source
// reached after one or more hops is visited; a walk that comes back to the
// source goes on through it. An entity no Through gate admits is not
// visited and the walk does not continue through it.
//
// A visited entity counts when its node's Counts holds on the row the gate
// selected. The roll-up folds the counted entities, each once per source.
type AlongWalk struct {
	Segments []AlongSegment
	// MaxDepth caps the hops, from 1 to [MaxAlongDepth].
	MaxDepth int
	// RowCap caps the gated triples each source's walk holds; zero means
	// [AlongRowCap].
	RowCap int
	// HopCap caps the edges each source reads per hop; zero means
	// [AlongHopCap].
	HopCap int
	// Visits asks for the counted visits themselves in
	// [RollupResult.Visits], for a caller that folds in Go.
	Visits bool
}

// AlongSegment is one segment of an [AlongWalk]: the identity-scoped edges
// of Relation, walked once or, with Repeat, zero or more times.
type AlongSegment struct {
	// Relation is the canonical relation type walked.
	Relation string
	// SourceIsTo says the edge runs from the entity reached to the entity
	// before it.
	SourceIsTo bool
	Repeat     bool
	// Through gates the entities the segment reaches, per entity type.
	Through map[string]AlongNode
}

// AlongNode gates and judges the entities of one type a segment reaches.
type AlongNode struct {
	// Gate selects the entities that may be visited, with the rules of
	// [RollupQuery.Related]: a world selection, no paging, sorting or Where.
	Gate GraphQuery
	// Counts is the condition a visited entity's selected row must meet to
	// count. Nil: accepted — the entity is walked through but never counts.
	Counts *Cond
}

// AlongVisit is one counted visit: the entity and the face of the row its
// gate selected.
type AlongVisit struct {
	ID   string
	Type string
	Face entity.Face
}

// validateAlong refuses an along query the backends would answer
// differently.
func validateAlong(q RollupQuery) error {
	a := q.Along
	switch {
	case q.Relation != "" || len(q.Path) > 0 || q.Related.EntityType != "":
		return fmt.Errorf("%w: rollup: along excludes relation, path and related", ErrInvalidQuery)
	case len(q.Require) > 0 || len(q.PickRequire) > 0:
		return fmt.Errorf("%w: rollup: along takes its conditions on the nodes", ErrInvalidQuery)
	case len(a.Segments) == 0:
		return fmt.Errorf("%w: rollup: along needs a segment", ErrInvalidQuery)
	case a.MaxDepth < 1 || a.MaxDepth > MaxAlongDepth:
		return fmt.Errorf("%w: rollup: along max depth %d is outside 1 to %d", ErrInvalidQuery, a.MaxDepth, MaxAlongDepth)
	case a.RowCap < 0 || a.HopCap < 0:
		return fmt.Errorf("%w: rollup: along row or hop cap is negative", ErrInvalidQuery)
	}
	switch q.Func {
	case RollupCount:
	case RollupMin, RollupMax:
		if q.Value == "" {
			return fmt.Errorf("%w: rollup: value property is required", ErrInvalidQuery)
		}
	case RollupSum, RollupLatest:
		return fmt.Errorf("%w: rollup: along folds count, min or max", ErrInvalidQuery)
	default:
		return fmt.Errorf("%w: rollup: unknown function %d", ErrInvalidQuery, q.Func)
	}
	for i, seg := range a.Segments {
		if seg.Relation == "" {
			return fmt.Errorf("%w: rollup: along segment %d: relation is required", ErrInvalidQuery, i)
		}
		for _, typ := range slices.Sorted(maps.Keys(seg.Through)) {
			node := seg.Through[typ]
			what := fmt.Sprintf("along segment %d type %q", i, typ)
			if node.Gate.EntityType != typ {
				return fmt.Errorf("%w: rollup: %s: the gate is for type %q", ErrInvalidQuery, what, node.Gate.EntityType)
			}
			if err := validateRollupGate(what, node.Gate); err != nil {
				return err
			}
			if node.Counts != nil {
				if err := ValidateCond(*node.Counts); err != nil {
					return fmt.Errorf("rollup: %s: %w", what, err)
				}
			}
		}
	}
	return nil
}

// AlongNext returns the segments a walk in state may take next: state
// again when it repeats, and each later segment that only repeated segments
// separate from it. State -1 is the start.
func AlongNext(segs []AlongSegment, state int) []int {
	var out []int
	if state >= 0 && segs[state].Repeat {
		out = append(out, state)
	}
	for j := state + 1; j < len(segs); j++ {
		out = append(out, j)
		if !segs[j].Repeat {
			break
		}
	}
	return out
}

// AlongHop is one hop of a level of an unrolled walk: segment Seg, taken
// from the frontier triples whose state is in From.
type AlongHop struct {
	Seg  int
	From []int
}

// AlongLevels unrolls a's walk for a SQL backend: per level, the hops its
// frontier can take, in segment order. A segment that gates no type reaches
// nothing, so it is left out, and the levels end early when no hop remains.
// The level count is at most a.MaxDepth.
func AlongLevels(a *AlongWalk) [][]AlongHop {
	var levels [][]AlongHop
	states := []int{-1}
	for len(levels) < a.MaxDepth {
		from := map[int][]int{}
		for _, st := range states {
			for _, j := range AlongNext(a.Segments, st) {
				if len(a.Segments[j].Through) > 0 {
					from[j] = append(from[j], st)
				}
			}
		}
		if len(from) == 0 {
			break
		}
		hops := make([]AlongHop, 0, len(from))
		for _, j := range slices.Sorted(maps.Keys(from)) {
			hops = append(hops, AlongHop{Seg: j, From: from[j]})
		}
		levels = append(levels, hops)
		states = slices.Sorted(maps.Keys(from))
	}
	return levels
}

// alongKey is one walk triple: a source, an entity reached, and the
// segment it was reached by.
type alongKey struct {
	src, id string
	state   int
}

// alongVisit is a visited entity with the row its gate selected.
type alongVisit struct {
	header EntityHeader
	state  int
}

// rollupAlong is [RollupGeneric] for an [AlongWalk]: breadth first, per
// level and segment one relation read and one gated header read per type,
// then one MatchingFaces per counting node.
func rollupAlong(ctx context.Context, st RollupReader, q RollupQuery) (map[string]RollupResult, error) {
	a := q.Along
	w, err := walkAlong(ctx, st, a, q.Sources)
	if err != nil {
		return nil, err
	}
	counted, err := alongCounted(ctx, st, a, w.seen, w.visited)
	if err != nil {
		return nil, err
	}
	out := map[string]RollupResult{}
	for src := range w.over {
		out[src] = RollupResult{OverCap: true}
	}
	for src, ids := range w.visited {
		var rows []EntityHeader
		for _, id := range slices.Sorted(maps.Keys(ids)) {
			if counted[alongKey{src: src, id: id}] {
				rows = append(rows, ids[id].header)
			}
		}
		if len(rows) == 0 {
			continue
		}
		res := FoldHeaders(q, rows)
		if a.Visits {
			for _, h := range rows {
				res.Visits = append(res.Visits, AlongVisit{ID: h.ID, Type: h.Type, Face: h.Face})
			}
		}
		out[src] = res
	}
	return out, nil
}

// alongWalked is a walk's outcome for the sources within their cap: every
// triple held, each entity other than the source visited at the row and
// state of its first arrival, and the triples held per source. over are
// the sources whose walk went over the cap; nothing of theirs is kept.
type alongWalked struct {
	seen    map[alongKey]bool
	visited map[string]map[string]alongVisit
	held    map[string]int
	over    map[string]bool
}

// walkAlong walks a from sources, each source capped alone at the walk's
// row cap over the gated triples it holds and at its hop cap over the
// edges it reads per hop.
func walkAlong(ctx context.Context, st RollupReader, a *AlongWalk, sources []string) (alongWalked, error) {
	limit := a.RowCap
	if limit == 0 {
		limit = AlongRowCap
	}
	hopCap := a.HopCap
	if hopCap == 0 {
		hopCap = AlongHopCap
	}
	w := alongWalked{
		seen: map[alongKey]bool{}, visited: map[string]map[string]alongVisit{},
		held: map[string]int{}, over: map[string]bool{},
	}
	frontier := make([]alongKey, 0, len(sources))
	for _, src := range slices.Sorted(slices.Values(sources)) {
		frontier = append(frontier, alongKey{src: src, id: src, state: -1})
	}
	frontier = slices.Compact(frontier)
	for depth := 0; depth < a.MaxDepth && len(frontier) > 0; depth++ {
		bySeg := map[int][]alongKey{}
		for _, k := range frontier {
			for _, j := range AlongNext(a.Segments, k.state) {
				bySeg[j] = append(bySeg[j], k)
			}
		}
		var next []alongKey
		for _, j := range slices.Sorted(maps.Keys(bySeg)) {
			reached, over, err := alongHop(ctx, st, a.Segments[j], bySeg[j], hopCap)
			if err != nil {
				return alongWalked{}, err
			}
			for _, src := range over {
				w.over[src] = true
			}
			for _, r := range reached {
				k := alongKey{src: r.src, id: r.header.ID, state: j}
				if w.over[k.src] || w.seen[k] {
					continue
				}
				w.seen[k] = true
				w.held[k.src]++
				if w.held[k.src] > limit {
					w.over[k.src] = true
					continue
				}
				next = append(next, k)
				noteVisit(w.visited, k, r.header)
			}
		}
		frontier = slices.DeleteFunc(next, func(k alongKey) bool { return w.over[k.src] })
	}
	w.dropOver()
	return w, nil
}

// dropOver forgets everything the sources over their cap held.
func (w alongWalked) dropOver() {
	for k := range w.seen {
		if w.over[k.src] {
			delete(w.seen, k)
		}
	}
	for src := range w.over {
		delete(w.visited, src)
		delete(w.held, src)
	}
}

// noteVisit records k's entity as visited from its source at h and k's
// state, unless it is the source itself or was reached earlier.
func noteVisit(visited map[string]map[string]alongVisit, k alongKey, h EntityHeader) {
	if k.id == k.src {
		return
	}
	if visited[k.src] == nil {
		visited[k.src] = map[string]alongVisit{}
	}
	if _, ok := visited[k.src][k.id]; !ok {
		visited[k.src][k.id] = alongVisit{header: h, state: k.state}
	}
}

// alongReach is one entity a hop reached from a source.
type alongReach struct {
	src    string
	header EntityHeader
}

// alongHop follows seg from the frontier triples: one relation read for
// every frontier id, then one gated header read per type seg reaches.
// over are the sources that read more than hopCap edges; they reach
// nothing.
func alongHop(
	ctx context.Context, st RollupReader, seg AlongSegment, from []alongKey, hopCap int,
) (reached []alongReach, over []string, err error) {
	srcsOf := map[string][]string{}
	for _, k := range from {
		if !slices.Contains(srcsOf[k.id], k.src) {
			srcsOf[k.id] = append(srcsOf[k.id], k.src)
		}
	}
	near := slices.Sorted(maps.Keys(srcsOf))
	linked, edges, err := alongLinks(ctx, st, seg, near, srcsOf)
	if err != nil {
		return nil, nil, err
	}
	for _, src := range slices.Sorted(maps.Keys(edges)) {
		if edges[src] > hopCap {
			over = append(over, src)
		}
	}
	if len(linked) == 0 {
		return nil, over, nil
	}
	for _, typ := range slices.Sorted(maps.Keys(seg.Through)) {
		gate := seg.Through[typ].Gate
		gate.Related = append(slices.Clone(gate.Related), DirectedRelation{
			Incoming: !seg.SourceIsTo,
			Pred:     RelationPredicate{OfTypes: []string{seg.Relation}, Endpoints: near},
		})
		for h, err := range GraphQueryHeaders(ctx, st, gate) {
			if err != nil {
				return nil, nil, err
			}
			h.Type = typ
			for _, src := range linked[h.ID] {
				if !slices.Contains(over, src) {
					reached = append(reached, alongReach{src: src, header: h})
				}
			}
		}
	}
	return reached, over, nil
}

// alongLinks reads seg's identity-scoped edges at near: the sources that
// reach each far entity, and the edges each source read.
func alongLinks(
	ctx context.Context, st RollupReader, seg AlongSegment, near []string, srcsOf map[string][]string,
) (linked map[string][]string, edges map[string]int, err error) {
	identity := entity.Face("")
	rq := RelationQuery{Type: seg.Relation, EntityIDs: near, FromFace: &identity, Direction: DirectionOutgoing}
	if seg.SourceIsTo {
		rq.Direction = DirectionIncoming
	}
	linked, edges = map[string][]string{}, map[string]int{}
	for r, err := range st.ListRelations(ctx, rq) {
		if err != nil {
			return nil, nil, err
		}
		nearID, far := r.From, r.To
		if seg.SourceIsTo {
			nearID, far = r.To, r.From
		}
		for _, src := range srcsOf[nearID] {
			edges[src]++
			if !slices.Contains(linked[far], src) {
				linked[far] = append(linked[far], src)
			}
		}
	}
	return linked, edges, nil
}

// alongCounted judges every counting node once over the ids visited through
// it, and returns the (source, id) pairs that count. A visit counts when its
// node's Counts holds on the face the walk's gate selected.
func alongCounted(
	ctx context.Context, st GraphQueryer, a *AlongWalk,
	seen map[alongKey]bool, visited map[string]map[string]alongVisit,
) (map[alongKey]bool, error) {
	type node struct {
		seg int
		typ string
	}
	keys := map[node][]alongKey{}
	for k := range seen {
		if k.id == k.src {
			continue
		}
		n := node{seg: k.state, typ: visited[k.src][k.id].header.Type}
		if a.Segments[n.seg].Through[n.typ].Counts != nil {
			keys[n] = append(keys[n], k)
		}
	}
	counted := map[alongKey]bool{}
	for n, list := range keys {
		spec := a.Segments[n.seg].Through[n.typ]
		gate := spec.Gate
		gate.Where = AndWhere(gate.Where, spec.Counts)
		ids := make([]string, len(list))
		for i, k := range list {
			ids[i] = k.id
		}
		slices.Sort(ids)
		faces, err := st.MatchingFaces(ctx, gate, slices.Compact(ids))
		if err != nil {
			return nil, err
		}
		for _, k := range list {
			if slices.Contains(faces[k.id], visited[k.src][k.id].header.Face) {
				counted[alongKey{src: k.src, id: k.id}] = true
			}
		}
	}
	return counted, nil
}

// FoldHeaders folds rows, each counted once by id, with q's function: the
// fold [RollupGeneric] applies after its gates, for a caller that judges
// the rows in Go. Require and PickRequire are not applied.
func FoldHeaders(q RollupQuery, rows []EntityHeader) RollupResult {
	f := &rollupFold{q: q}
	done := map[string]bool{}
	for _, h := range rows {
		if done[h.ID] {
			continue
		}
		done[h.ID] = true
		f.add(h, true)
	}
	return f.result()
}

// AlongVisitsResult is one source's answer from its counted visits, for a
// SQL backend that returns them rather than folding: rows carry q.Value
// when they supply one. The visits are listed in id order, as the generic
// walk lists them.
func AlongVisitsResult(q RollupQuery, rows []EntityHeader) RollupResult {
	slices.SortFunc(rows, func(a, b EntityHeader) int { return strings.Compare(a.ID, b.ID) })
	res := FoldHeaders(q, rows)
	for _, h := range rows {
		res.Visits = append(res.Visits, AlongVisit{ID: h.ID, Type: h.Type, Face: h.Face})
	}
	return res
}
