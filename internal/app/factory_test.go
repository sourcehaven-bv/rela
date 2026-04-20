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
	"github.com/Sourcehaven-BV/rela/internal/storage/integrity"
	"github.com/Sourcehaven-BV/rela/internal/userstate"
)

// newTestUserState returns a per-test user-state service rooted at
// a fresh tempdir. Tests that don't care about state-path details
// pair it with FSFactory construction via the helper below.
func newTestUserState(t *testing.T) userstate.FSService {
	t.Helper()
	return userstate.NewForTest(t.TempDir())
}

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

	factory := &app.FSFactory{FS: fs, Paths: paths, UserState: newTestUserState(t)}
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
	// When <root>/recipients.age exists, OpenStore loads the keyring
	// and installs a real age Crypto. Entity writes hit disk sealed.
	root := t.TempDir()
	us := newTestUserState(t)
	id, err := encryption.GenerateIdentity()
	require.NoError(t, err)
	setupEncryptedRepo(t, root, id, us, true)

	paths := &project.Context{
		Root:         root,
		CacheDir:     filepath.Join(root, ".rela"),
		EntitiesDir:  filepath.Join(root, "entities"),
		RelationsDir: filepath.Join(root, "relations"),
	}
	factory := &app.FSFactory{FS: storage.NewSafeFS(storage.NewOsFS()), Paths: paths, UserState: us}
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

	factory := &app.FSFactory{FS: fs, Paths: paths, UserState: newTestUserState(t)}
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
	us := newTestUserState(t)
	id, err := encryption.GenerateIdentity()
	require.NoError(t, err)
	setupEncryptedRepo(t, root, id, us, true)

	paths := &project.Context{
		Root:         root,
		CacheDir:     filepath.Join(root, ".rela"),
		EntitiesDir:  filepath.Join(root, "entities"),
		RelationsDir: filepath.Join(root, "relations"),
	}
	// Raw OsFS (no SafeFS wrap) on an encrypted repo must be refused.
	factory := &app.FSFactory{FS: storage.NewOsFS(), Paths: paths, UserState: us}
	_, err = factory.OpenStore(&metamodel.Metamodel{})
	require.Error(t, err)
	assert.ErrorIs(t, err, app.ErrEncryptedRepoNeedsSafeFS)
}

