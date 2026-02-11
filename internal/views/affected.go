package views

import (
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
)

// AffectedRoots finds which root entities are affected by changes to the given entity IDs.
// It executes the view for each root and checks if any changed entity appears in the result.
// Roots that don't exist or don't match the view's entry type are silently skipped.
// The returned entities are sorted by ID for deterministic output.
func (e *Engine) AffectedRoots(view ViewDef, changedIDs, rootIDs []string) ([]*model.Entity, error) {
	changedSet := make(map[string]bool, len(changedIDs))
	for _, id := range changedIDs {
		changedSet[id] = true
	}

	var affected []*model.Entity

	for _, rootID := range rootIDs {
		entity, ok := e.graph.GetNode(rootID)
		if !ok || entity.Type != view.Entry.Type {
			continue
		}

		deps, err := e.CollectDeps(view, []string{rootID})
		if err != nil {
			return nil, err
		}

		for _, dep := range deps {
			if changedSet[dep] {
				affected = append(affected, entity)
				break
			}
		}
	}

	sort.Slice(affected, func(i, j int) bool {
		return natsort.Less(affected[i].ID, affected[j].ID)
	})

	return affected, nil
}
