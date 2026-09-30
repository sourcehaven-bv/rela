package search

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// fieldMatchProvenance reports which logical fields the query text matched for
// an ALREADY-LOADED entity. [Service] implements it (delegating to the backend
// FieldMatcher, else the ground-truth substring matcher). Taking the entity —
// not an id — means the seam loads it once and both the provenance and the
// hidden-field verdict see the same snapshot.
type fieldMatchProvenance interface {
	MatchedFields(e *entity.Entity, text string) map[string]struct{}
}

// Visible is the generic VisibleSearcher: it wraps any Searcher and
// filters its hits through a store.GraphQueryer. This is the
// implementation for the simple backends (bleve, LinearSearch), which
// only ever pair with in-process stores where MatchingFaces is cheap.
//
// Candidates are fetched UNCAPPED (inner Limit 0) and q.Limit is
// applied after visibility filtering, per the VisibleSearcher
// contract. Note the bleve backend maps limit ≤ 0 to a practical
// 10000-candidate ceiling — within that window restricted principals
// get their true top-K; beyond it the bound is documented in the ACL
// security guide.
type Visible struct {
	inner  Searcher
	gq     store.GraphQueryer
	reader store.EntityReader
	// prov is the match-provenance source used by SearchVisibleFields; set
	// when inner satisfies fieldMatchProvenance (the Service case). When nil,
	// SearchVisibleFields FAILS CLOSED (yields ErrScope) rather than silently
	// skipping field redaction — a missing provenance source is a wiring bug,
	// not a reason to leak.
	prov fieldMatchProvenance
}

// compile-time checks
var (
	_ VisibleSearcher      = (*Visible)(nil)
	_ FieldVisibleSearcher = (*Visible)(nil)
)

