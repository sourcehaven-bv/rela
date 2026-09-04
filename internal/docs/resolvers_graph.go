package docs

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/mermaid"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

const (
	graphDefaultDepth = 2
	graphMaxDepth     = 5
)

// luaLifecycle emits a mermaid state diagram for a state-machine field. A flat
// enum (no transitions) is not a lifecycle and fails loud, pointing at values{}.
// lifecycle{type="risico", field="status"}.
func (dr *docRuntime) luaLifecycle(ls *lua.LState) int {
	tbl := argTable(ls)
	typ := fieldString(ls, tbl, "type")
	field := fieldString(ls, tbl, "field")
	def, ok := dr.entityDef(ls, "lifecycle", typ)
	if !ok {
		return 0
	}
	prop, ok := def.Properties[field]
	if !ok {
		return dr.luaFail(ls, "lifecycle: %q has no field %q", typ, field)
	}
	ct, named := dr.meta.Types[prop.Type]
	if !named {
		return dr.luaFail(ls, "lifecycle: field %q of %q is not a named enum/state machine", field, typ)
	}

	if len(ct.Transitions) == 0 {
		// A flat enum is not a lifecycle. Fail loud pointing at values{} rather
		// than silently rendering a near-duplicate of it — keeps the two
		// resolvers' jobs distinct (lifecycle = state machine, values = the
		// allowed values).
		return dr.luaFail(ls, "lifecycle: field %q of %q is a flat enum with no transitions — use values{} for its allowed values", field, typ)
	}

	initial := ct.Initial
	if initial == "" {
		initial = ct.Default
	}
	ts := make([]mermaid.Transition, 0, len(ct.Transitions))
	for _, tr := range ct.Transitions {
		label := tr.Label
		if label == "" {
			label = tr.To
		}
		ts = append(ts, mermaid.Transition{From: tr.From, To: tr.To, Label: label})
	}
	dr.emit("```mermaid\n" + mermaid.StateDiagram(initial, ts) + "```\n\n")
	return 0
}

// luaGraph emits a mermaid flow graph. Two grains by the `from` value:
//   - an entity TYPE → the schema neighborhood (which types connect to which),
//     read from the metamodel.
//   - an entity ID → the seeded instance neighborhood, traversed via tracer.
//
// graph{from=..., depth=2, direction="out"|"in"|"both", exclude={...}, only={...}}.
func (dr *docRuntime) luaGraph(ls *lua.LState) int {
	tbl := argTable(ls)
	from := fieldString(ls, tbl, "from")
	if from == "" {
		return dr.luaFail(ls, "graph: `from` is required")
	}
	depth := clampDepth(fieldInt(ls, tbl, "depth", graphDefaultDepth))
	direction := fieldString(ls, tbl, "direction")
	if direction == "" {
		direction = "out"
	}
	exclude := fieldStringSlice(ls, tbl, "exclude")
	only := fieldStringSlice(ls, tbl, "only")
	if len(exclude) > 0 && len(only) > 0 {
		return dr.luaFail(ls, "graph: `exclude` and `only` are mutually exclusive")
	}
	filter := relFilter{exclude: toSet(exclude), only: toSet(only)}

	if _, isType := dr.meta.GetEntityDef(from); isType {
		dr.emit(dr.schemaGraph(from, depth, direction, filter))
		return 0
	}
	// Otherwise treat `from` as an instance id.
	if _, err := dr.store.GetEntity(dr.ctx, from); err != nil {
		return dr.luaFail(ls, "graph: %q is neither an entity type nor a seeded id", from)
	}
	dr.emit(dr.instanceGraph(from, depth, direction, filter))
	return 0
}

// relFilter selects which relation types survive. only wins if set; else
// exclude prunes; empty means keep all.
type relFilter struct {
	exclude map[string]bool
	only    map[string]bool
}

func (f relFilter) keep(rel string) bool {
	if len(f.only) > 0 {
		return f.only[rel]
	}
	return !f.exclude[rel]
}

