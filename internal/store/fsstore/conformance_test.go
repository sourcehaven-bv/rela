package fsstore_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

func factory(t *testing.T) store.Store {
	t.Helper()
	s, err := fsstore.New(newConfig(storage.NewMemFS()))
	require.NoError(t, err)
	return s
}

func fuzzFactory() store.Store {
	s, err := fsstore.New(newConfig(storage.NewMemFS()))
	if err != nil {
		panic(err)
	}
	return s
}

func searchFactory(t *testing.T) (store.Store, search.Searcher) {
	t.Helper()
	idx := search.NewLinearSearch()
	cfg := newConfig(storage.NewMemFS())
	cfg.Observers = []store.EntityObserver{idx}
	s, err := fsstore.New(cfg)
	require.NoError(t, err)
	return s, search.New(s, idx)
}

func TestConformance(t *testing.T) {
	storetest.RunAll(t, factory, searchFactory, storetest.Capabilities{Attachments: true})
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
