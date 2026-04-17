package fsstore_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Orphaned temp file cleanup ---

func TestRecovery_OrphanedEntityTempFile(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	// Create an entity normally.
	s1 := openStore(t, fs)
	require.NoError(t, s1.CreateEntity(ctx, entity.New("REQ-1", "requirement")))
	require.NoError(t, s1.Close())

	// Simulate a crash that left a .new temp file behind.
	require.NoError(t, fs.WriteFile("/entities/requirements/REQ-2.md.new", []byte("partial write"), 0644))

	// Reopen — the temp file should be cleaned up.
	s2 := openStore(t, fs)
	defer s2.Close()

	_, err := fs.ReadFile("/entities/requirements/REQ-2.md.new")
	assert.True(t, isNotExist(err), "orphaned .new file should be cleaned up on startup")

	// Original entity still accessible.
	_, err = s2.GetEntity(ctx, "REQ-1")
	assert.NoError(t, err)
}

func TestRecovery_OrphanedRelationTempFile(t *testing.T) {
	fs := storage.NewMemFS()

	s1 := openStore(t, fs)
	require.NoError(t, s1.Close())

	// Simulate orphaned relation temp file.
	require.NoError(t, fs.MkdirAll("/relations", 0755))
	require.NoError(t, fs.WriteFile("/relations/A--rel--B.md.new", []byte("partial"), 0644))

	s2 := openStore(t, fs)
	defer s2.Close()

	_, err := fs.ReadFile("/relations/A--rel--B.md.new")
	assert.True(t, isNotExist(err), "orphaned relation .new file should be cleaned up")
}

// --- Crash mid-create: file on disk but index not updated ---

func TestRecovery_EntityFileWithoutIndex(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	// Start with empty store.
	s1 := openStore(t, fs)
	require.NoError(t, s1.Close())

	// Simulate crash after file written but before the in-memory index was updated
	// (equivalent to an externally created file). On reopen, directory scan picks it up.
	require.NoError(t, fs.MkdirAll("/entities/requirements", 0755))
	require.NoError(t, fs.WriteFile("/entities/requirements/REQ-1.md", []byte(`---
id: REQ-1
type: requirement
title: Orphaned entity
---
`), 0644))

	s2 := openStore(t, fs)
	defer s2.Close()

	got, err := s2.GetEntity(ctx, "REQ-1")
	require.NoError(t, err)
	assert.Equal(t, "Orphaned entity", got.Properties["title"])
}

func TestRecovery_RelationFileWithoutIndex(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	// Create entities.
	s1 := openStore(t, fs)
	require.NoError(t, s1.CreateEntity(ctx, entity.New("A-1", "artifact")))
	require.NoError(t, s1.CreateEntity(ctx, entity.New("A-2", "artifact")))
	require.NoError(t, s1.Close())

	// Simulate orphaned relation file (written but index not updated before crash).
	require.NoError(t, fs.MkdirAll("/relations", 0755))
	require.NoError(t, fs.WriteFile("/relations/A-1--depends--A-2.md", []byte(`---
from: A-1
relation: depends
to: A-2
---
`), 0644))

	s2 := openStore(t, fs)
	defer s2.Close()

	rel, err := s2.GetRelation(ctx, "A-1", "depends", "A-2")
	require.NoError(t, err)
	assert.Equal(t, "A-1", rel.From)
}

// --- Crash mid-rename: both old and new entity files exist ---

