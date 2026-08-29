package dataentry

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
)

// ganttHandler owns the read-only gantt tree endpoint. Its own struct rather
// than App methods because App is at its plimsoll method load line; like
// viewsHandler it never mutates, so it takes no writeMu.
//
// The pipeline order in buildGanttForest is a SECURITY property, not a
// performance choice (TKT-MW28U5, RR-5KEF8E/RR-7PK0YW/RR-Y7MINP):
//
//  1. row-gate    — entities enter through the ACL-scoped lister only
//  2. redact once — visibility.Redact per entity, BEFORE any date is read,
//     so a `visible:`-hidden date can never reach the fold
//  3. fold        — the roll-up runs over gated, redacted nodes only
//  4. cap         — MaxNodes/truncated are computed on the filtered tree
//
// Folding before gating (or reading raw dates) would launder a hidden
// child's dates into a visible parent's rolled span — a value disclosure,
// worse than the one-bit membership channel the _views pipeline accepts.
// Consequently the roll-up is per-principal and must never be cached across
// principals.
type ganttHandler struct {
	schema func() *Schema
	// store lists hierarchy edges in bulk (one query per relation type).
	// Edge visibility is NOT re-derived here: an edge is used only when BOTH
	// endpoints are already in the gated node set, which is strictly narrower
	// than PolicyReader.FilterRelations' both-endpoints rule and costs no
	// extra reads. Edge properties never reach the response — relation meta
	// carries no redaction on this path (TKT-0RBFN0).
	store store.Store
	// scoped is App.scopedSortedEntities: the ACL-scoped entity lister.
	// Going through it (not the raw store) is what makes step 1 structural.
	scoped func(ctx context.Context, typeName string, query map[string][]string) ([]*entity.Entity, error)
	// redactor is the field-redaction seam (appRedactor); a closure so test
	// builders that rebind app.affordances keep it live.
	redactor func() visibility.FieldRedactor
}

// ganttNode is one entity in the build, carrying parsed dates and tree links.
type ganttNode struct {
	id        string
	entType   string
	title     string
	color     string
	start     *time.Time // own declared window (planned)
	end       *time.Time
	committed *time.Time
	rolled    ganttSpan // envelope of descendants, filled by the fold
	children  []string
}

// ganttSpan is a nullable date interval.
type ganttSpan struct {
	start *time.Time
	end   *time.Time
}

// union widens s to include o.
func (s *ganttSpan) union(o ganttSpan) {
	if o.start != nil && (s.start == nil || o.start.Before(*s.start)) {
		s.start = o.start
	}
	if o.end != nil && (s.end == nil || o.end.After(*s.end)) {
		s.end = o.end
	}
}

// handleV1Gantt serves GET /api/v1/_gantts/{id}[?root=<entityID>].
//
// {id} is a CONFIG key, so an unknown one gets a plain named 404 (config is
// not a secret). ?root= is an ENTITY id, so a hidden, missing, or
// out-of-scope root all get the uniform entity 404 — indistinguishable by
// contract.
func (h *ganttHandler) handleV1Gantt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/_gantts/")
	s := h.schema()
	g, ok := s.Cfg.Gantts[id]
	if !ok || strings.Contains(id, "/") {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Gantt not found", "")
		return
	}

	forest, err := h.buildGanttForest(r.Context(), s, g)
	if err != nil {
		err.write(w, r)
		return
	}

	rootIDs := forest.roots
	if rootParam := r.URL.Query().Get("root"); rootParam != "" {
		// Drill-down re-root. Reachable-and-visible is the only way in; a
		// hidden root was never in the forest, so it falls into the same
		// uniform 404 as a genuinely absent id.
		if _, ok := forest.nodes[rootParam]; !ok || !forest.reachable[rootParam] {
			writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
			return
		}
		rootIDs = []string{rootParam}
	}

	resp := v1.GanttResponse{Roots: []v1.GanttNode{}}
	budget := &ganttBudget{remaining: g.MaxNodes}
	for _, rootID := range rootIDs {
		if !budget.take() {
			break
		}
		resp.Roots = append(resp.Roots, emitGanttNode(forest, rootID, 0, g.MaxDepth, budget))
	}
	resp.Truncated = budget.truncated
	writeV1JSON(w, http.StatusOK, resp)
}

