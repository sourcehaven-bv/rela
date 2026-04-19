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
	root := t.TempDir()
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths := &project.Context{
		Root:         root,
		EntitiesDir:  filepath.Join(root, "entities"),
		RelationsDir: filepath.Join(root, "relations"),
		CacheDir:     filepath.Join(root, ".rela"),
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

	data, err := os.ReadFile(filepath.Join(root, "entities", "policies", "POL-1.md"))
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
	factory := &app.FSFactory{FS: storage.NewSafeFS(storage.NewOsFS()), Paths: paths}
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
	root := t.TempDir()
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths := &project.Context{
		Root:         root,
		EntitiesDir:  filepath.Join(root, "entities"),
		RelationsDir: filepath.Join(root, "relations"),
		CacheDir:     filepath.Join(root, ".rela"),
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

// TestFSFactory_EncryptedNeedsSafeFS asserts the factory fails loudly
// when an encrypted repo is opened with a raw FS handle. Without
// SafeFS we cannot install the OnPostWrite observer that keeps the
// watcher's self-echo LRU correct — silently proceeding would break
// self-echo detection on every write.
func TestFSFactory_EncryptedNeedsSafeFS(t *testing.T) {
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
	// Raw OsFS (no SafeFS wrap) on an encrypted repo must be refused.
	factory := &app.FSFactory{FS: storage.NewOsFS(), Paths: paths}
	_, err = factory.OpenStore(&metamodel.Metamodel{})
	require.Error(t, err)
	assert.ErrorIs(t, err, app.ErrEncryptedRepoNeedsSafeFS)
}

// TestFSFactory_SingleBranchInvariant asserts that the factory's
// decision to install cryptofs and the fsstore's expectation of
// sealed bytes come from the same signal (.rela/encryption.yaml
// presence). Opens two parallel projects — one encrypted, one
// cleartext — and verifies the observable on-disk bytes match the
// declared mode, proving the single branch controls both sides.
//
// This is the regression gate for the "decorator installed vs
// wantSealed consistency check drift" class of bugs.
func TestFSFactory_SingleBranchInvariant(t *testing.T) {
	newProject := func(t *testing.T, encrypted bool) (string, *project.Context) {
		t.Helper()
		root := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".rela"), 0o755))
		if encrypted {
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
		}
		return root, &project.Context{
			Root:         root,
			CacheDir:     filepath.Join(root, ".rela"),
			EntitiesDir:  filepath.Join(root, "entities"),
			RelationsDir: filepath.Join(root, "relations"),
		}
	}

	writeSentinel := func(t *testing.T, paths *project.Context) {
		t.Helper()
		t.Setenv("RELA_KEY_FILE", "")
		factory := &app.FSFactory{
			FS:    storage.NewSafeFS(storage.NewOsFS()),
			Paths: paths,
		}
		s, err := factory.OpenStore(&metamodel.Metamodel{
			Entities: map[string]metamodel.EntityDef{"ticket": {Plural: "tickets"}},
		})
		require.NoError(t, err)
		defer s.Close()
		require.NoError(t, s.CreateEntity(context.Background(), &entity.Entity{
			ID:   "TKT-SENTINEL",
			Type: "ticket",
		}))
	}

	t.Run("encrypted branch writes sealed bytes", func(t *testing.T) {
		root, paths := newProject(t, true)
		writeSentinel(t, paths)
		raw, err := os.ReadFile(filepath.Join(root, "entities", "tickets", "TKT-SENTINEL.md"))
		require.NoError(t, err)
		assert.True(t, encryption.LooksSealed(raw),
			"encrypted branch installed cryptofs AND passed wantSealed=true "+
				"— on-disk bytes must be sealed")
	})

	t.Run("cleartext branch writes plaintext bytes", func(t *testing.T) {
		root, paths := newProject(t, false)
		writeSentinel(t, paths)
		raw, err := os.ReadFile(filepath.Join(root, "entities", "tickets", "TKT-SENTINEL.md"))
		require.NoError(t, err)
		assert.False(t, encryption.LooksSealed(raw),
			"cleartext branch skipped cryptofs AND passed wantSealed=false "+
				"— on-disk bytes must be plaintext")
		assert.Contains(t, string(raw), "id: TKT-SENTINEL")
	})
}

// TestFSFactory_EncryptedNeedsIdentity asserts the factory fails loudly
// when an encrypted repo is opened but no local age identity is
// configured. Silent success would cripple every read path.
func TestFSFactory_EncryptedNeedsIdentity(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".rela"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "keys"), 0o755))

	id, err := encryption.GenerateIdentity()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "keys", "alice.pub"),
		[]byte(id.PublicRecipient().String()+"\n"), 0o644))
	require.NoError(t, os.WriteFile(
		filepath.Join(root, ".rela", encryption.ConfigFileName),
		[]byte("recipients:\n  - alice\n"), 0o644))
	// NB: no identity written to .rela/key and no $RELA_KEY_FILE.
	t.Setenv("RELA_KEY_FILE", "")
	t.Setenv("HOME", t.TempDir())

	paths := &project.Context{
		Root:         root,
		CacheDir:     filepath.Join(root, ".rela"),
		EntitiesDir:  filepath.Join(root, "entities"),
		RelationsDir: filepath.Join(root, "relations"),
	}
	factory := &app.FSFactory{FS: storage.NewSafeFS(storage.NewOsFS()), Paths: paths}
	_, err = factory.OpenStore(&metamodel.Metamodel{})
	require.Error(t, err)
	assert.ErrorIs(t, err, app.ErrEncryptedRepoNeedsIdentity)
}
