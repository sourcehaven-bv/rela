package dataentry

import (
	"context"
	"fmt"
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
	// props are the configured tooltip property values, read from the
	// redacted entity — a hidden value is absent, not blank.
	props map[string]string
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

	rootParam := r.URL.Query().Get("root")

	// Drilled requests try the SQL-scoped subtree first (TKT-5LUGYP): the
	// full forest is built only when the fast path cannot answer with
	// identical ACL semantics.
	var forest *ganttForest
	if rootParam != "" {
		sub, gerr := h.buildGanttSubtree(r.Context(), s, g, rootParam)
		if gerr != nil {
			gerr.write(w, r)
			return
		}
		forest = sub // nil means "fast path declined": the full build answers
	}
	if forest == nil {
		full, gerr := h.buildGanttForest(r.Context(), s, g)
		if gerr != nil {
			gerr.write(w, r)
			return
		}
		forest = full
	}

	rootIDs := forest.roots
	if rootParam != "" {
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
	f, _, gerr := h.finishGanttForest(ctx, g, nodes, "")
	return f, gerr
}

// finishGanttForest runs the shared tail of both build paths: edge linking,
// the multi-parent and cycle policies, and the fold. Shared so the subtree
// fast path cannot drift from the full build's semantics. subtreeRoot is ""
// for the full build; set, it arms the external-parent detection whose true
// return tells the fast path to decline (see buildGanttSubtree).
func (h *ganttHandler) finishGanttForest(
	ctx context.Context, g dataentryconfig.Gantt, nodes map[string]*ganttNode, subtreeRoot string,
) (*ganttForest, bool, *ganttError) {
	parent, multiParent, external, gerr := h.linkGanttParents(ctx, g, nodes, subtreeRoot)
	if gerr != nil {
		return nil, false, gerr
	}
	if external {
		return nil, true, nil
	}
	if g.MultiParent == "error" && len(multiParent) > 0 {
		sort.Strings(multiParent)
		return nil, false, &ganttError{http.StatusUnprocessableEntity, "multi_parent",
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
			return f, false, nil
		}
		var cyclic []string
		for id := range nodes {
			if !f.reachable[id] {
				cyclic = append(cyclic, id)
			}
		}
		sort.Strings(cyclic)
		return nil, false, &ganttError{http.StatusUnprocessableEntity, "containment_cycle",
			"Containment cycle detected",
			"these entities form a containment loop no root can reach: " + strings.Join(cyclic, ", ")}
	}
	return f, false, nil
}

// loadGanttNodes runs pipeline steps 1-2: list each source type through the
// read gate, redact each entity once, then apply the source filters and parse
// the date roles from what survives.
func (h *ganttHandler) loadGanttNodes(
	ctx context.Context, s *Schema, g dataentryconfig.Gantt,
) (map[string]*ganttNode, *ganttError) {
	nodes := map[string]*ganttNode{}
	for _, typeName := range ganttSortedKeys(g.Sources) {
		ents, gerr := h.loadGanttType(ctx, typeName)
		if gerr != nil {
			return nil, gerr
		}
		if gerr := h.addGanttNodes(s, g, typeName, ents, nodes); gerr != nil {
			return nil, gerr
		}
	}
	return nodes, nil
}

// ganttVerdict is the read-gate outcome for one source type, resolved through
// ONE shared helper so the defensive zero-value arm cannot be lost in a copy:
// a zero ReadQueryResult would otherwise alias AllowAll (the exact hazard
// scopedSortedEntities guards against), so it fails loud here, once.
type ganttVerdict int

const (
	ganttDeny ganttVerdict = iota
	ganttAllow
	ganttScoped
)

func ganttReadVerdict(ctx context.Context, typeName string) (ganttVerdict, *ganttError) {
	rqr := readGateFromContext(ctx).ReadQuery(ctx, typeName)
	switch {
	case rqr.DenyAll:
		return ganttDeny, nil
	case rqr.AllowAll:
		return ganttAllow, nil
	case rqr.Query == nil:
		// Defensive: fail loud instead of silently widening (see
		// scopedSortedEntities, which carries the same arm).
		return ganttDeny, &ganttError{http.StatusInternalServerError, "internal",
			"Invalid read verdict", ""}
	default:
		return ganttScoped, nil
	}
}

// loadGanttType returns the REDACTED entities of one source type, honoring
// the per-request read-gate verdict (TKT-5LUGYP):
//
//   - DenyAll: nothing.
//   - AllowAll: the fast path — store.ListEntityHeaders streams
//     ID/Type/Properties WITHOUT markdown bodies (which the gantt never
//     reads; profiling showed body decode dominated the request), redacted
//     via visibility.RedactHeader.
//   - scoped Query verdict: the ACL pushdown has no header variant, so this
//     falls back to the full-entity scoped lister — correct, slower. Falling
//     back rather than approximating keeps the failure direction closed.
//
// Redaction happens HERE, exactly once per entity on either path
// (visibility.Redact/RedactHeader must never see already-redacted input).
func (h *ganttHandler) loadGanttType(
	ctx context.Context, typeName string,
) ([]*entity.Entity, *ganttError) {
	verdict, gerr := ganttReadVerdict(ctx, typeName)
	if gerr != nil {
		return nil, gerr
	}
	switch verdict {
	case ganttDeny:
		return nil, nil
	case ganttAllow:
		var out []*entity.Entity
		for hdr, err := range store.ListEntityHeaders(ctx, h.store, store.EntityQuery{Type: typeName}) {
			if err != nil {
				slog.Error("gantt: header list failed", "type", typeName, "error", err)
				return nil, &ganttError{http.StatusInternalServerError, "internal", "Failed to list entities", ""}
			}
			red := visibility.RedactHeader(ctx, h.redactor(), hdr)
			out = append(out, &entity.Entity{ID: red.ID, Type: red.Type, Properties: red.Properties})
		}
		return out, nil
	default:
		ents, err := h.scoped(ctx, typeName, map[string][]string{})
		if err != nil {
			return nil, &ganttError{http.StatusInternalServerError, "internal", "Failed to list entities", ""}
		}
		out := make([]*entity.Entity, 0, len(ents))
		for _, e := range ents {
			out = append(out, visibility.Redact(ctx, h.redactor(), e))
		}
		return out, nil
	}
}

// addGanttNodes filters ALREADY-REDACTED entities through the source's
// where: clauses and builds their nodes. Filters evaluate post-redact by
// design: membership must not reflect a predicate over a value the principal
// cannot read.
func (h *ganttHandler) addGanttNodes(
	s *Schema, g dataentryconfig.Gantt,
	typeName string, ents []*entity.Entity, nodes map[string]*ganttNode,
) *ganttError {
	src := g.Sources[typeName]
	entDef, ok := s.Meta.GetEntityDef(typeName)
	if !ok {
		return nil // validated at load; a raced schema change degrades, not panics
	}
	tooltipProps := make([]string, 0, len(g.Tooltip.Fields))
	for _, f := range g.Tooltip.Fields {
		if f.Property != "" {
			if _, ok := entDef.Properties[f.Property]; ok {
				tooltipProps = append(tooltipProps, f.Property)
			}
		}
	}
	filters, ferr := filter.ParseAll(src.Where)
	if ferr != nil {
		// Load validation makes this unreachable; refusing (rather than
		// proceeding unfiltered) keeps the failure direction closed.
		return &ganttError{http.StatusInternalServerError, "internal", "Invalid source filter", ""}
	}
	for _, red := range ents {
		if len(filters) > 0 {
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
			props:     ganttProps(red, tooltipProps),
		}
	}
	return nil
}

// ganttClosureRoundDepth is the per-round depth passed to GraphQuery when
// resolving the drilled subtree. Every backend CLAMPS RelationPredicate.Depth
// to 5 (graphquerynaive.DepthCap; pgstore's cappedDepth mirrors it so SQL and
// Go agree), so asking for more is a silent fiction — the closure is instead
// resolved ITERATIVELY: each round expands the frontier by up to this many
// levels, and the loop runs until the descendant set stabilizes. The round
// value therefore only tunes how many queries a deep hierarchy needs, never
// which nodes are found.
const ganttClosureRoundDepth = 5

// ganttClosureMaxRounds is a non-termination backstop for the iterative
// closure (covers hierarchies ~150 levels deep). Hitting it does NOT truncate:
// the fast path declines and the full build answers instead — silently
// returning a smaller, differently-folded tree is the one unacceptable
// failure mode (it would hide exactly the overruns this view exists to show).
const ganttClosureMaxRounds = 25

// ganttSubtreeVerdicts resolves every source type's verdict for the fast
// path. A nil map means the caller must not take it: either a scoped verdict
// appeared (nil error) or verdict resolution itself failed.
func ganttSubtreeVerdicts(
	ctx context.Context, g dataentryconfig.Gantt,
) (map[string]bool, *ganttError) {
	verdicts := map[string]bool{} // type -> AllowAll
	for _, typeName := range ganttSortedKeys(g.Sources) {
		verdict, gerr := ganttReadVerdict(ctx, typeName)
		if gerr != nil {
			return nil, gerr
		}
		switch verdict {
		case ganttDeny:
			verdicts[typeName] = false
		case ganttAllow:
			verdicts[typeName] = true
		default:
			return nil, nil // scoped verdict: full build only
		}
	}
	return verdicts, nil
}

// buildGanttSubtree is the ?root= fast path (TKT-5LUGYP, closes RR-FJWAZS):
// resolve the drilled subtree with per-type GraphQuery pushdown instead of
// building and discarding the global forest.
//
// EQUIVALENCE IS THE CONTRACT: for any input this path answers, the response
// must be byte-identical to the full build's subtree (pinned by
// TestGantt_SubtreeDrillMatchesFullBuild and the generated-graph property
// test). Where equivalence cannot be guaranteed the path DECLINES (returns
// nil forest, no error) and the caller runs the full build:
//
//   - a scoped ACL Query verdict on any source type (cannot compose with the
//     subtree predicate in one query);
//   - multi_parent "error" (a global integrity check — an offender outside
//     the subtree must still fail the request, and the policy is opt-in so
//     the cost of declining is paid only by those who asked for it);
//   - an edge from a parent OUTSIDE the loaded subtree to a node inside it
//     (parent selection would otherwise pick a different winner than the
//     full build — detected during linking and bailed on);
//   - a closure that does not stabilize within ganttClosureMaxRounds.
//
// on_cycle "error" (the DEFAULT) does NOT decline: the cycle diagnostic is
// evaluated over the drilled SUBTREE. A cycle inside it 422s exactly like the
// full build; a cycle through the drilled root bails to the full build (the
// external-edge rule above); a cycle DISJOINT from the subtree is reported by
// the root view, not by this drill — the one deliberate divergence, chosen
// because declining would forfeit the fast path for every default-config
// gantt while the landing view still surfaces the corruption.
//
// A root that is denied, missing, not a source type, or filtered out by its
// where: clause yields an empty forest, which the caller turns into the
// uniform entity 404.
func (h *ganttHandler) buildGanttSubtree(
	ctx context.Context, s *Schema, g dataentryconfig.Gantt, rootID string,
) (*ganttForest, *ganttError) {
	if g.MultiParent == "error" {
		return nil, nil // global integrity policy needs the global build
	}
	verdicts, verdictErr := ganttSubtreeVerdicts(ctx, g)
	if verdicts == nil {
		return nil, verdictErr // scoped verdict (nil,nil) or a real error
	}

	nodes := map[string]*ganttNode{}
	empty := &ganttForest{nodes: nodes, reachable: map[string]bool{}}

	// The root itself: must exist, be a permitted source type, and survive
	// its own where: filter — otherwise it is indistinguishable from absent.
	rootEnt, err := h.store.GetEntity(ctx, rootID)
	if err != nil || rootEnt == nil {
		return empty, nil
	}
	if _, isSource := g.Sources[rootEnt.Type]; !isSource || !verdicts[rootEnt.Type] {
		return empty, nil
	}
	red := visibility.Redact(ctx, h.redactor(), rootEnt)
	if gerr := h.addGanttNodes(s, g, red.Type, []*entity.Entity{red}, nodes); gerr != nil {
		return nil, gerr
	}

	// Iterative descendant closure: query descendants of the frontier, feed
	// newly discovered ids back in, stop when a round finds nothing new.
	frontier := []string{rootID}
	for round := 0; len(frontier) > 0; round++ {
		if round >= ganttClosureMaxRounds {
			return nil, nil // did not stabilize: decline, never truncate
		}
		next, gerr := h.collectGanttRound(ctx, s, g, verdicts, frontier, nodes)
		if gerr != nil {
			return nil, gerr
		}
		frontier = next
	}

	f, external, gerr := h.finishGanttForest(ctx, g, nodes, rootID)
	if gerr != nil {
		return nil, gerr
	}
	if external {
		return nil, nil // out-of-subtree parent edge: full build decides placement
	}
	return f, nil
}

// collectGanttRound runs one closure round: for each permitted source type,
// query descendants of the frontier, redact and add the unseen ones, and
// return their ids as the next frontier.
func (h *ganttHandler) collectGanttRound(
	ctx context.Context, s *Schema, g dataentryconfig.Gantt,
	verdicts map[string]bool, frontier []string, nodes map[string]*ganttNode,
) ([]string, *ganttError) {
	var next []string
	for _, typeName := range ganttSortedKeys(g.Sources) {
		if !verdicts[typeName] {
			continue
		}
		q := store.GraphQuery{
			EntityType: typeName,
			HasInbound: &store.RelationPredicate{
				// Endpoints expand through the hierarchy closure, so the
				// predicate matches descendants of the frontier.
				Endpoints:      frontier,
				OfTypes:        g.Hierarchy,
				InheritThrough: g.Hierarchy,
				Depth:          ganttClosureRoundDepth,
			},
		}
		var fresh []*entity.Entity
		for e, qerr := range h.store.GraphQuery(ctx, q) {
			if qerr != nil {
				slog.Error("gantt: subtree query failed", "type", typeName, "error", qerr)
				return nil, &ganttError{http.StatusInternalServerError, "internal", "Failed to query subtree", ""}
			}
			if _, seen := nodes[e.ID]; seen {
				continue
			}
			fresh = append(fresh, visibility.Redact(ctx, h.redactor(), e))
			next = append(next, e.ID)
		}
		if gerr := h.addGanttNodes(s, g, typeName, fresh, nodes); gerr != nil {
			return nil, gerr
		}
	}
	return next, nil
}

// linkGanttParents loads the hierarchy edges (one bulk query per relation
// type) and picks each node's parent. An edge is used only when both
// endpoints are already in the gated node set; extra parents are collected
// for the multi_parent policy rather than silently dropped.
//
// In subtree mode (subtreeRoot != "") it additionally reports external=true
// when an edge from OUTSIDE the node set claims a node inside it (other than
// the root's own ancestry, which every drill legitimately has), or when an
// in-set edge points back AT the root (a cycle through the root). In either
// shape parent selection could differ from the full build's, so the fast
// path must decline rather than guess.
func (h *ganttHandler) linkGanttParents(
	ctx context.Context, g dataentryconfig.Gantt, nodes map[string]*ganttNode, subtreeRoot string,
) (parent map[string]string, multiParent []string, external bool, gerr *ganttError) {
	parent = map[string]string{}
	for _, relType := range g.Hierarchy {
		edges, ext, gerr := h.ganttEdgesForType(ctx, relType, nodes, subtreeRoot)
		if gerr != nil {
			return nil, nil, false, gerr
		}
		if ext {
			return nil, nil, true, nil
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
	return parent, multiParent, false, nil
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

// ganttEdgesForType streams one relation type's edges, keeping those whose
// endpoints are both in the gated node set. In subtree mode it reports
// external=true for the shapes linkGanttParents' doc describes.
func (h *ganttHandler) ganttEdgesForType(
	ctx context.Context, relType string, nodes map[string]*ganttNode, subtreeRoot string,
) (edges [][2]string, external bool, gerr *ganttError) {
	for rel, err := range h.store.ListRelations(ctx, store.RelationQuery{Type: relType}) {
		if err != nil {
			slog.Error("gantt: relation list failed", "relation", relType, "error", err)
			return nil, false, &ganttError{http.StatusInternalServerError, "internal", "Failed to list relations", ""}
		}
		if rel.From == rel.To {
			continue // self-loop: degenerate cycle, never a tree edge
		}
		if subtreeRoot != "" && nodes[rel.To] != nil {
			if rel.To == subtreeRoot && nodes[rel.From] != nil {
				return nil, true, nil // cycle through the drilled root
			}
			if rel.To != subtreeRoot && nodes[rel.From] == nil {
				return nil, true, nil // out-of-subtree parent claims an in-set node
			}
		}
		if nodes[rel.From] == nil || nodes[rel.To] == nil {
			continue
		}
		edges = append(edges, [2]string{rel.From, rel.To})
	}
	return edges, false, nil
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
	out.Props = n.props
	out.Breach.Before = n.start != nil && n.rolled.start != nil && n.rolled.start.Before(*n.start)
	out.Breach.After = n.end != nil && n.rolled.end != nil && n.rolled.end.After(*n.end)

	if depth >= maxDepth {
		// Children beyond the cap are not emitted, but their dates are
		// already inside this node's rolled span — the fold ran first.
		// HasMoreChildren keeps the withholding visible on the wire: with
		// Children omitempty, a capped node would otherwise be
		// byte-identical to a genuine leaf.
		out.HasMoreChildren = len(n.children) > 0
		return out
	}
	for _, childID := range n.children {
		if !budget.take() {
			out.HasMoreChildren = true
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

// ganttProps reads the configured tooltip properties from the redacted
// entity. Only present values ship: a hidden or unset property is absent from
// the map, so the card never renders a blank row for a value the principal
// may not see.
func ganttProps(e *entity.Entity, props []string) map[string]string {
	var out map[string]string
	for _, prop := range props {
		raw, ok := e.Properties[prop]
		if !ok || raw == nil {
			continue
		}
		var v string
		switch t := raw.(type) {
		case string:
			v = t
		case time.Time:
			v = t.Format("2006-01-02")
		default:
			v = fmt.Sprintf("%v", t)
		}
		if v == "" {
			continue
		}
		if out == nil {
			out = map[string]string{}
		}
		out[prop] = v
	}
	return out
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
