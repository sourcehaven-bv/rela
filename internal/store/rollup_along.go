package store

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// MaxAlongDepth is the most hops an [AlongWalk] may take. Each level of the
// walk is one more pair of reads (or one more unrolled level of a SQL
// statement), so the cap bounds the cost of a statement.
const MaxAlongDepth = 32

// AlongRowCap is the default most (source, entity, state) triples one walk
// may hold. A walk that would hold more fails with [ErrAlongRowCap] rather
// than return a truncated, and so inexact, answer.
const AlongRowCap = 50000

// ErrAlongRowCap is returned when an along walk exceeds its row cap. The
// caller leaves the roll-up absent: it fails closed.
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
	// RowCap caps the triples the walk holds; zero means [AlongRowCap].
	RowCap int
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
	case a.RowCap < 0:
		return fmt.Errorf("%w: rollup: along row cap is negative", ErrInvalidQuery)
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
	limit := a.RowCap
	if limit == 0 {
		limit = AlongRowCap
	}
	seen := map[alongKey]bool{}
	visited := map[string]map[string]alongVisit{}
	frontier := make([]alongKey, 0, len(q.Sources))
	for _, src := range slices.Sorted(slices.Values(q.Sources)) {
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
			reached, err := alongHop(ctx, st, a.Segments[j], bySeg[j])
			if err != nil {
				return nil, err
			}
			for _, r := range reached {
				k := alongKey{src: r.src, id: r.header.ID, state: j}
				if seen[k] {
					continue
				}
				seen[k] = true
				if len(seen) > limit {
					return nil, ErrAlongRowCap
				}
				next = append(next, k)
				if k.id == k.src {
					continue
				}
				if visited[k.src] == nil {
					visited[k.src] = map[string]alongVisit{}
				}
				if _, ok := visited[k.src][k.id]; !ok {
					visited[k.src][k.id] = alongVisit{header: r.header, state: j}
				}
			}
		}
		frontier = next
	}
	counted, err := alongCounted(ctx, st, a, seen, visited)
	if err != nil {
		return nil, err
	}
	out := map[string]RollupResult{}
	for src, ids := range visited {
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

// alongReach is one entity a hop reached from a source.
type alongReach struct {
	src    string
	header EntityHeader
}

// alongHop follows seg from the frontier triples: one relation read for
// every frontier id, then one gated header read per type seg reaches.
func alongHop(ctx context.Context, st RollupReader, seg AlongSegment, from []alongKey) ([]alongReach, error) {
	srcsOf := map[string][]string{}
	for _, k := range from {
		if !slices.Contains(srcsOf[k.id], k.src) {
			srcsOf[k.id] = append(srcsOf[k.id], k.src)
		}
	}
	near := slices.Sorted(maps.Keys(srcsOf))
	identity := entity.Face("")
	rq := RelationQuery{Type: seg.Relation, EntityIDs: near, FromFace: &identity, Direction: DirectionOutgoing}
	if seg.SourceIsTo {
		rq.Direction = DirectionIncoming
	}
	linked := map[string][]string{}
	for r, err := range st.ListRelations(ctx, rq) {
		if err != nil {
			return nil, err
		}
		nearID, far := r.From, r.To
		if seg.SourceIsTo {
			nearID, far = r.To, r.From
		}
		for _, src := range srcsOf[nearID] {
			if !slices.Contains(linked[far], src) {
				linked[far] = append(linked[far], src)
			}
		}
	}
	if len(linked) == 0 {
		return nil, nil
	}
	var out []alongReach
	for _, typ := range slices.Sorted(maps.Keys(seg.Through)) {
		gate := seg.Through[typ].Gate
		gate.Related = append(slices.Clone(gate.Related), DirectedRelation{
			Incoming: !seg.SourceIsTo,
			Pred:     RelationPredicate{OfTypes: []string{seg.Relation}, Endpoints: near},
		})
		for h, err := range GraphQueryHeaders(ctx, st, gate) {
			if err != nil {
				return nil, err
			}
			h.Type = typ
			for _, src := range linked[h.ID] {
				out = append(out, alongReach{src: src, header: h})
			}
		}
	}
	return out, nil
}

// alongCounted judges every counting node once over the ids visited through
// it, and returns the (source, id) pairs that count. A visit counts when its
// node's Counts holds on the face the walk's gate selected.
func alongCounted(
	ctx context.Context, st GraphQueryer, a *AlongWalk, seen map[alongKey]bool, visited map[string]map[string]alongVisit,
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
