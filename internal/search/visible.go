package search

import (
	"context"
	"errors"
	"fmt"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// fieldProvenance reports which logical fields the query text matched for a
// given hit id. [Service] implements it; Visible takes the narrow interface so
// it stays decoupled from the concrete searcher. (found=false → the entity
// could not be loaded; treat as no provenance.)
type fieldProvenance interface {
	MatchedFields(ctx context.Context, id, text string) (fields map[string]struct{}, found bool)
}

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
	// prov is the optional match-provenance source used by
	// SearchVisibleFields; nil disables field-level filtering (entity-level
	// only). Set when inner also satisfies fieldProvenance (the Service case).
	prov fieldProvenance
}

// compile-time checks
var (
	_ VisibleSearcher      = (*Visible)(nil)
	_ FieldVisibleSearcher = (*Visible)(nil)
)

// NewVisible builds the generic scope-filtering wrapper around an
// existing searcher and the store's graph-query capability. When inner also
// reports match provenance (a *Service does), the field-visibility filter in
// SearchVisibleFields is enabled; otherwise that method degrades to
// entity-level filtering only.
func NewVisible(inner Searcher, gq store.GraphQueryer) (*Visible, error) {
	if inner == nil {
		return nil, errors.New("search.NewVisible: inner Searcher is required")
	}
	if gq == nil {
		return nil, errors.New("search.NewVisible: GraphQueryer is required")
	}
	v := &Visible{inner: inner, gq: gq}
	if p, ok := inner.(fieldProvenance); ok {
		v.prov = p
	}
	return v, nil
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

// SearchVisibleFields is SearchVisible plus property-level redaction of the
// search oracle: after entity-level scoping, a hit is dropped when every field
// its text matched is one the principal may not see (hidden per the consumer's
// HiddenFieldsFunc). A hit that matched a visible property — or matched the id
// or content, which are never property-gated — always survives. This closes
// the match-on-hidden-field oracle (a search that matched only a redacted
// property must not confirm that property's value by returning the entity).
//
// hidden is supplied by the consumer (the search package never sees a
// principal). A nil hidden func, or one returning an empty set, leaves the hit
// unaffected. Errors from hidden fail closed: the hit is dropped and the error
// surfaced, so an ACL resolution failure never widens visibility.
//
// When this Visible has no provenance source (inner is not a *Service), the
// field filter cannot run; the method degrades to entity-level SearchVisible.
// Callers that rely on field redaction for confidentiality must wire a
// provenance-capable searcher — enforced at the composition root, documented
// in the ACL security guide.
func (v *Visible) SearchVisibleFields(
	ctx context.Context, q Query, scope map[string]TypeScope, hidden HiddenFieldsFunc,
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
			keep, err := v.fieldVisible(ctx, q, h, hidden)
			if err != nil {
				yield(Hit{}, err)
				return
			}
			if !keep {
				continue // matched only on hidden fields — drop (oracle closed)
			}
			if !yield(h, nil) {
				return
			}
			emitted++
		}
	}
}

// fieldVisible decides whether hit h survives the property-level filter. It
// returns true (keep) when field filtering does not apply — no hidden func, no
// provenance source, no text query, or an empty hidden set — and otherwise
// keeps the hit only if at least one matched field is NOT hidden.
//
// Fail-closed points: a hidden-func error drops the hit (returns the error);
// and if the principal hides some fields but provenance is unavailable (entity
// vanished, or a non-provenance backend), the hit is dropped rather than
// risk leaking a hidden-only match.
func (v *Visible) fieldVisible(ctx context.Context, q Query, h Hit, hidden HiddenFieldsFunc) (bool, error) {
	if hidden == nil || v.prov == nil || q.Text == "" {
		return true, nil
	}
	hiddenFields, err := hidden(ctx, h)
	if err != nil {
		return false, fmt.Errorf("%w: hidden-fields for %q: %w", ErrScope, h.ID, err)
	}
	if len(hiddenFields) == 0 {
		return true, nil // nothing hidden for this principal on this hit
	}
	matched, found := v.prov.MatchedFields(ctx, h.ID, q.Text)
	if !found {
		// Entity unavailable: cannot prove the match came from a visible
		// field, so fail closed rather than leak a possible hidden-only match.
		return false, nil
	}
	for f := range matched {
		if _, isHidden := hiddenFields[f]; !isHidden {
			return true, nil // at least one visible field matched — keep
		}
	}
	return false, nil // every matched field is hidden — drop
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
