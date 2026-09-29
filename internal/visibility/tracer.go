package visibility

import (
	"context"
	"errors"
	"iter"
	"log/slog"
	"maps"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// VisibleTracer is the visibility decorator over a pure [tracer.Tracer]
// (DEC-ZBI39P): it filters the BASE tracer's results post-hoc — the base
// stays ACL-free (the TKT-QU7REX precedent: whole-graph scans ungated,
// visibility applied to the results) — with hidden = nonexistent
// semantics:
//
//   - the base traverses only edges whose ends and tail face are readable
//     ([Resolver.EndpointsReadable]), so nothing is reached through a
//     hidden node or over an edge hung from a hidden face, and a visible
//     node keeps its visible subtree whatever the traversal order;
//   - a hidden root, or a path with a hidden step, yields nil, exactly
//     like an unknown id or no path;
//   - HasCycle on a hidden start behaves as on a nonexistent start, and a
//     cycle counts only over readable edges.
//
// A node is a family (BUG-95W7MV). Every node is re-read from headers through
// the [Resolver]'s batch gate, so its Faces list only the faces the principal
// may read, and its Title and Properties come from the face the world selects
// AMONG those faces, redacted (RR-VN71BT). The base tree's own titles and
// face lists are never passed through: they were read without a principal.
// A redacted title falls back to the id (RR-5N4K35); property maps are always
// fresh (RR-6IL3X7).
//
// FindOrphans is the exception to post-hoc filtering: an orphan is a fold
// over edges, and folding raw edges would let a hidden edge turn a visible
// family from orphan into non-orphan. It gates families and edges first and
// folds after (the gate-before-fold rule), without calling the base.
type VisibleTracer struct {
	base  tracer.Tracer
	res   *Resolver
	rels  relationLister
	world store.WorldScope
}

// EdgeGatable is a tracer that can follow only the edges a gate admits.
// [NewVisibleTracer] requires it: filtering a tree after an ungated
// traversal loses visible descendants whose node was first reached over a
// hidden edge, because the traversal expands each id once. Satisfied by
// *tracer.GenericTracer.
type EdgeGatable interface {
	WithEdgeGate(gate tracer.EdgeGate) tracer.Tracer
}

// relationLister is the whole-store edge scan [VisibleTracer.FindOrphans]
// folds over. Satisfied by store.Store.
type relationLister interface {
	ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error]
}

// NewVisibleTracer builds the decorator. All collaborators are required.
// base traverses only the edges [Resolver.EndpointsReadable] admits, so a
// trace, path or cycle never passes through a hidden node or an edge hung
// from a hidden face. res supplies the gate, the redactor and the header
// reads; world selects a node's title face, and wiring passes
// worlds.Compiled.Default().
func NewVisibleTracer(
	base EdgeGatable, res *Resolver, rels relationLister, world store.WorldScope,
) (*VisibleTracer, error) {
	if base == nil {
		return nil, errors.New("visibility: NewVisibleTracer: base must be non-nil")
	}
	if res == nil {
		return nil, errors.New("visibility: NewVisibleTracer: resolver must be non-nil")
	}
	if rels == nil {
		return nil, errors.New("visibility: NewVisibleTracer: relation lister must be non-nil")
	}
	gated := base.WithEdgeGate(res.EndpointsReadable)
	return &VisibleTracer{base: gated, res: res, rels: rels, world: world}, nil
}

// TraceFrom implements [tracer.Tracer].
func (t *VisibleTracer) TraceFrom(ctx context.Context, id string, maxDepth int) *tracer.TraceResult {
	return t.filterTree(ctx, t.base.TraceFrom(ctx, id, maxDepth))
}

// TraceTo implements [tracer.Tracer].
func (t *VisibleTracer) TraceTo(ctx context.Context, id string, maxDepth int) *tracer.TraceResult {
	return t.filterTree(ctx, t.base.TraceTo(ctx, id, maxDepth))
}

