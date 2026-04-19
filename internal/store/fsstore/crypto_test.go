package fsstore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/encryption"
	"github.com/Sourcehaven-BV/rela/internal/encryption/cryptofs"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
)

// setupEncryptedRepo writes a keys dir, identity file, and returns
// the root path plus a keyring ready for fsstore.NewAgeCrypto.
func setupEncryptedRepo(t *testing.T) (root string, kr *encryption.Keyring) {
	t.Helper()
	root = t.TempDir()
	id := mustGenerateIdentity(t)
	keysDir := filepath.Join(root, "keys")
	mustMkdir(t, keysDir)
	mustWrite(t, filepath.Join(keysDir, "alice.pub"),
		[]byte(id.PublicRecipient().String()+"\n"), 0o644)
	idPath := filepath.Join(root, ".rela", "key")
	mustMkdir(t, filepath.Dir(idPath))
	mustWrite(t, idPath, []byte(identityPrivate(t, id)+"\n"), 0o600)

	var err error
	kr, err = encryption.LoadKeyring(keysDir, idPath)
	if err != nil {
		t.Fatal(err)
	}
	return root, kr
}

// buildEncryptedStore sets up a brand-new fsstore wired with the
// full production decorator stack: cryptofs.FS(SafeFS(OsFS)) using a
// single-recipient keyring.
func buildEncryptedStore(t *testing.T) (s *fsstore.FSStore, root string) {
	t.Helper()
	var kr *encryption.Keyring
	root, kr = setupEncryptedRepo(t)
	s = mustOpenEncryptedStore(t, root, kr, true)
	return s, root
}

// mustOpenEncryptedStore wires cryptofs.FS(SafeFS(OsFS)) for the
// given keyring and opens an fsstore with it. The raw SafeFS is
// also used as the fsstore's directory handle; the PostWrite hook
// is subscribed so the watcher's self-echo LRU stays correct.
func mustOpenEncryptedStore(
	t *testing.T, root string, kr *encryption.Keyring, withAttachments bool,
) *fsstore.FSStore {
	t.Helper()
	safe := storage.NewSafeFS(storage.NewOsFS())
	enc := cryptofs.New(safe, kr.Recipients(), kr.Identity())

	cfg := fsstore.Config{
		FS:           safe,
		Bytes:        enc,
		WantSealed:   true,
		EntitiesDir:  filepath.Join(root, "entities"),
		RelationsDir: filepath.Join(root, "relations"),
		CacheDir:     filepath.Join(root, ".rela"),
		Schemas:      map[string]store.EntityTypeSchema{"ticket": {Plural: "tickets"}},
	}
	if withAttachments {
		cfg.AttachmentsDir = filepath.Join(root, "attachments")
	}
	s, err := fsstore.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	safe.OnPostWrite(s.RecordWrite)
	return s
}

// mustOpenCleartextStore opens an fsstore with no encryption
// decorator — SafeFS(OsFS) is both the byte handle and the dir
// handle. WantSealed=false, matching the factory's decision branch.
func mustOpenCleartextStore(t *testing.T, root string) *fsstore.FSStore {
	t.Helper()
	safe := storage.NewSafeFS(storage.NewOsFS())
	cfg := fsstore.Config{
		FS:           safe,
		EntitiesDir:  filepath.Join(root, "entities"),
		RelationsDir: filepath.Join(root, "relations"),
		CacheDir:     filepath.Join(root, ".rela"),
		Schemas:      map[string]store.EntityTypeSchema{"ticket": {Plural: "tickets"}},
	}
	s, err := fsstore.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	safe.OnPostWrite(s.RecordWrite)
	return s
}

func TestFSStore_Encrypted_RoundTrip(t *testing.T) {
	s, root := buildEncryptedStore(t)
	ctx := context.Background()

	e := entity.New("TKT-001", "ticket")
	e.Properties["title"] = "confidential ticket"
	e.Content = "only authorized recipients should read this body"
	if err := s.CreateEntity(ctx, e); err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}

	path := filepath.Join(root, "entities", "tickets", "TKT-001.md")
	raw := mustReadFile(t, path)
	if !encryption.LooksSealed(raw) {
		t.Fatalf("on-disk file is not sealed:\n%s", raw)
	}

	got, err := s.GetEntity(ctx, "TKT-001")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got.Properties["title"] != "confidential ticket" {
		t.Errorf("title = %v, want original", got.Properties["title"])
	}
	if got.Content != "only authorized recipients should read this body" {
		t.Errorf("content = %q, want original", got.Content)
	}
}

