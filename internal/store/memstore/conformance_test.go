package memstore_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storesearch"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

func factory(t *testing.T) store.Store {
	t.Helper()
	return memstore.New()
}

func searchFactory(t *testing.T) (store.Store, store.Searcher) {
	t.Helper()
	idx := fsstore.NewLinearSearch()
	s := memstore.New(memstore.WithObserver(idx))
	return s, storesearch.New(s, idx)
}

func fuzzFactory() store.Store {
	return memstore.New()
}

func TestConformance(t *testing.T) {
	storetest.RunAll(t, factory, searchFactory)
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
