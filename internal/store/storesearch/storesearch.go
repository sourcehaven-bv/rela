// Package storesearch provides a generic Searcher implementation that
// combines a store.EntityReader with a store.SearchIndex.
//
// This is the default search service for store backends that don't provide
// native search (e.g. filesystem stores). Smart backends (e.g. Postgres with
// ts_vector) can implement store.Searcher directly instead.
package storesearch

import (
	"context"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// Searcher implements store.Searcher by combining an EntityReader with a
// SearchIndex. Text queries go to the index; type/property filters are
// applied by loading entities from the reader.
type Searcher struct {
	reader store.EntityReader
	index  store.SearchIndex
}

// compile-time check
var _ store.Searcher = (*Searcher)(nil)

// New creates a Searcher backed by the given reader and search index.
func New(reader store.EntityReader, index store.SearchIndex) *Searcher {
	return &Searcher{reader: reader, index: index}
}

func (s *Searcher) Search(ctx context.Context, q store.SearchQuery) iter.Seq2[store.SearchHit, error] {
	if q.Text == "" {
		return s.listAll(ctx, q)
	}

	ids, err := s.index.Search(q.Text, 0)
	if err != nil {
		return func(yield func(store.SearchHit, error) bool) {
			yield(store.SearchHit{}, err)
		}
	}

	typeSet := toSet(q.Types)

	return func(yield func(store.SearchHit, error) bool) {
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

			if !storeutil.MatchFilters(e, q.Filters) {
				continue
			}

			if !yield(store.SearchHit{ID: e.ID, Type: e.Type, Title: e.Title()}, nil) {
				return
			}
			emitted++
		}
	}
}

// listAll handles searches with no text query — returns all entities matching
// type and property filters.
func (s *Searcher) listAll(ctx context.Context, q store.SearchQuery) iter.Seq2[store.SearchHit, error] {
	return func(yield func(store.SearchHit, error) bool) {
		emitted := 0
		for e, err := range s.reader.ListEntities(ctx, store.EntityQuery{}) {
			if err != nil {
				if !yield(store.SearchHit{}, err) {
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

			if !storeutil.MatchFilters(e, q.Filters) {
				continue
			}

			if !yield(store.SearchHit{ID: e.ID, Type: e.Type, Title: e.Title()}, nil) {
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
