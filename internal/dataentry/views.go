package dataentry

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// viewResult holds the entry entity and collected entities after traversal.
type viewResult struct {
	Entry       *entity.Entity
	Collections map[string][]*entity.Entity
	// World is the world this result was executed in, carried so the section
	// builders can label each entity's face provenance (TKT-WRLDAPI item 4b).
	//
	// On the RESULT rather than threaded through every builder because the
	// answer is the same for every entity in the result — the world resolved
	// them all — and because a builder that took a world parameter it did not
	// otherwise need would invite someone to pass a different one.
	World viewWorld
}

// executeView runs a view's traversal rules and returns the result.
//
// It is a SHARED ENGINE, not the `_views` handler's private helper: three
// surfaces call it (the `_views` route, `_sidepanel` via executeSidePanel, and
// the command runner's `kind: view`). That is why the world arrives as an
// explicit PARAMETER rather than being read off ctx — see [viewWorld].
//
// entryID is an ADDRESS (`ID` or `ID@face`, see [entityRef]); an address the
// grammar rejects is the same not-found a missing entry is.
func (h *viewsHandler) executeView(
	ctx context.Context, view ViewConfig, entryID string, w viewWorld,
) (*viewResult, error) {
	ref, ok := parseEntityRef(entryID)
	if !ok {
		return nil, errViewEntryNotFound(entryID)
	}
	return h.executeViewRef(ctx, view, ref, w)
}

// executeViewRef is [viewsHandler.executeView] for an already-parsed entry
// address.
func (h *viewsHandler) executeViewRef(
	ctx context.Context, view ViewConfig, entryRef entityRef, w viewWorld,
) (*viewResult, error) {
	entry, err := h.viewEntry(ctx, entryRef, w)
	if err != nil {
		return nil, err
	}
	if entry.Type != view.Entry.Type {
		return nil, fmt.Errorf("entry entity %s is type %s, expected %s", entryRef, entry.Type, view.Entry.Type)
	}
	// Source-gate the entry (BUG-9Z20WH). Every production caller already
	// row-gates the entry before invoking executeView (the `_views` route, the
	// form side-panel, the command runner's `kind: view`), so this is normally
	// a redundant same-principal probe returning the same verdict. It is kept
	// as defense in depth: executeView is a SHARED ENGINE reachable via the
	// synthetic ViewConfig in sections.go, and a future caller must not be able
	// to feed it an entry the principal cannot read.
	//
	// This does NOT contradict viewEntry's "the entry is not row-gated here"
	// note: that note is about not RE-gating an entry the handler just cleared,
	// and this gate is world-INDEPENDENT (guard rule 1), so for every caller
	// that did gate it returns the identical verdict and cannot 404 a cleared
	// entry. A hidden entry is reported as the ordinary not-found so it stays
	// indistinguishable from a missing one. Under NopACL the gate permits, so
	// behavior is unchanged for a full-read principal.
	if ok, gerr := readGateFromContext(ctx).PermitsRead(ctx, entry.Type, entry.ID); gerr != nil || !ok {
		return nil, errViewEntryNotFound(entryRef.String())
	}

	result := &viewResult{
		Entry:       entry,
		Collections: map[string][]*entity.Entity{"entry": {entry}},
		World:       w,
	}

	// Multi-pass traversal (up to 10 passes until stable)
	maxPasses := 10
	for range maxPasses {
		before := countViewEntities(result.Collections)
		for _, rule := range view.Traverse {
			h.applyViewTraverse(ctx, rule, result, w)
		}
		if countViewEntities(result.Collections) == before {
			break
		}
	}

	// Remove internal "entry" collection
	delete(result.Collections, "entry")

	// Row-gate + field-redact on the way out (DEC-ZBI39P). Traversal above runs
	// on raw store entities on purpose: a rule's where: filter may reference a
	// hidden property, and edges are walked by id — redacting mid-traversal
	// would break both. Redacting here, once, gives every section builder
	// already-redacted entities (closes BUG-9QL9XV property-value leak and
	// BUG-R9EHKV title leak) and drops hidden neighbors from collections
	// (Filter gates by row). The entry is already row-gated at the handler
	// (api_v1.go, TKT-BNX2PN), so it only needs field redaction, not a re-gate
	// that could 404 an entry the caller was just cleared to read.
	//
	// Residual (accepted, like the computed-path timing note in
	// internal/visibility): a traverse rule's where: runs on raw entities above,
	// so for a READABLE neighbor whose only hidden aspect is a field value, its
	// presence/absence in a collection still reflects whether it matched a
	// predicate over that hidden field. The value is redacted; membership is a
	// one-bit inference channel, not a value disclosure.
	result.Entry = visibility.Redact(ctx, h.redactor(), result.Entry)
	for name, entities := range result.Collections {
		result.Collections[name] = h.viewReader.Filter(ctx, entities)
	}

	return result, nil
}

