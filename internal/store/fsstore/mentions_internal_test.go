package fsstore

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// reopen opens a second store over fs, as the next process would.
func reopen(t *testing.T, fs *storage.MemFS) *FSStore {
	t.Helper()
	rooted, err := storage.NewRootedFS(fs, "/")
	require.NoError(t, err)
	s, err := New(Config{
		FS: fs, Rooted: rooted, EntitiesKey: "entities", RelationsKey: "relations", CacheKey: ".rela",
		Schemas: map[string]store.EntityTypeSchema{
			"requirement": {Plural: "requirements", PropertyOrder: []string{"title", "status"}},
		},
	})
	require.NoError(t, err)
	return s
}

func mentionSources(t *testing.T, s *FSStore, target string) []entity.Ref {
	t.Helper()
	refs, err := s.MentionSources(context.Background(), target, entity.Ref{}, 10)
	require.NoError(t, err)
	return refs
}

// TestMentions_ExternalEditUpdatesIndex: an edit made outside rela reaches
// the built index through the watcher, both adding and dropping a mention.
func TestMentions_ExternalEditUpdatesIndex(t *testing.T) {
	s, fs := newTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateEntity(ctx, &entity.Entity{ID: "T-1", Type: "ticket"}))
	require.NoError(t, s.CreateEntity(ctx, &entity.Entity{ID: "T-2", Type: "ticket"}))
	require.Empty(t, mentionSources(t, s, "T-1")) // builds the index

	path := "/entities/tickets/T-2.md"
	write := func(body string) {
		require.NoError(t, fs.WriteFileExternal(path, []byte("---\nid: T-2\ntype: ticket\n---\n\n"+body+"\n"), 0o644))
		s.handleExternalEvents([]storage.ChangeEvent{{Path: path, Op: storage.OpModify}})
	}
	write("See `T-1`.")
	require.Equal(t, []entity.Ref{{ID: "T-2"}}, mentionSources(t, s, "T-1"))
	write("No longer.")
	require.Empty(t, mentionSources(t, s, "T-1"))
}

// TestMentions_StaleCacheIsReread: a file changed since the cache was
// written is read again by the next process rather than trusted.
func TestMentions_StaleCacheIsReread(t *testing.T) {
	s, fs := newTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateEntity(ctx, &entity.Entity{ID: "T-1", Type: "requirement"}))
	require.NoError(t, s.CreateEntity(ctx, &entity.Entity{ID: "T-2", Type: "requirement", Content: "See `T-1`."}))
	require.Len(t, mentionSources(t, s, "T-1"), 1) // builds and persists the cache

	require.NoError(t, fs.WriteFileExternal("/entities/requirements/T-2.md",
		[]byte("---\nid: T-2\ntype: requirement\n---\n\nNow about `T-3` instead.\n"), 0o644))
	next := reopen(t, fs)
	require.Empty(t, mentionSources(t, next, "T-1"))
	require.Equal(t, []entity.Ref{{ID: "T-2"}}, mentionSources(t, next, "T-3"))
}

// TestMentions_CacheReusedForUnchangedFile: an entry whose file stamp still
// matches is taken from the cache without reading the file. The planted
// target proves which side answered.
func TestMentions_CacheReusedForUnchangedFile(t *testing.T) {
	s, fs := newTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateEntity(ctx, &entity.Entity{ID: "T-1", Type: "requirement", Content: "See `T-9`."}))
	require.Len(t, mentionSources(t, s, "T-9"), 1)

	data, err := fs.ReadFile("/.rela/" + mentionIndexFile)
	require.NoError(t, err)
	var entries map[string]mentionEntry
	require.NoError(t, json.Unmarshal(data, &entries))
	ent := entries["T-1"]
	ent.Targets = []string{"PLANTED-1"}
	entries["T-1"] = ent
	data, err = json.Marshal(entries)
	require.NoError(t, err)
	require.NoError(t, fs.WriteFileExternal("/.rela/"+mentionIndexFile, data, 0o644))

	next := reopen(t, fs)
	require.Equal(t, []entity.Ref{{ID: "T-1"}}, mentionSources(t, next, "PLANTED-1"))
}

// TestMentions_WriteDuringBuildIsKept: the build reads files without the
// store lock, so a write that lands while it runs must survive the merge.
// Run under -race this also checks that the build shares no state unlocked.
func TestMentions_WriteDuringBuildIsKept(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	for i := range 200 {
		require.NoError(t, s.CreateEntity(ctx, &entity.Entity{ID: fmt.Sprintf("T-%03d", i), Type: "ticket"}))
	}
	done := make(chan error)
	go func() {
		var err error
		for i := range 50 {
			e := &entity.Entity{ID: fmt.Sprintf("T-%03d", i), Type: "ticket", Content: "`R-1`"}
			if err = s.UpdateEntity(ctx, e); err != nil {
				break
			}
		}
		done <- err
	}()
	_ = mentionSources(t, s, "R-1")
	require.NoError(t, <-done)
	refs, err := s.MentionSources(ctx, "R-1", entity.Ref{}, 100)
	require.NoError(t, err)
	require.Len(t, refs, 50)
}
