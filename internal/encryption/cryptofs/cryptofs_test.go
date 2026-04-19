package cryptofs_test

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/encryption"
	"github.com/Sourcehaven-BV/rela/internal/encryption/cryptofs"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// newTestIdentity returns a fresh hybrid identity, failing the test
// if generation fails.
func newTestIdentity(t *testing.T) encryption.Identity {
	t.Helper()
	id, err := encryption.GenerateIdentity()
	if err != nil {
		t.Fatalf("GenerateIdentity: %v", err)
	}
	return id
}

// newCryptoFS builds a cryptofs.FS over a raw MemFS (no SafeFS) with
// a single-recipient, single-identity setup. The identity is kept
// internal; tests that need an explicit identity build their own.
func newCryptoFS(t *testing.T) (*cryptofs.FS, *storage.MemFS) {
	t.Helper()
	mem := storage.NewMemFS()
	id := newTestIdentity(t)
	fs := cryptofs.New(mem, []encryption.Recipient{id.PublicRecipient()}, id)
	return fs, mem
}

func TestWriteFile_SealsBeforeReachingInner(t *testing.T) {
	fs, mem := newCryptoFS(t)

	path := "/note.md"
	plaintext := []byte("top secret body\n")
	if err := fs.WriteFile(path, plaintext, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	raw, err := mem.ReadFile(path)
	if err != nil {
		t.Fatalf("mem.ReadFile: %v", err)
	}
	if !encryption.LooksSealed(raw) {
		t.Errorf("inner FS received unsealed bytes: %q", raw)
	}
	if bytes.Contains(raw, plaintext) {
		t.Errorf("plaintext leaked into inner storage: %q", raw)
	}
}

func TestReadFile_UnsealsWhatWasWritten(t *testing.T) {
	fs, _ := newCryptoFS(t)

	path := "/note.md"
	plaintext := []byte("round-trip bytes")
	if err := fs.WriteFile(path, plaintext, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := fs.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("ReadFile = %q, want %q", got, plaintext)
	}
}

func TestReadFile_NoMatchingKeyClassifiedViaErrorsIs(t *testing.T) {
	mem := storage.NewMemFS()

	alice := newTestIdentity(t)
	bob := newTestIdentity(t)

	// Alice seals; Bob tries to read with Alice as a stranger.
	writer := cryptofs.New(mem, []encryption.Recipient{alice.PublicRecipient()}, alice)
	if err := writer.WriteFile("/not-for-bob.md", []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Bob's cryptofs uses Bob's identity; sealed blob isn't addressed
	// to Bob, so Unseal returns ErrNoMatchingKey.
	reader := cryptofs.New(mem, []encryption.Recipient{bob.PublicRecipient()}, bob)
	_, err := reader.ReadFile("/not-for-bob.md")
	if err == nil {
		t.Fatal("expected error when bob reads a blob addressed to alice")
	}
	if !encryption.IsNoMatchingKey(err) {
		t.Errorf("IsNoMatchingKey(err) = false (err = %v)", err)
	}
	// Tamper classification must NOT collapse to no-matching-key.
	if encryption.IsCorrupted(err) {
		t.Errorf("IsCorrupted(err) = true; no-match must not surface as corruption")
	}
}

func TestReadFile_CorruptedClassifiedViaErrorsIs(t *testing.T) {
	fs, mem := newCryptoFS(t)

	path := "/corrupt.md"
	if err := fs.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Flip a late byte so the header parses but AEAD fails.
	raw, _ := mem.ReadFile(path)
	raw[len(raw)-3] ^= 0xff
	if err := mem.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := fs.ReadFile(path)
	if err == nil {
		t.Fatal("expected error reading corrupted file")
	}
	if !encryption.IsCorrupted(err) {
		t.Errorf("IsCorrupted(err) = false (err = %v)", err)
	}
	if encryption.IsNoMatchingKey(err) {
		t.Errorf("tamper surfaced as IsNoMatchingKey (err = %v); must surface as IsCorrupted", err)
	}
}

func TestReadFile_NoPrivateKeyClassifiedViaErrorsIs(t *testing.T) {
	mem := storage.NewMemFS()
	writer := newTestIdentity(t)

	// Write with a valid writer so the blob on disk is well-formed.
	fs := cryptofs.New(mem, []encryption.Recipient{writer.PublicRecipient()}, writer)
	if err := fs.WriteFile("/blob.md", []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Now read through a FS with nil identity — simulates "no
	// $RELA_KEY_FILE, no .rela/key, no ~/.config/rela/key".
	noID := cryptofs.New(mem, []encryption.Recipient{writer.PublicRecipient()}, nil)
	_, err := noID.ReadFile("/blob.md")
	if err == nil {
		t.Fatal("expected error reading without an identity")
	}
	if !encryption.IsNoPrivateKey(err) {
		t.Errorf("IsNoPrivateKey(err) = false (err = %v)", err)
	}
}

func TestReadFile_NotExistPassesThrough(t *testing.T) {
	fs, _ := newCryptoFS(t)
	_, err := fs.ReadFile("/never/written.md")
	if err == nil {
		t.Fatal("expected error reading missing file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
}

func TestWriteFile_EmptyRecipientsFailsAtSeal(t *testing.T) {
	mem := storage.NewMemFS()
	id := newTestIdentity(t)
	fs := cryptofs.New(mem, nil, id)

	err := fs.WriteFile("/nobody.md", []byte("x"), 0o644)
	if err == nil {
		t.Fatal("expected error sealing for zero recipients")
	}
	// Inner FS must NOT have been touched when seal failed.
	if _, statErr := mem.Stat("/nobody.md"); statErr == nil {
		t.Error("inner FS has file despite sealing failure")
	}
}

func TestRemove_PassesThrough(t *testing.T) {
	fs, mem := newCryptoFS(t)
	if err := fs.WriteFile("/gone.md", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := mem.Stat("/gone.md"); err != nil {
		t.Fatalf("pre-remove stat: %v", err)
	}
	if err := fs.Remove("/gone.md"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := mem.Stat("/gone.md"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("file not removed: %v", err)
	}
}

func TestRename_PassesThrough(t *testing.T) {
	fs, mem := newCryptoFS(t)
	if err := fs.WriteFile("/old.md", []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fs.Rename("/old.md", "/new.md"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if _, err := mem.Stat("/new.md"); err != nil {
		t.Errorf("new path missing after rename: %v", err)
	}

	// Content at the new path must still decrypt cleanly through fs.
	got, err := fs.ReadFile("/new.md")
	if err != nil {
		t.Fatalf("ReadFile after rename: %v", err)
	}
	if !bytes.Equal(got, []byte("payload")) {
		t.Errorf("post-rename plaintext = %q, want %q", got, "payload")
	}
}

func TestStat_ReflectsCiphertextSize(t *testing.T) {
	fs, _ := newCryptoFS(t)

	plaintext := []byte("small")
	if err := fs.WriteFile("/s.md", plaintext, 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := fs.Stat("/s.md")
	if err != nil {
		t.Fatal(err)
	}
	// Age overhead is a fixed constant; ciphertext is always bigger
	// than plaintext. This is a documented side effect of Stat
	// reporting ciphertext metadata.
	if info.Size() <= int64(len(plaintext)) {
		t.Errorf("ciphertext size %d not greater than plaintext %d", info.Size(), len(plaintext))
	}
}
