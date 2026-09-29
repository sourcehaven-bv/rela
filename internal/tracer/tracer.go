// Package tracer provides graph traversal operations (trace, path,
// orphan detection, cycle detection, clustering) as a service separate
// from the store.
//
// The generic Tracer reads from a store.EntityReader + store.RelationReader.
// Smart backends (e.g. Postgres) can provide native implementations using
// recursive CTEs without going through the store abstraction.
//
// # Faces
//
// A node is a family: every stored face of one id (BUG-95W7MV). Edges are
// followed on every tail, so a content-scoped edge on any face connects its
// family. A node's Title and Properties come from the face the tracer's
// world selects (see [New]). A faced type has no face in the default world
// until TKT-7IZHP0, so its node carries its id and faces and no title,
// instead of vanishing from the trace.
package tracer

import (
	"cmp"
	"context"
	"iter"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TraceResult represents a tree of entities reachable from a starting point.
//
// Title is the entity's literal `title` property (the tracer is a pure reader
// with no metamodel, so it cannot resolve display_property). Properties carries
// the raw property map so a metamodel-aware output layer can render the display
// title itself (see internal/output.Writer.traceTitle); it is `json:"-"` so the
// trace JSON schema stays lean — properties are plumbing for text rendering,
// not part of the trace's data contract.
type TraceResult struct {
	ID    string
	Type  string
	Title string
	// Faces lists the node's faces; see [FamilyFaces]. Nil for a faceless
	// entity, so a faceless project's trace JSON is unchanged.
	Faces      []entity.Face  `json:",omitempty"`
	Properties map[string]any `json:"-"`
	Depth      int
	Relation   string // relation that led to this node
	Incoming   bool   // reached via an incoming relation
	// Tail is the tail of the edge that led to this node; zero for the root.
	Tail     Tail `json:"-"`
	Children []*TraceResult
}

// Tail is the From end of an edge: its entity and, for a content-scoped
// edge, the face it hangs from. A reader that may not read that face may
// not see the edge (see visibility's Resolver.EndpointsReadable).
type Tail struct {
	ID   string
	Face entity.Face
}

// tailOf returns r's tail.
func tailOf(r *entity.Relation) Tail {
	return Tail{ID: r.From, Face: r.FromFace}
}

// PathStep represents one step in a path between two entities. (No Properties
// field: path text output renders only ID/Type, never a title, so there is
// nothing to resolve.)
type PathStep struct {
	ID    string
	Type  string
	Title string
	// Faces is [TraceResult.Faces] for this step.
	Faces    []entity.Face `json:",omitempty"`
	Relation string        // relation that led to this step
	// Tail is [TraceResult.Tail] for this step.
	Tail Tail `json:"-"`
}

// Orphan is a family with no incident edge on any tail.
type Orphan struct {
	ID   string
	Type string
	// Faces is [TraceResult.Faces] for this family.
	Faces []entity.Face `json:",omitempty"`
	// Title is [Node.Title]: the title of the face the world serves, empty
	// when it serves none.
	Title string `json:",omitempty"`
}

// Tracer provides graph traversal operations.
type Tracer interface {
	// TraceFrom follows outgoing and incoming edges from the given entity.
	TraceFrom(ctx context.Context, id string, maxDepth int) *TraceResult

	// TraceTo follows incoming edges only (upstream dependencies).
	TraceTo(ctx context.Context, id string, maxDepth int) *TraceResult

	// FindPath finds the shortest path between two entities (BFS, undirected).
	FindPath(ctx context.Context, fromID, toID string) []PathStep

	// FindOrphans returns the families with no relations, sorted by id.
	FindOrphans(ctx context.Context) ([]Orphan, error)

	// HasCycle returns true if there is a cycle reachable from the given entity.
	HasCycle(ctx context.Context, startID string) bool
}

// reader combines EntityReader and RelationReader for the generic tracer.
type reader interface {
	store.EntityReader
	store.RelationReader
}

// New creates a generic Tracer backed by a store's entity and relation
// readers. world selects the face a node's title and properties come from;
// wiring passes worlds.Compiled.Default().
func New(r reader, world store.WorldScope) *GenericTracer {
	return &GenericTracer{r: r, world: world}
}

// GenericTracer implements Tracer by reading from the store.
type GenericTracer struct {
	r     reader
	world store.WorldScope
	gate  EdgeGate
}

// EdgeGate reports, for each relation in rels, whether the traversal may
// follow it. It is called once per expanded node with that node's edges in
// one direction, so an implementation can batch its reads. A false entry
// removes the edge before the traversal sees it: its far end is not
// reached, expanded or counted against a depth budget over that edge.
type EdgeGate func(ctx context.Context, rels []*entity.Relation) []bool

// WithEdgeGate returns a copy of t that follows only the edges gate admits.
// The tracer stays a pure reader; the gate is supplied by the caller (the
// visibility decorator passes its endpoint-and-tail-face check), so a
// traversal never passes through what the caller may not see.
//
// Nil: accepted, and admits every edge, as [New] does.
func (t *GenericTracer) WithEdgeGate(gate EdgeGate) Tracer {
	c := *t
	c.gate = gate
	return &c
}

// edges returns the relations q selects that the gate admits. A read error
// ends the list, as it always has for traversal: a node whose edges cannot
// be read is shown without them.
func (t *GenericTracer) edges(ctx context.Context, q store.RelationQuery) []*entity.Relation {
	var rels []*entity.Relation
	for r, err := range t.r.ListRelations(ctx, q) {
		if err != nil {
			break
		}
		if r != nil {
			rels = append(rels, r)
		}
	}
	if t.gate == nil || len(rels) == 0 {
		return rels
	}
	ok := t.gate(ctx, rels)
	kept := rels[:0]
	for i, r := range rels {
		if i < len(ok) && ok[i] {
			kept = append(kept, r)
		}
	}
	return kept
}

var _ Tracer = (*GenericTracer)(nil)

// Node is one family as a trace presents it.
type Node struct {
	Type string
	// Faces is the family's faces; see [FamilyFaces].
	Faces []entity.Face
	// Served reports that the world selects a face of the family. Face,
	// Title and Properties describe that face, and are zero when Served is
	// false.
	Served     bool
	Face       entity.Face
	Title      string
	Properties map[string]any
}

// NodeOf presents one family's headers as a trace node. The headers must be
// every face of the family the caller may see: world resolution decides on
// absence, so a partial set can select the wrong face. ok is false for no
// headers, or for headers of mixed type (a corrupt family is not trusted,
// as in visibility's batch reads).
//
// Exported so the visibility decorator presents a gated family by the same
// rule, over the faces the principal may read.
func NodeOf(w store.WorldScope, headers []store.EntityHeader) (Node, bool) {
	if len(headers) == 0 {
		return Node{}, false
	}
	n := Node{Type: headers[0].Type}
	faces := make([]entity.Face, 0, len(headers))
	candidates := make([]store.WorldCandidate, 0, len(headers))
	for _, h := range headers {
		if h.Type != n.Type {
			return Node{}, false
		}
		faces = append(faces, h.Face)
		candidates = append(candidates, store.WorldCandidate{ID: h.ID, Type: h.Type, Face: h.Face})
	}
	n.Faces = FamilyFaces(faces)
	prime, ok := store.ResolveWorldPrimes(w, candidates)[headers[0].ID]
	if !ok {
		return n, true
	}
	for _, h := range headers {
		if h.Face == prime.Face {
			n.Served, n.Face = true, h.Face
			n.Properties = h.Properties
			n.Title, _ = h.Properties["title"].(string)
			break
		}
	}
	return n, true
}

// FamilyFaces returns faces sorted, or nil when the only face is the
// implicit one: a faceless entity lists no faces, so output for a faceless
// project does not change.
func FamilyFaces(faces []entity.Face) []entity.Face {
	if len(faces) == 0 || (len(faces) == 1 && faces[0].IsDefault()) {
		return nil
	}
	out := slices.Clone(faces)
	slices.Sort(out)
	return slices.Compact(out)
}

// node reads every stored face header of id in one query.
func (t *GenericTracer) node(ctx context.Context, id string) (Node, bool) {
	var headers []store.EntityHeader
	q := store.EntityQuery{IDs: []string{id}, Faces: store.AllFaces()}
	for h, err := range store.ListEntityHeaders(ctx, t.r, q) {
		if err != nil {
			return Node{}, false
		}
		if h.ID == id {
			headers = append(headers, h)
		}
	}
	return NodeOf(t.world, headers)
}

func (t *GenericTracer) TraceFrom(ctx context.Context, id string, maxDepth int) *TraceResult {
	visited := make(map[string]bool)
	return t.traceBidirectional(ctx, id, 0, maxDepth, "", Tail{}, false, visited)
}

func (t *GenericTracer) TraceTo(ctx context.Context, id string, maxDepth int) *TraceResult {
	visited := make(map[string]bool)
	return t.traceTo(ctx, id, 0, maxDepth, "", Tail{}, visited)
}

// result builds the trace node for id, reached over an edge of type
// relation with tail tail, or nil when id has no row.
func (t *GenericTracer) result(ctx context.Context, id string, depth int, relation string, tail Tail) *TraceResult {
	n, ok := t.node(ctx, id)
	if !ok {
		return nil
	}
	return &TraceResult{
		ID:         id,
		Type:       n.Type,
		Title:      n.Title,
		Faces:      n.Faces,
		Properties: n.Properties,
		Depth:      depth,
		Relation:   relation,
		Tail:       tail,
	}
}

func (t *GenericTracer) traceBidirectional(
	ctx context.Context,
	id string, depth, maxDepth int, relation string, tail Tail, incoming bool,
	visited map[string]bool,
) *TraceResult {
	if maxDepth > 0 && depth > maxDepth {
		return nil
	}

	result := t.result(ctx, id, depth, relation, tail)
	if result == nil {
		return nil
	}
	result.Incoming = incoming

	if visited[id] {
		return result
	}
	visited[id] = true

	// Outgoing edges
	for _, r := range t.edges(ctx, store.RelationQuery{EntityID: id, Direction: store.DirectionOutgoing}) {
		child := t.traceBidirectional(ctx, r.To, depth+1, maxDepth, r.Type, tailOf(r), false, visited)
		if child != nil {
			result.Children = append(result.Children, child)
		}
	}

	// Incoming edges
	for _, r := range t.edges(ctx, store.RelationQuery{EntityID: id, Direction: store.DirectionIncoming}) {
		child := t.traceBidirectional(ctx, r.From, depth+1, maxDepth, r.Type, tailOf(r), true, visited)
		if child != nil {
			result.Children = append(result.Children, child)
		}
	}

	return result
}

func (t *GenericTracer) traceTo(
	ctx context.Context,
	id string, depth, maxDepth int, relation string, tail Tail,
	visited map[string]bool,
) *TraceResult {
	if maxDepth > 0 && depth > maxDepth {
		return nil
	}

	result := t.result(ctx, id, depth, relation, tail)
	if result == nil {
		return nil
	}

	if visited[id] {
		return result
	}
	visited[id] = true

	for _, r := range t.edges(ctx, store.RelationQuery{EntityID: id, Direction: store.DirectionIncoming}) {
		child := t.traceTo(ctx, r.From, depth+1, maxDepth, r.Type, tailOf(r), visited)
		if child != nil {
			result.Children = append(result.Children, child)
		}
	}

	return result
}

type pathNeighbor struct {
	id, relation string
	tail         Tail
}

type pathQueueItem struct {
	id   string
	path []PathStep
}

func (t *GenericTracer) FindPath(ctx context.Context, fromID, toID string) []PathStep {
	if fromID == toID {
		if step, ok := t.step(ctx, fromID, pathNeighbor{}); ok {
			return []PathStep{step}
		}
		return nil
	}

	queue := t.initPathQueue(ctx, fromID)
	visited := make(map[string]bool)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current.id] {
			continue
		}
		visited[current.id] = true

		for _, nb := range t.collectNeighbors(ctx, current.id) {
			if nb.id == toID {
				if step, ok := t.step(ctx, toID, nb); ok {
					return append(clonePath(current.path), step)
				}
			}
			if visited[nb.id] {
				continue
			}
			if step, ok := t.step(ctx, nb.id, nb); ok {
				newPath := append(clonePath(current.path), step)
				queue = append(queue, pathQueueItem{id: nb.id, path: newPath})
			}
		}
	}

	return nil
}