// TestFSFactory_SingleBranchInvariant asserts that the factory's
// decision to install cryptofs and the fsstore's expectation of
// sealed bytes come from the same signal (<root>/recipients.age
// presence). Opens two parallel projects — one encrypted, one
// cleartext — and verifies the observable on-disk bytes match the
// declared mode, proving the single branch controls both sides.
//
// Regression gate for the "decorator installed vs wantSealed
// consistency check drift" class of bugs.
func TestFSFactory_SingleBranchInvariant(t *testing.T) {
	newProject := func(t *testing.T, encrypted bool) (string, *project.Context, userstate.FSService) {
		t.Helper()
		root := t.TempDir()
		us := newTestUserState(t)
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".rela"), 0o755))
		if encrypted {
			id, err := encryption.GenerateIdentity()
			require.NoError(t, err)
			setupEncryptedRepo(t, root, id, us, true)
		}
		return root, &project.Context{
			Root:         root,
			CacheDir:     filepath.Join(root, ".rela"),
			EntitiesDir:  filepath.Join(root, "entities"),
			RelationsDir: filepath.Join(root, "relations"),
		}, us
	}

	writeSentinel := func(t *testing.T, paths *project.Context, us userstate.FSService) {
		t.Helper()
		t.Setenv("RELA_KEY_FILE", "")
		factory := &app.FSFactory{
			FS:        storage.NewSafeFS(storage.NewOsFS()),
			Paths:     paths,
			UserState: us,
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
		root, paths, us := newProject(t, true)
		writeSentinel(t, paths, us)
		raw, err := os.ReadFile(filepath.Join(root, "entities", "tickets", "TKT-SENTINEL.md"))
		require.NoError(t, err)
		assert.True(t, encryption.LooksSealed(raw),
			"encrypted branch installed cryptofs AND passed wantSealed=true "+
				"— on-disk bytes must be sealed")
	})

	t.Run("cleartext branch writes plaintext bytes", func(t *testing.T) {
		root, paths, us := newProject(t, false)
		writeSentinel(t, paths, us)
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
	us := newTestUserState(t)
	id, err := encryption.GenerateIdentity()
	require.NoError(t, err)
	// writeIdentity=false: encrypted repo state on disk but no
	// private key anywhere resolvable.
	setupEncryptedRepo(t, root, id, us, false)
	t.Setenv("HOME", t.TempDir())

	paths := &project.Context{
		Root:         root,
		CacheDir:     filepath.Join(root, ".rela"),
		EntitiesDir:  filepath.Join(root, "entities"),
		RelationsDir: filepath.Join(root, "relations"),
	}
	factory := &app.FSFactory{FS: storage.NewSafeFS(storage.NewOsFS()), Paths: paths, UserState: us}
	_, err = factory.OpenStore(&metamodel.Metamodel{})
	require.Error(t, err)
	assert.ErrorIs(t, err, app.ErrEncryptedRepoNeedsIdentity)
}

// TestFSFactory_Encrypted_RefusesCleartextDataFiles: when the repo
// is encryption-enabled but a data file on disk is cleartext, the
// factory (via integrity.Verify) refuses to open. Covers the
// "half-migrated" case — e.g. a file added on a branch that didn't
// know about encryption.
func TestFSFactory_Encrypted_RefusesCleartextDataFiles(t *testing.T) {
	root := t.TempDir()
	us := newTestUserState(t)
	id, err := encryption.GenerateIdentity()
	require.NoError(t, err)
	setupEncryptedRepo(t, root, id, us, true)

	// Plant a cleartext entity file that the integrity verifier must reject.
	entitiesDir := filepath.Join(root, "entities", "tickets")
	require.NoError(t, os.MkdirAll(entitiesDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(entitiesDir, "TKT-CT.md"),
		[]byte("---\nid: TKT-CT\ntype: ticket\n---\ncleartext body\n"), 0o644))

	paths := &project.Context{
		Root:         root,
		CacheDir:     filepath.Join(root, ".rela"),
		EntitiesDir:  filepath.Join(root, "entities"),
		RelationsDir: filepath.Join(root, "relations"),
	}
	factory := &app.FSFactory{FS: storage.NewSafeFS(storage.NewOsFS()), Paths: paths, UserState: us}
	_, err = factory.OpenStore(&metamodel.Metamodel{})
	require.Error(t, err)
	assert.ErrorIs(t, err, integrity.ErrRepoHasCleartextFilesButEncryptionEnabled)
}

// TestFSFactory_Cleartext_RefusesSealedDataFiles: the inverse — when
// encryption is NOT configured but a sealed file is already on disk
// (e.g. a merge from a branch that was encrypted), the factory refuses.
func TestFSFactory_Cleartext_RefusesSealedDataFiles(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".rela"), 0o755))

	// Plant a sealed file under entities. The verifier peeks headers,
	// so a real age-sealed blob is needed (not just the magic).
	id, err := encryption.GenerateIdentity()
	require.NoError(t, err)
	sealed, err := encryption.Seal(
		[]byte("---\nid: TKT-S\ntype: ticket\n---\n"),
		[]encryption.Recipient{id.PublicRecipient()})
	require.NoError(t, err)
	entitiesDir := filepath.Join(root, "entities", "tickets")
	require.NoError(t, os.MkdirAll(entitiesDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(entitiesDir, "TKT-S.md"), sealed, 0o644))

	paths := &project.Context{
		Root:         root,
		CacheDir:     filepath.Join(root, ".rela"),
		EntitiesDir:  filepath.Join(root, "entities"),
		RelationsDir: filepath.Join(root, "relations"),
	}
	factory := &app.FSFactory{FS: storage.NewSafeFS(storage.NewOsFS()), Paths: paths, UserState: newTestUserState(t)}
	_, err = factory.OpenStore(&metamodel.Metamodel{})
	require.Error(t, err)
	assert.ErrorIs(t, err, integrity.ErrRepoHasSealedFilesButNoConfig)
}