func TestFSStore_Encrypted_TamperedPayloadSurfacesAsCorrupted(t *testing.T) {
	// C1 regression: tampered on-disk bytes MUST classify as
	// IsCorrupted via the production fsstore.GetEntity path.
	s, root := buildEncryptedStore(t)
	ctx := context.Background()

	e := entity.New("TKT-002", "ticket")
	e.Properties["title"] = "will be tampered"
	e.Content = "payload bytes must be long enough to tamper"
	if err := s.CreateEntity(ctx, e); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(root, "entities", "tickets", "TKT-002.md")
	raw := mustReadFile(t, path)
	raw[len(raw)-5] ^= 0x01
	mustWrite(t, path, raw, 0o644)

	_, err := s.GetEntity(ctx, "TKT-002")
	if err == nil {
		t.Fatal("expected error reading tampered file, got nil")
	}
	if !encryption.IsCorrupted(err) {
		t.Errorf("IsCorrupted(err) = false (err = %v)", err)
	}
	if encryption.IsNoMatchingKey(err) {
		t.Errorf("tamper must not collapse to IsNoMatchingKey (err = %v)", err)
	}
}

func TestFSStore_Encrypted_AttachmentRoundTrip(t *testing.T) {
	root, kr := setupEncryptedRepo(t)
	s := mustOpenEncryptedStore(t, root, kr, true)
	ctx := context.Background()

	e := entity.New("TKT-003", "ticket")
	e.Properties["title"] = "attached"
	if err := s.CreateEntity(ctx, e); err != nil {
		t.Fatal(err)
	}

	body := []byte("secret pdf contents placeholder")
	if err := s.AttachFile(ctx, "TKT-003", "spec", "spec.bin", bytesReader(body)); err != nil {
		t.Fatalf("AttachFile: %v", err)
	}

	attachPath := filepath.Join(root, "attachments", "TKT-003", "spec", "spec.bin")
	rawAttach := mustReadFile(t, attachPath)
	if !encryption.LooksSealed(rawAttach) {
		t.Fatalf("attachment is not sealed:\n%s", rawAttach)
	}

	rc, err := s.ReadAttachment(ctx, "TKT-003", "spec")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("attachment read = %q, want %q", got, body)
	}
}

func TestFSStore_Encrypted_Refuses_CleartextDataFiles(t *testing.T) {
	// WantSealed=true but a cleartext entity file exists on disk.
	// fsstore.New must refuse to open the repo.
	root, kr := setupEncryptedRepo(t)
	entitiesDir := filepath.Join(root, "entities", "tickets")
	mustMkdir(t, entitiesDir)
	mustWrite(t, filepath.Join(entitiesDir, "TKT-C1.md"),
		[]byte("---\nid: TKT-C1\ntype: ticket\ntitle: cleartext\n---\n"), 0o644)

	safe := storage.NewSafeFS(storage.NewOsFS())
	enc := cryptofs.New(safe, kr.Recipients(), kr.Identity())
	_, err := fsstore.New(fsstore.Config{
		FS:           safe,
		Bytes:        enc,
		WantSealed:   true,
		EntitiesDir:  filepath.Join(root, "entities"),
		RelationsDir: filepath.Join(root, "relations"),
		CacheDir:     filepath.Join(root, ".rela"),
		Schemas:      map[string]store.EntityTypeSchema{"ticket": {Plural: "tickets"}},
	})
	if err == nil {
		t.Fatal("fsstore.New should refuse cleartext files when encryption is enabled")
	}
	if !errors.Is(err, fsstore.ErrRepoHasCleartextFilesButEncryptionEnabled) {
		t.Errorf("wrong error: %v", err)
	}
}

func TestFSStore_Cleartext_Refuses_SealedDataFiles(t *testing.T) {
	// Crypto is identityCrypto (cleartext mode), but a sealed file
	// exists on disk. fsstore.New must refuse.
	root := t.TempDir()
	id := mustGenerateIdentity(t)
	sealed, err := encryption.Seal([]byte("---\nid: TKT-S1\ntype: ticket\n---\n"),
		[]encryption.Recipient{id.PublicRecipient()})
	if err != nil {
		t.Fatal(err)
	}
	entitiesDir := filepath.Join(root, "entities", "tickets")
	mustMkdir(t, entitiesDir)
	mustWrite(t, filepath.Join(entitiesDir, "TKT-S1.md"), sealed, 0o644)

	_, err = fsstore.New(fsstore.Config{
		FS:           storage.NewOsFS(),
		EntitiesDir:  filepath.Join(root, "entities"),
		RelationsDir: filepath.Join(root, "relations"),
		CacheDir:     filepath.Join(root, ".rela"),
		Schemas:      map[string]store.EntityTypeSchema{"ticket": {Plural: "tickets"}},
	})
	if err == nil {
		t.Fatal("fsstore.New should refuse sealed files without encryption configured")
	}
	if !errors.Is(err, fsstore.ErrRepoHasSealedFilesButNoConfig) {
		t.Errorf("wrong error: %v", err)
	}
}

func TestFSStore_Cleartext_Roundtrip_Unchanged(t *testing.T) {
	// Cleartext mode: files on disk are plain markdown, no sealing.
	root := t.TempDir()
	s := mustOpenCleartextStore(t, root)
	ctx := context.Background()
	e := entity.New("TKT-C2", "ticket")
	e.Properties["title"] = "cleartext"
	if err := s.CreateEntity(ctx, e); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "entities", "tickets", "TKT-C2.md")
	raw := mustReadFile(t, path)
	if encryption.LooksSealed(raw) {
		t.Fatalf("cleartext mode wrote a sealed file!\n%s", raw)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