func (h *viewsHandler) applyViewTraverse(
	ctx context.Context, rule ViewTraverse, result *viewResult, w viewWorld,
) {
	// Gather source entities
	var sources []*entity.Entity
	if rule.From == "*" {
		seen := map[string]bool{}
		for _, entities := range result.Collections {
			for _, e := range entities {
				if !seen[e.ID] {
					sources = append(sources, e)
					seen[e.ID] = true
				}
			}
		}
	} else if entities, ok := result.Collections[rule.From]; ok {
		sources = entities
	}

	// Traverse from each source, collecting NEIGHBOR IDS rather than entities.
	//
	// Splitting id-collection from entity-loading is what makes the world path
	// affordable: ids are cheap and the recursive walk needs them anyway to
	// decide where to step next, while LOADING an entity under a world costs a
	// resolution. Collect the whole rule's ids first, then resolve once.
	maxRecursionDepth := 10
	sourceIDs := make([]string, 0, len(sources))
	for _, src := range sources {
		sourceIDs = append(sourceIDs, src.ID)
	}
	var foundIDs []string
	if rule.Recursive {
		maxD := rule.MaxDepth
		if maxD <= 0 {
			maxD = maxRecursionDepth
		}
		foundIDs = h.traverseViewBreadthFirst(ctx, sourceIDs, rule, maxD, w)
	} else {
		// One relation query for every source at once (TKT-1U8XYN), in the
		// same order the per-source loop produced: sources in collection
		// order, each source's edges in store order.
		foundIDs = h.traverseViewMany(ctx, sourceIDs, rule)
	}

	// ONE resolution for the whole rule application, not one per hop.
	//
	// The per-hop shape would be an N+1 multiplied by the 10-pass fixpoint and
	// again by the recursive walk's depth — materially worse than the per-row
	// cost item 4 documented as known. Batching here is a design choice made up
	// front rather than an optimisation deferred (see the PR body).
	found := h.loadViewEntities(ctx, foundIDs, w)

	// Apply where filter if specified.
	//
	// RULING 16: this runs against the RESOLVED FACES, because `found` now
	// holds whatever face the world selected. A `where:` filtering on draft
	// values while the page renders published content would contradict its own
	// page. filterEntities itself needed no change — it reads e.Properties off
	// whatever it is handed, so feeding it faces makes it filter faces.
	if rule.Where != "" {
		filtered, err := h.filterEntities(found, rule.Where)
		if err == nil {
			found = filtered
		}
		// On error, continue with unfiltered results (silent failure for
		// robustness). This SILENTLY WIDENS a construct whose job is to narrow
		// — tracked as BUG-WHEREWIDE, decided (RULING 17) to become a load-time
		// error. Deliberately not fixed here: it predates worlds and deserves
		// its own change rather than riding along in a world PR.
	}

	// Deduplicate into collection
	if result.Collections[rule.CollectAs] == nil {
		result.Collections[rule.CollectAs] = []*entity.Entity{}
	}
	existing := map[string]bool{}
	for _, e := range result.Collections[rule.CollectAs] {
		existing[e.ID] = true
	}
	for _, e := range found {
		if !existing[e.ID] {
			result.Collections[rule.CollectAs] = append(result.Collections[rule.CollectAs], e)
			existing[e.ID] = true
		}
	}
}

