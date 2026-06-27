package memstore_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

func factory(t *testing.T) store.Store {
	t.Helper()
	return memstore.New()
}

func searchFactory(t *testing.T) (store.Store, search.Searcher) {
	t.Helper()
	idx := search.NewLinearSearch()
	s := memstore.New(memstore.WithObserver(idx))
	return s, search.New(s, idx)
}

// visibleSearchFactory derives the generic scope-filtering wrapper from
// the same store+searcher pair, per the TKT-BA8BSX wiring rule: simple
// backends get search.NewVisible, smart backends implement natively.
func visibleSearchFactory(t *testing.T) (store.Store, search.Searcher, search.VisibleSearcher) {
	t.Helper()
	s, searcher := searchFactory(t)
	v, err := search.NewVisible(searcher, s)
	if err != nil {
		t.Fatalf("NewVisible: %v", err)
	}
	return s, searcher, v
}

func fuzzFactory() store.Store {
	return memstore.New()
}

func TestConformance(t *testing.T) {
	storetest.RunAll(t, factory, searchFactory, visibleSearchFactory, storetest.Capabilities{Attachments: true})
}

func FuzzRelationKeyCollision(f *testing.F) {
	storetest.FuzzRelationKeyCollision(f, fuzzFactory)
}

func FuzzAttachmentKeyCollision(f *testing.F) {
	storetest.FuzzAttachmentKeyCollision(f, fuzzFactory)
}

func FuzzRenameKeyCollapse(f *testing.F) {
	storetest.FuzzRenameKeyCollapse(f, fuzzFactory)
}

func FuzzConcurrentOps(f *testing.F) {
	storetest.FuzzConcurrentOps(f, fuzzFactory)
}

func FuzzCloneNestedValues(f *testing.F) {
	storetest.FuzzCloneNestedValues(f, fuzzFactory)
}

func FuzzPropertyValuesTypeZoo(f *testing.F) {
	storetest.FuzzPropertyValuesTypeZoo(f, fuzzFactory)
}
