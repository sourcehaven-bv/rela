package fsstore

import (
	"context"
	"fmt"
	"iter"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// --- RelationReader ---

func (s *FSStore) GetRelation(_ context.Context, from, relType, to string) (*entity.Relation, error) {
	s.mu.RLock()
	key := from + "--" + relType + "--" + to
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

	type relKey struct {
		from, typ, to string
	}
	var matches []relKey
	for _, key := range s.relationOrder {
		rm := s.relations[key]
		r := &entity.Relation{From: rm.From, Type: rm.Type, To: rm.To}
		if !storeutil.MatchRelation(r, q) {
			continue
		}
		matches = append(matches, relKey{rm.From, rm.Type, rm.To})
	}
	s.mu.RUnlock()

	return func(yield func(*entity.Relation, error) bool) {
		for _, m := range matches {
			r, err := s.loadRelation(m.from, m.typ, m.to)
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

// --- RelationWriter ---

func (s *FSStore) CreateRelation(_ context.Context, from, relType, to string, data *store.RelationData) (*entity.Relation, error) {
	for _, id := range []string{from, to} {
		if err := storeutil.ValidateID(id); err != nil {
			return nil, err
		}
	}
	if strings.Contains(relType, "--") {
		return nil, fmt.Errorf("store: relation type %q contains consecutive dashes", relType)
	}
	if relType == "" {
		return nil, fmt.Errorf("store: empty relation type")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := from + "--" + relType + "--" + to
	if _, exists := s.relations[key]; exists {
		return nil, store.ErrConflict
	}

	r := entity.NewRelation(from, relType, to)
	r.UpdatedAt = time.Now()
	if data != nil {
		r.Content = data.Content
		if data.Properties != nil {
			r.Properties = make(map[string]interface{}, len(data.Properties))
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
	s.relations[key] = relationMeta{From: from, Type: relType, To: to}
	s.relationOrder = storeutil.SortedInsert(s.relationOrder, key)

	s.emit(store.Event{
		Op:           store.EventRelationCreated,
		RelationType: relType,
		From:         from,
		To:           to,
	})
	return r.Clone(), nil
}

func (s *FSStore) UpdateRelation(_ context.Context, from, relType, to string, data store.RelationData) (*entity.Relation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := from + "--" + relType + "--" + to
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
		r.Properties = make(map[string]interface{}, len(data.Properties))
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

func (s *FSStore) DeleteRelation(_ context.Context, from, relType, to string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := from + "--" + relType + "--" + to
	if _, ok := s.relations[key]; !ok {
		return store.ErrNotFound
	}

	// Delete file.
	path := s.relationFilePath(from, relType, to)
	if err := s.fs.Remove(path); err != nil {
		return err
	}

	// Update index.
	delete(s.relations, key)
	s.relationOrder = storeutil.SortedRemove(s.relationOrder, key)

	s.emit(store.Event{
		Op:           store.EventRelationDeleted,
		RelationType: relType,
		From:         from,
		To:           to,
	})
	return nil
}

// writeRelation writes a relation to disk using temp-file + rename.
func (s *FSStore) writeRelation(r *entity.Relation) error {
	return s.writeRelationFile(r)
}
