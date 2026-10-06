package analysis

import (
	"context"
	"fmt"
	"slices"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// OwningIssue is an ownership the write path refuses but imported or
// hand-edited data can hold (TKT-QO14GB). Readers tolerate it, so it is a
// warning: an entity with no clear owner is shown on its own page.
type OwningIssue struct {
	EntityID string
	// Kind is "self" (the entity owns itself), "multiple-owners", or
	// "nested" (the entity is owned and also owns).
	Kind   string
	Owners []string
}

// CheckOwning reports owning edges that break the one-owner, one-level rule.
// It reads each owning relation type once. A store error fails the check,
// since a report built from a partial read would claim the data is clean.
func (s *Service) CheckOwning(ctx context.Context, opts Options) ([]OwningIssue, error) {
	meta := s.deps.Meta
	owners := map[string][]string{}
	owns := map[string]bool{}
	var self []string
	types := make([]string, 0, len(meta.Relations))
	for name := range meta.Relations {
		if metamodel.IsOwning(meta, name) {
			types = append(types, name)
		}
	}
	sort.Strings(types)
	for _, relType := range types {
		for r, err := range s.deps.Store.ListRelations(ctx, store.RelationQuery{Type: relType}) {
			if err != nil {
				return nil, fmt.Errorf("read %s relations: %w", relType, err)
			}
			if r.From == r.To {
				self = append(self, r.From)
				continue
			}
			if !slices.Contains(owners[r.To], r.From) {
				owners[r.To] = append(owners[r.To], r.From)
			}
			owns[r.From] = true
		}
	}

	issues := make([]OwningIssue, 0)
	for _, id := range self {
		if inScope(id, opts.Scope) {
			issues = append(issues, OwningIssue{EntityID: id, Kind: "self", Owners: []string{id}})
		}
	}
	ids := make([]string, 0, len(owners))
	for id := range owners {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if !inScope(id, opts.Scope) {
			continue
		}
		ps := owners[id]
		sort.Strings(ps)
		if len(ps) > 1 {
			issues = append(issues, OwningIssue{EntityID: id, Kind: "multiple-owners", Owners: ps})
		}
		if owns[id] {
			issues = append(issues, OwningIssue{EntityID: id, Kind: "nested", Owners: ps})
		}
	}
	return issues, nil
}