// traverseViewMany is [viewsHandler.traverseViewOnce] for many sources in ONE
// relation query. The result is ordered as the per-source calls would have
// been concatenated: by source in the given order, then by the store's edge
// order within a source. A source with no edges contributes nothing.
func (h *viewsHandler) traverseViewMany(ctx context.Context, sourceIDs []string, rule ViewTraverse) []string {
	if len(sourceIDs) == 0 {
		return nil
	}
	var relType string
	var direction store.Direction
	var useTarget bool
	switch {
	case rule.Follow != "":
		relType, direction, useTarget = rule.Follow, store.DirectionOutgoing, true
	case rule.FollowIncoming != "":
		relType, direction, useTarget = rule.FollowIncoming, store.DirectionIncoming, false
	default:
		return nil
	}
	bySource := make(map[string][]string, len(sourceIDs))
	q := store.RelationQuery{EntityIDs: sourceIDs, Type: relType, Direction: direction}
	for r, err := range h.store.ListRelations(ctx, q) {
		if err != nil {
			break
		}
		sourceID, targetID := r.From, r.To
		if !useTarget {
			sourceID, targetID = r.To, r.From
		}
		if targetID != "" {
			bySource[sourceID] = append(bySource[sourceID], targetID)
		}
	}
	var out []string
	for _, id := range sourceIDs {
		out = append(out, bySource[id]...)
	}
	return out
}

// traverseViewBreadthFirst walks the relation graph from every source at
// once, one relation query per level (TKT-1U8XYN) instead of one per visited
// node, up to maxDepth levels. It returns the neighbor IDS found, level by
// level; like traverseViewMany it loads no entities. A node is expanded at
// most once, but an id reached again is still reported — the caller dedupes
// when it loads the collection, exactly as it did for the former depth-first
// walk, and the recursive tests pin the SET of ids, not their order.
func (h *viewsHandler) traverseViewBreadthFirst(
	ctx context.Context, sourceIDs []string, rule ViewTraverse, maxDepth int, w viewWorld,
) []string {
	visited := make(map[string]bool, len(sourceIDs))
	frontier := make([]string, 0, len(sourceIDs))
	for _, id := range sourceIDs {
		if visited[id] {
			continue
		}
		visited[id] = true
		frontier = append(frontier, id)
	}
	all := make([]string, 0, len(frontier))
	for depth := 0; depth < maxDepth && len(frontier) > 0; depth++ {
		found := h.traverseViewMany(ctx, frontier, rule)
		all = append(all, found...)

		// SOURCE-GATE the frontier (BUG-9Z20WH). An id the principal cannot
		// read is not expanded, so it cannot act as a stepping-stone to a
		// descendant reachable only through it.
		//
		// This gate has to be HERE, not only at the load. The walk is
		// id-only by design (TKT-1U8XYN) — it never materializes an entity —
		// so a hidden node H would otherwise be expanded on ids alone and its
		// child V collected. V is readable in its own right, so neither the
		// load-time gate nor the out-gate (viewReader.Filter) would drop it,
		// and the leak would survive both. Gating the frontier is the only
		// point at which H's UNREADABILITY can stop the walk.
		//
		// `all` deliberately keeps the ungated ids: they are the rule's raw
		// result and every one of them is gated again at load time, so a
		// hidden node still never reaches a collection. What this gate
		// changes is REACHABILITY — which nodes get to be walked THROUGH —
		// which is the bug.
		next := make([]string, 0, len(found))
		for _, id := range found {
			if visited[id] {
				continue
			}
			visited[id] = true
			next = append(next, id)
		}
		frontier = h.readableViewIDs(ctx, next, w)
	}
	return all
}