func (t *GenericTracer) initPathQueue(ctx context.Context, fromID string) []pathQueueItem {
	step, ok := t.step(ctx, fromID, pathNeighbor{})
	if !ok {
		return nil
	}
	return []pathQueueItem{{id: fromID, path: []PathStep{step}}}
}

func (t *GenericTracer) collectNeighbors(ctx context.Context, id string) []pathNeighbor {
	outgoing := t.edges(ctx, store.RelationQuery{EntityID: id, Direction: store.DirectionOutgoing})
	incoming := t.edges(ctx, store.RelationQuery{EntityID: id, Direction: store.DirectionIncoming})
	out := make([]pathNeighbor, 0, len(outgoing)+len(incoming))
	for _, r := range outgoing {
		out = append(out, pathNeighbor{r.To, r.Type, tailOf(r)})
	}
	for _, r := range incoming {
		out = append(out, pathNeighbor{r.From, r.Type, tailOf(r)})
	}
	return out
}

// step builds the path step for id, reached over nb's edge (zero for the
// first step).
func (t *GenericTracer) step(ctx context.Context, id string, nb pathNeighbor) (PathStep, bool) {
	n, ok := t.node(ctx, id)
	if !ok {
		return PathStep{}, false
	}
	return PathStep{ID: id, Type: n.Type, Title: n.Title, Faces: n.Faces, Relation: nb.relation, Tail: nb.tail}, true
}

