package search

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"
	"sync/atomic"

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
	// prov is the match-provenance source the field check uses; set when
	// inner satisfies fieldMatchProvenance (the Service case). When nil, a
	// search over a field-gated type FAILS CLOSED (yields ErrScope) rather
	// than silently skipping the check: a missing provenance source is a
	// wiring bug, not a reason to leak.
	prov fieldMatchProvenance
}

// compile-time check
var _ VisibleSearcher = (*Visible)(nil)

// NewVisible builds the generic scope-filtering wrapper around an
// existing searcher and the store's graph-query capability. inner must also
// satisfy [store.EntityReader] and (for the field check) report match
// provenance — a *Service does both. A non-provenance inner does not disable
// the field check silently: a search over a field-gated type fails closed on
// it (a scope without [TypeScope.Fields] is unaffected).
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

// SearchVisible yields the hits the scope admits, in backend order, at most
// q.Limit of them. A type whose scope entry carries [TypeScope.Fields] keeps a
// hit only when [FieldScope.AdmitsMatch] does, so a search cannot confirm a
// hidden property's value by returning its entity; the limit counts the hits
// that survive.
//
// It first collects every hit the scope admits, uncapped (see [Visible] for
// the backend's own candidate bound), so memory grows with that hit list. The
// field check then runs on it in pages of 256 hits, and stops once q.Limit
// hits survived. A page costs one entity read, and one
// [store.GraphQueryer.MatchingFaces] per conditional grant per type in the
// page, so the reads per page do not grow with the number of rows.
//
// It fails closed (ErrScope) when a gated type needs the match provenance and
// inner reports none: skipping the check would reopen the oracle.
func (v *Visible) SearchVisible(
	ctx context.Context, q Query, scope map[string]TypeScope,
) iter.Seq2[Hit, error] {
	return func(yield func(Hit, error) bool) {
		hits, err := v.visibleHits(ctx, q, scope)
		if err != nil {
			yield(Hit{}, err)
			return
		}
		gated := q.Text != "" && anyFieldScope(scope)
		if gated && v.prov == nil {
			yield(Hit{}, fmt.Errorf(
				"%w: field visibility requested but the searcher reports no match provenance", ErrScope))
			return
		}
		emitted := 0
		pageSize := int(fieldCheckPage.Load())
		for start := 0; start < len(hits); start += pageSize {
			page := hits[start:min(start+pageSize, len(hits))]
			if gated {
				if page, err = v.fieldVisible(ctx, q, scope, page); err != nil {
					yield(Hit{}, err)
					return
				}
			}
			for _, h := range page {
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
}

// fieldCheckPage is how many hits one field check judges together. It is a
// variable only so a test can shrink it and see the per-page cost.
var fieldCheckPage = func() *atomic.Int64 {
	var n atomic.Int64
	n.Store(defaultFieldCheckPage)
	return &n
}()

// defaultFieldCheckPage is the production value of fieldCheckPage.
const defaultFieldCheckPage = 256

// anyFieldScope reports whether some entry gates properties.
func anyFieldScope(scope map[string]TypeScope) bool {
	for _, ts := range scope {
		if ts.Fields != nil {
			return true
		}
	}
	return false
}

// fieldVisible keeps the hits of page that [FieldScope.AdmitsMatch] keeps,
// in order.
//
// Each hit is judged on the FACE that matched, not the bare id: the verdict
// and the match provenance must both be computed against the text the backend
// scored. The rows are read once, and that snapshot feeds both the provenance
// and the grant conditions. A hit whose row is gone (a stale index entry) is
// dropped, since nothing can prove its match came from a visible field.
func (v *Visible) fieldVisible(ctx context.Context, q Query, scope map[string]TypeScope, page []Hit) ([]Hit, error) {
	idsByType := map[string][]string{}
	for _, h := range page {
		if ts, ok := scope[h.Type]; ok && ts.Fields != nil {
			idsByType[h.Type] = append(idsByType[h.Type], h.ID)
		}
	}
	if len(idsByType) == 0 {
		return page, nil
	}
	var ids []string
	for _, typeIDs := range idsByType {
		ids = append(ids, typeIDs...)
	}
	rows := make(map[entity.Ref]*entity.Entity, len(ids))
	for e, err := range v.reader.ListEntities(ctx, store.EntityQuery{IDs: ids, Faces: store.AllFaces()}) {
		if err != nil {
			return nil, fmt.Errorf("%w: field visibility: %w", ErrScope, err)
		}
		rows[e.Ref()] = e
	}
	holds, err := v.grantRows(ctx, scope, idsByType)
	if err != nil {
		return nil, err
	}
	out := page[:0:0]
	for _, h := range page {
		ts := scope[h.Type]
		if ts.Fields == nil {
			out = append(out, h)
			continue
		}
		ref := entity.Ref{ID: h.ID, Face: h.Face}
		e, ok := rows[ref]
		if !ok {
			continue
		}
		matched := v.prov.MatchedFields(e, q.Text)
		if ts.Fields.AdmitsMatch(matched, func(i int) bool { return holds[h.Type][i][ref] }) {
			out = append(out, h)
		}
	}
	return out, nil
}

// grantRows answers, for each conditional field grant of each type on the
// page, which face rows its When matches: holds[type][grant] is that set.
// One MatchingFaces call per conditional grant per type, whatever the page
// size.
func (v *Visible) grantRows(
	ctx context.Context, scope map[string]TypeScope, idsByType map[string][]string,
) (map[string]map[int]map[entity.Ref]bool, error) {
	holds := map[string]map[int]map[entity.Ref]bool{}
	for typ, typeIDs := range idsByType {
		holds[typ] = map[int]map[entity.Ref]bool{}
		for i, g := range scope[typ].Fields.Grants {
			if g.When == nil {
				continue
			}
			probe := *g.When // copy: the scope is shared across the request
			probe.EntityType = typ
			probe.Faces = store.AllFaces()
			m, err := v.gq.MatchingFaces(ctx, probe, typeIDs)
			if err != nil {
				return nil, fmt.Errorf("%w: field grant on %q: %w", ErrScope, typ, err)
			}
			set := map[entity.Ref]bool{}
			for id, faces := range m {
				for _, f := range faces {
					set[entity.Ref{ID: id, Face: f}] = true
				}
			}
			holds[typ][i] = set
		}
	}
	return holds, nil
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
	if callerAdmit := q.Admit; callerAdmit != nil {
		// A caller's own admission narrows the scope's further; it never
		// replaces it.
		inner.Admit = func(c []Candidate) ([]Candidate, error) {
			admitted, err := adm.admit(c)
			if err != nil {
				return nil, err
			}
			return callerAdmit(admitted)
		}
	}
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
