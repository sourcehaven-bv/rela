// Package search provides full-text search over a [store.Store],
// satisfied by a [Backend] (today: [bleveindex.Index]). The [Service]
// type combines a [store.EntityReader] with a [Backend] to produce a
// [Searcher] implementation that callers consume.
package search

import (
	"context"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Service implements Searcher by combining an EntityReader with a Backend.
// Text queries go to the backend; type/property filters are applied by
// loading entities from the reader.
type Service struct {
	reader  store.EntityReader
	backend Backend
}

// compile-time check
var _ Searcher = (*Service)(nil)

// New creates a Searcher backed by the given reader and search backend.
func New(reader store.EntityReader, backend Backend) *Service {
	return &Service{reader: reader, backend: backend}
}

func (s *Service) Search(ctx context.Context, q Query) iter.Seq2[Hit, error] {
	if q.Text == "" {
		return s.listAll(ctx, q)
	}

	ids, err := s.backend.Search(q.Text, 0)
	if err != nil {
		return func(yield func(Hit, error) bool) {
			yield(Hit{}, err)
		}
	}

	typeSet := toSet(q.Types)

	return func(yield func(Hit, error) bool) {
		emitted := 0
		for _, id := range ids {
			if q.Limit > 0 && emitted >= q.Limit {
				return
			}

			e, err := s.reader.GetEntity(ctx, id)
			if err != nil {
				continue // entity may have been deleted since indexing
			}

			if len(typeSet) > 0 && !typeSet[e.Type] {
				continue
			}

			if !MatchFilters(e, q.Filters) {
				continue
			}

			if !yield(Hit{ID: e.ID, Type: e.Type, Title: e.Title()}, nil) {
				return
			}
			emitted++
		}
	}
}

// listAll handles searches with no text query — returns all entities matching
// type and property filters.
func (s *Service) listAll(ctx context.Context, q Query) iter.Seq2[Hit, error] {
	return func(yield func(Hit, error) bool) {
		emitted := 0
		for e, err := range s.reader.ListEntities(ctx, store.EntityQuery{}) {
			if err != nil {
				if !yield(Hit{}, err) {
					return
				}
				continue
			}

			if q.Limit > 0 && emitted >= q.Limit {
				return
			}

			if len(q.Types) > 0 && !toSet(q.Types)[e.Type] {
				continue
			}

			if !MatchFilters(e, q.Filters) {
				continue
			}

			if !yield(Hit{ID: e.ID, Type: e.Type, Title: e.Title()}, nil) {
				return
			}
			emitted++
		}
	}
}

func toSet(ss []string) map[string]bool {
	if len(ss) == 0 {
		return nil
	}
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}
