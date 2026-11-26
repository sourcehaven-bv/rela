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
// viewsHandler it never mutates.
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
	// scoped is listPipeline.scopedSortedEntities: the ACL-scoped entity lister.
	// Going through it (not the raw store) is what makes step 1 structural.
	scoped func(ctx context.Context, typeName string, query map[string][]string) ([]*entity.Entity, error)

	// scanTypeHeaders is the body-free read of every readable row of a type the
	// AllowAll path uses: the gantt never reads markdown, and profiling
	// showed body decode dominated the request. Injected rather than called
	// directly so the handler keeps its narrow collaborator set.
	scanTypeHeaders func(ctx context.Context, typeName string) ([]store.EntityHeader, error)
	// scopedHeadersByIDs is scanTypeHeaders narrowed to the named ids, so a
	// caller that already knows its candidates (sankey pips) pays for those
	// and not for the whole type. Every verdict still applies to each id.
	scopedHeadersByIDs func(ctx context.Context, typeName string, ids []string) ([]store.EntityHeader, bool, error)
	// redactor is the field-redaction seam (appRedactor); a closure so test
	// builders that rebind app.affordances keep it live.
	redactor func() visibility.FieldRedactor
	// fields is the field-verdict resolver the reader's own `filter[...]`
	// params are checked against, as on a list. A closure for the same
	// reason as redactor. Nil: accepted. Only a request that carries filter
	// params reads it, and a handler built without it (the edge tests)
	// receives none.
	fields func() fieldVisibility
}

// ganttNode is one entity in the build, carrying parsed dates and tree links.
type ganttNode struct {
	id      string
	entType string
	// face is the face the node's row was loaded at; it decides which
	// content-scoped edges the node owns.
	face      entity.Face
	title     string
	color     string
	start     *time.Time // own declared window (planned)
	end       *time.Time
	committed *time.Time
	rolled    ganttSpan // envelope of descendants, filled by the fold
	children  []string
	// cutChildren are children dropped from THIS node's descent because the
	// edge closed a loop (on_cycle:"mark" only). Kept so emit can report the
	// node as having withheld children rather than looking like a leaf — the
	// same honesty HasMoreChildren provides for the depth cap.
	cutChildren []string
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
// out-of-scope root all get the uniform entity 404. Drilled requests without
// a reader's own search or filters try the SQL-scoped subtree first.
//
//archplan:auth=required
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
	uq := parseGanttUserQuery(r.URL.Query(), rootParam)
	if err := uq.validate(s.Meta, g); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_filter", "Invalid filter parameter", err.Error())
		return
	}

	var forest *ganttForest
	if rootParam != "" && uq.empty() {
		sub, gerr := h.buildGanttSubtree(r.Context(), s, g, rootParam)
		if gerr != nil {
			gerr.write(w, r)
			return
		}
		forest = sub
	}
	if forest == nil {
		full, gerr := h.buildGanttForest(r.Context(), s, g, uq)
		if gerr != nil {
			gerr.write(w, r)
			return
		}
		forest = full
	}

	rootIDs := forest.roots
	if rootParam != "" {
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
	// cyclic holds the nodes a containment loop closes back onto, under
	// on_cycle:"mark" only — "error" and "prune" leave it empty. Two writers,
	// because a loop is cut at whichever stage first has the information:
	// reseatCycleParents when a member had a root-reachable alternative
	// parent, foldGantt's back-edge check when none did. Membership is
	// derived from the gated node set exactly like reachable, so it never
	// reflects a loop running through hidden entities.
	cyclic map[string]bool
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
	ctx context.Context, s *Schema, g dataentryconfig.Gantt, uq ganttUserQuery,
) (*ganttForest, *ganttError) {
	nodes, gerr := h.loadGanttNodes(ctx, s, g, uq)
	if gerr != nil {
		return nil, gerr
	}
	edges, gerr := h.ganttEdges(ctx, s.Meta, g, nodes)
	if gerr != nil {
		return nil, gerr
	}
	return finishGanttForest(g, nodes, edges, nil)
}

