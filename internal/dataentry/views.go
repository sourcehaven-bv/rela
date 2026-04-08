package dataentry

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// viewResult holds the entry entity and collected entities after traversal.
type viewResult struct {
	Entry       *model.Entity
	Collections map[string][]*model.Entity
}

// executeView runs a view's traversal rules and returns the result.
func (a *App) executeView(view ViewConfig, entryID string) (*viewResult, error) {
	entry, ok := a.Graph().GetNode(entryID)
	if !ok {
		return nil, fmt.Errorf("entry entity not found: %s", entryID)
	}
	if entry.Type != view.Entry.Type {
		return nil, fmt.Errorf("entry entity %s is type %s, expected %s", entryID, entry.Type, view.Entry.Type)
	}

	result := &viewResult{
		Entry:       entry,
		Collections: map[string][]*model.Entity{"entry": {entry}},
	}

	// Multi-pass traversal (up to 10 passes until stable)
	maxPasses := 10
	for pass := 0; pass < maxPasses; pass++ {
		before := countViewEntities(result.Collections)
		for _, rule := range view.Traverse {
			a.applyViewTraverse(rule, result)
		}
		if countViewEntities(result.Collections) == before {
			break
		}
	}

	// Remove internal "entry" collection
	delete(result.Collections, "entry")

	return result, nil
}

func (a *App) applyViewTraverse(rule ViewTraverse, result *viewResult) {
	// Gather source entities
	var sources []*model.Entity
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

	// Traverse from each source
	maxRecursionDepth := 10
	var found []*model.Entity
	for _, src := range sources {
		if rule.Recursive {
			maxD := rule.MaxDepth
			if maxD <= 0 {
				maxD = maxRecursionDepth
			}
			found = append(found, a.traverseViewRecursive(src.ID, rule, 0, maxD, map[string]bool{})...)
		} else {
			found = append(found, a.traverseViewOnce(src.ID, rule)...)
		}
	}

	// Apply where filter if specified
	if rule.Where != "" {
		filtered, err := a.filterEntities(found, rule.Where)
		if err == nil {
			found = filtered
		}
		// On error, continue with unfiltered results (silent failure for robustness)
	}

	// Deduplicate into collection
	if result.Collections[rule.CollectAs] == nil {
		result.Collections[rule.CollectAs] = []*model.Entity{}
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

func (a *App) traverseViewOnce(sourceID string, rule ViewTraverse) []*model.Entity {
	var out []*model.Entity
	if rule.Follow != "" {
		for _, edge := range a.Graph().OutgoingEdges(sourceID) {
			if edge.Type == rule.Follow {
				if target, ok := a.Graph().GetNode(edge.To); ok {
					out = append(out, target)
				}
			}
		}
	} else if rule.FollowIncoming != "" {
		for _, edge := range a.Graph().IncomingEdges(sourceID) {
			if edge.Type == rule.FollowIncoming {
				if src, ok := a.Graph().GetNode(edge.From); ok {
					out = append(out, src)
				}
			}
		}
	}
	return out
}

func (a *App) traverseViewRecursive(sourceID string, rule ViewTraverse, depth, maxDepth int, visited map[string]bool) []*model.Entity {
	if depth >= maxDepth || visited[sourceID] {
		return nil
	}
	visited[sourceID] = true
	immediate := a.traverseViewOnce(sourceID, rule)
	var all []*model.Entity
	all = append(all, immediate...)
	for _, e := range immediate {
		all = append(all, a.traverseViewRecursive(e.ID, rule, depth+1, maxDepth, visited)...)
	}
	return all
}

func countViewEntities(collections map[string][]*model.Entity) int {
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
func (a *App) filterEntities(entities []*model.Entity, whereExpr string) ([]*model.Entity, error) {
	f, err := filter.Parse(whereExpr)
	if err != nil {
		return nil, fmt.Errorf("invalid where expression: %w", err)
	}

	var result []*model.Entity
	for _, entity := range entities {
		// Special handling for "type" pseudo-property
		if f.Property == "type" {
			if filter.MatchValue(entity.Type, f) {
				result = append(result, entity)
			}
			continue
		}

		// Regular property - use metamodel-aware matching
		entityDef, ok := a.Meta().GetEntityDef(entity.Type)
		if !ok {
			continue
		}
		propDef, ok := entityDef.Properties[f.Property]
		if !ok {
			continue
		}
		matches, err := filter.Match(entity, f, &propDef, a.Meta())
		if err != nil {
			continue
		}
		if matches {
			result = append(result, entity)
		}
	}
	return result, nil
}
