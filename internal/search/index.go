// Package search provides full-text search over a [store.Store],
// satisfied by a [Backend] (today: [bleveindex.Index]). The [Service]
// type combines a [store.EntityReader] with a [Backend] to produce a
// [Searcher] implementation that callers consume.
//
// # This package is world-agnostic BY CONSTRUCTION
//
// Content states and worlds (TKT-WAV8XP) do not appear here, and that is
// deliberate rather than an omission. No type in this package carries a
// [store.WorldScope]: [Query] is text plus filters, and the [Searcher]
// interface has no world parameter. The package therefore serves whatever
// scope its BACKEND applies — today every backend serves the default
// world (pgstore pins `pointer = ”` at both of its search sites; the
// bleve index only ever receives default-face documents).
//
// Two consequences worth stating, because the alternative was considered
// and rejected (RULING 3, 2026-08-22):
//
//   - There is no world-refusal seam here, and one must not be added. A
//     refusal in a package that cannot represent the thing it refuses
//     would be either dead code or a new concept introduced solely to
//     reject itself — and it would read as protection while protecting
//     nothing, which is the failure mode this area keeps hitting.
//   - The fail-closed property lives at the WIRING site instead: a
//     world-bound surface must not be constructible over a searcher that
//     cannot honor its world (the DEC-ZBI39P stance — structurally
//     incapable, not "defaults to safe").
//
// A future world-aware search backend adds the scope AT THE BACKEND, and
// would need per-world indexing (Step 5, TKT-9KZGJO) to do it. Note the
// ACL row gate cannot compensate in the meantime: guard rule 1 makes the
// row gate world-independent, so a draft surfacing through search would
// not be caught downstream.
package search

import (
	"context"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/entity"
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

// MatchedFields reports which logical fields of the already-loaded entity the
// query text matched, in the [FieldID] / [FieldContent] / [PropFieldPrefix]
// vocabulary. It delegates to the backend's [FieldMatcher] when the backend
// implements one, so provenance is computed with the same matcher the backend
// searched with; otherwise it falls back to the ground-truth [MatchTextFields].
//
// It takes the entity rather than loading it so the seam can thread a single
// snapshot through both provenance and the hidden-field verdict (no
// re-load, no cross-snapshot race). An empty text query has no provenance and
// returns nil.
func (s *Service) MatchedFields(e *entity.Entity, text string) map[string]struct{} {
	if e == nil || text == "" {
		return nil
	}
	if fm, ok := s.backend.(FieldMatcher); ok {
		return fm.MatchedFields(e, text)
	}
	return MatchTextFields(e, text)
}

func (s *Service) Search(ctx context.Context, q Query) iter.Seq2[Hit, error] {
	if err := ValidateFilters(q.Filters); err != nil {
		return func(yield func(Hit, error) bool) {
			yield(Hit{}, err)
		}
	}

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