// finishGanttForest runs the shared tail of both build paths: parent
// selection, the multi-parent and cycle policies, and the fold. Shared so the
// subtree fast path cannot drift from the full build's semantics.
// edges holds the admissible hierarchy edges per relation type; nil
// checkMultiParent checks every node. Cycle detection runs on the VISIBLE
// subgraph only: a loop through hidden nodes must not be reportable, or
// on_cycle:error becomes a one-bit oracle on hidden topology.
func finishGanttForest(
	g dataentryconfig.Gantt, nodes map[string]*ganttNode, edges map[string][][2]string,
	checkMultiParent map[string]bool,
) (*ganttForest, *ganttError) {
	parent, allParents, multiParent := linkGanttParents(g, nodes, edges)
	if checkMultiParent != nil {
		kept := multiParent[:0]
		for _, id := range multiParent {
			if checkMultiParent[id] {
				kept = append(kept, id)
			}
		}
		multiParent = kept
	}
	if g.MultiParent == "error" && len(multiParent) > 0 {
		sort.Strings(multiParent)
		return nil, &ganttError{http.StatusUnprocessableEntity, "multi_parent",
			"Entity has multiple parents",
			"multi_parent is \"error\" and these entities are contained by more than one parent: " +
				strings.Join(dedupSorted(multiParent), ", ")}
	}

	reseated := map[string]bool{}
	if g.OnCycle == "mark" {
		reseated = reseatCycleParents(nodes, parent, allParents)
	}

	f := &ganttForest{nodes: nodes, reachable: map[string]bool{}, cyclic: reseated}
	for id := range nodes {
		if _, hasParent := parent[id]; !hasParent {
			f.roots = append(f.roots, id)
		}
	}
	sort.Strings(f.roots)

	markCycles := g.OnCycle == "mark"
	for _, rootID := range f.roots {
		foldGantt(f, rootID, markCycles)
	}
	if len(f.reachable) == len(nodes) {
		return f, nil
	}

	if markCycles {
		for len(f.reachable) < len(nodes) {
			var entry string
			for _, id := range ganttSortedKeys(nodes) {
				if !f.reachable[id] {
					entry = id
					break
				}
			}
			f.roots = append(f.roots, entry)
			foldGantt(f, entry, true)
		}
		sort.Strings(f.roots)
		return f, nil
	}

	if g.OnCycle == "prune" {
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

// loadGanttNodes runs pipeline steps 1-2: list each source type through the
// read gate, redact each entity once, then apply the source filters and parse
// the date roles from what survives.
//
// The reader's own search and filters (uq) narrow the redacted set here,
// BEFORE the nodes exist, so a dropped entity is in no parent link and no
// roll-up: the fold runs over what matched, as it does over what a where:
// clause kept.
func (h *ganttHandler) loadGanttNodes(
	ctx context.Context, s *Schema, g dataentryconfig.Gantt, uq ganttUserQuery,
) (map[string]*ganttNode, *ganttError) {
	nodes := map[string]*ganttNode{}
	for _, typeName := range ganttSortedKeys(g.Sources) {
		ents, gerr := h.loadGanttType(ctx, typeName)
		if gerr != nil {
			return nil, gerr
		}
		ents, gerr = h.applyGanttUserQuery(ctx, s, typeName, ents, uq)
		if gerr != nil {
			return nil, gerr
		}
		if gerr := h.addGanttNodes(requestReadTime(ctx), s, g, typeName, ents, nodes); gerr != nil {
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

// newGanttHandler builds the gantt handler from an App.
//
// A constructor rather than a struct literal at each wiring site: the test
// helper used to restate the literal, so a collaborator added in production
// left tests constructing a handler with a nil field that segfaulted on the
// first request instead of failing at construction.
func newGanttHandler(app *App, st store.Store) *ganttHandler {
	return &ganttHandler{
		schema: app.State,
		store:  st,
		scoped: app.lists.scopedSortedEntities,
		scanTypeHeaders: func(ctx context.Context, typeName string) ([]store.EntityHeader, error) {
			rqr := readGateFromContext(ctx).ReadQuery(ctx, typeName)
			req := scopeRequest{Type: typeName, Faces: rqr.Faces}
			q, errClass, withheld, err := scopedQuery(ctx, rqr, req)
			if withheld || err != nil {
				return nil, err
			}
			return scanHeaders(ctx, app.Services(), q, errClass, req)
		},
		scopedHeadersByIDs: func(
			ctx context.Context, typeName string, ids []string,
		) ([]store.EntityHeader, bool, error) {
			rqr := readGateFromContext(ctx).ReadQuery(ctx, typeName)
			return scopedHeaders(ctx, app.Services(), rqr,
				scopeRequest{Type: typeName, Faces: rqr.Faces, ByIDs: true, IDs: ids})
		},
		redactor: func() visibility.FieldRedactor { return appRedactor(app) },
		fields:   func() fieldVisibility { return app.fieldResolver },
	}
}

func ganttReadVerdict(ctx context.Context, typeName string) (ganttVerdict, *ganttError) {
	rqr := readGateFromContext(ctx).ReadQuery(ctx, typeName)
	switch {
	case rqr.DenyAll:
		return ganttDeny, nil
	case rqr.AllowAll:
		return ganttAllow, nil
	case rqr.Query == nil:
		return ganttDeny, &ganttError{http.StatusInternalServerError, "internal",
			"Invalid read verdict", ""}
	default:
		return ganttScoped, nil
	}
}

// loadGanttType returns the REDACTED entities of one source type, honoring
// the per-request read-gate verdict: DenyAll yields nothing, AllowAll
// streams content-free headers, and a scoped Query verdict reads through the
// ACL-narrowing funnel. Redaction happens HERE, exactly once per entity on
// either path, because visibility.Redact is non-composable.
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
		hdrs, err := h.scanTypeHeaders(ctx, typeName)
		if err != nil {
			slog.Error("gantt: header list failed", "type", typeName, "error", err)
			return nil, &ganttError{http.StatusInternalServerError, "internal", "Failed to list entities", ""}
		}
		out := make([]*entity.Entity, 0, len(hdrs))
		for _, hdr := range hdrs {
			red := visibility.RedactHeader(ctx, h.redactor(), hdr)
			out = append(out, &entity.Entity{ID: red.ID, Type: red.Type, Face: red.Face, Properties: red.Properties})
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

// loadGanttTypeByIDs is loadGanttType for the named ids only: the same verdict,
// world and face narrowing, redacted once, at a cost that follows len(ids)
// and not the size of the type. An id the principal may not read is simply
// absent from the result.
func (h *ganttHandler) loadGanttTypeByIDs(
	ctx context.Context, typeName string, ids []string,
) ([]*entity.Entity, *ganttError) {
	if len(ids) == 0 {
		return nil, nil
	}
	if _, gerr := ganttReadVerdict(ctx, typeName); gerr != nil {
		return nil, gerr
	}
	hdrs, _, err := h.scopedHeadersByIDs(ctx, typeName, ids)
	if err != nil {
		slog.Error("gantt: header list by ids failed", "type", typeName, "error", err)
		return nil, &ganttError{http.StatusInternalServerError, "internal", "Failed to list entities", ""}
	}
	out := make([]*entity.Entity, 0, len(hdrs))
	for _, hdr := range hdrs {
		red := visibility.RedactHeader(ctx, h.redactor(), hdr)
		out = append(out, &entity.Entity{ID: red.ID, Type: red.Type, Face: red.Face, Properties: red.Properties})
	}
	return out, nil
}

// addGanttNodes filters ALREADY-REDACTED entities through the source's
// where: clauses and builds their nodes. Filters evaluate post-redact by
// design: membership must not reflect a predicate over a value the principal
// cannot read.
func (h *ganttHandler) addGanttNodes(
	now time.Time, s *Schema, g dataentryconfig.Gantt,
	typeName string, ents []*entity.Entity, nodes map[string]*ganttNode,
) *ganttError {
	src := g.Sources[typeName]
	entDef, ok := s.Meta.GetEntityDef(typeName)
	if !ok {
		return nil
	}
	tooltipProps := make([]string, 0, len(g.Tooltip.Fields))
	for _, f := range g.Tooltip.Fields {
		if f.Property != "" {
			if _, ok := entDef.Properties[f.Property]; ok {
				tooltipProps = append(tooltipProps, f.Property)
			}
		}
	}
	filters, ferr := parseWhereAt(src.Where, now)
	if ferr != nil {
		return &ganttError{http.StatusInternalServerError, "internal", "Invalid source filter", ""}
	}
	for _, red := range ents {
		if len(filters) > 0 {
			matched, merr := filter.MatchAll(entityRecord(red), filters, entDef, s.Meta)
			if merr != nil {
				slog.Warn("gantt: where filter errored; entity excluded",
					"entity", red.ID, "error", merr)
				continue
			}
			if !matched {
				continue
			}
		}
		if prev, dup := nodes[red.ID]; dup && prev.face != red.Face {
			slog.Error("gantt: two faces of one entity in the node set",
				"entity", red.ID, "faces", []entity.Face{prev.face, red.Face})
			return &ganttError{http.StatusInternalServerError, "internal", "Ambiguous entity face", ""}
		}
		nodes[red.ID] = &ganttNode{
			id:        red.ID,
			entType:   typeName,
			face:      red.Face,
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
	verdicts := map[string]bool{}
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
			return nil, nil
		}
	}
	return verdicts, nil
}

// ganttHasFacedSource reports whether any source type declares faces. The
// drill closure (collectGanttRound) reads in the request's world under the
// grant's face ceiling, as the full build does, but the drill's ROOT is read
// by id with no face, and a faced type has no such row: the drill would 404
// a root the full build serves. It declines until that read names an
// address (TKT-KQXVF7 entity flip).
func ganttHasFacedSource(meta *metamodel.Metamodel, g dataentryconfig.Gantt) bool {
	for typeName := range g.Sources {
		if def, ok := meta.Entities[typeName]; ok && len(def.Faces) > 0 {
			return true
		}
	}
	return false
}

// buildGanttSubtree is the ?root= fast path: it resolves the drilled subtree
// with per-type GraphQuery pushdown instead of building the global forest.
// Its response must be byte-identical to the full build's subtree; outside
// parents are fetched and gated like full-build nodes. Where equivalence
// cannot be guaranteed (scoped ACL verdict, a cycle through the root,
// on_cycle "mark" with an outside parent, an unstable closure) it returns a
// nil forest and no error, and the caller runs the full build. A root that
// is denied, missing, not a source type, or filtered out by its where:
// clause yields an empty forest, which the caller turns into the uniform 404.
func (h *ganttHandler) buildGanttSubtree(
	ctx context.Context, s *Schema, g dataentryconfig.Gantt, rootID string,
) (*ganttForest, *ganttError) {
	if ganttHasFacedSource(s.Meta, g) {
		return nil, nil
	}
	if worldFromContext(ctx).blocksAllReads() {
		return nil, nil
	}
	verdicts, verdictErr := ganttSubtreeVerdicts(ctx, g)
	if verdicts == nil {
		return nil, verdictErr
	}

	nodes := map[string]*ganttNode{}
	empty := &ganttForest{nodes: nodes, reachable: map[string]bool{}}

	rootEnt, err := h.store.GetEntity(ctx, entity.Ref{ID: rootID})
	if err != nil || rootEnt == nil {
		return empty, nil
	}
	if _, isSource := g.Sources[rootEnt.Type]; !isSource || !verdicts[rootEnt.Type] {
		return empty, nil
	}
	red := visibility.Redact(ctx, h.redactor(), rootEnt)
	if gerr := h.addGanttNodes(requestReadTime(ctx), s, g, red.Type, []*entity.Entity{red}, nodes); gerr != nil {
		return nil, gerr
	}
	if nodes[rootID] == nil {
		return empty, nil
	}

	rejected := map[string]bool{}
	frontier := []string{rootID}
	for round := 0; len(frontier) > 0; round++ {
		if round >= ganttClosureMaxRounds {
			return nil, nil
		}
		next, gerr := h.collectGanttRound(ctx, s, g, verdicts, frontier, nodes, rejected)
		if gerr != nil {
			return nil, gerr
		}
		frontier = next
	}

	edges, subtree, gerr := h.subtreeGanttEdges(ctx, s, g, verdicts, nodes, rejected, rootID)
	if gerr != nil || edges == nil {
		return nil, gerr
	}
	return finishGanttForest(g, nodes, edges, subtree)
}

// collectGanttRound runs one closure round: for each permitted source type,
// query descendants of the frontier, redact and add the unseen ones, and
// return their ids as the next frontier.
func (h *ganttHandler) collectGanttRound(
	ctx context.Context, s *Schema, g dataentryconfig.Gantt,
	verdicts map[string]bool, frontier []string, nodes map[string]*ganttNode, rejected map[string]bool,
) ([]string, *ganttError) {
	var next []string
	for _, typeName := range ganttSortedKeys(g.Sources) {
		if !verdicts[typeName] {
			continue
		}
		q := store.GraphQuery{
			EntityType: typeName,
			Related: []store.DirectedRelation{{Incoming: true, Pred: store.RelationPredicate{
				Endpoints:      frontier,
				OfTypes:        g.Hierarchy,
				InheritThrough: g.Hierarchy,
				Depth:          ganttClosureRoundDepth,
			}}},
		}
		rqr := readGateFromContext(ctx).ReadQuery(ctx, typeName)
		q = stampScope(ctx, q, scopeRequest{Type: typeName, Faces: rqr.Faces})
		var fresh []*entity.Entity
		for hdr, qerr := range store.GraphQueryHeaders(ctx, h.store, q) {
			if qerr != nil {
				slog.Error("gantt: subtree query failed", "type", typeName, "error", qerr)
				return nil, &ganttError{http.StatusInternalServerError, "internal", "Failed to query subtree", ""}
			}
			if _, seen := nodes[hdr.ID]; seen || rejected[hdr.ID] {
				continue
			}
			red := visibility.RedactHeader(ctx, h.redactor(), hdr)
			fresh = append(fresh, &entity.Entity{
				ID: red.ID, Type: red.Type, Face: red.Face, Properties: red.Properties,
			})
			next = append(next, hdr.ID)
		}
		if gerr := h.addGanttNodes(requestReadTime(ctx), s, g, typeName, fresh, nodes); gerr != nil {
			return nil, gerr
		}
		for _, e := range fresh {
			if nodes[e.ID] == nil {
				rejected[e.ID] = true
			}
		}
	}
	return next, nil
}

// subtreeGanttEdges loads the incoming hierarchy edges of every loaded node,
// one query per relation type. Outside parents are read and gated exactly as
// the full build gates its nodes, so a hidden parent can neither win a node
// nor count toward multi_parent. It returns the admissible edges and the
// drilled subtree. Nil edges with a nil error means the fast path declines.
func (h *ganttHandler) subtreeGanttEdges(
	ctx context.Context, s *Schema, g dataentryconfig.Gantt, verdicts map[string]bool,
	nodes map[string]*ganttNode, rejected map[string]bool, rootID string,
) (edges map[string][][2]string, subtree map[string]bool, gerr *ganttError) {
	raw, outside, cyclic, gerr := h.incomingGanttEdges(ctx, g, nodes, rejected, rootID)
	if gerr != nil || cyclic {
		return nil, nil, gerr
	}
	if gerr := h.addOutsideParents(ctx, s, g, verdicts, ganttSortedKeys(outside), nodes); gerr != nil {
		return nil, nil, gerr
	}

	edges = map[string][][2]string{}
	children := map[string][]string{}
	for relType, list := range raw {
		for _, e := range list {
			if nodes[e[0]] == nil {
				continue
			}
			edges[relType] = append(edges[relType], e)
			children[e[0]] = append(children[e[0]], e[1])
		}
	}

	subtree = ganttDescendants(rootID, children)
	if g.OnCycle == "mark" {
		for relType := range edges {
			for _, e := range edges[relType] {
				if e[1] != rootID && subtree[e[1]] && !subtree[e[0]] {
					return nil, nil, nil
				}
			}
		}
	}
	return edges, subtree, nil
}

// incomingGanttEdges lists the incoming hierarchy edges of every loaded node,
// one query per relation type. outside holds the parents it found that are
// neither loaded nor already rejected by the where: filter. cyclic reports an
// edge from a loaded node back to the drilled root.
func (h *ganttHandler) incomingGanttEdges(
	ctx context.Context, g dataentryconfig.Gantt, nodes map[string]*ganttNode,
	rejected map[string]bool, rootID string,
) (raw map[string][][2]string, outside map[string]bool, cyclic bool, gerr *ganttError) {
	ids := ganttSortedKeys(nodes)
	raw = map[string][][2]string{}
	outside = map[string]bool{}
	for _, relType := range g.Hierarchy {
		q := store.RelationQuery{Type: relType, EntityIDs: ids, Direction: store.DirectionIncoming}
		for rel, err := range h.store.ListRelations(ctx, q) {
			if err != nil {
				slog.Error("gantt: relation list failed", "relation", relType, "error", err)
				return nil, nil, false, &ganttError{http.StatusInternalServerError, "internal",
					"Failed to list relations", ""}
			}
			if rel.From == rel.To || nodes[rel.To] == nil {
				continue
			}
			if rel.To == rootID && nodes[rel.From] != nil {
				return nil, nil, true, nil
			}
			if nodes[rel.From] == nil && !rejected[rel.From] {
				outside[rel.From] = true
			}
			raw[relType] = append(raw[relType], [2]string{rel.From, rel.To})
		}
	}
	return raw, outside, false, nil
}

// ganttDescendants returns rootID plus every node reachable from it over
// children. It walks every admissible edge, not only the winning ones: a node
// with any parent inside the subtree is inside it. Iterative for the same
// reason foldGantt is.
func ganttDescendants(rootID string, children map[string][]string) map[string]bool {
	seen := map[string]bool{rootID: true}
	stack := []string{rootID}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, c := range children[id] {
			if !seen[c] {
				seen[c] = true
				stack = append(stack, c)
			}
		}
	}
	return seen
}

// addOutsideParents reads the given ids through the read gate, redacts each
// once, and adds the ones the where: filter keeps to nodes. The read mirrors
// an AllowAll read through scopedQuery, narrowed to ids; the fast path only
// runs when every source type is AllowAll or DenyAll, and a denied type is
// never queried.
func (h *ganttHandler) addOutsideParents(
	ctx context.Context, s *Schema, g dataentryconfig.Gantt, verdicts map[string]bool,
	ids []string, nodes map[string]*ganttNode,
) *ganttError {
	if len(ids) == 0 || worldFromContext(ctx).blocksAllReads() {
		return nil
	}
	for _, typeName := range ganttSortedKeys(g.Sources) {
		if !verdicts[typeName] {
			continue
		}
		q := store.EntityQuery{
			Type:   typeName,
			IDs:    ids,
			Faces:  store.InWorld(worldScopeFrom(ctx)),
			FaceIn: readGateFromContext(ctx).ReadQuery(ctx, typeName).Faces,
		}
		var ents []*entity.Entity
		for hdr, err := range store.ListEntityHeaders(ctx, h.store, q) {
			if err != nil {
				slog.Error("gantt: parent lookup failed", "type", typeName, "error", err)
				return &ganttError{http.StatusInternalServerError, "internal", "Failed to list entities", ""}
			}
			red := visibility.RedactHeader(ctx, h.redactor(), hdr)
			ents = append(ents, &entity.Entity{ID: red.ID, Type: red.Type, Properties: red.Properties})
		}
		if gerr := h.addGanttNodes(requestReadTime(ctx), s, g, typeName, ents, nodes); gerr != nil {
			return gerr
		}
	}
	return nil
}

// linkGanttParents picks each node's parent from the admissible hierarchy
// edges. Extra parents are collected for the multi_parent policy rather than
// dropped. allParents carries every candidate per node so that
// reseatCycleParents can repair a node a loop claimed by sort order.
func linkGanttParents(
	g dataentryconfig.Gantt, nodes map[string]*ganttNode, edgesByType map[string][][2]string,
) (parent map[string]string, allParents map[string][]string, multiParent []string) {
	parent = map[string]string{}
	allParents = map[string][]string{}
	for _, relType := range g.Hierarchy {
		edges := edgesByType[relType]
		sort.Slice(edges, func(i, j int) bool {
			if edges[i][0] != edges[j][0] {
				return edges[i][0] < edges[j][0]
			}
			return edges[i][1] < edges[j][1]
		})
		for _, e := range edges {
			allParents[e[1]] = append(allParents[e[1]], e[0])
			if prev, taken := parent[e[1]]; taken {
				if prev != e[0] {
					multiParent = append(multiParent, e[1])
				}
				continue
			}
			parent[e[1]] = e[0]
			if p := nodes[e[0]]; p != nil {
				p.children = append(p.children, e[1])
			}
		}
	}
	return parent, allParents, multiParent
}

// reseatCycleParents moves a node whose chosen parent is unreachable from
// any root to the first candidate parent that is reachable, under
// on_cycle:"mark". Nodes with a reachable parent are untouched, so a graph
// with no loop is left byte-identical. Candidates are tried in the order
// linkGanttParents collected them, so the outcome is deterministic.
//
// Nil: nodes/parent/allParents are never nil; finishGanttForest builds them.
func reseatCycleParents(
	nodes map[string]*ganttNode, parent map[string]string, allParents map[string][]string,
) (cyclic map[string]bool) {
	cyclic = map[string]bool{}

	reachable := make(map[string]bool, len(nodes))
	growFrom := make([]string, 0, len(nodes))
	grow := func(seed string) {
		growFrom = growFrom[:0]
		growFrom = append(growFrom, seed)
		for len(growFrom) > 0 {
			id := growFrom[len(growFrom)-1]
			growFrom = growFrom[:len(growFrom)-1]
			if reachable[id] {
				continue
			}
			reachable[id] = true
			growFrom = append(growFrom, nodes[id].children...)
		}
	}
	for _, id := range ganttSortedKeys(nodes) {
		if _, has := parent[id]; !has {
			grow(id)
		}
	}

	for _, id := range ganttSortedKeys(nodes) {
		if reachable[id] {
			continue
		}
		for _, cand := range allParents[id] {
			if cand == parent[id] || !reachable[cand] {
				continue
			}
			loopParent := parent[id]
			if p := nodes[loopParent]; p != nil {
				p.children = removeGanttChild(p.children, id)
				p.cutChildren = append(p.cutChildren, id)
			}
			cyclic[id] = true
			parent[id] = cand
			nodes[cand].children = append(nodes[cand].children, id)
			grow(id)
			break
		}
	}
	return cyclic
}

// removeGanttChild drops one occurrence of id, preserving order.
func removeGanttChild(children []string, id string) []string {
	for i, c := range children {
		if c == id {
			return append(children[:i:i], children[i+1:]...)
		}
	}
	return children
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

// ganttEdges lists the hierarchy edges between nodes of the gated node set:
// one query per relation type, outgoing from a node to a node, so it reads
// no edge that leaves the set (TKT-FED6FMM).
//
// A content-scoped edge is kept only when its tail is the face its parent
// node was loaded at ([ownedByFace], BUG-BZQQDP). Another face's edge would
// place a child, and fold its dates, under a face that does not own the
// edge. The check runs here, before the fold, so the roll-up only ever sees
// owned edges. The subtree path declines for faced sources
// (ganttHasFacedSource), so it needs no such check.
func (h *ganttHandler) ganttEdges(
	ctx context.Context, meta *metamodel.Metamodel, g dataentryconfig.Gantt, nodes map[string]*ganttNode,
) (map[string][][2]string, *ganttError) {
	out := map[string][][2]string{}
	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, relType := range g.Hierarchy {
		q := store.RelationQuery{Type: relType, Direction: store.DirectionOutgoing, EntityIDs: ids, PeerIDs: ids}
		for rel, err := range h.store.ListRelations(ctx, q) {
			if err != nil {
				slog.Error("gantt: relation list failed", "relation", relType, "error", err)
				return nil, &ganttError{http.StatusInternalServerError, "internal", "Failed to list relations", ""}
			}
			if rel.From == rel.To {
				continue
			}
			if nodes[rel.From] == nil || nodes[rel.To] == nil {
				continue
			}
			if !ownedByFace(meta, rel, nodes[rel.From].face) {
				continue
			}
			out[relType] = append(out[relType], [2]string{rel.From, rel.To})
		}
	}
	return out, nil
}

// foldGantt computes every node's rolled span, post-order, and marks
// reachability. It is iterative because recursion depth is data-controlled
// and a stack overflow kills the process. With markCycles set, an edge to a
// node on the active stack closes a loop: the target goes into f.cyclic and
// that one edge is cut, which drops no node. A merely visited node is a
// multi-parent diamond, not a cycle.
func foldGantt(f *ganttForest, rootID string, markCycles bool) {
	type frame struct {
		id   string
		next int
	}
	stack := []frame{{id: rootID}}
	f.reachable[rootID] = true
	sort.Strings(f.nodes[rootID].children)

	var onStack map[string]bool
	if markCycles {
		onStack = map[string]bool{rootID: true}
	}

	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		n := f.nodes[top.id]
		if top.next < len(n.children) {
			childID := n.children[top.next]
			top.next++
			if markCycles {
				if onStack[childID] {
					f.cyclic[childID] = true
					n.cutChildren = append(n.cutChildren, childID)
					continue
				}
				onStack[childID] = true
			}
			f.reachable[childID] = true
			sort.Strings(f.nodes[childID].children)
			stack = append(stack, frame{id: childID})
			continue
		}
		stack = stack[:len(stack)-1]
		if markCycles {
			delete(onStack, top.id)
		}
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
		Face:  string(n.face),
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
	out.InCycle = f.cyclic[id]

	children := n.children
	if len(n.cutChildren) > 0 {
		cut := map[string]bool{}
		for _, id := range n.cutChildren {
			cut[id] = true
		}
		children = make([]string, 0, len(n.children))
		for _, childID := range n.children {
			if !cut[childID] {
				children = append(children, childID)
			}
		}
		out.HasMoreChildren = true
	}

	if depth >= maxDepth {
		out.HasMoreChildren = out.HasMoreChildren || len(children) > 0
		return out
	}
	for _, childID := range children {
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
