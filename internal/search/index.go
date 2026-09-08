// Package search provides full-text search over a [store.Store],
// satisfied by a [Backend] (today: [bleveindex.Index]). The [Service]
// type combines a [store.EntityReader] with a [Backend] to produce a
// [Searcher] implementation that callers consume.
//
// # A world IS the search scope
//
// Worlds (TKT-WAV8XP) reach this package by exactly one route: [Query.World]
// carries a [store.WorldScope], and [Backend.Search] takes it. Nothing else
// here knows what a world is — no world NAME, no metamodel, no policy. The
// scope arrives compiled, from the consumer that resolved it.
//
// The scope RESOLVES rather than filters, and that is the load-bearing
// choice ([ResolvePrimes] carries the argument in full). A world picks at
// most one face per entity — its prime — and only that face is matched. So:
//
//   - A hit's text is the text the reader will be shown. Matching a
//     non-prime face would let a published-world search hit on a term that
//     exists only in the draft while displaying published bytes that lack
//     it.
//   - An entity the world resolves to nothing contributes nothing. Under
//     `otherwise: exclude` that absence IS the publication bit.
//   - At most one row per entity comes out STRUCTURALLY, so [Query.Limit]
//     counts entities with no grouping pass.
//
// The zero [store.WorldScope] is the default world and must reduce to
// exactly the pre-worlds query — every construction site that never heard
// of worlds keeps working, allocating nothing.
//
// Consequently the ACL row gate is NOT what keeps a draft out of a
// published-world result: guard rule 1 makes that gate world-independent.
// The resolution above is. A backend that matched first and resolved after
// (or skipped resolution entirely) would leak drafts with nothing
// downstream to catch it — see the doc on [Backend.Search].
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

	// The backend resolves the world and returns FACES, so each result
	// already names which face matched. The limit is applied here rather
	// than pushed down because the type and property filters below can still
	// reject a face, and a backend-side limit would silently shorten the
	// page.
	faces, err := s.backend.Search(q.Text, 0, q.World)
	if err != nil {
		return func(yield func(Hit, error) bool) {
			yield(Hit{}, err)
		}
	}

	typeSet := toSet(q.Types)

	return func(yield func(Hit, error) bool) {
		emitted := 0
		for _, f := range faces {
			if q.Limit > 0 && emitted >= q.Limit {
				return
			}

			// Load the FACE that matched, not the entity's default state.
			// Reading the bare id here would display default-face bytes for a
			// hit scored against a different face — the mismatch between what
			// was searched and what is shown that world-scoped search exists
			// to close.
			e, err := s.reader.GetEntityState(ctx, f.ID, f.Face)
			if err != nil {
				continue // face may have been deleted since indexing
			}

			if len(typeSet) > 0 && !typeSet[e.Type] {
				continue
			}

			if !MatchFilters(e, q.Filters) {
				continue
			}

			hit := Hit{
				ID:            e.ID,
				Type:          e.Type,
				Title:         e.Title(),
				Face:          f.Face,
				Via:           f.Via,
				ChainPosition: f.ChainPosition,
			}
			if !yield(hit, nil) {
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