// ganttForest is the gated, folded tree set.
type ganttForest struct {
	nodes     map[string]*ganttNode
	roots     []string
	reachable map[string]bool
}

// ganttError pairs an HTTP shape with a build failure.
type ganttError struct {
	status         int
	errType, title string
	detail         string
}

func (e *ganttError) write(w http.ResponseWriter, r *http.Request) {
	writeV1Error(w, r, e.status, e.errType, e.title, e.detail)
}

// buildGanttForest runs pipeline steps 1-3 (gate, redact, fold) plus the
// multi-parent and cycle policies. See the type doc for why the order is
// load-bearing.
func (h *ganttHandler) buildGanttForest(
	ctx context.Context, s *Schema, g dataentryconfig.Gantt,
) (*ganttForest, *ganttError) {
	nodes, gerr := h.loadGanttNodes(ctx, s, g)
	if gerr != nil {
		return nil, gerr
	}

	parent, multiParent, gerr := h.linkGanttParents(ctx, g, nodes)
	if gerr != nil {
		return nil, gerr
	}
	if g.MultiParent == "error" && len(multiParent) > 0 {
		sort.Strings(multiParent)
		return nil, &ganttError{http.StatusUnprocessableEntity, "multi_parent",
			"Entity has multiple parents",
			"multi_parent is \"error\" and these entities are contained by more than one parent: " +
				strings.Join(dedupSorted(multiParent), ", ")}
	}

	f := &ganttForest{nodes: nodes, reachable: map[string]bool{}}
	for id := range nodes {
		if _, hasParent := parent[id]; !hasParent {
			f.roots = append(f.roots, id)
		}
	}
	sort.Strings(f.roots)

	// Step 3: fold, post-order from the roots. Under multi_parent:first every
	// node has at most one parent, so the reachable set is a forest and the
	// walk cannot revisit a node. What it CANNOT reach is a containment loop
	// — a component whose every member's parent stays inside the loop — which
	// is exactly the cycle case. Detection is therefore "any gated node left
	// unreached", and it runs on the VISIBLE subgraph only: a loop through
	// hidden nodes must not be reportable, or on_cycle:error becomes a
	// one-bit oracle on hidden topology.
	for _, rootID := range f.roots {
		foldGantt(f, rootID)
	}
	if len(f.reachable) < len(nodes) {
		if g.OnCycle == "prune" {
			// The loop members simply do not render; nothing visible is lost
			// (no root can reach them) and nothing hidden is disclosed.
			return f, nil
		}
		var cyclic []string
		for id := range nodes {
			if !f.reachable[id] {
				cyclic = append(cyclic, id)
			}
		}
		sort.Strings(cyclic)
		return nil, &ganttError{http.StatusUnprocessableEntity, "containment_cycle",
			"Containment cycle detected",
			"these entities form a containment loop no root can reach: " + strings.Join(cyclic, ", ")}
	}
	return f, nil
}