// instanceGraph traverses the seeded memstore from an id and renders it. The
// tracer is bidirectional; direction filters on the child's .Incoming flag,
// which is SEPARATE from the relation-type filter.
func (dr *docRuntime) instanceGraph(id string, depth int, direction string, filter relFilter) string {
	var res *tracer.TraceResult
	if direction == "in" {
		res = dr.tracer.TraceTo(dr.ctx, id, depth)
	} else {
		res = dr.tracer.TraceFrom(dr.ctx, id, depth)
	}

	nodes := map[string]mermaid.Node{}
	var nodeOrder []string
	var edges []mermaid.Edge
	seenEdge := map[string]bool{}

	addNode := func(key, text string) {
		if _, ok := nodes[key]; !ok {
			nodes[key] = mermaid.Node{Key: key, Text: text}
			nodeOrder = append(nodeOrder, key)
		}
	}
	addNode(res.ID, nodeLabel(res.ID, res.Title))

	var walk func(n *tracer.TraceResult)
	walk = func(n *tracer.TraceResult) {
		for _, c := range n.Children {
			// Direction filter: "out" keeps only outgoing edges; "in" already
			// used TraceTo (all incoming); "both" keeps everything. A
			// relation-type filter (exclude/only) is SEPARATE. When an edge is
			// filtered out we still recurse into the child so a pruned edge
			// doesn't sever the subgraph beyond it.
			edgeKept := (direction != "out" || !c.Incoming) && filter.keep(c.Relation)
			if edgeKept {
				addNode(c.ID, nodeLabel(c.ID, c.Title))
				fromKey, toKey := n.ID, c.ID
				if c.Incoming {
					fromKey, toKey = c.ID, n.ID
				}
				ek := fromKey + "\x00" + c.Relation + "\x00" + toKey
				if !seenEdge[ek] {
					seenEdge[ek] = true
					edges = append(edges, mermaid.Edge{FromKey: fromKey, ToKey: toKey, Label: c.Relation})
				}
			}
			walk(c)
		}
	}
	walk(res)

	return renderGraph(nodeOrder, nodes, edges)
}

// schemaGraph renders the metamodel neighborhood of an entity type: a BFS over
// relation definitions to the given depth.
func (dr *docRuntime) schemaGraph(root string, depth int, direction string, filter relFilter) string {
	sb := &schemaBuilder{nodes: map[string]mermaid.Node{}, seenEdge: map[string]bool{}}
	sb.addNode(root)

	frontier := []string{root}
	visited := map[string]bool{root: true}
	relNames := sortedRelNames(dr.meta.Relations)

	for d := 0; d < depth && len(frontier) > 0; d++ {
		var next []string
		for _, typ := range frontier {
			for _, name := range relNames {
				if !filter.keep(name) {
					continue
				}
				next = append(next, sb.expand(dr.meta.Relations[name], name, typ, direction, visited)...)
			}
		}
		frontier = next
	}
	return renderGraph(sb.nodeOrder, sb.nodes, sb.edges)
}

// schemaBuilder accumulates the schema-graph nodes and edges during the BFS.
type schemaBuilder struct {
	nodes     map[string]mermaid.Node
	nodeOrder []string
	edges     []mermaid.Edge
	seenEdge  map[string]bool
}

func (sb *schemaBuilder) addNode(t string) {
	if _, ok := sb.nodes[t]; !ok {
		sb.nodes[t] = mermaid.Node{Key: t, Text: t}
		sb.nodeOrder = append(sb.nodeOrder, t)
	}
}

func (sb *schemaBuilder) addEdge(from, rel, to string) {
	ek := from + "\x00" + rel + "\x00" + to
	if sb.seenEdge[ek] {
		return
	}
	sb.seenEdge[ek] = true
	sb.edges = append(sb.edges, mermaid.Edge{FromKey: from, ToKey: to, Label: rel})
}

// expand adds the edges from `typ` along relation `rel` (named `name`) in the
// requested direction and returns the newly-discovered neighbor types to visit.
func (sb *schemaBuilder) expand(
	rel metamodel.RelationDef, name, typ, direction string, visited map[string]bool,
) []string {
	var next []string
	discover := func(neighbor string) {
		sb.addNode(neighbor)
		if !visited[neighbor] {
			visited[neighbor] = true
			next = append(next, neighbor)
		}
	}
	// outgoing: typ in From → each To; incoming: typ in To → each From.
	if direction != "in" && slices.Contains(rel.From, typ) {
		for _, to := range rel.To {
			sb.addEdge(typ, name, to)
			discover(to)
		}
	}
	if direction != "out" && slices.Contains(rel.To, typ) {
		for _, fromT := range rel.From {
			sb.addEdge(fromT, name, typ)
			discover(fromT)
		}
	}
	return next
}

func renderGraph(order []string, nodes map[string]mermaid.Node, edges []mermaid.Edge) string {
	ns := make([]mermaid.Node, 0, len(order))
	for _, k := range order {
		ns = append(ns, nodes[k])
	}
	return "```mermaid\n" + mermaid.Graph(ns, edges) + "```\n\n"
}

func nodeLabel(id, title string) string {
	if title != "" {
		return title
	}
	return id
}

