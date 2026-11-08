package memstore

import (
	"context"
	"slices"
	"strings"

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

// rewriteMentions makes every live body that mentions oldID name newID
// instead, as [store.EntityWriter.RenameFamily] requires of a store without
// [store.RetiredIDs] (TKT-J6GISC3). Like the relation re-key it runs below
// the ACL: it replaces an id, and discloses nothing. Soft-deleted bodies are
// left alone. It returns the rewritten rows. Caller holds m.mu.
func rewriteMentions(m *MemStore, oldID, newID string) []entity.MentionRewrite {
	keys := make([]string, 0)
	for key, e := range m.entities {
		if strings.Contains(e.Content, oldID) {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	var rewritten []entity.MentionRewrite
	for _, key := range keys {
		e := m.entities[key]
		content, changed := mentions.Rewrite(e.Content, oldID, newID)
		if !changed {
			continue
		}
		stored := e.Clone()
		stored.Content = content
		stored.UpdatedAt = m.clock()
		m.entities[key] = stored
		rewritten = append(rewritten, entity.MentionRewrite{ID: stored.ID, Type: stored.Type, Face: stored.Face})
		m.notifyPut(stored)
		m.emit(store.Event{
			Op:         store.EventEntityUpdated,
			EntityType: stored.Type,
			EntityID:   stored.ID,
			Face:       stored.Face,
		})
	}
	return rewritten
}