func clonePath(p []PathStep) []PathStep {
	out := make([]PathStep, len(p), len(p)+1)
	copy(out, p)
	return out
}

// FindOrphans implements [Tracer]: every family in the store, folded over
// every edge. Two whole-store reads (headers, then relations) and one read
// of the orphans' headers for their titles, not one count per entity.
// Headers, not entities: nothing here reads a body (TKT-1ESTYJ).
func (t *GenericTracer) FindOrphans(ctx context.Context) ([]Orphan, error) {
	fams := make(map[string]Family)
	for h, err := range store.ListEntityHeaders(ctx, t.r, store.EntityQuery{Faces: store.AllFaces()}) {
		if err != nil {
			return nil, err
		}
		AddHeader(fams, h)
	}
	out, err := FoldOrphans(fams, t.r.ListRelations(ctx, store.RelationQuery{}))
	if err != nil || len(out) == 0 {
		return out, err
	}
	ids := make([]string, len(out))
	for i, o := range out {
		ids[i] = o.ID
	}
	byID := make(map[string][]store.EntityHeader, len(ids))
	for h, err := range store.ListEntityHeaders(ctx, t.r, store.EntityQuery{IDs: ids, Faces: store.AllFaces()}) {
		if err != nil {
			return nil, err
		}
		byID[h.ID] = append(byID[h.ID], h)
	}
	for i := range out {
		if n, ok := NodeOf(t.world, byID[out[i].ID]); ok {
			out[i].Title = n.Title
		}
	}
	return out, nil
}