func clampDepth(d int) int {
	if d < 1 {
		return graphDefaultDepth
	}
	if d > graphMaxDepth {
		return graphMaxDepth
	}
	return d
}

func toSet(ss []string) map[string]bool {
	if len(ss) == 0 {
		return nil
	}
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

func sortedRelNames(rels map[string]metamodel.RelationDef) []string {
	names := make([]string, 0, len(rels))
	for n := range rels {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// luaResolution renders how ONE world resolves the seeded entities of a type:
// which face each entity answers with, and which are excluded entirely.
//
// # Why a diagram rather than a table
//
// `shows{world=...}` ASSERTS the outcome, which is what keeps the page honest,
// but a reader meeting worlds for the first time needs to see the shape: the
// same set of entities, resolved differently, with some of them absent. The
// assertion proves it; this shows it.
//
// Rendered from the same resolution the store performs, not from a hand-drawn
// figure — so a diagram cannot illustrate behavior the code stopped having.
func (dr *docRuntime) luaResolution(ls *lua.LState) int {
	tbl := argTable(ls)
	typ := fieldString(ls, tbl, "type")
	world := fieldString(ls, tbl, "world")
	if typ == "" {
		return dr.luaFail(ls, "resolution: `type` is required")
	}
	if _, ok := dr.meta.GetEntityDef(typ); !ok {
		return dr.luaFail(ls, "resolution: no such entity type %q (declared: %s)",
			typ, strings.Join(declaredTypes(dr.meta), ", "))
	}
	scope, err := dr.worldScope(world)
	if err != nil {
		return dr.luaFail(ls, "resolution: %v", err)
	}

	ids, err := dr.entityIDs(typ, store.WorldScope{})
	if err != nil {
		return dr.luaFail(ls, "resolution: %v", err)
	}
	// What the world answers with, per entity. An id absent here is one the
	// projection dropped.
	selected := map[string]entity.Face{}
	for e, lerr := range dr.store.ListEntities(dr.ctx, store.EntityQuery{Type: typ, World: scope}) {
		if lerr != nil {
			return dr.luaFail(ls, "resolution: %v", lerr)
		}
		selected[e.ID] = e.Face
	}

	g, err := dr.buildResolutionGraph(typ, world, ids, selected)
	if err != nil {
		return dr.luaFail(ls, "resolution: %v", err)
	}
	dr.emit(mermaidBlock(g))
	return 0
}

// resolutionStyles are the three states a face can be in under a projection.
// Color carries the same information as the label, never information the label
// omits — a reader who cannot distinguish them loses nothing.
var resolutionStyles = map[string]string{
	"world":    "fill:#e8eefc,stroke:#3b5bdb,stroke-width:2px",
	"chosen":   "fill:#dff0d8,stroke:#3c763d,stroke-width:2px",
	"unchosen": "fill:#f5f5f5,stroke:#bbb,color:#777",
	"dropped":  "fill:#fdecea,stroke:#c9302c,stroke-dasharray:4 3",
}

// buildResolutionGraph draws every FACE of every seeded entity of one type,
// marks which face the world selected, and carries the relations between them.
//
// The earlier version drew a star from the world to one node per entity, which
// showed the outcome but hid two things a reader needs: that an entity HAS
// other faces the projection passed over, and that the faces are connected. A
// content-scoped edge belongs to one face, so "which edges come along" is part
// of what a world decides.
func (dr *docRuntime) buildResolutionGraph(
	typ, world string, ids []string, selected map[string]entity.Face,
) (string, error) {
	var nodes []mermaid.Node

	// nodeKey addresses one face of one entity; faceOf records which faces
	// exist so an edge can be drawn only between faces actually present.
	nodeKey := func(id string, f entity.Face) string { return id + "\x00" + f.String() }
	present := map[string]bool{}
	// subject marks nodes that are faces of the type being projected, as
	// opposed to neighbors pulled in by a relation.
	subject := map[string]bool{}

	for _, id := range ids {
		faces, err := dr.facesOf(typ, id)
		if err != nil {
			return "", err
		}
		sel, kept := selected[id]
		for _, f := range faces {
			name := metamodel.DeclaredFace(dr.meta, typ, f.String())
			if name == "" {
				name = "(unnamed)"
			}
			class := "unchosen"
			switch {
			case !kept:
				class = "dropped"
			case f == sel:
				class = "chosen"
			}
			nodes = append(nodes, mermaid.Node{
				Key: nodeKey(id, f), Text: id + " @" + name, Class: class,
			})
			present[nodeKey(id, f)] = true
			subject[nodeKey(id, f)] = true
		}
	}

	relNodes, relEdges, err := dr.resolutionRelations(typ, ids, selected, present, nodeKey)
	if err != nil {
		return "", err
	}
	nodes = append(nodes, relNodes...)
	edges := relEdges

	// Anchor every face to a world node.
	//
	// Without this a projection over entities that happen to have no relations
	// is a set of ISOLATED nodes, which mermaid stacks in one tall column with
	// the boxes marooned in whitespace — the figure that should be easiest to
	// read renders worst. The edge also carries the fact the diagram is about:
	// which face this world chose, and why the others are here.
	label := world
	if label == "" {
		label = "default"
	}
	worldKey := "\x00world"
	anchors := make([]mermaid.Edge, 0, len(nodes))
	for _, n := range nodes {
		// Neighbors reached by a relation are context, not subjects of this
		// projection — anchoring them would claim the world passed over a face
		// of a type it was never asked about.
		if !subject[n.Key] {
			continue
		}
		switch n.Class {
		case "chosen":
			anchors = append(anchors, mermaid.Edge{FromKey: worldKey, ToKey: n.Key, Label: "selects"})
		case "unchosen":
			// Connected too, or it floats: "this face exists and the world
			// passed over it" is half of what the diagram is showing.
			anchors = append(anchors, mermaid.Edge{FromKey: worldKey, ToKey: n.Key, Label: "passes over", Dashed: true})
		case "dropped":
			anchors = append(anchors, mermaid.Edge{FromKey: worldKey, ToKey: n.Key, Label: "no face here", Dashed: true})
		}
	}
	all := append([]mermaid.Node{{Key: worldKey, Text: label + " world", Class: "world"}}, nodes...)
	return mermaid.Graph(all, append(anchors, edges...)) +
		mermaid.ClassDefs(resolutionStyles), nil
}

// facesOf lists the faces one entity actually has, bare-id row first.
func (dr *docRuntime) facesOf(typ, id string) ([]entity.Face, error) {
	var out []entity.Face
	def, ok := dr.meta.GetEntityDef(typ)
	if !ok {
		return nil, fmt.Errorf("no such entity type %q", typ)
	}
	// The bare-id row, then each declared face that exists on this entity.
	if _, err := dr.store.GetEntityState(dr.ctx, id, entity.Face("")); err == nil {
		out = append(out, entity.Face(""))
	}
	for _, name := range sortedFaceNames(def) {
		stored := entity.Face(metamodel.StoredFace(dr.meta, typ, name))
		if stored.IsDefault() {
			continue // already emitted as the bare-id row
		}
		if _, err := dr.store.GetEntityState(dr.ctx, id, stored); err == nil {
			out = append(out, stored)
		}
	}
	return out, nil
}

// mermaidBlock fences a mermaid source string as a Markdown code block, the
// form every renderer keys on. Emitting the source bare renders it as a
// paragraph of graph syntax rather than a diagram.
func mermaidBlock(src string) string {
	return "```mermaid\n" + src + "```\n\n"
}

// resolutionRelations draws the edges between the faces on a resolution
// diagram, pulling in neighbor nodes the projection did not already place.
//
// Split from buildResolutionGraph to keep each half readable: one decides what
// the world did with every face, this one decides what that means for the
// edges between them.
//
// An identity-scoped edge has no tail, so it hangs off the entity's bare-id
// face and is drawn dashed — "shared by every face" without a second color for
// a reader to decode.
func (dr *docRuntime) resolutionRelations(
	typ string, ids []string, selected map[string]entity.Face,
	present map[string]bool, nodeKey func(string, entity.Face) string,
) ([]mermaid.Node, []mermaid.Edge, error) {
	var nodes []mermaid.Node
	var edges []mermaid.Edge
	for _, id := range ids {
		faces, err := dr.facesOf(typ, id)
		if err != nil {
			return nil, nil, err
		}
		for _, f := range faces {
			tail := f
			for r, rerr := range dr.store.ListRelations(dr.ctx, store.RelationQuery{
				From: id, FromFace: &tail,
			}) {
				if rerr != nil {
					return nil, nil, rerr
				}
				// The head is an entity, not a face: draw to whichever face of
				// the target the projection chose, else its bare-id row.
				to := nodeKey(r.To, selected[r.To])
				if !present[to] {
					nodes = append(nodes, mermaid.Node{Key: to, Text: r.To, Class: "unchosen"})
					present[to] = true
				}
				edges = append(edges, mermaid.Edge{
					FromKey: nodeKey(id, f), ToKey: to,
					Label: r.Type, Dashed: tail.IsDefault(),
				})
			}
		}
	}
	return nodes, edges, nil
}
