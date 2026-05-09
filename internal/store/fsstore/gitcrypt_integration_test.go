package fsstore_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
)

// gitCryptHeader is the 10-byte magic prefix git-crypt writes.
var gitCryptHeader = []byte{0x00, 'G', 'I', 'T', 'C', 'R', 'Y', 'P', 'T', 0x00}

func gitCryptBlob(extra string) []byte {
	out := make([]byte, 0, len(gitCryptHeader)+len(extra))
	out = append(out, gitCryptHeader...)
	out = append(out, []byte(extra)...)
	return out
}

// seedAndClose runs setup against a freshly opened store, then closes
// it. Used to populate test fixtures before tampering with on-disk
// bytes (e.g. injecting a git-crypt blob in place of a real entity).
func seedAndClose(t *testing.T, fs *storage.MemFS, setup func(*fsstore.FSStore)) {
	t.Helper()
	s := openStore(t, fs)
	setup(s)
	require.NoError(t, s.Close())
}

// writeEncrypted overwrites the given key with a git-crypt blob.
func writeEncrypted(t *testing.T, fs *storage.MemFS, key string) {
	t.Helper()
	require.NoError(t, fs.WriteFile(key, gitCryptBlob("ciphertext"), 0o644))
}

func TestGitCrypt_GetEntityReturnsInaccessibleEntity(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	seedAndClose(t, fs, func(s *fsstore.FSStore) {
		require.NoError(t, s.CreateEntity(ctx, entity.New("REQ-1", "requirement")))
	})
	writeEncrypted(t, fs, "/entities/requirements/REQ-2.md")

	s := openStore(t, fs)
	defer s.Close()

	got, err := s.GetEntity(ctx, "REQ-2")
	require.NoError(t, err, "encrypted file should load as an inaccessible entity, not error")
	assert.Equal(t, "REQ-2", got.ID)
	assert.Equal(t, "requirement", got.Type)
	assert.Empty(t, got.Properties, "encrypted entity has no readable properties")
	assert.True(t, got.IsLocked())

	// Inaccessible should list every schema-declared property plus content.
	wantNames := map[string]bool{"title": true, "status": true, "description": true, entity.InaccessibleFieldContent: true}
	require.Len(t, got.Inaccessible, len(wantNames))
	for _, f := range got.Inaccessible {
		assert.True(t, wantNames[f.Name], "unexpected inaccessible field: %s", f.Name)
		assert.Equal(t, entity.InaccessibleReasonGitCrypt, f.Reason)
	}
}

func TestGitCrypt_ListEntitiesIncludesInaccessibleAlongsideCleartext(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	seedAndClose(t, fs, func(s *fsstore.FSStore) {
		e := entity.New("REQ-1", "requirement")
		e.Properties["title"] = "Cleartext one"
		require.NoError(t, s.CreateEntity(ctx, e))
	})
	writeEncrypted(t, fs, "/entities/requirements/REQ-2.md")

	s := openStore(t, fs)
	defer s.Close()

	var cleartext, encrypted []*entity.Entity
	for got, err := range s.ListEntities(ctx, store.EntityQuery{}) {
		require.NoError(t, err, "encrypted file must not surface as iterator error")
		if got.IsLocked() {
			encrypted = append(encrypted, got)
		} else {
			cleartext = append(cleartext, got)
		}
	}
	require.Len(t, cleartext, 1)
	require.Len(t, encrypted, 1)
	assert.Equal(t, "REQ-1", cleartext[0].ID)
	assert.Equal(t, "Cleartext one", cleartext[0].Properties["title"])
	assert.Equal(t, "REQ-2", encrypted[0].ID)
	assert.Empty(t, encrypted[0].Properties)
}

func TestGitCrypt_GetRelationReturnsInaccessibleRelation(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	seedAndClose(t, fs, func(s *fsstore.FSStore) {
		require.NoError(t, s.CreateEntity(ctx, entity.New("REQ-1", "requirement")))
		require.NoError(t, s.CreateEntity(ctx, entity.New("SOL-1", "solution")))
		_, err := s.CreateRelation(ctx, "SOL-1", "implements", "REQ-1", nil)
		require.NoError(t, err)
	})
	writeEncrypted(t, fs, "/relations/SOL-1--implements--REQ-1.md")

	s := openStore(t, fs)
	defer s.Close()

	got, err := s.GetRelation(ctx, "SOL-1", "implements", "REQ-1")
	require.NoError(t, err, "encrypted relation should load with Inaccessible populated, not error")
	assert.Equal(t, "SOL-1", got.From)
	assert.Equal(t, "implements", got.Type)
	assert.Equal(t, "REQ-1", got.To)
	assert.Empty(t, got.Properties)
	assert.True(t, got.IsLocked())
	require.Len(t, got.Inaccessible, 1)
	assert.Equal(t, entity.InaccessibleFieldContent, got.Inaccessible[0].Name)
	assert.Equal(t, entity.InaccessibleReasonGitCrypt, got.Inaccessible[0].Reason)
}