// readableViewIDs filters ids down to those the request principal may read,
// preserving order. It is the traversal's source gate (BUG-9Z20WH).
//
// # Why it resolves rows the same way the loader does
//
// The gate takes the world and queries with the SAME `World`/`FaceIn` shape as
// [viewsHandler.loadViewEntities]. That is load-bearing, not tidiness: a type
// declaring `faces:` stores NO row at the zero coordinate (BUG-HC6I2T), so a
// default-world scan returns nothing for a faced entity. Gating on such a scan
// would drop every faced entity from the frontier even under NopACL — a silent
// functional regression dressed up as a denial, and a gate that decides about a
// different graph than the one being walked.
//
// # Both halves of the verdict
//
// A row gate is face-BLIND, so it is paired with [faceReadable], exactly as
// visibleHeaderIDs pairs them (TKT-O7R2A1). Without the face half a principal
// granted only `policy@published` could walk THROUGH a draft-only entity to
// reach its descendants — the same reachability leak this gate exists to close,
// re-opened one coordinate down.
//
// Ids are resolved to their type and face with a content-free HEADER scan,
// because the gate is keyed by (type, id) plus a face and the id-only walk has
// neither. Headers keep the walk's "no entity loads during traversal" property.
//
// FAIL CLOSED throughout: a header scan that faults, an id whose header never
// arrives (so its type is unknown), and a gate probe that errors all drop the
// affected ids, each with a warning so an operator sees a cause rather than a
// silently-truncated view. Under NopACL every probe permits and every face is
// allowed, so the traversal is unchanged for a full-read principal.
func (h *viewsHandler) readableViewIDs(ctx context.Context, ids []string, w viewWorld) []string {
	if len(ids) == 0 {
		return nil
	}
	if w.denied {
		// The world itself is denied; nothing in it is expandable.
		return nil
	}

	type idHeader struct {
		typ  string
		face entity.Face
	}
	hdrs := make(map[string]idHeader, len(ids))
	q := store.EntityQuery{IDs: ids, World: w.scope}
	for hdr, err := range store.ListEntityHeaders(ctx, h.store, q) {
		if err != nil {
			// A header-scan fault is not "everything is hidden", but it is
			// also not a license to expand un-gated nodes. Drop the level.
			slog.Warn("dataentry: view traversal: source-gate header scan failed; "+
				"frontier dropped", "world", w.name, "ids", len(ids), "err", err)
			return nil
		}
		hdrs[hdr.ID] = idHeader{typ: hdr.Type, face: hdr.Face}
	}

	byType := make(map[string][]string, 4)
	ordered := make([]string, 0, len(ids))
	for _, id := range ids {
		h, ok := hdrs[id]
		if !ok {
			// No row for this id in this world: a dangling edge, or a face
			// the world excludes. Either way it is not expandable here.
			continue
		}
		byType[h.typ] = append(byType[h.typ], id)
		ordered = append(ordered, id)
	}

	gate := readGateFromContext(ctx)
	permitted := make(map[string]bool, len(ordered))
	for typ, typeIDs := range byType {
		verdicts, err := gate.PermitsReadMany(ctx, typ, typeIDs)
		if err != nil {
			// Fail closed: none of this type is expandable. Logged loud so
			// operators see the cause rather than a silently-short chain.
			slog.Warn("dataentry: view traversal: source-gate probe failed; "+
				"dropping type from frontier", "type", typ, "ids", len(typeIDs), "err", err)
			continue
		}
		for _, id := range typeIDs {
			if verdicts[id] {
				permitted[id] = true
			}
		}
	}

	out := make([]string, 0, len(ordered))
	for _, id := range ordered {
		if !permitted[id] {
			continue
		}
		// The row gate cleared the ENTITY; it says nothing about which FACE
		// this principal may read. Both halves or neither.
		if !faceReadable(ctx, hdrs[id].typ, hdrs[id].face) {
			continue
		}
		out = append(out, id)
	}
	return out
}

func countViewEntities(collections map[string][]*entity.Entity) int {
	seen := map[string]bool{}
	for _, entities := range collections {
		for _, e := range entities {
			seen[e.ID] = true
		}
	}
	return len(seen)
}

// filterEntities filters entities based on a where expression.
// Supports the "type" pseudo-property to filter by entity type.
func (h *viewsHandler) filterEntities(entities []*entity.Entity, whereExpr string) ([]*entity.Entity, error) {
	f, err := filter.Parse(whereExpr)
	if err != nil {
		return nil, fmt.Errorf("invalid where expression: %w", err)
	}

	s := h.schema()
	var result []*entity.Entity
	for _, e := range entities {
		// Special handling for "type" pseudo-property
		if f.Property == "type" {
			if filter.MatchValue(e.Type, f) {
				result = append(result, e)
			}
			continue
		}

		// Regular property - use metamodel-aware matching
		entityDef, ok := s.Meta.GetEntityDef(e.Type)
		if !ok {
			continue
		}
		propDef, ok := entityDef.Properties[f.Property]
		if !ok {
			continue
		}
		rec := filter.Record{ID: e.ID, Type: e.Type, Properties: e.Properties, ModifiedAt: e.UpdatedAt}
		matches, err := filter.Match(rec, f, &propDef, s.Meta)
		if err != nil {
			continue
		}
		if matches {
			result = append(result, e)
		}
	}
	return result, nil
}
