package fsstore_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
)

// newConfig builds an fsstore Config for the given in-memory FS
// rooted at "/". Shared across fsstore_test files.
func newConfig(fs *storage.MemFS) fsstore.Config {
	rooted, err := storage.NewRootedFS(fs, "/")
	if err != nil {
		panic(err)
	}
	return fsstore.Config{
		FS:             fs,
		Rooted:         rooted,
		EntitiesKey:    "entities",
		RelationsKey:   "relations",
		AttachmentsKey: "attachments",
		CacheKey:       ".rela",
	}
}

func openStore(t *testing.T, fs *storage.MemFS) *fsstore.FSStore {
	t.Helper()
	s, err := fsstore.New(newConfig(fs))
	require.NoError(t, err)
	return s
}

func TestPersistence_EntitiesSurviveReopen(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	// Create entities in the first store instance.
	s1 := openStore(t, fs)
	e := entity.New("REQ-1", "requirement")
	e.Properties["title"] = "First requirement"
	e.Properties["status"] = "open"
	e.Content = "Some body text."
	require.NoError(t, s1.CreateEntity(ctx, e))
	require.NoError(t, s1.Close())

	// Reopen — entity must be there with all fields intact.
	s2 := openStore(t, fs)
	defer s2.Close()

	got, err := s2.GetEntity(ctx, "REQ-1")
	require.NoError(t, err)
	assert.Equal(t, "REQ-1", got.ID)
	assert.Equal(t, "requirement", got.Type)
	assert.Equal(t, "First requirement", got.Properties["title"])
	assert.Equal(t, "open", got.Properties["status"])
	assert.Equal(t, "Some body text.", strings.TrimSpace(got.Content))

	count, err := s2.CountEntities(ctx, store.EntityQuery{})
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestPersistence_RelationsSurviveReopen(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	s1 := openStore(t, fs)
	require.NoError(t, s1.CreateEntity(ctx, entity.New("REQ-1", "requirement")))
	require.NoError(t, s1.CreateEntity(ctx, entity.New("SOL-1", "solution")))
	_, err := s1.CreateRelation(ctx, "SOL-1", "implements", "REQ-1", &store.RelationData{
		Content: "This solution implements the requirement.",
	})
	require.NoError(t, err)
	require.NoError(t, s1.Close())

	s2 := openStore(t, fs)
	defer s2.Close()

	rel, err := s2.GetRelation(ctx, "SOL-1", "implements", "REQ-1")
	require.NoError(t, err)
	assert.Equal(t, "SOL-1", rel.From)
	assert.Equal(t, "implements", rel.Type)
	assert.Equal(t, "REQ-1", rel.To)
	assert.Contains(t, rel.Content, "implements the requirement")
}

func TestPersistence_AttachmentsSurviveReopen(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	s1 := openStore(t, fs)
	require.NoError(t, s1.CreateEntity(ctx, entity.New("DOC-1", "document")))
	require.NoError(t, s1.AttachFile(ctx, "DOC-1", "diagram", "arch.png", bytes.NewReader([]byte("PNG-DATA"))))
	require.NoError(t, s1.Close())

	s2 := openStore(t, fs)
	defer s2.Close()

	rc, err := s2.ReadAttachment(ctx, "DOC-1", "diagram")
	require.NoError(t, err)
	data, _ := io.ReadAll(rc)
	rc.Close()
	assert.Equal(t, "PNG-DATA", string(data))
}

func TestPersistence_PropertyCacheSurvivesReopen(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	s1 := openStore(t, fs)
	for i, status := range []string{"open", "open", "closed"} {
		e := entity.New("T-"+string(rune('1'+i)), "ticket")
		e.Properties["status"] = status
		require.NoError(t, s1.CreateEntity(ctx, e))
	}
	require.NoError(t, s1.Close())

	s2 := openStore(t, fs)
	defer s2.Close()

	vals, err := s2.PropertyValues(ctx, "status", 0)
	require.NoError(t, err)
	assert.Contains(t, vals, "open")
	assert.Contains(t, vals, "closed")
}

func TestPersistence_ExternalEntityModification(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	// Create an entity via the store.
	s1 := openStore(t, fs)
	e := entity.New("REQ-1", "requirement")
	e.Properties["status"] = "draft"
	require.NoError(t, s1.CreateEntity(ctx, e))
	require.NoError(t, s1.Close())

	// Modify the file directly on the filesystem (simulating external edit).
	path := "/entities/requirements/REQ-1.md"
	require.NoError(t, fs.WriteFile(path, []byte(`---
id: REQ-1
type: requirement
status: approved
title: Externally added title
---

Updated body.
`), 0644))

	// Reopen — the store should pick up the modified content.
	s2 := openStore(t, fs)
	defer s2.Close()

	got, err := s2.GetEntity(ctx, "REQ-1")
	require.NoError(t, err)
	assert.Equal(t, "approved", got.Properties["status"])
	assert.Equal(t, "Externally added title", got.Properties["title"])
	assert.Equal(t, "Updated body.", strings.TrimSpace(got.Content))
}

func TestPersistence_ExternalEntityAdded(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	// Start with an empty store.
	s1 := openStore(t, fs)
	require.NoError(t, s1.Close())

	// Write a brand-new entity file directly.
	require.NoError(t, fs.MkdirAll("/entities/decisions", 0755))
	require.NoError(t, fs.WriteFile("/entities/decisions/DEC-1.md", []byte(`---
id: DEC-1
type: decision
status: accepted
title: Use Go
---

We decided to use Go.
`), 0644))

	s2 := openStore(t, fs)
	defer s2.Close()

	got, err := s2.GetEntity(ctx, "DEC-1")
	require.NoError(t, err)
	assert.Equal(t, "decision", got.Type)
	assert.Equal(t, "accepted", got.Properties["status"])

	count, err := s2.CountEntities(ctx, store.EntityQuery{})
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestPersistence_ExternalRelationAdded(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	// Create two entities.
	s1 := openStore(t, fs)
	require.NoError(t, s1.CreateEntity(ctx, entity.New("A-1", "artifact")))
	require.NoError(t, s1.CreateEntity(ctx, entity.New("A-2", "artifact")))
	require.NoError(t, s1.Close())

	// Write a relation file directly.
	require.NoError(t, fs.MkdirAll("/relations", 0755))
	require.NoError(t, fs.WriteFile("/relations/A-1--depends-on--A-2.md", []byte(`---
from: A-1
relation: depends-on
to: A-2
---

Externally created relation.
`), 0644))

	s2 := openStore(t, fs)
	defer s2.Close()

	rel, err := s2.GetRelation(ctx, "A-1", "depends-on", "A-2")
	require.NoError(t, err)
	assert.Equal(t, "A-1", rel.From)
	assert.Equal(t, "depends-on", rel.Type)
	assert.Equal(t, "A-2", rel.To)
	assert.Contains(t, rel.Content, "Externally created")
}

func TestPersistence_ExternalEntityDeleted(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	s1 := openStore(t, fs)
	require.NoError(t, s1.CreateEntity(ctx, entity.New("DEL-1", "thing")))
	require.NoError(t, s1.CreateEntity(ctx, entity.New("DEL-2", "thing")))
	require.NoError(t, s1.Close())

	// Remove one entity file externally.
	require.NoError(t, fs.Remove("/entities/things/DEL-1.md"))

	s2 := openStore(t, fs)
	defer s2.Close()

	_, err := s2.GetEntity(ctx, "DEL-1")
	require.ErrorIs(t, err, store.ErrNotFound)

	count, err := s2.CountEntities(ctx, store.EntityQuery{})
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestPersistence_PropertyCacheRebuildAfterExternalEdit(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	// Create entities and close (which flushes the property cache).
	s1 := openStore(t, fs)
	e := entity.New("T-1", "ticket")
	e.Properties["priority"] = "low"
	require.NoError(t, s1.CreateEntity(ctx, e))
	require.NoError(t, s1.Close())

	// Modify the entity externally — the property cache is now stale.
	require.NoError(t, fs.WriteFile("/entities/tickets/T-1.md", []byte(`---
id: T-1
type: ticket
priority: critical
---
`), 0644))

	// Reopen — the store should detect staleness and rebuild the cache.
	s2 := openStore(t, fs)
	defer s2.Close()

	vals, err := s2.PropertyValues(ctx, "priority", 0)
	require.NoError(t, err)
	assert.Equal(t, []string{"critical"}, vals)
}

func TestPersistence_UpdateSurvivedReopen(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	s1 := openStore(t, fs)
	e := entity.New("REQ-1", "requirement")
	e.Properties["status"] = "draft"
	require.NoError(t, s1.CreateEntity(ctx, e))

	// Update via the store.
	e.Properties["status"] = "approved"
	e.Content = "Approved content."
	require.NoError(t, s1.UpdateEntity(ctx, e))
	require.NoError(t, s1.Close())

	s2 := openStore(t, fs)
	defer s2.Close()

	got, err := s2.GetEntity(ctx, "REQ-1")
	require.NoError(t, err)
	assert.Equal(t, "approved", got.Properties["status"])
	assert.Equal(t, "Approved content.", strings.TrimSpace(got.Content))
}

func TestPersistence_DeleteSurvivedReopen(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	s1 := openStore(t, fs)
	require.NoError(t, s1.CreateEntity(ctx, entity.New("REQ-1", "requirement")))
	_, err := s1.DeleteEntity(ctx, "REQ-1", false)
	require.NoError(t, err)
	require.NoError(t, s1.Close())

	s2 := openStore(t, fs)
	defer s2.Close()

	_, err = s2.GetEntity(ctx, "REQ-1")
	require.ErrorIs(t, err, store.ErrNotFound)

	count, err := s2.CountEntities(ctx, store.EntityQuery{})
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestPersistence_RenameSurvivedReopen(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	s1 := openStore(t, fs)
	e := entity.New("REQ-OLD", "requirement")
	e.Properties["title"] = "Keep this"
	require.NoError(t, s1.CreateEntity(ctx, e))
	require.NoError(t, s1.CreateEntity(ctx, entity.New("SOL-1", "solution")))
	_, err := s1.CreateRelation(ctx, "SOL-1", "implements", "REQ-OLD", nil)
	require.NoError(t, err)

	_, err = s1.RenameEntity(ctx, "REQ-OLD", "REQ-NEW")
	require.NoError(t, err)
	require.NoError(t, s1.Close())

	s2 := openStore(t, fs)
	defer s2.Close()

	// Old ID is gone.
	_, err = s2.GetEntity(ctx, "REQ-OLD")
	require.ErrorIs(t, err, store.ErrNotFound)

	// New ID exists with same properties.
	got, err := s2.GetEntity(ctx, "REQ-NEW")
	require.NoError(t, err)
	assert.Equal(t, "Keep this", got.Properties["title"])

	// Relation updated to new ID.
	rel, err := s2.GetRelation(ctx, "SOL-1", "implements", "REQ-NEW")
	require.NoError(t, err)
	assert.Equal(t, "REQ-NEW", rel.To)
}
