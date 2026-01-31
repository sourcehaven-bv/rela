package views

import (
	"sort"
)

// CollectDeps executes a view for each root and returns all unique entity IDs touched.
// If a root entity doesn't exist in the graph or doesn't match the view's entry type,
// it is silently skipped.
func (e *Engine) CollectDeps(view ViewDef, rootIDs []string) ([]string, error) {
	seen := make(map[string]bool)

	for _, rootID := range rootIDs {
		// Skip roots that don't exist
		entity, ok := e.graph.GetNode(rootID)
		if !ok {
			continue
		}

		// Skip roots that don't match the view's entry type
		if entity.Type != view.Entry.Type {
			continue
		}

		result, err := e.Execute(view, rootID)
		if err != nil {
			return nil, err
		}

		// Always include the root entity itself as a dependency
		seen[rootID] = true

		// Collect entry entity (may differ from root if enrichResult modified it)
		if result.Entry != nil {
			seen[result.Entry.ID] = true
		}

		// Collect all entities from all collections
		for _, entities := range result.Collections {
			for _, ent := range entities {
				seen[ent.ID] = true
			}
		}
	}

	// Sort for deterministic output
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	return ids, nil
}
