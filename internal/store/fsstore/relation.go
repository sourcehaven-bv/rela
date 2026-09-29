package fsstore

import (
	"context"
	"iter"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// --- RelationReader ---

// keyOf is the index key and filename stem of the edge at k, tail included.
func keyOf(k entity.RelationKey) string {
	return relKey(k.From, k.FromFace, k.Type, k.To)
}

func (s *FSStore) GetRelation(_ context.Context, k entity.RelationKey) (*entity.Relation, error) {
	s.mu.RLock()
	rm, ok := s.relations[keyOf(k)]
	s.mu.RUnlock()

	if !ok {
		return nil, store.ErrNotFound
	}
	return s.loadRelationMeta(rm)
}

func (s *FSStore) ListRelations(_ context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error] {
	s.mu.RLock()

	match := storeutil.NewRelationMatcher(q)
	matches := make([]relationMeta, 0)
	for _, key := range s.relationOrder {
		if !matchRelationKey(s, key, match) {
			continue
		}
		matches = append(matches, s.relations[key])
	}
	s.mu.RUnlock()

	return func(yield func(*entity.Relation, error) bool) {
		for _, m := range matches {
			r, err := s.loadRelationMeta(m)
			if err != nil {
				if !yield(nil, err) {
					return
				}
				continue
			}
			if !yield(r, nil) {
				return
			}
		}
	}
}

func (s *FSStore) ListRelationsPage(_ context.Context, q store.RelationQuery) (store.Page[*entity.Relation], error) {
	cursorKey, err := storeutil.DecodeCursor(q.Cursor)
	if err != nil {
		return store.Page[*entity.Relation]{}, err
	}

	s.mu.RLock()
	match := storeutil.NewRelationMatcher(q)
	keys := storeutil.PaginateSortedKeys(s.relationOrder, cursorKey, q.Limit, func(key string) bool {
		return matchRelationKey(s, key, match)
	})

	pairs := make([]relationMeta, 0, len(keys.Keys))
	for _, key := range keys.Keys {
		pairs = append(pairs, s.relations[key])
	}
	s.mu.RUnlock()

	items := make([]*entity.Relation, 0, len(pairs))
	for _, p := range pairs {
		r, err := s.loadRelationMeta(p)
		if err != nil {
			return store.Page[*entity.Relation]{}, err
		}
		items = append(items, r)
	}
	return store.Page[*entity.Relation]{Items: items, NextCursor: keys.NextCursor}, nil
}

// matchRelationKey reports whether the relation at key in s.relations
// matches q. Callers must hold s.mu (at least for reading).
func matchRelationKey(s *FSStore, key string, match func(*entity.Relation) bool) bool {
	rm := s.relations[key]
	r := &entity.Relation{From: rm.From, FromFace: rm.FromFace, Type: rm.Type, To: rm.To}
	return match(r)
}

func (s *FSStore) CountRelations(_ context.Context, q store.RelationQuery) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	match := storeutil.NewRelationMatcher(q)
	count := 0
	for _, key := range s.relationOrder {
		if matchRelationKey(s, key, match) {
			count++
		}
	}
	return count, nil
}

// --- RelationWriter ---

func (s *FSStore) createRelation(
	_ context.Context, k entity.RelationKey, data *store.RelationData,
) (*entity.Relation, error) {
	for _, id := range []string{k.From, k.To} {
		if err := storeutil.ValidateID(id); err != nil {
			return nil, err
		}
	}
	if err := storeutil.ValidateRelationType(k.Type); err != nil {
		return nil, err
	}
	if data != nil {
		if err := storeutil.ValidateProperties(data.Properties); err != nil {
			return nil, err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := keyOf(k)
	if _, exists := s.relations[key]; exists {
		return nil, store.ErrConflict
	}

	r := entity.NewRelation(k.From, k.Type, k.To)
	r.FromFace = k.FromFace
	r.UpdatedAt = time.Now()
	if data != nil {
		r.Content = data.Content
		if data.Properties != nil {
			r.Properties = make(map[string]any, len(data.Properties))
			for pk, v := range data.Properties {
				r.Properties[pk] = entity.CloneValue(v)
			}
		}
	}

	// Write to disk.
	if err := s.writeRelation(r); err != nil {
		return nil, err
	}

	// Update index.
	s.relations[key] = relationMeta{From: k.From, Type: k.Type, To: k.To, FromFace: k.FromFace}
	s.relationOrder = storeutil.SortedInsert(s.relationOrder, key)

	s.emit(store.Event{
		Op:           store.EventRelationCreated,
		RelationType: k.Type,
		From:         k.From,
		To:           k.To,
		Face:         k.FromFace,
	})
	return r.Clone(), nil
}

// updateRelation writes the edge with EXACTLY this key, tail included. The
// tail is part of a relation's identity, so addressing the wrong one updates
// a different edge rather than failing (BUG-64MU2Q).
func (s *FSStore) updateRelation(
	_ context.Context, k entity.RelationKey, data store.RelationData,
) (*entity.Relation, error) {
	if err := storeutil.ValidateProperties(data.Properties); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	rm, ok := s.relations[keyOf(k)]
	if !ok {
		return nil, store.ErrNotFound
	}

	// Load existing, then apply update. Via the index meta, so the loaded
	// edge carries the tail it is stored under.
	r, err := s.loadRelationMeta(rm)
	if err != nil {
		return nil, err
	}

	r.Content = data.Content
	if data.Properties != nil {
		r.Properties = make(map[string]any, len(data.Properties))
		for pk, v := range data.Properties {
			r.Properties[pk] = entity.CloneValue(v)
		}
	} else {
		r.Properties = nil
	}
	r.UpdatedAt = time.Now()

	// Write to disk.
	if err := s.writeRelation(r); err != nil {
		return nil, err
	}

	s.emit(store.Event{
		Op:           store.EventRelationUpdated,
		RelationType: k.Type,
		From:         k.From,
		To:           k.To,
		Face:         k.FromFace,
	})
	return r.Clone(), nil
}

// deleteRelation removes the edge with EXACTLY this key, tail included
// (TKT-C1XUA8).
func (s *FSStore) deleteRelation(_ context.Context, k entity.RelationKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := keyOf(k)
	rm, ok := s.relations[key]
	if !ok {
		return store.ErrNotFound
	}

	// Delete file.
	fileKey := s.layout.relationFileKeyMeta(rm)
	if err := s.rooted.Remove(fileKey); err != nil {
		return err
	}
	s.echoes.Forget(s.layout.absPath(fileKey))

	// Update index.
	delete(s.relations, key)
	s.relationOrder = storeutil.SortedRemove(s.relationOrder, key)

	s.emit(store.Event{
		Op:           store.EventRelationDeleted,
		RelationType: k.Type,
		From:         k.From,
		To:           k.To,
		Face:         k.FromFace,
	})
	return nil
}

// writeRelation writes a relation to disk using temp-file + rename.
func (s *FSStore) writeRelation(r *entity.Relation) error {
	return s.codec.writeRelationFile(r)
}
