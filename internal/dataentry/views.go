package dataentry

import (
	"context"
	"fmt"

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
func (h *viewsHandler) executeView(
	ctx context.Context, view ViewConfig, entryID string, w viewWorld,
) (*viewResult, error) {
	entry, err := h.viewEntry(ctx, entryID, w)
	if err != nil {
		return nil, err
	}
	if entry.Type != view.Entry.Type {
		return nil, fmt.Errorf("entry entity %s is type %s, expected %s", entryID, entry.Type, view.Entry.Type)
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
		foundIDs = h.traverseViewBreadthFirst(ctx, sourceIDs, rule, maxD)
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
	ctx context.Context, sourceIDs []string, rule ViewTraverse, maxDepth int,
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
		next := make([]string, 0, len(found))
		for _, id := range found {
			if visited[id] {
				continue
			}
			visited[id] = true
			next = append(next, id)
		}
		frontier = next
	}
	return all
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
