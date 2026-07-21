package docs

import (
	"fmt"
	"sort"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/mermaid"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

const (
	graphDefaultDepth = 2
	graphMaxDepth     = 5
)

// luaLifecycle emits a mermaid state diagram for a state-machine field, or a
// flat value list when the field's custom type declares no transitions.
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
		return dr.luaFail(ls, "resolve", "lifecycle: %q has no field %q", typ, field)
	}
	ct, named := dr.meta.Types[prop.Type]
	if !named {
		return dr.luaFail(ls, "resolve", "lifecycle: field %q of %q is not a named enum/state machine", field, typ)
	}

	if len(ct.Transitions) == 0 {
		// Flat enum: no state machine — fall back to a value list.
		var b strings.Builder
		for i, v := range ct.Values {
			if i > 0 {
				b.WriteString(" · ")
			}
			fmt.Fprintf(&b, "`%s`", v)
		}
		b.WriteString("\n\n")
		dr.emit(b.String())
		return 0
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
//   - an entity TYPE → the schema neighbourhood (which types connect to which),
//     read from the metamodel.
//   - an entity ID → the seeded instance neighbourhood, traversed via tracer.
//
// graph{from=..., depth=2, direction="out"|"in"|"both", exclude={...}, only={...}}.
func (dr *docRuntime) luaGraph(ls *lua.LState) int {
	tbl := argTable(ls)
	from := fieldString(ls, tbl, "from")
	if from == "" {
		return dr.luaFail(ls, "resolve", "graph: `from` is required")
	}
	depth := clampDepth(fieldInt(ls, tbl, "depth", graphDefaultDepth))
	direction := fieldString(ls, tbl, "direction")
	if direction == "" {
		direction = "out"
	}
	exclude := fieldStringSlice(ls, tbl, "exclude")
	only := fieldStringSlice(ls, tbl, "only")
	if len(exclude) > 0 && len(only) > 0 {
		return dr.luaFail(ls, "resolve", "graph: `exclude` and `only` are mutually exclusive")
	}
	filter := relFilter{exclude: toSet(exclude), only: toSet(only)}

	if _, isType := dr.meta.GetEntityDef(from); isType {
		dr.emit(dr.schemaGraph(from, depth, direction, filter))
		return 0
	}
	// Otherwise treat `from` as an instance id.
	if _, err := dr.store.GetEntity(dr.ctx, from); err != nil {
		return dr.luaFail(ls, "resolve", "graph: %q is neither an entity type nor a seeded id", from)
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
			edgeKept := !(direction == "out" && c.Incoming) && filter.keep(c.Relation)
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

// schemaGraph renders the metamodel neighbourhood of an entity type: a BFS over
// relation definitions to the given depth.
func (dr *docRuntime) schemaGraph(root string, depth int, direction string, filter relFilter) string {
	nodes := map[string]mermaid.Node{}
	var nodeOrder []string
	var edges []mermaid.Edge
	seenEdge := map[string]bool{}
	addNode := func(t string) {
		if _, ok := nodes[t]; !ok {
			nodes[t] = mermaid.Node{Key: t, Text: t}
			nodeOrder = append(nodeOrder, t)
		}
	}
	addNode(root)

	frontier := []string{root}
	visited := map[string]bool{root: true}
	relNames := sortedRelNames(dr.meta.Relations)

	for d := 0; d < depth && len(frontier) > 0; d++ {
		var next []string
		for _, typ := range frontier {
			for _, name := range relNames {
				rel := dr.meta.Relations[name]
				if !filter.keep(name) {
					continue
				}
				// outgoing: typ in From → each To; incoming: typ in To → each From.
				if direction != "in" && containsStr(rel.From, typ) {
					for _, to := range rel.To {
						addNode(to)
						addSchemaEdge(&edges, seenEdge, typ, name, to)
						if !visited[to] {
							visited[to] = true
							next = append(next, to)
						}
					}
				}
				if direction != "out" && containsStr(rel.To, typ) {
					for _, fromT := range rel.From {
						addNode(fromT)
						addSchemaEdge(&edges, seenEdge, fromT, name, typ)
						if !visited[fromT] {
							visited[fromT] = true
							next = append(next, fromT)
						}
					}
				}
			}
		}
		frontier = next
	}
	return renderGraph(nodeOrder, nodes, edges)
}

func addSchemaEdge(edges *[]mermaid.Edge, seen map[string]bool, from, rel, to string) {
	ek := from + "\x00" + rel + "\x00" + to
	if seen[ek] {
		return
	}
	seen[ek] = true
	*edges = append(*edges, mermaid.Edge{FromKey: from, ToKey: to, Label: rel})
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
