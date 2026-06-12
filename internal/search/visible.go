package search

import (
	"context"
	"errors"
	"fmt"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Visible is the generic VisibleSearcher: it wraps any Searcher and
// filters its hits through a store.GraphQueryer. This is the
// implementation for the simple backends (bleve, LinearSearch), which
// only ever pair with in-process stores where MatchingIDs is cheap.
//
// Candidates are fetched UNCAPPED (inner Limit 0) and q.Limit is
// applied after visibility filtering, per the VisibleSearcher
// contract. Note the bleve backend maps limit ≤ 0 to a practical
// 10000-candidate ceiling — within that window restricted principals
// get their true top-K; beyond it the bound is documented in the ACL
// security guide.
type Visible struct {
	inner Searcher
	gq    store.GraphQueryer
}

// compile-time check
var _ VisibleSearcher = (*Visible)(nil)

// NewVisible builds the generic scope-filtering wrapper around an
// existing searcher and the store's graph-query capability.
func NewVisible(inner Searcher, gq store.GraphQueryer) (*Visible, error) {
	if inner == nil {
		return nil, errors.New("search.NewVisible: inner Searcher is required")
	}
	if gq == nil {
		return nil, errors.New("search.NewVisible: GraphQueryer is required")
	}
	return &Visible{inner: inner, gq: gq}, nil
}

func (v *Visible) SearchVisible(
	ctx context.Context, q Query, scope map[string]TypeScope,
) iter.Seq2[Hit, error] {
	return func(yield func(Hit, error) bool) {
		hits, err := v.visibleHits(ctx, q, scope)
		if err != nil {
			yield(Hit{}, err)
			return
		}
		emitted := 0
		for _, h := range hits {
			if q.Limit > 0 && emitted >= q.Limit {
				return
			}
			if !yield(h, nil) {
				return
			}
			emitted++
		}
	}
}

// visibleHits collects the full candidate stream and drops every hit
// the scope denies, preserving backend order. Collected (not streamed)
// because visibility probes are batched per type, which needs the hits
// grouped before any MatchingIDs call.
func (v *Visible) visibleHits(ctx context.Context, q Query, scope map[string]TypeScope) ([]Hit, error) {
	// Validate up front for parity with the pgstore-native impl: both
	// implementations reject an unsupported filter with the same
	// sentinel before any backend or scope work (conformance-pinned).
	if err := ValidateFilters(q.Filters); err != nil {
		return nil, err
	}
	if len(scope) == 0 {
		return nil, nil // nothing is visible; do not touch the backend
	}
	if ts, ok := scope[WildcardType]; ok && ts.Query != nil {
		return nil, fmt.Errorf("%w: wildcard scope entry cannot carry a GraphQuery", ErrScope)
	}

	inner := q
	inner.Limit = 0
	var hits []Hit
	for h, err := range v.inner.Search(ctx, inner) {
		if err != nil {
			// Plain search failure — deliberately NOT ErrScope.
			return nil, err
		}
		hits = append(hits, h)
	}

	allowed, err := v.allowedIDs(ctx, hits, scope)
	if err != nil {
		return nil, err
	}
	visible := hits[:0]
	for _, h := range hits {
		if allowed[h.ID] {
			visible = append(visible, h)
		}
	}
	return visible, nil
}

// allowedIDs resolves the scope verdict for every hit, batching
// MatchingIDs probes per entity type.
func (v *Visible) allowedIDs(ctx context.Context, hits []Hit, scope map[string]TypeScope) (map[string]bool, error) {
	byType := make(map[string][]string)
	for _, h := range hits {
		byType[h.Type] = append(byType[h.Type], h.ID)
	}

	allowed := make(map[string]bool, len(hits))
	for typ, ids := range byType {
		ts, ok := ResolveTypeScope(scope, typ)
		if !ok {
			continue // denied type: drop its hits
		}
		if ts.AllowAll {
			for _, id := range ids {
				allowed[id] = true
			}
			continue
		}
		m, err := v.gq.MatchingIDs(ctx, *ts.Query, ids)
		if err != nil {
			return nil, fmt.Errorf("%w: type %q: %w", ErrScope, typ, err)
		}
		for id, ok := range m {
			if ok {
				allowed[id] = true
			}
		}
	}
	return allowed, nil
}