// NewVisible builds the generic scope-filtering wrapper around an
// existing searcher and the store's graph-query capability. inner must also
// satisfy [store.EntityReader] and (for the field-visibility filter) report
// match provenance — a *Service does both. A non-provenance inner does not
// disable field redaction silently: SearchVisibleFields fails closed on it (a
// consumer that never calls SearchVisibleFields is unaffected).
//
// The reader defaults to inner when inner is a store.EntityReader; pass a
// distinct reader only in tests. Requiring the reader here means the single
// snapshot load lives at the seam, not in each consumer.
func NewVisible(inner Searcher, gq store.GraphQueryer) (*Visible, error) {
	if inner == nil {
		return nil, errors.New("search.NewVisible: inner Searcher is required")
	}
	if gq == nil {
		return nil, errors.New("search.NewVisible: GraphQueryer is required")
	}
	reader, ok := gq.(store.EntityReader)
	if !ok {
		return nil, errors.New("search.NewVisible: GraphQueryer must also be a store.EntityReader")
	}
	v := &Visible{inner: inner, gq: gq, reader: reader}
	if p, ok := inner.(fieldMatchProvenance); ok {
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
// If a hidden func is supplied but this Visible has no provenance source
// (inner is not a *Service), the method FAILS CLOSED — it yields ErrScope
// rather than silently returning un-redacted hits. A missing provenance source
// is a wiring bug (a Searcher wrapped so the type assertion in NewVisible
// misses), and silently skipping redaction is exactly the oracle this closes.
func (v *Visible) SearchVisibleFields(
	ctx context.Context, q Query, scope map[string]TypeScope, hidden HiddenFieldsFunc,
) iter.Seq2[Hit, error] {
	return func(yield func(Hit, error) bool) {
		if hidden != nil && q.Text != "" && v.prov == nil {
			yield(Hit{}, fmt.Errorf(
				"%w: field-visibility requested but the searcher reports no match provenance", ErrScope))
			return
		}
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
// returns true (keep) when field filtering does not apply — no hidden func or
// no text query — and otherwise keeps the hit only if at least one matched
// field is NOT hidden.
//
// The entity is loaded ONCE here and threaded through BOTH the hidden-field
// verdict and the match-provenance computation, so a concurrent write cannot
// make the two observe different snapshots (fail-closed stays snapshot-
// consistent). Fail-closed points: a stale/missing entity drops the hit (a
// deleted-but-still-indexed candidate cannot prove a visible-field match); a
// hidden-func error drops the hit and surfaces the error.
func (v *Visible) fieldVisible(ctx context.Context, q Query, h Hit, hidden HiddenFieldsFunc) (bool, error) {
	if hidden == nil || q.Text == "" {
		return true, nil
	}
	// The FACE that matched, not the bare id: the hidden-field verdict and the
	// match provenance must both be computed against the text the backend
	// actually scored. Reading the default face under a non-default world would
	// ask "which fields matched?" of bytes that were never searched — and the
	// answer decides whether the hit survives, so a mismatch drops hits the
	// principal is entitled to (or keeps ones the oracle should close). Under
	// the default world h.Face is zero and this is exactly GetEntity.
	e, err := v.reader.GetEntity(ctx, entity.Ref{ID: h.ID, Face: h.Face})
	if err != nil {
		// Stale hit: entity vanished between indexing and now. Cannot prove the
		// match came from a visible field → drop (fail closed).
		return false, nil //nolint:nilerr // stale index hit is dropped, not an error
	}
	hiddenFields, herr := hidden(ctx, h, e)
	if herr != nil {
		return false, fmt.Errorf("%w: hidden-fields for %q: %w", ErrScope, h.ID, herr)
	}
	if len(hiddenFields) == 0 {
		return true, nil // nothing hidden for this principal on this hit
	}
	matched := v.prov.MatchedFields(e, q.Text)
	return MatchHasVisibleField(matched, hiddenFields), nil
}

// MatchHasVisibleField reports whether at least one matched field is NOT in the
// hidden set — i.e. the hit matched something the principal may see, so it must
// survive. [FieldID] and [FieldContent] are never property-gated, so a match on
// either always counts as visible regardless of the hidden set: this guards the
// documented never-gated invariant even if a caller mis-supplies "id"/"content"
// in the hidden set (a false drop would silently vanish an entity from search).
//
// Shared by the generic and pgstore-native filters so both apply the invariant
// identically (conformance-pinned).
func MatchHasVisibleField(matched, hidden map[string]struct{}) bool {
	for f := range matched {
		if f == FieldID || f == FieldContent {
			return true
		}
		if _, isHidden := hidden[f]; !isHidden {
			return true
		}
	}
	return false
}

// visibleHits runs the search with the scope as its admission step and keeps
// the hits whose face was admitted, preserving backend order.
//
// The scope trims each entity's candidate faces BEFORE the world ranks them
// ([Query.Admit]), as on lists and the single-entity read: a principal
// whose grant denies the prime is served the next readable face rather than
// losing the entity. The hit filter afterwards is the fail-closed half: a
// hit whose face was never admitted (an inner searcher that ignores Admit)
// is dropped, never served.
func (v *Visible) visibleHits(ctx context.Context, q Query, scope map[string]TypeScope) ([]Hit, error) {
	// Validate up front for parity with the pgstore-native impl: both
	// implementations reject an unsupported filter with the same
	// sentinel before any backend or scope work (conformance-pinned).
	if err := ValidateQuery(q); err != nil {
		return nil, err
	}
	if len(scope) == 0 {
		return nil, nil // nothing is visible; do not touch the backend
	}
	if err := ValidateScope(scope); err != nil {
		return nil, err
	}

	adm := &admission{ctx: ctx, gq: v.gq, scope: scope, admitted: make(map[faceKey]struct{})}
	inner := q
	inner.Limit = 0
	inner.Admit = adm.admit
	var hits []Hit
	for h, err := range v.inner.Search(ctx, inner) {
		if err != nil {
			// A plain search failure stays a plain failure; an admission
			// failure arrives already wrapped in ErrScope.
			return nil, err
		}
		hits = append(hits, h)
	}

	visible := hits[:0]
	for _, h := range hits {
		if _, ok := adm.admitted[faceKey{id: h.ID, face: h.Face}]; ok {
			visible = append(visible, h)
		}
	}
	return visible, nil
}

// admission is the [AdmitFunc] a [Visible] search runs with, and the record
// of what it admitted.
type admission struct {
	ctx      context.Context //nolint:containedctx // one search's admission step; never outlives it
	gq       store.GraphQueryer
	scope    map[string]TypeScope
	admitted map[faceKey]struct{}
}

// admit keeps the candidates the scope admits. A Query verdict is evaluated
// per face row with one MatchingFaces probe per entity type, over every
// face and trimmed to the scope's face allowlist, so its cost does not grow
// with the number of candidates. The probe reads the rows the ACL gate
// reads (acl.Request.ReadableFacesMany), so search and the bare-id read
// agree per face.
func (a *admission) admit(cands []Candidate) ([]Candidate, error) {
	byType := make(map[string][]string)
	seen := make(map[string]bool, len(cands))
	for _, c := range cands {
		if seen[c.ID] {
			continue
		}
		seen[c.ID] = true
		byType[c.Type] = append(byType[c.Type], c.ID)
	}
	matched := make(map[string]map[string][]entity.Face, len(byType))
	for typ, ids := range byType {
		ts, ok := ResolveTypeScope(a.scope, typ)
		if !ok || ts.AllowAll {
			continue
		}
		probe := *ts.Query // copy: the scope is shared across requests
		probe.Faces = store.AllFaces()
		probe.FaceIn = ts.Faces
		m, err := a.gq.MatchingFaces(a.ctx, probe, ids)
		if err != nil {
			return nil, fmt.Errorf("%w: type %q: %w", ErrScope, typ, err)
		}
		matched[typ] = m
	}
	out := make([]Candidate, 0, len(cands))
	for _, c := range cands {
		ts, ok := ResolveTypeScope(a.scope, c.Type)
		if !ok {
			continue // denied type
		}
		if ts.AllowAll {
			if !ts.AdmitsFace(c.Face) {
				continue
			}
		} else if !slices.Contains(matched[c.Type][c.ID], c.Face) {
			continue
		}
		a.admitted[faceKey{id: c.ID, face: c.Face}] = struct{}{}
		out = append(out, c)
	}
	return out, nil
}
