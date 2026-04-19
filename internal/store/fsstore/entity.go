package fsstore

import (
	"context"
	"fmt"
	"iter"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// --- EntityReader ---

func (s *FSStore) GetEntity(_ context.Context, id string) (*entity.Entity, error) {
	s.mu.RLock()
	meta, ok := s.entities[id]
	s.mu.RUnlock()

	if !ok {
		return nil, store.ErrNotFound
	}

	e, err := s.loadEntity(meta.ID, meta.Type)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (s *FSStore) ListEntities(_ context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	s.mu.RLock()
	idSet := entityIDSet(q.IDs)

	// Collect matching IDs + types from index.
	type idType struct {
		id, typ string
	}
	matches := make([]idType, 0)
	for _, id := range s.entityOrder {
		if !matchEntityQuery(s.entities[id], q, idSet) {
			continue
		}
		meta := s.entities[id]
		matches = append(matches, idType{meta.ID, meta.Type})
	}
	s.mu.RUnlock()

	return func(yield func(*entity.Entity, error) bool) {
		for _, m := range matches {
			e, err := s.loadEntity(m.id, m.typ)
			if err != nil {
				if !yield(nil, err) {
					return
				}
				continue
			}
			if !yield(e, nil) {
				return
			}
		}
	}
}

func (s *FSStore) ListEntitiesPage(_ context.Context, q store.EntityQuery) (store.Page[*entity.Entity], error) {
	cursorKey, err := storeutil.DecodeCursor(q.Cursor)
	if err != nil {
		return store.Page[*entity.Entity]{}, err
	}

	s.mu.RLock()
	idSet := entityIDSet(q.IDs)
	keys := storeutil.PaginateSortedKeys(s.entityOrder, cursorKey, q.Limit, func(id string) bool {
		return matchEntityQuery(s.entities[id], q, idSet)
	})

	// Capture (id, type) pairs while still holding the lock — loadEntity
	// needs the type and we must not call it under the lock.
	type idType struct{ id, typ string }
	pairs := make([]idType, 0, len(keys.Keys))
	for _, id := range keys.Keys {
		if meta, ok := s.entities[id]; ok {
			pairs = append(pairs, idType{meta.ID, meta.Type})
		}
	}
	s.mu.RUnlock()

	items := make([]*entity.Entity, 0, len(pairs))
	for _, p := range pairs {
		e, err := s.loadEntity(p.id, p.typ)
		if err != nil {
			return store.Page[*entity.Entity]{}, err
		}
		items = append(items, e)
	}
	return store.Page[*entity.Entity]{Items: items, NextCursor: keys.NextCursor}, nil
}

// entityIDSet builds the ID-lookup set used by ListEntities and its page
// variant. Returns nil when ids is empty so callers can test with len().
func entityIDSet(ids []string) map[string]bool {
	if len(ids) == 0 {
		return nil
	}
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// matchEntityQuery reports whether an indexed entity satisfies q's Type
// and IDs filters. idSet must be pre-computed from q.IDs.
func matchEntityQuery(m entityMeta, q store.EntityQuery, idSet map[string]bool) bool {
	if q.Type != "" && m.Type != q.Type {
		return false
	}
	if len(idSet) > 0 && !idSet[m.ID] {
		return false
	}
	return true
}

func (s *FSStore) CountEntities(_ context.Context, q store.EntityQuery) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	idSet := make(map[string]bool, len(q.IDs))
	for _, id := range q.IDs {
		idSet[id] = true
	}

	count := 0
	for _, meta := range s.entities {
		if q.Type != "" && meta.Type != q.Type {
			continue
		}
		if len(idSet) > 0 && !idSet[meta.ID] {
			continue
		}
		count++
	}
	return count, nil
}

func (s *FSStore) HighestID(_ context.Context, prefix string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	highest := 0
	pfx := prefix + "-"
	for id := range s.entities {
		if !strings.HasPrefix(id, pfx) {
			continue
		}
		suffix := id[len(pfx):]
		var n int
		if _, err := fmt.Sscanf(suffix, "%d", &n); err == nil && n > highest {
			highest = n
		}
	}
	return highest, nil
}

func (s *FSStore) PropertyValues(_ context.Context, property string, limit int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	counts, ok := s.propCache[property]
	if !ok {
		return []string{}, nil
	}

	type vc struct {
		value string
		count int
	}
	sorted := make([]vc, 0, len(counts))
	for v, c := range counts {
		sorted = append(sorted, vc{v, c})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].count != sorted[j].count {
			return sorted[i].count > sorted[j].count
		}
		return sorted[i].value < sorted[j].value
	})

	result := make([]string, 0, limit)
	for i := 0; i < len(sorted) && (limit == 0 || i < limit); i++ {
		result = append(result, sorted[i].value)
	}
	return result, nil
}

