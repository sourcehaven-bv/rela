package fsstore

import (
	"context"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

func (s *FSStore) Search(_ context.Context, q store.SearchQuery) iter.Seq2[*entity.Entity, error] {
	if q.Text == "" {
		return s.listAll(q)
	}

	// Query the search index outside the lock — SearchIndex must be concurrency-safe.
	ids, err := s.searchIndex.Search(q.Text, 0)
	if err != nil {
		return func(yield func(*entity.Entity, error) bool) {
			yield(nil, err)
		}
	}

	typeSet := make(map[string]bool, len(q.Types))
	for _, t := range q.Types {
		typeSet[t] = true
	}

	// Filter IDs through the in-memory index for type matching and existence.
	s.mu.RLock()
	type idType struct {
		id, typ string
	}
	var candidates []idType
	for _, id := range ids {
		meta, ok := s.entities[id]
		if !ok {
			continue
		}
		if len(typeSet) > 0 && !typeSet[meta.Type] {
			continue
		}
		candidates = append(candidates, idType{meta.ID, meta.Type})
	}
	s.mu.RUnlock()

	return func(yield func(*entity.Entity, error) bool) {
		emitted := 0
		for _, c := range candidates {
			if q.Limit > 0 && emitted >= q.Limit {
				return
			}

			e, loadErr := s.loadEntity(c.id, c.typ)
			if loadErr != nil {
				if !yield(nil, loadErr) {
					return
				}
				continue
			}

			if !storeutil.MatchFilters(e, q.Filters) {
				continue
			}

			if !yield(e, nil) {
				return
			}
			emitted++
		}
	}
}

// listAll handles searches with no text query — returns all entities matching
// type and property filters.
func (s *FSStore) listAll(q store.SearchQuery) iter.Seq2[*entity.Entity, error] {
	s.mu.RLock()

	typeSet := make(map[string]bool, len(q.Types))
	for _, t := range q.Types {
		typeSet[t] = true
	}

	type idType struct {
		id, typ string
	}
	var candidates []idType
	for _, id := range s.entityOrder {
		meta := s.entities[id]
		if len(typeSet) > 0 && !typeSet[meta.Type] {
			continue
		}
		candidates = append(candidates, idType{meta.ID, meta.Type})
	}
	s.mu.RUnlock()

	return func(yield func(*entity.Entity, error) bool) {
		emitted := 0
		for _, c := range candidates {
			if q.Limit > 0 && emitted >= q.Limit {
				return
			}

			e, err := s.loadEntity(c.id, c.typ)
			if err != nil {
				if !yield(nil, err) {
					return
				}
				continue
			}

			if !storeutil.MatchFilters(e, q.Filters) {
				continue
			}

			if !yield(e, nil) {
				return
			}
			emitted++
		}
	}
}
