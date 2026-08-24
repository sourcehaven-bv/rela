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
	var foundIDs []string
	for _, src := range sources {
		if rule.Recursive {
			maxD := rule.MaxDepth
			if maxD <= 0 {
				maxD = maxRecursionDepth
			}
			foundIDs = append(foundIDs,
				h.traverseViewRecursive(ctx, src.ID, rule, 0, maxD, map[string]bool{})...)
		} else {
			foundIDs = append(foundIDs, h.traverseViewOnce(ctx, src.ID, rule)...)
		}
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

// traverseViewOnce returns the neighbor IDS one hop from sourceID.
//
// It returns IDS, not entities, so the caller can resolve the whole rule's
// neighbors in one batch — see applyViewTraverse. It also means this function
// does no entity read at all, which is what removes the per-hop
// default-world `GetEntity` that made the traversal world-blind.
//
// The relation query keeps a NIL tail deliberately. A view traverses the
// entity GRAPH: it follows an edge to reach a neighbor, and per design §2.3
// heads are entity-level, so which face the SOURCE is standing on does not
// change which ids it can reach. Filtering by the source's pointer here would
// hide identity edges from any entity whose prime is not the default state —
// the "fallback trap" documented on worldreader.RelationReader.Neighbors,
// where the zero pointer as a FromPointer VALUE means default-tail-only rather
// than unfiltered.
//
// (Content-scoped edges are therefore over-returned here relative to what item
// 4's entity GET shows. That is a known divergence, not an oversight: making
// view traversal honor edge scope needs the per-type dispatch, which is its
// own change with its own tests. Recorded in the PR body.)
func (h *viewsHandler) traverseViewOnce(ctx context.Context, sourceID string, rule ViewTraverse) []string {
	st := h.store
	var out []string

	var relType string
	var direction store.Direction
	var useTarget bool // true: collect edge.To; false: collect edge.From
	switch {
	case rule.Follow != "":
		relType = rule.Follow
		direction = store.DirectionOutgoing
		useTarget = true
	case rule.FollowIncoming != "":
		relType = rule.FollowIncoming
		direction = store.DirectionIncoming
		useTarget = false
	default:
		return nil
	}

	q := store.RelationQuery{EntityID: sourceID, Type: relType, Direction: direction}
	for r, err := range st.ListRelations(ctx, q) {
		if err != nil {
			break
		}
		targetID := r.To
		if !useTarget {
			targetID = r.From
		}
		if targetID != "" {
			out = append(out, targetID)
		}
	}
	return out
}

// traverseViewRecursive walks the relation graph depth-first, returning the
// neighbor IDS found. Like traverseViewOnce it loads no entities — the walk
// steps by id, so it never needed them.
func (h *viewsHandler) traverseViewRecursive(
	ctx context.Context, sourceID string, rule ViewTraverse, depth, maxDepth int, visited map[string]bool,
) []string {
	if depth >= maxDepth || visited[sourceID] {
		return nil
	}
	visited[sourceID] = true
	immediate := h.traverseViewOnce(ctx, sourceID, rule)
	var all []string
	all = append(all, immediate...)
	for _, id := range immediate {
		all = append(all, h.traverseViewRecursive(ctx, id, rule, depth+1, maxDepth, visited)...)
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
