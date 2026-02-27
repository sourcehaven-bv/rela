package dataentry

import (
	"fmt"
	"strings"

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
	entry, ok := a.g.GetNode(entryID)
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
	seen := map[string]bool{}

	// Handle special "*" case - gather from all collections
	if len(rule.From) == 1 && rule.From[0] == "*" {
		for _, entities := range result.Collections {
			for _, e := range entities {
				if !seen[e.ID] {
					sources = append(sources, e)
					seen[e.ID] = true
				}
			}
		}
	} else {
		// Gather from named collections
		for _, fromName := range rule.From {
			if entities, ok := result.Collections[fromName]; ok {
				for _, e := range entities {
					if !seen[e.ID] {
						sources = append(sources, e)
						seen[e.ID] = true
					}
				}
			}
		}
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

	// Deduplicate into collection(s)
	// CollectAs can specify multiple collection names to filter entities by type
	for _, collName := range rule.CollectAs {
		if result.Collections[collName] == nil {
			result.Collections[collName] = []*model.Entity{}
		}
	}

	// Build set of existing IDs per collection
	existingByCollection := make(map[string]map[string]bool)
	for _, collName := range rule.CollectAs {
		existingByCollection[collName] = make(map[string]bool)
		for _, e := range result.Collections[collName] {
			existingByCollection[collName][e.ID] = true
		}
	}

	// Add entities to matching collection(s)
	for _, e := range found {
		for _, collName := range rule.CollectAs {
			// If there's only one collection name, all entities go there.
			// If there are multiple, entities are filtered by type matching the collection name.
			if len(rule.CollectAs) == 1 || e.Type == collName || pluralize(e.Type) == collName {
				if !existingByCollection[collName][e.ID] {
					result.Collections[collName] = append(result.Collections[collName], e)
					existingByCollection[collName][e.ID] = true
				}
			}
		}
	}
}

func (a *App) traverseViewOnce(sourceID string, rule ViewTraverse) []*model.Entity {
	var out []*model.Entity
	if rule.Follow != "" {
		for _, edge := range a.g.OutgoingEdges(sourceID) {
			if edge.Type == rule.Follow {
				if target, ok := a.g.GetNode(edge.To); ok {
					out = append(out, target)
				}
			}
		}
	} else if rule.FollowIncoming != "" {
		for _, edge := range a.g.IncomingEdges(sourceID) {
			if edge.Type == rule.FollowIncoming {
				if src, ok := a.g.GetNode(edge.From); ok {
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
		entityDef, ok := a.meta.GetEntityDef(entity.Type)
		if !ok {
			continue
		}
		propDef, ok := entityDef.Properties[f.Property]
		if !ok {
			continue
		}
		matches, err := filter.Match(entity, f, &propDef, a.meta)
		if err != nil {
			continue
		}
		if matches {
			result = append(result, entity)
		}
	}
	return result, nil
}

// pluralize returns a simple pluralized form of a word.
// This is used to match entity types to collection names (e.g., "function" -> "functions").
func pluralize(s string) string {
	if s == "" {
		return s
	}
	// Simple English pluralization rules
	switch {
	case strings.HasSuffix(s, "s"), strings.HasSuffix(s, "x"), strings.HasSuffix(s, "ch"), strings.HasSuffix(s, "sh"):
		return s + "es"
	case strings.HasSuffix(s, "y") && len(s) > 1 && !isVowel(s[len(s)-2]):
		return s[:len(s)-1] + "ies"
	default:
		return s + "s"
	}
}

func isVowel(b byte) bool {
	return b == 'a' || b == 'e' || b == 'i' || b == 'o' || b == 'u'
}

// collectAsContains checks if a StringOrSlice contains a specific value.
func collectAsContains(s StringOrSlice, target string) bool {
	for _, v := range s {
		if v == target {
			return true
		}
	}
	return false
}
