package attachment_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/encryption"
	"github.com/Sourcehaven-BV/rela/internal/encryption/cryptofs"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// TestStoreWritesSealedBytesThroughCryptoFS verifies the fix for
// encryption-security-review finding C1: attachment contents and
// metadata sidecars written via attachment.Store on an encrypted repo
// must land sealed on disk, not cleartext.
//
// The test builds a store.Store with bytes = cryptofs.FS and then
// AttachFile()s a payload with distinctive plaintext. It then reads
// the raw file off the real filesystem (no decorator) and asserts:
//
//  1. The plaintext never appears on disk.
//  2. The file starts with the age magic ("age-encryption.org/v1").
//  3. The metadata sidecar is also sealed (doesn't contain the
//     original filename in the clear).
//  4. Reading back through the store returns the original plaintext.
func TestStoreWritesSealedBytesThroughCryptoFS(t *testing.T) {
	root := t.TempDir()
	id, err := encryption.GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}

	// Real FS + cryptofs decorator, matching production wiring.
	raw := storage.NewSafeFS(storage.NewOsFS())
	sealed, err := cryptofs.New(cryptofs.Config{
		Inner:        raw,
		Recipients:   []encryption.Recipient{id.PublicRecipient()},
		Identity:     id,
		RepoRoot:     root,
		WriteVersion: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	// bytes = sealed; dirs = raw. This is the split the C1 fix
	// introduced.
	s := attachment.NewStore(sealed, raw, root)

	const plaintextMarker = "TOP-SECRET-PAYLOAD-ABCDEF"
	const originalName = "secret-design.pdf"

	_, err = s.AttachFile(
		context.Background(),
		"TKT-001", "spec", originalName,
		strings.NewReader(plaintextMarker),
	)
	if err != nil {
		t.Fatalf("AttachFile: %v", err)
	}

	// Walk attachments/ on the REAL filesystem and inspect every file.
	attachDir := filepath.Join(root, "attachments")
	filesFound := 0
	err = filepath.WalkDir(attachDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		filesFound++
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		if !strings.HasPrefix(string(data), "age-encryption.org/v1") {
			t.Errorf("file %s is not age-sealed; starts with %q", path, headBytes(data, 30))
		}
		if strings.Contains(string(data), plaintextMarker) {
			t.Errorf("plaintext leaked in %s", path)
		}
		if strings.Contains(string(data), originalName) {
			t.Errorf("original filename %q leaked in sidecar %s", originalName, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if filesFound < 2 {
		// Expect content + .yaml sidecar — both must be sealed.
		t.Errorf("expected at least 2 files on disk, found %d", filesFound)
	}

	// Round-trip: reading through the decorated store returns the
	// original plaintext (proves decryption path works too).
	info, err := s.AttachFile(
		context.Background(),
		"TKT-002", "spec", "dup.pdf",
		strings.NewReader(plaintextMarker),
	)
	if err != nil {
		t.Fatalf("AttachFile dup: %v", err)
	}
	got, err := s.Get(info.Key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != plaintextMarker {
		t.Errorf("round-trip = %q, want %q", got, plaintextMarker)
	}
}

func headBytes(b []byte, n int) string {
	if len(b) < n {
		return string(b)
	}
	return string(b[:n])
}
