package app_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/app"
	"github.com/Sourcehaven-BV/rela/internal/encryption"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func TestFSFactoryOpensWorkingStore(t *testing.T) {
	fs := storage.NewMemFS()
	paths := &project.Context{
		Root:         "/proj",
		EntitiesDir:  "/proj/entities",
		RelationsDir: "/proj/relations",
		CacheDir:     "/proj/.rela",
	}
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"policy": {Plural: "policies"},
		},
	}

	factory := &app.FSFactory{FS: fs, Paths: paths}
	s, err := factory.OpenStore(meta)
	require.NoError(t, err)
	defer s.Close()

	require.NoError(t, s.CreateEntity(context.Background(), &entity.Entity{
		ID:   "POL-1",
		Type: "policy",
	}))

	data, err := fs.ReadFile("/proj/entities/policies/POL-1.md")
	require.NoError(t, err)
	assert.Contains(t, string(data), "id: POL-1")
}

func TestFSFactory_EncryptedModeInstallsAgeCrypto(t *testing.T) {
	// When .rela/encryption.yaml exists, OpenStore loads the keyring
	// and installs a real age Crypto. Entity writes hit disk sealed.
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".rela"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "keys"), 0o755))

	id, err := encryption.GenerateIdentity()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "keys", "alice.pub"),
		[]byte(id.PublicRecipient().String()+"\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".rela", "key"),
		[]byte(encryption.MarshalIdentity(id)+"\n"), 0o600))
	require.NoError(t, os.WriteFile(
		filepath.Join(root, ".rela", encryption.ConfigFileName),
		[]byte("recipients:\n  - alice\n"), 0o644))
	t.Setenv("RELA_KEY_FILE", "")

	paths := &project.Context{
		Root:         root,
		CacheDir:     filepath.Join(root, ".rela"),
		EntitiesDir:  filepath.Join(root, "entities"),
		RelationsDir: filepath.Join(root, "relations"),
	}
	factory := &app.FSFactory{FS: storage.NewOsFS(), Paths: paths}
	s, err := factory.OpenStore(&metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Plural: "tickets"},
		},
	})
	require.NoError(t, err)
	defer s.Close()

	require.NoError(t, s.CreateEntity(context.Background(), &entity.Entity{
		ID:   "TKT-1",
		Type: "ticket",
	}))

	raw, err := os.ReadFile(filepath.Join(root, "entities", "tickets", "TKT-1.md"))
	require.NoError(t, err)
	assert.True(t, encryption.LooksSealed(raw), "expected sealed file, got cleartext")
}

func TestFSFactoryOpenStoreReturnsIndependentStores(t *testing.T) {
	fs := storage.NewMemFS()
	paths := &project.Context{
		Root:         "/proj",
		EntitiesDir:  "/proj/entities",
		RelationsDir: "/proj/relations",
		CacheDir:     "/proj/.rela",
	}

	factory := &app.FSFactory{FS: fs, Paths: paths}
	s1, err := factory.OpenStore(nil)
	require.NoError(t, err)
	defer s1.Close()

	s2, err := factory.OpenStore(nil)
	require.NoError(t, err)
	defer s2.Close()

	assert.NotSame(t, s1, s2, "each OpenStore call returns a fresh store")
}
