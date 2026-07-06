package search_test

import (
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/search/bleveindex"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// TestVisibleConformance_Bleve runs the VisibleSearcher conformance
// suite over the generic wrapper with the bleve backend — the
// combination the default (fsstore) build ships. The linear-backend
// runs live in the memstore/fsstore conformance tests; pgstore's
// native implementation has its own DB-gated run.
func TestVisibleConformance_Bleve(t *testing.T) {
	storetest.RunVisibleSearchTests(t, func(t *testing.T) (store.Store, search.Searcher, search.VisibleSearcher) {
		t.Helper()
		idx, err := bleveindex.NewMem()
		require.NoError(t, err)
		t.Cleanup(func() { _ = idx.Close() })
		s := memstore.New(memstore.WithObserver(idx))
		searcher := search.New(s, idx)
		v, err := search.NewVisible(searcher, s)
		require.NoError(t, err)
		return s, searcher, v
	})
}

// TestVisibleFieldConformance_Bleve runs the property-level (match-on-hidden-
// field) suite over the generic wrapper + bleve backend, exercising bleve's
// own MatchedFields provenance path (TKT-GGQ0JT).
func TestVisibleFieldConformance_Bleve(t *testing.T) {
	storetest.RunVisibleFieldSearchTests(t, func(t *testing.T) (store.Store, search.Searcher, search.FieldVisibleSearcher) {
		t.Helper()
		idx, err := bleveindex.NewMem()
		require.NoError(t, err)
		t.Cleanup(func() { _ = idx.Close() })
		s := memstore.New(memstore.WithObserver(idx))
		searcher := search.New(s, idx)
		v, err := search.NewVisible(searcher, s)
		require.NoError(t, err)
		return s, searcher, v
	})
}

// TestVisibleFieldConformance_Linear runs the property-level suite over the
// generic wrapper + LinearSearch backend — the ground-truth matcher the other
// backends are checked against.
func TestVisibleFieldConformance_Linear(t *testing.T) {
	storetest.RunVisibleFieldSearchTests(t, func(t *testing.T) (store.Store, search.Searcher, search.FieldVisibleSearcher) {
		t.Helper()
		ls := search.NewLinearSearch()
		s := memstore.New(memstore.WithObserver(ls))
		searcher := search.New(s, ls)
		v, err := search.NewVisible(searcher, s)
		require.NoError(t, err)
		return s, searcher, v
	})
}

func TestNewVisible_RejectsNil(t *testing.T) {
	s := memstore.New()
	searcher := search.New(s, search.NewLinearSearch())

	if _, err := search.NewVisible(nil, s); err == nil {
		t.Error("nil inner Searcher accepted")
	}
	if _, err := search.NewVisible(searcher, nil); err == nil {
		t.Error("nil GraphQueryer accepted")
	}
}

// nonProvenanceSearcher wraps a Searcher WITHOUT forwarding match provenance —
// standing in for any decorator (metrics, caching, tracing) that a future
// refactor might slip between the Service and NewVisible. It must NOT satisfy
// the unexported fieldMatchProvenance interface.
type nonProvenanceSearcher struct{ inner search.Searcher }

func (n nonProvenanceSearcher) Search(ctx context.Context, q search.Query) iter.Seq2[search.Hit, error] {
	return n.inner.Search(ctx, q)
}

// TestSearchVisibleFields_FailsClosedWithoutProvenance is the regression guard
// for the reviewer's finding #1: if the wired searcher can't report match
// provenance, SearchVisibleFields must FAIL CLOSED (yield ErrScope), never
// silently return un-redacted hits. A wrapped Service loses the provenance
// type assertion — this proves that path errors instead of leaking.
func TestSearchVisibleFields_FailsClosedWithoutProvenance(t *testing.T) {
	s := memstore.New()
	e := entity.New("TKT-1", "ticket")
	e.SetString("title", "alpha rocket")
	e.SetString("code", "zeta777")
	if err := s.CreateEntity(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	ls := search.NewLinearSearch()
	_ = ls.EntityPut(e)

	wrapped := nonProvenanceSearcher{inner: search.New(s, ls)}
	v, err := search.NewVisible(wrapped, s)
	if err != nil {
		t.Fatal(err)
	}

	hide := func(_ context.Context, _ search.Hit, _ *entity.Entity) (map[string]struct{}, error) {
		return map[string]struct{}{search.PropFieldPrefix + "code": {}}, nil
	}
	scope := map[string]search.TypeScope{"ticket": {AllowAll: true}}

	var streamErr error
	hits := 0
	for _, iterErr := range v.SearchVisibleFields(
		context.Background(), search.Query{Text: "zeta777"}, scope, hide) {
		if iterErr != nil {
			streamErr = iterErr
			break
		}
		hits++
	}
	if hits != 0 {
		t.Errorf("leak: %d hits returned without provenance; want fail-closed", hits)
	}
	if !errors.Is(streamErr, search.ErrScope) {
		t.Errorf("want ErrScope on missing provenance, got %v", streamErr)
	}
}