// TestFSFactory_OpenBytesFS_Cleartext asserts the byte-I/O handle
// returned by the factory on a cleartext repo is the raw FS itself
// — no decorator wrap when there's nothing to seal.
func TestFSFactory_OpenBytesFS_Cleartext(t *testing.T) {
	root := t.TempDir()
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths := &project.Context{
		Root:         root,
		CacheDir:     filepath.Join(root, ".rela"),
		EntitiesDir:  filepath.Join(root, "entities"),
		RelationsDir: filepath.Join(root, "relations"),
	}
	factory := &app.FSFactory{FS: fs, Paths: paths, UserState: newTestUserState(t)}

	handle, err := factory.OpenBytesFS()
	require.NoError(t, err)
	require.NotNil(t, handle)

	// Round-trip through the returned handle: the bytes on disk are
	// exactly what we wrote (no seal wrap).
	testPath := filepath.Join(root, "probe.txt")
	payload := []byte("plaintext")
	require.NoError(t, handle.WriteFile(testPath, payload, 0o644))

	raw, err := os.ReadFile(testPath)
	require.NoError(t, err)
	assert.Equal(t, payload, raw, "cleartext repo must not seal via OpenBytesFS")
}

// TestFSFactory_OpenBytesFS_Encrypted asserts that on an encrypted
// repo the handle seals writes transparently — the same guarantee
// the store itself provides. Workspace-level components (today:
// the attachment store) rely on this so attachments land sealed.
func TestFSFactory_OpenBytesFS_Encrypted(t *testing.T) {
	root := t.TempDir()
	us := newTestUserState(t)
	id, err := encryption.GenerateIdentity()
	require.NoError(t, err)
	setupEncryptedRepo(t, root, id, us, true)

	paths := &project.Context{
		Root:         root,
		CacheDir:     filepath.Join(root, ".rela"),
		EntitiesDir:  filepath.Join(root, "entities"),
		RelationsDir: filepath.Join(root, "relations"),
	}
	factory := &app.FSFactory{FS: storage.NewSafeFS(storage.NewOsFS()), Paths: paths, UserState: us}

	handle, err := factory.OpenBytesFS()
	require.NoError(t, err)
	require.NotNil(t, handle)

	// Write through the handle and assert the on-disk bytes are sealed.
	testPath := filepath.Join(root, "attachments", "probe.bin")
	require.NoError(t, os.MkdirAll(filepath.Dir(testPath), 0o755))
	plaintext := []byte("top-secret-attachment-contents")
	require.NoError(t, handle.WriteFile(testPath, plaintext, 0o644))

	raw, err := os.ReadFile(testPath)
	require.NoError(t, err)
	assert.True(t, encryption.LooksSealed(raw),
		"encrypted repo: OpenBytesFS handle must seal writes")
	assert.NotContains(t, string(raw), "top-secret-attachment-contents",
		"plaintext must not survive on disk")
}

