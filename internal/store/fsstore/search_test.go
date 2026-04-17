package fsstore_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/bleveindex"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openSearchStore(t *testing.T) *fsstore.FSStore {
	t.Helper()
	idx, err := bleveindex.NewMem()
	require.NoError(t, err)

	s, err := fsstore.New(fsstore.Config{
		FS:             storage.NewMemFS(),
		EntitiesDir:    "/entities",
		RelationsDir:   "/relations",
		AttachmentsDir: "/attachments",
		CacheDir:       "/.rela",
		SearchIndex:    idx,
	})
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSearchIndex_TextSearch(t *testing.T) {
	s := openSearchStore(t)
	ctx := context.Background()

	e1 := entity.New("REQ-1", "requirement")
	e1.SetString("title", "User authentication")
	e1.Content = "Users log in with email."
	require.NoError(t, s.CreateEntity(ctx, e1))

	e2 := entity.New("REQ-2", "requirement")
	e2.SetString("title", "Data export")
	e2.Content = "Export data as CSV."
	require.NoError(t, s.CreateEntity(ctx, e2))

	var results []*entity.Entity
	for e, err := range s.Search(ctx, store.SearchQuery{Text: "authentication"}) {
		require.NoError(t, err)
		results = append(results, e)
	}

	assert.Len(t, results, 1)
	assert.Equal(t, "REQ-1", results[0].ID)
}

func TestSearchIndex_TextWithTypeFilter(t *testing.T) {
	s := openSearchStore(t)
	ctx := context.Background()

	e1 := entity.New("REQ-1", "requirement")
	e1.SetString("title", "Common keyword")
	require.NoError(t, s.CreateEntity(ctx, e1))

	e2 := entity.New("T-1", "ticket")
	e2.SetString("title", "Common keyword")
	require.NoError(t, s.CreateEntity(ctx, e2))

	var results []*entity.Entity
	for e, err := range s.Search(ctx, store.SearchQuery{Text: "common", Types: []string{"ticket"}}) {
		require.NoError(t, err)
		results = append(results, e)
	}

	assert.Len(t, results, 1)
	assert.Equal(t, "T-1", results[0].ID)
}

func TestSearchIndex_TextWithPropertyFilter(t *testing.T) {
	s := openSearchStore(t)
	ctx := context.Background()

	e1 := entity.New("REQ-1", "requirement")
	e1.SetString("title", "Searchable item")
	e1.SetString("status", "open")
	require.NoError(t, s.CreateEntity(ctx, e1))

	e2 := entity.New("REQ-2", "requirement")
	e2.SetString("title", "Searchable item")
	e2.SetString("status", "closed")
	require.NoError(t, s.CreateEntity(ctx, e2))

	var results []*entity.Entity
	for e, err := range s.Search(ctx, store.SearchQuery{
		Text:    "searchable",
		Filters: []store.PropertyFilter{{Property: "status", Value: "open", Op: store.FilterEq}},
	}) {
		require.NoError(t, err)
		results = append(results, e)
	}

	assert.Len(t, results, 1)
	assert.Equal(t, "REQ-1", results[0].ID)
}

func TestSearchIndex_UpdateReflectedInSearch(t *testing.T) {
	s := openSearchStore(t)
	ctx := context.Background()

	e := entity.New("REQ-1", "requirement")
	e.SetString("title", "Original title")
	require.NoError(t, s.CreateEntity(ctx, e))

	e.SetString("title", "Replaced title")
	require.NoError(t, s.UpdateEntity(ctx, e))

	// Old term should not match.
	var results []*entity.Entity
	for ent, err := range s.Search(ctx, store.SearchQuery{Text: "Original"}) {
		require.NoError(t, err)
		results = append(results, ent)
	}
	assert.Empty(t, results)

	// New term should match.
	results = nil
	for ent, err := range s.Search(ctx, store.SearchQuery{Text: "Replaced"}) {
		require.NoError(t, err)
		results = append(results, ent)
	}
	assert.Len(t, results, 1)
}

func TestSearchIndex_DeleteRemovesFromSearch(t *testing.T) {
	s := openSearchStore(t)
	ctx := context.Background()

	e := entity.New("REQ-1", "requirement")
	e.SetString("title", "Deletable thing")
	require.NoError(t, s.CreateEntity(ctx, e))

	_, err := s.DeleteEntity(ctx, "REQ-1", false)
	require.NoError(t, err)

	var results []*entity.Entity
	for ent, err := range s.Search(ctx, store.SearchQuery{Text: "Deletable"}) {
		require.NoError(t, err)
		results = append(results, ent)
	}
	assert.Empty(t, results)
}

func TestSearchIndex_RenameUpdatesSearch(t *testing.T) {
	s := openSearchStore(t)
	ctx := context.Background()

	e := entity.New("REQ-OLD", "requirement")
	e.SetString("title", "Renameable entity")
	require.NoError(t, s.CreateEntity(ctx, e))

	_, err := s.RenameEntity(ctx, "REQ-OLD", "REQ-NEW")
	require.NoError(t, err)

	var results []*entity.Entity
	for ent, err := range s.Search(ctx, store.SearchQuery{Text: "Renameable"}) {
		require.NoError(t, err)
		results = append(results, ent)
	}
	require.Len(t, results, 1)
	assert.Equal(t, "REQ-NEW", results[0].ID)
}

func TestSearchIndex_FallbackWithoutIndex(t *testing.T) {
	// Without a SearchIndex, text search falls back to linear scan.
	s, err := fsstore.New(fsstore.Config{
		FS:             storage.NewMemFS(),
		EntitiesDir:    "/entities",
		RelationsDir:   "/relations",
		AttachmentsDir: "/attachments",
		CacheDir:       "/.rela",
	})
	require.NoError(t, err)
	defer s.Close()

	ctx := context.Background()

	e := entity.New("REQ-1", "requirement")
	e.SetString("title", "Findable item")
	require.NoError(t, s.CreateEntity(ctx, e))

	var results []*entity.Entity
	for ent, err := range s.Search(ctx, store.SearchQuery{Text: "Findable"}) {
		require.NoError(t, err)
		results = append(results, ent)
	}
	assert.Len(t, results, 1)
	assert.Equal(t, "REQ-1", results[0].ID)
}