// Family is one id and the faces of it a reader may see, as input to
// [FoldOrphans].
type Family struct {
	Type  string
	Faces []entity.Face
}

// AddHeader adds h's face to its family in fams.
func AddHeader(fams map[string]Family, h store.EntityHeader) {
	f := fams[h.ID]
	f.Type = h.Type
	f.Faces = append(f.Faces, h.Face)
	fams[h.ID] = f
}

// FoldOrphans returns the families in fams that no edge in rels connects,
// sorted by id.
//
// An edge counts only when both ends are in fams and its tail is too: a
// content-scoped edge (non-empty FromFace) counts when that face is among
// its From family's faces, any other edge when the From family is present.
// So fams decides what "connected" means. The generic tracer passes every
// stored family; the visibility decorator passes only what the principal
// may read, so a hidden edge cannot turn a family from orphan into
// non-orphan (the gate-before-fold rule). An edge to a missing entity
// connects nothing.
func FoldOrphans(fams map[string]Family, rels iter.Seq2[*entity.Relation, error]) ([]Orphan, error) {
	connected := make(map[string]bool)
	for r, err := range rels {
		if err != nil {
			return nil, err
		}
		if r == nil || !edgeCounts(fams, r) {
			continue
		}
		connected[r.From] = true
		connected[r.To] = true
	}
	var out []Orphan
	for id, f := range fams {
		if !connected[id] {
			out = append(out, Orphan{ID: id, Type: f.Type, Faces: FamilyFaces(f.Faces)})
		}
	}
	slices.SortFunc(out, func(a, b Orphan) int { return cmp.Compare(a.ID, b.ID) })
	return out, nil
}

// edgeCounts reports whether r joins two families in fams at a present tail.
func edgeCounts(fams map[string]Family, r *entity.Relation) bool {
	from, ok := fams[r.From]
	if !ok {
		return false
	}
	if _, ok := fams[r.To]; !ok {
		return false
	}
	return r.FromFace.IsDefault() || slices.Contains(from.Faces, r.FromFace)
}

func (t *GenericTracer) HasCycle(ctx context.Context, startID string) bool {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	return t.hasCycle(ctx, startID, visited, recStack)
}

func (t *GenericTracer) hasCycle(ctx context.Context, id string, visited, recStack map[string]bool) bool {
	visited[id] = true
	recStack[id] = true

	for _, r := range t.edges(ctx, store.RelationQuery{EntityID: id, Direction: store.DirectionOutgoing}) {
		if !visited[r.To] {
			if t.hasCycle(ctx, r.To, visited, recStack) {
				return true
			}
		} else if recStack[r.To] {
			return true
		}
	}

	recStack[id] = false
	return false
}