// loadGanttNodes runs pipeline steps 1-2: list each source type through the
// ACL-scoped lister, redact each entity once, then apply the source filters
// and parse the date roles from what survives.
func (h *ganttHandler) loadGanttNodes(
	ctx context.Context, s *Schema, g dataentryconfig.Gantt,
) (map[string]*ganttNode, *ganttError) {
	nodes := map[string]*ganttNode{}

	for _, typeName := range ganttSortedKeys(g.Sources) {
		src := g.Sources[typeName]
		entDef, ok := s.Meta.GetEntityDef(typeName)
		if !ok {
			continue // validated at load; a raced schema change degrades, not panics
		}

		// Step 1: row-gate. Only the ACL-scoped lister, never the raw store.
		ents, err := h.scoped(ctx, typeName, map[string][]string{})
		if err != nil {
			return nil, &ganttError{http.StatusInternalServerError, "internal", "Failed to list entities", ""}
		}

		filters, ferr := filter.ParseAll(src.Where)
		if ferr != nil {
			// Load validation makes this unreachable; refusing (rather than
			// proceeding unfiltered) keeps the failure direction closed.
			return nil, &ganttError{http.StatusInternalServerError, "internal", "Invalid source filter", ""}
		}

		for _, e := range ents {
			// Step 2: redact ONCE, on the raw store entity, before any
			// property is read. Everything below — where-matching, date
			// parsing, the title — sees only what the principal may see.
			// (visibility.Redact must never run on an already-redacted
			// entity, so this is the single redaction point on this path.)
			red := visibility.Redact(ctx, h.redactor(), e)

			if len(filters) > 0 {
				// Evaluated post-redact by design: membership must not
				// reflect a predicate over a value the principal cannot
				// read. A where: on a hidden property therefore matches
				// nothing for that principal.
				matched, merr := filter.MatchAll(entityRecord(red), filters, entDef, s.Meta)
				if merr != nil {
					// A match ERROR (an unparseable stored value, not a
					// non-match) excludes the entity — for a filter that is
					// the closed direction — but never silently: exclusion
					// also narrows every ancestor's rolled span, which is
					// exactly the "looks on schedule" failure this view
					// exists to prevent, so the drop is logged.
					slog.Warn("gantt: where filter errored; entity excluded",
						"entity", red.ID, "error", merr)
					continue
				}
				if !matched {
					continue
				}
			}

			nodes[red.ID] = &ganttNode{
				id:        red.ID,
				entType:   typeName,
				title:     ganttTitle(red, src, entDef),
				color:     src.Color,
				start:     ganttDate(red, src.Start, entDef),
				end:       ganttDate(red, src.End, entDef),
				committed: ganttDate(red, src.Committed, entDef),
			}
		}
	}
	return nodes, nil
}

// linkGanttParents loads the hierarchy edges (one bulk query per relation
// type) and picks each node's parent. An edge is used only when both
// endpoints are already in the gated node set; extra parents are collected
// for the multi_parent policy rather than silently dropped.
func (h *ganttHandler) linkGanttParents(
	ctx context.Context, g dataentryconfig.Gantt, nodes map[string]*ganttNode,
) (parent map[string]string, multiParent []string, gerr *ganttError) {
	parent = map[string]string{}
	for _, relType := range g.Hierarchy {
		var edges [][2]string
		for rel, err := range h.store.ListRelations(ctx, store.RelationQuery{Type: relType}) {
			if err != nil {
				return nil, nil, &ganttError{http.StatusInternalServerError, "internal", "Failed to list relations", ""}
			}
			if rel.From == rel.To {
				continue // self-loop: degenerate cycle, never a tree edge
			}
			if nodes[rel.From] == nil || nodes[rel.To] == nil {
				continue
			}
			edges = append(edges, [2]string{rel.From, rel.To})
		}
		// Deterministic parent choice under multi_parent:first — sort within
		// the relation type, and relation types apply in config order.
		sort.Slice(edges, func(i, j int) bool {
			if edges[i][0] != edges[j][0] {
				return edges[i][0] < edges[j][0]
			}
			return edges[i][1] < edges[j][1]
		})
		for _, e := range edges {
			if prev, taken := parent[e[1]]; taken {
				if prev != e[0] {
					multiParent = append(multiParent, e[1])
				}
				continue
			}
			parent[e[1]] = e[0]
			// Guard adjacent to the deref, not 20 lines up: the edge loop
			// above filters unknown endpoints today, but this must not
			// become a panic if a future edge source skips that filter.
			if p := nodes[e[0]]; p != nil {
				p.children = append(p.children, e[1])
			}
		}
	}
	return parent, multiParent, nil
}

// ganttBudget is the node-cap accounting shared by every emission site, so
// the truncation rule exists ONCE: `truncated` becomes true exactly when a
// visible node was denied emission — never merely because the budget landed
// on zero after the last node (that would leak how close a principal's
// visible tree sits to the cap).
type ganttBudget struct {
	remaining int
	truncated bool
}

// take consumes one node's worth. False means the node was cut, and the flag
// is recorded at that moment — the caller just stops.
func (b *ganttBudget) take() bool {
	if b.remaining <= 0 {
		b.truncated = true
		return false
	}
	b.remaining--
	return true
}