// TestFSFactory_OpenStore_RecoversInterruptedRotation asserts the
// factory detects an XDG-local reseal sentinel at open time and
// resumes the rotation transparently — mid-flight re-seal is
// finished, recipients.age is rewritten at the new version, and
// OpenStore returns a store wired to the post-rotation state.
func TestFSFactory_OpenStore_RecoversInterruptedRotation(t *testing.T) {
	root := t.TempDir()
	us := newTestUserState(t)
	alice, err := encryption.GenerateIdentity()
	require.NoError(t, err)
	setupEncryptedRepo(t, root, alice, us, true)

	// Seed one sealed entity at v=1 (matching the repo's current state).
	entityDir := filepath.Join(root, "entities", "tickets")
	require.NoError(t, os.MkdirAll(entityDir, 0o755))

	// Load keyring to confirm the repo loads under the test us.
	_, err = encryption.LoadFromDir(root, us)
	require.NoError(t, err)

	// Plant a reseal sentinel pointing at a v=2 rotation to alice+bob.
	bob, err := encryption.GenerateIdentity()
	require.NoError(t, err)
	sentinel := &encryption.ResealSentinel{
		FromVersion: 1,
		ToVersion:   2,
		RepoRoot:    root,
		NewRecipients: map[string]string{
			"alice": alice.PublicRecipient().String(),
			"bob":   bob.PublicRecipient().String(),
		},
		Operation: "keys add bob",
	}
	require.NoError(t, encryption.WriteResealSentinel(us, sentinel))

	paths := &project.Context{
		Root:         root,
		CacheDir:     filepath.Join(root, ".rela"),
		EntitiesDir:  filepath.Join(root, "entities"),
		RelationsDir: filepath.Join(root, "relations"),
	}
	factory := &app.FSFactory{FS: storage.NewSafeFS(storage.NewOsFS()), Paths: paths, UserState: us}

	// Factory should auto-resume and open the store cleanly.
	s, err := factory.OpenStore(&metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{"ticket": {Plural: "tickets"}},
	})
	require.NoError(t, err)
	defer s.Close()

	// After recovery, recipients.age should be at v=2 with bob present.
	rf, err := encryption.ReadRecipientsFile(
		filepath.Join(root, encryption.RecipientsFileName), alice)
	require.NoError(t, err)
	assert.Equal(t, 2, rf.Version)
	assert.Contains(t, rf.Recipients, "bob")
}

// TestFSFactory_OpenBytesFS_EncryptedMissingIdentity asserts the
// factory surfaces ErrEncryptedRepoNeedsIdentity rather than silently
// returning a cripple-read handle when no identity is configured.
func TestFSFactory_OpenBytesFS_EncryptedMissingIdentity(t *testing.T) {
	root := t.TempDir()
	us := newTestUserState(t)
	id, err := encryption.GenerateIdentity()
	require.NoError(t, err)
	// writeIdentity=false: recipients.age present but no key resolvable.
	setupEncryptedRepo(t, root, id, us, false)
	t.Setenv("HOME", t.TempDir())

	paths := &project.Context{
		Root:         root,
		CacheDir:     filepath.Join(root, ".rela"),
		EntitiesDir:  filepath.Join(root, "entities"),
		RelationsDir: filepath.Join(root, "relations"),
	}
	factory := &app.FSFactory{FS: storage.NewSafeFS(storage.NewOsFS()), Paths: paths, UserState: us}

	_, err = factory.OpenBytesFS()
	require.Error(t, err)
	assert.ErrorIs(t, err, app.ErrEncryptedRepoNeedsIdentity)
}

// mustMarshalIdentity wraps encryption.MarshalIdentity for test use:
// any serialization failure is treated as a test failure rather than
// a value to thread through call sites.
func mustMarshalIdentity(t *testing.T, id encryption.Identity) string {
	t.Helper()
	s, err := encryption.MarshalIdentity(id)
	require.NoError(t, err)
	return s
}

// setupEncryptedRepo wires an encrypted repo on disk at root for
// factory tests. When writeIdentity is true it installs the private
// identity into the user-state service (us.Path("key")) so
// LoadFromDir resolves it; otherwise it skips that write so tests
// can assert "missing identity" errors.
//
// Callers must pass a user-state service that matches the one they
// attach to the FSFactory being tested — otherwise LoadFromDir's
// resolver won't find the key.
func setupEncryptedRepo(t *testing.T, root string, id encryption.Identity,
	us userstate.FSService, writeIdentity bool,
) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".rela"), 0o755))
	if writeIdentity {
		require.NoError(t, os.WriteFile(us.Path("key"),
			[]byte(mustMarshalIdentity(t, id)+"\n"), 0o600))
	}
	repoID, err := encryption.NewRepoID()
	require.NoError(t, err)
	rf := &encryption.RecipientsFile{
		Version:    1,
		RepoID:     repoID,
		Recipients: map[string]string{"alice": id.PublicRecipient().String()},
	}
	require.NoError(t, encryption.WriteRecipientsFile(
		filepath.Join(root, encryption.RecipientsFileName), rf))
	t.Setenv("RELA_KEY_FILE", "")
}