// filterTree presents every node of the returned tree through one batched
// gate, then rebuilds the tree pruning hidden nodes (and their subtrees). A
// nil or hidden root yields nil — the same shape the base tracer returns for
// an unknown id.
func (t *VisibleTracer) filterTree(ctx context.Context, root *tracer.TraceResult) *tracer.TraceResult {
	if root == nil {
		return nil
	}
	seen := map[string]bool{}
	var ids []string
	collectNodeIDs(root, &ids, seen)
	return rebuild(root, t.present(ctx, ids))
}

// collectNodeIDs walks the tree gathering distinct node ids.
func collectNodeIDs(n *tracer.TraceResult, ids *[]string, seen map[string]bool) {
	if n == nil {
		return
	}
	if !seen[n.ID] {
		seen[n.ID] = true
		*ids = append(*ids, n.ID)
	}
	for _, c := range n.Children {
		collectNodeIDs(c, ids, seen)
	}
}

// rebuild returns a filtered COPY of the tree: hidden nodes, and nodes
// reached over a hidden edge, prune their whole subtree; surviving nodes
// take their type, faces, title and properties from nodes (the base tree's
// are never mutated).
func rebuild(n *tracer.TraceResult, nodes map[string]tracer.Node) *tracer.TraceResult {
	if n == nil {
		return nil
	}
	v, ok := nodes[n.ID]
	if !ok || !tailReadable(n.Tail, nodes) {
		return nil
	}
	out := *n
	out.Type, out.Faces, out.Title, out.Properties = v.Type, v.Faces, v.Title, v.Properties
	out.Children = nil
	for _, c := range n.Children {
		if fc := rebuild(c, nodes); fc != nil {
			out.Children = append(out.Children, fc)
		}
	}
	return &out
}

// present reads the readable faces of ids in one header query and presents
// each readable family as a node with [tracer.NodeOf], then redacts the
// served face once. An id absent from the result is hidden or missing. A
// failed read presents nothing (fail-closed, logged by readableHeaders).
func (t *VisibleTracer) present(ctx context.Context, ids []string) map[string]tracer.Node {
	readable, ok := t.res.readableHeaders(ctx, ids)
	if !ok {
		return nil
	}
	nodes := make(map[string]tracer.Node, len(readable))
	var probes []*entity.Entity
	for id, byFace := range readable {
		n, ok := tracer.NodeOf(t.world, slices.Collect(maps.Values(byFace)))
		if !ok {
			continue
		}
		nodes[id] = n
		if n.Served {
			probes = append(probes, nodeEntity(id, n))
		}
	}
	ctx = PrimeTraversals(ctx, t.res.redact, probes)
	for id, n := range nodes {
		if n.Served {
			nodes[id] = t.redactNode(ctx, id, n)
		}
	}
	return nodes
}

// nodeEntity is the entity a node's served face describes, for field
// verdicts. The node carries the raw property map of that face, so `when:`
// predicates see what a store load would provide.
func nodeEntity(id string, n tracer.Node) *entity.Entity {
	return &entity.Entity{ID: id, Type: n.Type, Face: n.Face, Properties: n.Properties}
}

// redactNode strips hidden properties from a served node onto a fresh map
// and applies the ID title-fallback when the title property is hidden.
func (t *VisibleTracer) redactNode(ctx context.Context, id string, n tracer.Node) tracer.Node {
	hidden := t.res.redact.HiddenProperties(ctx, nodeEntity(id, n))
	if len(hidden) == 0 {
		n.Properties = maps.Clone(n.Properties)
		return n
	}
	n.Properties = filterProps(n.Properties, hidden)
	if _, h := hidden["title"]; h {
		// Title is the literal `title` property — the secondary channel
		// redaction must also close.
		n.Title = id
	}
	return n
}

