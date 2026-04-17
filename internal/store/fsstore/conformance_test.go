package fsstore_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/stretchr/testify/require"
)

func factory(t *testing.T) store.Store {
	t.Helper()
	fs := storage.NewMemFS()
	s, err := fsstore.New(fsstore.Config{
		FS:             fs,
		EntitiesDir:    "/entities",
		RelationsDir:   "/relations",
		AttachmentsDir: "/attachments",
		CacheDir:       "/.rela",
	})
	require.NoError(t, err)
	return s
}

func fuzzFactory() store.Store {
	fs := storage.NewMemFS()
	s, err := fsstore.New(fsstore.Config{
		FS:             fs,
		EntitiesDir:    "/entities",
		RelationsDir:   "/relations",
		AttachmentsDir: "/attachments",
		CacheDir:       "/.rela",
	})
	if err != nil {
		panic(err)
	}
	return s
}

func searchFactory(t *testing.T) (store.Store, store.Searcher) {
	t.Helper()
	fs := storage.NewMemFS()
	idx := search.NewLinearSearch()
	s, err := fsstore.New(fsstore.Config{
		FS:             fs,
		EntitiesDir:    "/entities",
		RelationsDir:   "/relations",
		AttachmentsDir: "/attachments",
		CacheDir:       "/.rela",
		Observers:      []store.EntityObserver{idx},
	})
	require.NoError(t, err)
	return s, search.New(s, idx)
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
