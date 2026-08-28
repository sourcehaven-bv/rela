package fsstore

import (
	"context"
	"fmt"
	"iter"
	"os"
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

	return storeutil.TopValues(counts, limit), nil
}

// --- EntityWriter ---

// idTaken reports whether any key of index case-folds to the same identity as
// id. except is skipped so a rename can ask "does any OTHER entity claim this
// identity?" without self-colliding.
//
// Scans the existing index on each call rather than maintaining a second
// case-folded map: the entity index is mutated at several sites, and a
// parallel index would silently drift at whichever one a future change
// forgets to update. Runs only on create and rename, never on a read path.
//
// A free function rather than a method — FSStore is at its plimsoll
// max-methods line, and this needs no receiver state beyond the index.
// Callers must hold s.mu.
func idTaken(index map[string]entityMeta, id, except string) bool {
	folded := storeutil.FoldID(id)
	for existing := range index {
		if existing == except {
			continue
		}
		if storeutil.FoldID(existing) == folded {
			return true
		}
	}
	return false
}

func (s *FSStore) createEntity(_ context.Context, e *entity.Entity) error {
	if err := storeutil.ValidateID(e.ID); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Case-folded: on a case-insensitive filesystem (macOS, Windows) "ABC"
	// and "abc" are the same file, so a byte-exact check here would let the
	// write silently overwrite the existing entity (BUG-3RCWNS).
	if idTaken(s.entities, e.ID, "") {
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

func (s *FSStore) updateEntity(_ context.Context, e *entity.Entity) error {
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

	// The entity's type determines its file path, so a type change wrote the
	// record at the NEW type's path — the file at the old path must go too,
	// or it would resurrect the stale record on the next reload. Part of the
	// type-change-on-update store contract (storetest UpdateChangesType).
	if meta.Type != e.Type {
		oldKey := s.layout.entityFileKey(meta.Type, e.ID)
		if err := s.rooted.Remove(oldKey); err != nil && !os.IsNotExist(err) {
			return err
		}
		s.echoes.Forget(s.layout.absPath(oldKey))
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

func (s *FSStore) deleteEntity(_ context.Context, id string, cascade bool) (*store.DeleteResult, error) {
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

	// Delete relation files first, then the entity file. A real removal
	// error (not "already gone") aborts before the entity file is touched
	// and before the in-memory index is mutated, so the entity is never
	// removed while one of its relations could not be — fail-secure, no
	// orphaned-from entity (BUG-C20T / issue #888). Not transactional: a
	// relation file removed before a later failure stays removed, but the
	// whole op runs under s.mu and the index is updated only on success.
	for _, rm := range related {
		key := s.layout.relationFileKey(rm.From, rm.Type, rm.To)
		if err := s.rooted.Remove(key); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("delete relation file %s--%s--%s: %w", rm.From, rm.Type, rm.To, err)
		}
		s.echoes.Forget(s.layout.absPath(key))
	}
	key := s.layout.entityFileKey(meta.Type, id)
	if err := s.rooted.Remove(key); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	s.echoes.Forget(s.layout.absPath(key))

	// Cascade attachments — under the per-entity layout the
	// attachment directory is owned 1:1 by the entity.
	if err := s.removeAttachmentDir(id); err != nil {
		return nil, err
	}

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

func (s *FSStore) renameEntity(_ context.Context, oldID, newID string) (*store.RenameResult, error) {
	if err := storeutil.ValidateID(newID); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	meta, ok := s.entities[oldID]
	if !ok {
		return nil, store.ErrNotFound
	}
	// except=oldID: an entity may change its own casing (abc -> ABC).
	if idTaken(s.entities, newID, oldID) {
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
		oldKey := s.layout.relationFileKey(rm.From, rm.Type, rm.To)
		_ = s.rooted.Remove(oldKey)
		s.echoes.Forget(s.layout.absPath(oldKey))
	}

	// Delete old entity file.
	oldKey := s.layout.entityFileKey(meta.Type, oldID)
	_ = s.rooted.Remove(oldKey)
	s.echoes.Forget(s.layout.absPath(oldKey))

	// Move the attachment directory (if any) onto the new ID.
	if err := s.renameAttachmentDir(oldID, newID); err != nil {
		return nil, err
	}

	// Update entity index.
	delete(s.entities, oldID)
	s.entityOrder = storeutil.SortedRemove(s.entityOrder, oldID)
	s.entities[newID] = entityMeta{ID: newID, Type: meta.Type}
	s.entityOrder = storeutil.SortedInsert(s.entityOrder, newID)
	s.notifyRenamed(oldID, renamed)

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
	return s.codec.writeEntityFile(e)
}
