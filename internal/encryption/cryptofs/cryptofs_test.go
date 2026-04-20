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

// newCryptoFS builds a cryptofs.FS over a raw MemFS using "/" as
// the repo root (MemFS treats /-prefixed paths as absolute). Each
// test has a single-recipient, single-identity setup.
//
// State is nil — rollback detection is covered by dedicated tests;
// round-trip tests don't need the XDG state machinery.
func newCryptoFS(t *testing.T) (*cryptofs.FS, *storage.MemFS) {
	t.Helper()
	mem := storage.NewMemFS()
	id := newTestIdentity(t)
	fs, err := cryptofs.New(cryptofs.Config{
		Inner:        mem,
		Recipients:   []encryption.Recipient{id.PublicRecipient()},
		Identity:     id,
		RepoRoot:     "/",
		WriteVersion: 1,
	})
	if err != nil {
		t.Fatalf("cryptofs.New: %v", err)
	}
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

	// Alice seals; Bob tries to read.
	writer, err := cryptofs.New(cryptofs.Config{
		Inner:        mem,
		Recipients:   []encryption.Recipient{alice.PublicRecipient()},
		Identity:     alice,
		RepoRoot:     "/",
		WriteVersion: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = writer.WriteFile("/not-for-bob.md", []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Bob's cryptofs uses Bob's identity; sealed blob isn't addressed
	// to Bob, so Unseal returns ErrNoMatchingKey.
	reader, err := cryptofs.New(cryptofs.Config{
		Inner:        mem,
		Recipients:   []encryption.Recipient{bob.PublicRecipient()},
		Identity:     bob,
		RepoRoot:     "/",
		WriteVersion: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = reader.ReadFile("/not-for-bob.md")
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

func TestNew_ValidatesConfig(t *testing.T) {
	mem := storage.NewMemFS()
	id := newTestIdentity(t)

	t.Run("empty recipients", func(t *testing.T) {
		_, err := cryptofs.New(cryptofs.Config{
			Inner: mem, Identity: id, RepoRoot: "/", WriteVersion: 1,
		})
		if err == nil {
			t.Fatal("expected error for empty recipients")
		}
	})
	t.Run("nil identity", func(t *testing.T) {
		_, err := cryptofs.New(cryptofs.Config{
			Inner:        mem,
			Recipients:   []encryption.Recipient{id.PublicRecipient()},
			RepoRoot:     "/",
			WriteVersion: 1,
		})
		if err == nil {
			t.Fatal("expected error for nil identity")
		}
	})
	t.Run("empty repo root", func(t *testing.T) {
		_, err := cryptofs.New(cryptofs.Config{
			Inner:        mem,
			Recipients:   []encryption.Recipient{id.PublicRecipient()},
			Identity:     id,
			WriteVersion: 1,
		})
		if err == nil {
			t.Fatal("expected error for empty repo root")
		}
	})
	t.Run("relative repo root", func(t *testing.T) {
		_, err := cryptofs.New(cryptofs.Config{
			Inner:        mem,
			Recipients:   []encryption.Recipient{id.PublicRecipient()},
			Identity:     id,
			RepoRoot:     "relative/path",
			WriteVersion: 1,
		})
		if err == nil {
			t.Fatal("expected error for relative repo root")
		}
	})
	t.Run("zero version", func(t *testing.T) {
		_, err := cryptofs.New(cryptofs.Config{
			Inner:        mem,
			Recipients:   []encryption.Recipient{id.PublicRecipient()},
			Identity:     id,
			RepoRoot:     "/",
			WriteVersion: 0,
		})
		if err == nil {
			t.Fatal("expected error for zero version")
		}
	})
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

func TestRename_TripsPathCheckOnRead(t *testing.T) {
	// A bare rename moves the ciphertext but the header inside still
	// names the old path. ReadFile at the new path surfaces
	// ErrFileRelocated — precisely the swap/rename defense.
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

	_, err := fs.ReadFile("/new.md")
	if err == nil {
		t.Fatal("expected ErrFileRelocated reading a renamed sealed file")
	}
	if !encryption.IsFileRelocated(err) {
		t.Errorf("IsFileRelocated(err) = false (err = %v)", err)
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
	if info.Size() <= int64(len(plaintext)) {
		t.Errorf("ciphertext size %d not greater than plaintext %d", info.Size(), len(plaintext))
	}
}

func TestReadFile_SwapBetweenFilesDetected(t *testing.T) {
	// Adversary swaps two sealed files on disk. Both still decrypt
	// (same recipient key) but each now has a mismatched path
	// header. Reads through cryptofs must surface ErrFileRelocated
	// rather than returning the wrong entity's bytes.
	fs, mem := newCryptoFS(t)
	if err := fs.WriteFile("/a.md", []byte("contents-of-a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fs.WriteFile("/b.md", []byte("contents-of-b"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Swap the ciphertext bytes between the two paths.
	rawA, _ := mem.ReadFile("/a.md")
	rawB, _ := mem.ReadFile("/b.md")
	_ = mem.WriteFile("/a.md", rawB, 0o644)
	_ = mem.WriteFile("/b.md", rawA, 0o644)

	_, err := fs.ReadFile("/a.md")
	if !encryption.IsFileRelocated(err) {
		t.Errorf("swap on /a.md: IsFileRelocated = false (err = %v)", err)
	}
}

func TestReadFile_RollbackDetected(t *testing.T) {
	// Wire a LocalState explicitly so the rollback check has
	// something to compare against.
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	state, err := encryption.NewLocalState("test-repo")
	if err != nil {
		t.Fatal(err)
	}

	mem := storage.NewMemFS()
	id := newTestIdentity(t)

	// v=5 writer seals the "new" version of the file.
	newWriter, err := cryptofs.New(cryptofs.Config{
		Inner:        mem,
		Recipients:   []encryption.Recipient{id.PublicRecipient()},
		Identity:     id,
		RepoRoot:     "/",
		WriteVersion: 5,
		State:        state,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = newWriter.WriteFile("/vers.md", []byte("v5 body"), 0o644); err != nil {
		t.Fatal(err)
	}
	// First read advances last-seen to 5.
	if _, rerr := newWriter.ReadFile("/vers.md"); rerr != nil {
		t.Fatal(rerr)
	}

	// Adversary rolls the file back: overwrite with a v=2 blob.
	oldWriter, err := cryptofs.New(cryptofs.Config{
		Inner:        mem,
		Recipients:   []encryption.Recipient{id.PublicRecipient()},
		Identity:     id,
		RepoRoot:     "/",
		WriteVersion: 2,
		State:        nil, // rollback attacker doesn't touch our state
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = oldWriter.WriteFile("/vers.md", []byte("v2 body"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Reading through the state-aware FS now detects the rollback.
	_, err = newWriter.ReadFile("/vers.md")
	if !encryption.IsRollbackDetected(err) {
		t.Errorf("IsRollbackDetected(err) = false (err = %v)", err)
	}
}

func TestReadFile_TOFUAcceptsFirstVersion(t *testing.T) {
	// First read on a new machine (no local state) must accept
	// whatever version it sees and persist it.
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	state, err := encryption.NewLocalState("tofu-repo")
	if err != nil {
		t.Fatal(err)
	}

	mem := storage.NewMemFS()
	id := newTestIdentity(t)
	fs, err := cryptofs.New(cryptofs.Config{
		Inner:        mem,
		Recipients:   []encryption.Recipient{id.PublicRecipient()},
		Identity:     id,
		RepoRoot:     "/",
		WriteVersion: 42,
		State:        state,
	})
	if err != nil {
		t.Fatal(err)
	}
	if werr := fs.WriteFile("/t.md", []byte("hello"), 0o644); werr != nil {
		t.Fatal(werr)
	}
	if _, rerr := fs.ReadFile("/t.md"); rerr != nil {
		t.Errorf("first-read TOFU should succeed, got %v", rerr)
	}
	// Subsequent rollback is now detected.
	stored, err := state.LoadVersion()
	if err != nil {
		t.Fatal(err)
	}
	if stored != 42 {
		t.Errorf("stored version = %d, want 42", stored)
	}
}