// FindPath implements [tracer.Tracer]. The base searches readable edges
// only, so it finds a visible path where one exists. A step that is still
// hidden when presented (a gate that changed between the two reads)
// withholds the WHOLE path, returning nil like the base's no-path result.
func (t *VisibleTracer) FindPath(ctx context.Context, fromID, toID string) []tracer.PathStep {
	steps := t.base.FindPath(ctx, fromID, toID)
	if len(steps) == 0 {
		return nil
	}
	ids := make([]string, 0, len(steps))
	for _, s := range steps {
		ids = append(ids, s.ID)
	}
	nodes := t.present(ctx, ids)
	out := make([]tracer.PathStep, len(steps))
	for i, s := range steps {
		n, ok := nodes[s.ID]
		if !ok || !tailReadable(s.Tail, nodes) {
			return nil
		}
		out[i] = tracer.PathStep{
			ID: s.ID, Type: n.Type, Title: n.Title, Faces: n.Faces, Relation: s.Relation, Tail: s.Tail,
		}
	}
	return out
}

// tailReadable reports whether an edge with tail may be shown: a
// content-scoped edge hangs from one face, which must be among its tail
// family's readable faces ([Resolver.EndpointsReadable]'s rule). An
// entity-level tail, or the zero tail of a root, needs nothing more than
// the node itself.
func tailReadable(tail tracer.Tail, nodes map[string]tracer.Node) bool {
	if tail.Face.IsDefault() {
		return true
	}
	return slices.Contains(nodes[tail.ID].Faces, tail.Face)
}

// FindOrphans implements [tracer.Tracer] by folding only what the principal
// may read: families reduced to their readable faces, and edges whose both
// ends and tail face are readable ([tracer.FoldOrphans], the same rule as
// [Resolver.EndpointsReadable]). A hidden edge therefore cannot rescue a
// visible family from the report, and a hidden face never appears in an
// orphan's Faces (RR-VN71BT).
//
// Two whole-store reads (headers, relations), one PermitsReadMany and one
// face-set lookup per type, then one batched presentation of the orphans
// for their redacted titles. A failed read is returned; a gate error hides
// that type fail-closed, logged.
func (t *VisibleTracer) FindOrphans(ctx context.Context) ([]tracer.Orphan, error) {
	stored, err := t.res.storedHeaders(ctx, store.EntityQuery{Faces: store.AllFaces()})
	if err != nil {
		return nil, err
	}
	readable, err := t.res.gateHeaders(ctx, stored, func(typ string, err error) error {
		slog.Warn("visibility: tracer orphan gate failed; dropping type fail-closed",
			"type", typ, "err", err)
		return nil
	})
	if err != nil { // coverage-ignore: the callback above never aborts
		return nil, err
	}
	fams := make(map[string]tracer.Family, len(readable))
	for _, byFace := range readable {
		for _, h := range byFace {
			tracer.AddHeader(fams, h)
		}
	}
	out, err := tracer.FoldOrphans(fams, t.rels.ListRelations(ctx, store.RelationQuery{}))
	if err != nil || len(out) == 0 {
		return out, err
	}
	ids := make([]string, len(out))
	for i, o := range out {
		ids[i] = o.ID
	}
	nodes := t.present(ctx, ids)
	for i := range out {
		out[i].Title = nodes[out[i].ID].Title
	}
	return out, nil
}

// HasCycle implements [tracer.Tracer]. A hidden (or missing, or
// gate-erroring) start returns false — the same result the base returns
// for a nonexistent start, so the bool is not an existence oracle for
// the start node. The base follows readable edges only, so a cycle through
// a hidden node or a hidden face's edge is not reported.
func (t *VisibleTracer) HasCycle(ctx context.Context, startID string) bool {
	readable, ok := t.res.readableHeaders(ctx, []string{startID})
	if !ok || len(readable[startID]) == 0 {
		return false
	}
	return t.base.HasCycle(ctx, startID)
}