func TestRecovery_PartialRename_BothFilesExist(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	// Create entity and relation.
	s1 := openStore(t, fs)
	e := entity.New("REQ-OLD", "requirement")
	e.Properties["title"] = "Important"
	require.NoError(t, s1.CreateEntity(ctx, e))
	require.NoError(t, s1.CreateEntity(ctx, entity.New("SOL-1", "solution")))
	_, err := s1.CreateRelation(ctx, "SOL-1", "implements", "REQ-OLD", nil)
	require.NoError(t, err)
	require.NoError(t, s1.Close())

	// Simulate crash mid-rename: new entity file written but old not yet deleted.
	require.NoError(t, fs.WriteFile("/entities/requirements/REQ-NEW.md", []byte(`---
id: REQ-NEW
type: requirement
title: Important
---
`), 0644))

	// Old relation file still references REQ-OLD. New relation file also written.
	require.NoError(t, fs.WriteFile("/relations/SOL-1--implements--REQ-NEW.md", []byte(`---
from: SOL-1
relation: implements
to: REQ-NEW
---
`), 0644))

	// Reopen — both old and new files exist. Store should index both.
	s2 := openStore(t, fs)
	defer s2.Close()

	// Both entities should be accessible (the store doesn't know about the rename).
	_, err = s2.GetEntity(ctx, "REQ-OLD")
	assert.NoError(t, err, "old entity should still be accessible")

	_, err = s2.GetEntity(ctx, "REQ-NEW")
	assert.NoError(t, err, "new entity should be accessible")

	// Both relation variants accessible.
	_, err = s2.GetRelation(ctx, "SOL-1", "implements", "REQ-OLD")
	assert.NoError(t, err, "old relation should still exist")

	_, err = s2.GetRelation(ctx, "SOL-1", "implements", "REQ-NEW")
	assert.NoError(t, err, "new relation should exist")
}

// --- Crash mid-cascade-delete: some relation files removed ---