func TestGitCrypt_PropertylessEntityType_StillLocks(t *testing.T) {
	// An entity type with an empty PropertyOrder is legitimate. The
	// inaccessible-shell still emits a content-marker so IsLocked()
	// triggers and write guards fire.
	fs := storage.NewMemFS()
	rooted, err := storage.NewRootedFS(fs, "/")
	require.NoError(t, err)

	cfg := fsstore.Config{
		FS:           fs,
		Rooted:       rooted,
		EntitiesKey:  "entities",
		RelationsKey: "relations",
		CacheKey:     ".rela",
		Schemas: map[string]store.EntityTypeSchema{
			"bare": {Plural: "bares", PropertyOrder: nil},
		},
	}
	require.NoError(t, fs.MkdirAll("/entities/bares", 0o755))
	writeEncrypted(t, fs, "/entities/bares/B-1.md")

	s, err := fsstore.New(cfg)
	require.NoError(t, err)
	defer s.Close()

	got, err := s.GetEntity(context.Background(), "B-1")
	require.NoError(t, err)
	assert.True(t, got.IsLocked(), "encrypted entity with no schema properties must still be locked")
	require.Len(t, got.Inaccessible, 1)
	assert.Equal(t, entity.InaccessibleFieldContent, got.Inaccessible[0].Name)
}

func TestNew_RejectsEmptySchemas(t *testing.T) {
	fs := storage.NewMemFS()
	rooted, err := storage.NewRootedFS(fs, "/")
	require.NoError(t, err)

	_, err = fsstore.New(fsstore.Config{
		FS:           fs,
		Rooted:       rooted,
		EntitiesKey:  "entities",
		RelationsKey: "relations",
		CacheKey:     ".rela",
		// Schemas omitted on purpose.
	})
	if err == nil {
		t.Fatal("expected fsstore.New to reject an empty Schemas map")
	}
}

func TestScan_SkipsUnknownEntityTypeDirectories(t *testing.T) {
	// A directory whose plural does not map to any metamodel-declared
	// type must be skipped at scan time. This is the invariant that
	// makes buildInaccessibleEntity safe to assume entityType is always
	// in s.schemas.
	fs := storage.NewMemFS()
	require.NoError(t, fs.MkdirAll("/entities/unknowns", 0o755))
	require.NoError(t, fs.WriteFile("/entities/unknowns/UNK-1.md", []byte(`---
id: UNK-1
type: unknown
---
`), 0o644))

	s := openStore(t, fs)
	defer s.Close()

	count, err := s.CountEntities(context.Background(), store.EntityQuery{})
	require.NoError(t, err)
	assert.Equal(t, 0, count, "entity in unknown-type directory should be skipped")
}

func TestGitCrypt_HalfEncrypted_CleartextRelationToEncryptedEntity(t *testing.T) {
	// A common partial-encryption pattern: the relation file is cleartext,
	// but the entity it points at is encrypted. The relation must remain
	// visible; the encrypted entity loads with Inaccessible populated.
	fs := storage.NewMemFS()
	ctx := context.Background()

	seedAndClose(t, fs, func(s *fsstore.FSStore) {
		require.NoError(t, s.CreateEntity(ctx, entity.New("REQ-1", "requirement")))
		require.NoError(t, s.CreateEntity(ctx, entity.New("SOL-1", "solution")))
		_, err := s.CreateRelation(ctx, "SOL-1", "implements", "REQ-1", nil)
		require.NoError(t, err)
	})
	writeEncrypted(t, fs, "/entities/requirements/REQ-1.md")

	s := openStore(t, fs)
	defer s.Close()

	rel, err := s.GetRelation(ctx, "SOL-1", "implements", "REQ-1")
	require.NoError(t, err)
	assert.False(t, rel.IsLocked(), "cleartext relation file should be readable")

	target, err := s.GetEntity(ctx, "REQ-1")
	require.NoError(t, err)
	assert.True(t, target.IsLocked(), "encrypted target entity has Inaccessible populated")
}
