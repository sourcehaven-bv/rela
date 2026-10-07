package memstore

import (
	"context"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/mentions"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// MentionSources implements [store.MentionIndex] by parsing every live body.
// memstore backs tests and experiments, where the entity count is small, so
// it keeps no index; the database and filesystem backends do.
func (m *MemStore) MentionSources(
	_ context.Context, target string, after entity.Ref, limit int,
) ([]entity.Ref, error) {
	if limit <= 0 {
		return nil, store.ErrInvalidQuery
	}
	m.mu.RLock()
	var out []entity.Ref
	for _, e := range m.entities {
		ref := entity.Ref{ID: e.ID, Face: e.Face}
		if after.ID != "" && store.CompareRefs(ref, after) <= 0 {
			continue
		}
		if _, found := slices.BinarySearch(mentions.Targets(e.Content), target); found {
			out = append(out, ref)
		}
	}
	m.mu.RUnlock()
	slices.SortFunc(out, store.CompareRefs)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
