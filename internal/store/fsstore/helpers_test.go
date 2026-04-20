package fsstore_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/encryption"
)

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path string, data []byte, perm os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, perm); err != nil {
		t.Fatal(err)
	}
}

// mustGenerateIdentity returns a fresh age identity.
func mustGenerateIdentity(t *testing.T) encryption.Identity {
	t.Helper()
	id, err := encryption.GenerateIdentity()
	if err != nil {
		t.Fatalf("GenerateIdentity: %v", err)
	}
	return id
}

// identityPrivate returns the AGE-SECRET-KEY-PQ-1... encoding of id
// so tests can write it to a key file and load it via LoadKeyring.
func identityPrivate(t *testing.T, id encryption.Identity) string {
	t.Helper()
	s, err := encryption.MarshalIdentity(id)
	if err != nil {
		t.Fatalf("MarshalIdentity: %v", err)
	}
	return s
}