// foldGantt computes every node's rolled span, post-order, and marks
// reachability. Iterative on an explicit stack: recursion depth here is
// DATA-controlled (a chain of containment edges), and a deep-enough chain
// would overflow the goroutine stack — which kills the process, not the
// request. The emit walk is depth-capped so it has no such exposure.
func foldGantt(f *ganttForest, rootID string) {
	type frame struct {
		id   string
		next int // index into children; len(children) means "fold and pop"
	}
	stack := []frame{{id: rootID}}
	f.reachable[rootID] = true
	sort.Strings(f.nodes[rootID].children)

	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		n := f.nodes[top.id]
		if top.next < len(n.children) {
			childID := n.children[top.next]
			top.next++
			f.reachable[childID] = true
			sort.Strings(f.nodes[childID].children)
			stack = append(stack, frame{id: childID})
			continue
		}
		// Post-order: children folded; contribute upward the union of what
		// this node declares and what rolled into it.
		stack = stack[:len(stack)-1]
		if len(stack) > 0 {
			parent := f.nodes[stack[len(stack)-1].id]
			contrib := ganttSpan{start: n.start, end: n.end}
			contrib.union(n.rolled)
			parent.rolled.union(contrib)
		}
	}
}

// emitGanttNode converts the folded build into wire nodes, applying the depth
// cap and the shared node budget. Depth is relative to the RESPONSE root —
// the flame-graph semantic: a drilled request re-roots the walk, so repeated
// drilling reaches arbitrary depth while each response stays bounded (see the
// MaxDepth field doc). Both caps run on the already-gated tree, so truncation
// can only ever reflect visible nodes.
func emitGanttNode(f *ganttForest, id string, depth, maxDepth int, budget *ganttBudget) v1.GanttNode {
	n := f.nodes[id]
	out := v1.GanttNode{
		ID:    n.id,
		Type:  n.entType,
		Title: n.title,
		Color: n.color,
	}
	if n.start != nil || n.end != nil {
		out.Planned = &v1.GanttSpan{Start: ganttDateString(n.start), End: ganttDateString(n.end)}
	}
	if n.rolled.start != nil || n.rolled.end != nil {
		out.Rolled = &v1.GanttSpan{Start: ganttDateString(n.rolled.start), End: ganttDateString(n.rolled.end)}
	}
	out.Committed = ganttDateString(n.committed)
	out.Breach.Before = n.start != nil && n.rolled.start != nil && n.rolled.start.Before(*n.start)
	out.Breach.After = n.end != nil && n.rolled.end != nil && n.rolled.end.After(*n.end)

	if depth >= maxDepth {
		// Children beyond the cap are not emitted, but their dates are
		// already inside this node's rolled span — the fold ran first.
		return out
	}
	for _, childID := range n.children {
		if !budget.take() {
			break
		}
		out.Children = append(out.Children, emitGanttNode(f, childID, depth+1, maxDepth, budget))
	}
	return out
}

// ganttTitle resolves the bar label from the redacted entity: the source's
// label property, else the type's display property. A redacted-away value
// yields "", and the SPA falls back to the id.
func ganttTitle(e *entity.Entity, src dataentryconfig.GanttSource, entDef *metamodel.EntityDef) string {
	prop := src.Label
	if prop == "" {
		prop = entDef.GetPrimaryProperty()
	}
	if prop == "" {
		return ""
	}
	return e.GetString(prop)
}

// ganttDate reads one date role from the redacted entity. Absent, redacted,
// and unparseable values all yield nil: bad data degrades a bar, it never
// fails the request. Reads through entityTimeValue, NOT GetString — YAML
// frontmatter yields a time.Time for an unquoted date, and GetString drops
// those silently (the hazard entityTimeValue's doc documents).
func ganttDate(e *entity.Entity, prop string, entDef *metamodel.EntityDef) *time.Time {
	if prop == "" {
		return nil
	}
	def, ok := entDef.Properties[prop]
	if !ok {
		return nil
	}
	t, ok := entityTimeValue(e, prop, &def)
	if !ok {
		return nil
	}
	return &t
}

// ganttDateString renders a date for the wire, date-granular by design: the
// view compares dates, never instants.
func ganttDateString(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

// ganttSortedKeys returns m's keys sorted, for deterministic iteration.
func ganttSortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// dedupSorted removes adjacent duplicates from a sorted slice.
func dedupSorted(s []string) []string {
	out := s[:0]
	for i, v := range s {
		if i == 0 || s[i-1] != v {
			out = append(out, v)
		}
	}
	return out
}