// --- EntityWriter ---

func (s *FSStore) CreateEntity(_ context.Context, e *entity.Entity) error {
	if err := storeutil.ValidateID(e.ID); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.entities[e.ID]; exists {
		return store.ErrConflict
	}

	// Write to disk
	stored := e.Clone()
	stored.UpdatedAt = time.Now()
	if err := s.writeEntity(stored); err != nil {
		return err
	}

	// Update index
	s.entities[e.ID] = entityMeta{ID: e.ID, Type: e.Type}
	s.entityOrder = storeutil.SortedInsert(s.entityOrder, e.ID)
	addEntityToCache(s.propCache, stored)
	s.notifyPut(stored)

	s.emit(store.Event{
		Op:         store.EventEntityCreated,
		EntityType: e.Type,
		EntityID:   e.ID,
	})
	return nil
}

func (s *FSStore) UpdateEntity(_ context.Context, e *entity.Entity) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, exists := s.entities[e.ID]
	if !exists {
		return store.ErrNotFound
	}

	// Load old entity for prop cache diff.
	old, err := s.loadEntity(meta.ID, meta.Type)
	if err != nil {
		return err
	}

	stored := e.Clone()
	stored.UpdatedAt = time.Now()
	if err := s.writeEntity(stored); err != nil {
		return err
	}

	// Update index
	s.entities[e.ID] = entityMeta{ID: e.ID, Type: e.Type}
	removeEntityFromCache(s.propCache, old)
	addEntityToCache(s.propCache, stored)
	s.notifyPut(stored)

	s.emit(store.Event{
		Op:         store.EventEntityUpdated,
		EntityType: e.Type,
		EntityID:   e.ID,
	})
	return nil
}

func (s *FSStore) DeleteEntity(_ context.Context, id string, cascade bool) (*store.DeleteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, ok := s.entities[id]
	if !ok {
		return nil, store.ErrNotFound
	}

	// Find related relations from index.
	var related []relationMeta
	for _, rm := range s.relations {
		if rm.From == id || rm.To == id {
			related = append(related, rm)
		}
	}

	if !cascade && len(related) > 0 {
		return nil, fmt.Errorf("%w: entity %s has %d relation(s)", store.ErrHasRelations, id, len(related))
	}

	// Load entity for result and prop cache.
	e, err := s.loadEntity(meta.ID, meta.Type)
	if err != nil {
		return nil, err
	}

	// Load relations for result.
	deletedRelations := make([]*entity.Relation, 0)
	for _, rm := range related {
		r, loadErr := s.loadRelation(rm.From, rm.Type, rm.To)
		if loadErr != nil {
			r = entity.NewRelation(rm.From, rm.Type, rm.To)
		}
		deletedRelations = append(deletedRelations, r)
	}

	// Delete relation files first, then entity file.
	for _, rm := range related {
		path := s.relationFilePath(rm.From, rm.Type, rm.To)
		_ = s.dirs.Remove(path)
		s.forgetHash(path)
	}
	path := s.entityFilePath(meta.Type, id)
	if err := s.dirs.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	s.forgetHash(path)

	// Update index
	delete(s.entities, id)
	s.entityOrder = storeutil.SortedRemove(s.entityOrder, id)
	removeEntityFromCache(s.propCache, e)
	s.notifyDelete(id)

	for _, rm := range related {
		key := rm.From + "--" + rm.Type + "--" + rm.To
		delete(s.relations, key)
		s.relationOrder = storeutil.SortedRemove(s.relationOrder, key)
	}

	result := &store.DeleteResult{
		DeletedEntities:  []*entity.Entity{e},
		DeletedRelations: deletedRelations,
	}

	s.emit(store.Event{
		Op:         store.EventEntityDeleted,
		EntityType: meta.Type,
		EntityID:   id,
	})
	for _, rm := range related {
		s.emit(store.Event{
			Op:           store.EventRelationDeleted,
			RelationType: rm.Type,
			From:         rm.From,
			To:           rm.To,
		})
	}

	return result, nil
}

