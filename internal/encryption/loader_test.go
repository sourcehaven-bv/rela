package encryption

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func staticHome(path string) func() (string, error) {
	return func() (string, error) { return path, nil }
}

func TestResolvePrivateKeyPath_EnvWins(t *testing.T) {
	tmp := t.TempDir()
	envPath := filepath.Join(tmp, "env-key")
	if err := os.WriteFile(envPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envKeyFile, envPath)
	got, err := resolvePrivateKeyPath(tmp, staticHome(tmp))
	if err != nil {
		t.Fatal(err)
	}
	if got != envPath {
		t.Fatalf("got %q, want %q", got, envPath)
	}
}

func TestResolvePrivateKeyPath_EnvMissingFile(t *testing.T) {
	t.Setenv(envKeyFile, filepath.Join(t.TempDir(), "nope"))
	_, err := resolvePrivateKeyPath(t.TempDir(), staticHome(t.TempDir()))
	if err == nil {
		t.Fatal("expected error for explicit env pointing at missing file")
	}
}

func TestResolvePrivateKeyPath_ProjectLocal(t *testing.T) {
	t.Setenv(envKeyFile, "")
	tmp := t.TempDir()
	relaDir := filepath.Join(tmp, projectRelaDir)
	if err := os.MkdirAll(relaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(relaDir, projectKeyFile)
	if err := os.WriteFile(want, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := resolvePrivateKeyPath(relaDir, staticHome(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolvePrivateKeyPath_UserDefault(t *testing.T) {
	t.Setenv(envKeyFile, "")
	home := t.TempDir()
	userKey := filepath.Join(home, ".config", userConfigSubdir, projectKeyFile)
	if err := os.MkdirAll(filepath.Dir(userKey), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userKey, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := resolvePrivateKeyPath(t.TempDir(), staticHome(home))
	if err != nil {
		t.Fatal(err)
	}
	if got != userKey {
		t.Fatalf("got %q, want %q", got, userKey)
	}
}

func TestResolvePrivateKeyPath_AllMissing(t *testing.T) {
	t.Setenv(envKeyFile, "")
	got, err := resolvePrivateKeyPath(t.TempDir(), staticHome(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestResolvePrivateKeyPath_NoHome(t *testing.T) {
	t.Setenv(envKeyFile, "")
	got, err := resolvePrivateKeyPath(t.TempDir(), func() (string, error) {
		return "", errors.New("no home")
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("got %q, want empty when home is unavailable", got)
	}
}

func TestLoadFromDir_EndToEnd(t *testing.T) {
	// Set up a temp projectRoot with keys/alice.pub and .rela/key.
	t.Setenv(envKeyFile, "")
	root := t.TempDir()
	keysDir := filepath.Join(root, projectKeysDir)
	if err := os.MkdirAll(keysDir, 0o755); err != nil {
		t.Fatal(err)
	}
	relaDir := filepath.Join(root, projectRelaDir)
	if err := os.MkdirAll(relaDir, 0o755); err != nil {
		t.Fatal(err)
	}

	alice := mustGenerate(t)
	pemBytes, err := MarshalPublicKeyPEM(alice.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	if werr := os.WriteFile(filepath.Join(keysDir, "alice.pub"), pemBytes, 0o644); werr != nil {
		t.Fatal(werr)
	}

	privBytes, err := MarshalPrivateKeyPEM(alice)
	if err != nil {
		t.Fatal(err)
	}
	if werr := os.WriteFile(filepath.Join(relaDir, projectKeyFile), privBytes, 0o600); werr != nil {
		t.Fatal(werr)
	}

	// Home should not be consulted because project-local wins.
	kr, err := LoadFromDir(root)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if !kr.HasPrivateKey() {
		t.Fatal("expected private key to be loaded")
	}
	recip, ok := kr.Recipient("alice")
	if !ok {
		t.Fatal("alice not loaded")
	}
	dk, _ := NewDataKey()
	wrapped, err := WrapKey(dk, recip)
	if err != nil {
		t.Fatal(err)
	}
	got, err := kr.Unwrap(wrapped)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, dk) {
		t.Fatal("round-trip mismatch via LoadFromDir")
	}

	// Seal/Open round-trip using the recovered data key.
	sealed, err := Seal([]byte("secret payload"), got)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := Open(sealed, got)
	if err != nil {
		t.Fatal(err)
	}
	if string(opened) != "secret payload" {
		t.Fatalf("opened = %q", opened)
	}
}

func TestLoadFromDir_NoPrivateKey(t *testing.T) {
	t.Setenv(envKeyFile, "")
	root := t.TempDir()
	// No keys dir, no .rela/key, no user default. userHomeDir returns
	// the real home; we override the env and rely on missing-file.
	// To avoid accidentally picking up a real ~/.config/rela/key, set
	// HOME to a tmp dir on platforms that respect it.
	t.Setenv("HOME", t.TempDir())
	kr, err := LoadFromDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if kr.HasPrivateKey() {
		t.Fatal("should have no private key")
	}
}
