package search_test

import (
	"testing"

	"github.com/stretchr/testify/require"

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