func TestRecovery_PartialCascadeDelete(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	// Create entity with two relations.
	s1 := openStore(t, fs)
	require.NoError(t, s1.CreateEntity(ctx, entity.New("REQ-1", "requirement")))
	require.NoError(t, s1.CreateEntity(ctx, entity.New("SOL-1", "solution")))
	require.NoError(t, s1.CreateEntity(ctx, entity.New("SOL-2", "solution")))
	_, err := s1.CreateRelation(ctx, "SOL-1", "implements", "REQ-1", nil)
	require.NoError(t, err)
	_, err = s1.CreateRelation(ctx, "SOL-2", "implements", "REQ-1", nil)
	require.NoError(t, err)
	require.NoError(t, s1.Close())

	// Simulate crash mid-cascade-delete: entity file and one relation file removed,
	// but the other relation file still on disk.
	require.NoError(t, fs.Remove("/entities/requirements/REQ-1.md"))
	require.NoError(t, fs.Remove("/relations/SOL-1--implements--REQ-1.md"))
	// SOL-2--implements--REQ-1.md still exists (orphaned relation).

	// Reopen — entity is gone, remaining orphaned relation is still indexed.
	s2 := openStore(t, fs)
	defer s2.Close()

	_, err = s2.GetEntity(ctx, "REQ-1")
	assert.ErrorIs(t, err, store.ErrNotFound)

	// The surviving orphaned relation is still loadable.
	rel, err := s2.GetRelation(ctx, "SOL-2", "implements", "REQ-1")
	require.NoError(t, err)
	assert.Equal(t, "REQ-1", rel.To)

	// The deleted relation is gone.
	_, err = s2.GetRelation(ctx, "SOL-1", "implements", "REQ-1")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// --- Search index is rebuilt on restart ---

func TestRecovery_SearchIndexRebuilt(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	// Create entities.
	s1 := openStore(t, fs)
	e1 := entity.New("REQ-1", "requirement")
	e1.Properties["title"] = "Authentication flow"
	require.NoError(t, s1.CreateEntity(ctx, e1))

	e2 := entity.New("REQ-2", "requirement")
	e2.Properties["title"] = "Database migration"
	require.NoError(t, s1.CreateEntity(ctx, e2))
	require.NoError(t, s1.Close())

	// Reopen with a fresh store. Since observers are no longer populated
	// from disk by fsstore itself, feed the search index manually by
	// iterating the store's current entities.
	idx := search.NewLinearSearch()
	s2, err := fsstore.New(fsstore.Config{
		FS:             fs,
		EntitiesDir:    "/entities",
		RelationsDir:   "/relations",
		AttachmentsDir: "/attachments",
		CacheDir:       "/.rela",
		Observers:      []store.EntityObserver{idx},
	})
	require.NoError(t, err)
	defer s2.Close()

	for e, err := range s2.ListEntities(ctx, store.EntityQuery{}) {
		require.NoError(t, err)
		require.NoError(t, idx.EntityPut(e))
	}

	searcher := search.New(s2, idx)

	results := collectSearch(t, searcher, store.SearchQuery{Text: "authentication"})
	require.Len(t, results, 1)
	assert.Equal(t, "REQ-1", results[0].ID)

	results = collectSearch(t, searcher, store.SearchQuery{Text: "migration"})
	require.Len(t, results, 1)
	assert.Equal(t, "REQ-2", results[0].ID)
}

// --- Property cache rebuilt after crash (stale cache) ---

func TestRecovery_PropertyCacheAfterCrashMidUpdate(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	// Create entity and close (flushes cache).
	s1 := openStore(t, fs)
	e := entity.New("T-1", "ticket")
	e.Properties["status"] = "open"
	require.NoError(t, s1.CreateEntity(ctx, e))
	require.NoError(t, s1.Close())

	// Simulate: entity was updated on disk (e.g., by the store writing the file)
	// but the property cache was not flushed before crash.
	require.NoError(t, fs.WriteFile("/entities/tickets/T-1.md", []byte(`---
id: T-1
type: ticket
status: closed
---
`), 0644))

	// Reopen — cache is stale (mtime newer than cache), should rebuild.
	s2 := openStore(t, fs)
	defer s2.Close()

	vals, err := s2.PropertyValues(ctx, "status", 0)
	require.NoError(t, err)
	assert.Equal(t, []string{"closed"}, vals)
}

// --- Multiple temp files from repeated crash-restart cycles ---

func TestRecovery_MultipleTempFiles(t *testing.T) {
	fs := storage.NewMemFS()

	s1 := openStore(t, fs)
	require.NoError(t, s1.Close())

	// Simulate multiple crashed writes leaving temp files.
	require.NoError(t, fs.MkdirAll("/entities/requirements", 0755))
	require.NoError(t, fs.WriteFile("/entities/requirements/REQ-1.md.new", []byte("crash 1"), 0644))
	require.NoError(t, fs.WriteFile("/entities/requirements/REQ-2.md.new", []byte("crash 2"), 0644))
	require.NoError(t, fs.MkdirAll("/relations", 0755))
	require.NoError(t, fs.WriteFile("/relations/A--rel--B.md.new", []byte("crash 3"), 0644))

	// Also write a valid entity to ensure it's not removed.
	require.NoError(t, fs.WriteFile("/entities/requirements/REQ-3.md", []byte(`---
id: REQ-3
type: requirement
---
`), 0644))

	s2 := openStore(t, fs)
	defer s2.Close()

	// Temp files cleaned up.
	for _, path := range []string{
		"/entities/requirements/REQ-1.md.new",
		"/entities/requirements/REQ-2.md.new",
		"/relations/A--rel--B.md.new",
	} {
		_, err := fs.ReadFile(path)
		assert.True(t, isNotExist(err), "temp file %s should be cleaned up", path)
	}

	// Valid entity untouched.
	ctx := context.Background()
	_, err := s2.GetEntity(ctx, "REQ-3")
	assert.NoError(t, err)
}

// --- Attachment file exists but entity deleted ---

func TestRecovery_OrphanedAttachmentAfterEntityDelete(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	// Create entity with attachment.
	s1 := openStore(t, fs)
	require.NoError(t, s1.CreateEntity(ctx, entity.New("DOC-1", "document")))
	require.NoError(t, s1.AttachFile(ctx, "DOC-1", "diagram", "arch.png",
		strings.NewReader("PNG-DATA")))
	require.NoError(t, s1.Close())

	// Simulate crash: entity file deleted but attachment directory remains.
	require.NoError(t, fs.Remove("/entities/documents/DOC-1.md"))

	// Reopen — entity is gone, attachment directory is orphaned.
	s2 := openStore(t, fs)
	defer s2.Close()

	_, err := s2.GetEntity(ctx, "DOC-1")
	assert.ErrorIs(t, err, store.ErrNotFound)

	// Attachment is still physically on disk (orphaned) but the store
	// loads attachment index from walking the directory, so it's indexed.
	// However, the entity doesn't exist, so operations should handle gracefully.
	_, err = s2.ListAttachments(ctx, "DOC-1")
	assert.ErrorIs(t, err, store.ErrNotFound, "attachments for missing entity should return not found")
}

// helpers

func collectSearch(t *testing.T, searcher store.Searcher, q store.SearchQuery) []store.SearchHit {
	t.Helper()
	ctx := context.Background()
	var results []store.SearchHit
	for hit, err := range searcher.Search(ctx, q) {
		require.NoError(t, err)
		results = append(results, hit)
	}
	return results
}

func isNotExist(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "not exist") || strings.Contains(err.Error(), "no such file")
}
