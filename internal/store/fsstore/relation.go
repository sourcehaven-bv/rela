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

// defaultTailKey addresses the DEFAULT-tail edge of a triple. Get,
// update, and delete are default-tail-only in Step 1 (TKT-DOFYR1) —
// see store.RelationData.FromFace; state-tailed edges are removed
// via the entity cascades.
func defaultTailKey(from, relType, to string) string {
	return relKey(from, "", relType, to)
}

func (s *FSStore) GetRelation(_ context.Context, from, relType, to string) (*entity.Relation, error) {
	s.mu.RLock()
	key := defaultTailKey(from, relType, to)
	_, ok := s.relations[key]
	s.mu.RUnlock()

	if !ok {
		return nil, store.ErrNotFound
	}

	r, err := s.loadRelation(from, relType, to)
	if err != nil {
		return nil, err
	}
	return r, nil
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
	_ context.Context, from, relType, to string, data *store.RelationData,
) (*entity.Relation, error) {
	for _, id := range []string{from, to} {
		if err := storeutil.ValidateID(id); err != nil {
			return nil, err
		}
	}
	if err := storeutil.ValidateRelationType(relType); err != nil {
		return nil, err
	}

	var fp entity.Face
	if data != nil {
		fp = data.FromFace
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := relKey(from, fp, relType, to)
	if _, exists := s.relations[key]; exists {
		return nil, store.ErrConflict
	}

	r := entity.NewRelation(from, relType, to)
	r.FromFace = fp
	r.UpdatedAt = time.Now()
	if data != nil {
		r.Content = data.Content
		if data.Properties != nil {
			r.Properties = make(map[string]any, len(data.Properties))
			for k, v := range data.Properties {
				r.Properties[k] = entity.CloneValue(v)
			}
		}
	}

	// Write to disk.
	if err := s.writeRelation(r); err != nil {
		return nil, err
	}

	// Update index.
	s.relations[key] = relationMeta{From: from, Type: relType, To: to, FromFace: fp}
	s.relationOrder = storeutil.SortedInsert(s.relationOrder, key)

	s.emit(store.Event{
		Op:           store.EventRelationCreated,
		RelationType: relType,
		From:         from,
		To:           to,
		Face:         fp,
	})
	return r.Clone(), nil
}

func (s *FSStore) updateRelation(
	_ context.Context, from, relType, to string, data store.RelationData,
) (*entity.Relation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := defaultTailKey(from, relType, to)
	if _, ok := s.relations[key]; !ok {
		return nil, store.ErrNotFound
	}

	// Load existing, then apply update.
	r, err := s.loadRelation(from, relType, to)
	if err != nil {
		return nil, err
	}

	r.Content = data.Content
	if data.Properties != nil {
		r.Properties = make(map[string]any, len(data.Properties))
		for k, v := range data.Properties {
			r.Properties[k] = entity.CloneValue(v)
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
		RelationType: relType,
		From:         from,
		To:           to,
	})
	return r.Clone(), nil
}

func (s *FSStore) deleteRelation(ctx context.Context, from, relType, to string) error {
	return s.deleteRelationState(ctx, from, "", relType, to)
}

// deleteRelationState removes the edge with EXACTLY this tail. The tail is
// part of a relation's identity, so addressing the wrong one deletes a
// different edge rather than failing (TKT-C1XUA8).
func (s *FSStore) deleteRelationState(
	_ context.Context, from string, p entity.Face, relType, to string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := relKey(from, p, relType, to)
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
		RelationType: relType,
		From:         from,
		To:           to,
		Face:         p,
	})
	return nil
}

// writeRelation writes a relation to disk using temp-file + rename.
func (s *FSStore) writeRelation(r *entity.Relation) error {
	return s.codec.writeRelationFile(r)
}