func (s *FSStore) RenameEntity(_ context.Context, oldID, newID string) (*store.RenameResult, error) {
	if err := storeutil.ValidateID(newID); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	meta, ok := s.entities[oldID]
	if !ok {
		return nil, store.ErrNotFound
	}
	if _, exists := s.entities[newID]; exists {
		return nil, store.ErrConflict
	}

	// Load entity from disk.
	e, err := s.loadEntity(meta.ID, meta.Type)
	if err != nil {
		return nil, err
	}

	// Prepare renamed entity.
	renamed := e.Clone()
	renamed.ID = newID
	renamed.UpdatedAt = time.Now()

	// Write new entity file.
	if err := s.writeEntity(renamed); err != nil {
		return nil, err
	}

	// Find and update affected relations.
	var toUpdate []relationMeta
	for _, rm := range s.relations {
		if rm.From == oldID || rm.To == oldID {
			toUpdate = append(toUpdate, rm)
		}
	}

	for _, rm := range toUpdate {
		// Load relation.
		r, loadErr := s.loadRelation(rm.From, rm.Type, rm.To)
		if loadErr != nil {
			r = entity.NewRelation(rm.From, rm.Type, rm.To)
		}

		// Update endpoints.
		if r.From == oldID {
			r.From = newID
		}
		if r.To == oldID {
			r.To = newID
		}

		// Write new relation file.
		if writeErr := s.writeRelation(r); writeErr != nil {
			return nil, writeErr
		}

		// Delete old relation file.
		oldPath := s.relationFilePath(rm.From, rm.Type, rm.To)
		_ = s.dirs.Remove(oldPath)
		s.forgetHash(oldPath)
	}

	// Delete old entity file.
	oldPath := s.entityFilePath(meta.Type, oldID)
	_ = s.dirs.Remove(oldPath)
	s.forgetHash(oldPath)

	// Update entity index.
	delete(s.entities, oldID)
	s.entityOrder = storeutil.SortedRemove(s.entityOrder, oldID)
	s.entities[newID] = entityMeta{ID: newID, Type: meta.Type}
	s.entityOrder = storeutil.SortedInsert(s.entityOrder, newID)
	s.notifyDelete(oldID)
	s.notifyPut(renamed)

	// Update relation index.
	for _, rm := range toUpdate {
		oldKey := rm.From + "--" + rm.Type + "--" + rm.To
		delete(s.relations, oldKey)
		s.relationOrder = storeutil.SortedRemove(s.relationOrder, oldKey)

		newFrom, newTo := rm.From, rm.To
		if newFrom == oldID {
			newFrom = newID
		}
		if newTo == oldID {
			newTo = newID
		}
		newKey := newFrom + "--" + rm.Type + "--" + newTo
		s.relations[newKey] = relationMeta{From: newFrom, Type: rm.Type, To: newTo}
		s.relationOrder = storeutil.SortedInsert(s.relationOrder, newKey)
	}

	s.emit(store.Event{
		Op:         store.EventEntityUpdated,
		EntityType: meta.Type,
		EntityID:   newID,
	})

	return &store.RenameResult{RelationsUpdated: len(toUpdate)}, nil
}

// writeEntity writes an entity to disk using temp-file + rename.
func (s *FSStore) writeEntity(e *entity.Entity) error {
	return s.writeEntityFile(e)
}
