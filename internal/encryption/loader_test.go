package encryption

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromDir_AllPresent(t *testing.T) {
	root := t.TempDir()
	alice := newTestIdentity(t)

	mustMkdir(t, filepath.Join(root, projectKeysDir))
	mustMkdir(t, filepath.Join(root, projectRelaDir))
	mustWrite(t, filepath.Join(root, projectKeysDir, "alice.pub"),
		[]byte(alice.PublicRecipient().String()+"\n"), 0o644)
	mustWrite(t, filepath.Join(root, projectRelaDir, projectKeyFile),
		[]byte(alice.(*hybridIdentity).i.String()+"\n"), 0o600)

	// Clear env so $RELA_KEY_FILE doesn't interfere.
	t.Setenv(envKeyFile, "")

	kr, err := LoadFromDir(root)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if kr.LocalName() != "alice" {
		t.Errorf("LocalName = %q, want alice", kr.LocalName())
	}
}

func TestLoadFromDir_MissingIdentityIsFine(t *testing.T) {
	root := t.TempDir()
	alice := newTestIdentity(t)
	mustMkdir(t, filepath.Join(root, projectKeysDir))
	mustWrite(t, filepath.Join(root, projectKeysDir, "alice.pub"),
		[]byte(alice.PublicRecipient().String()+"\n"), 0o644)

	t.Setenv(envKeyFile, "")
	t.Setenv("HOME", t.TempDir()) // empty home dir, no ~/.config/rela/key

	kr, err := LoadFromDir(root)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if kr.HasIdentity() {
		t.Error("HasIdentity = true, want false (no identity configured anywhere)")
	}
}

func TestLoadFromDir_EnvOverride(t *testing.T) {
	root := t.TempDir()
	alice := newTestIdentity(t)
	mustMkdir(t, filepath.Join(root, projectKeysDir))
	mustWrite(t, filepath.Join(root, projectKeysDir, "alice.pub"),
		[]byte(alice.PublicRecipient().String()+"\n"), 0o644)

	// Put identity at an arbitrary path and point $RELA_KEY_FILE at it.
	custom := filepath.Join(t.TempDir(), "custom.key")
	mustWrite(t, custom, []byte(alice.(*hybridIdentity).i.String()+"\n"), 0o600)
	t.Setenv(envKeyFile, custom)

	kr, err := LoadFromDir(root)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if kr.LocalName() != "alice" {
		t.Errorf("LocalName = %q, want alice (should have loaded via $RELA_KEY_FILE)", kr.LocalName())
	}
}

func TestLoadFromDir_EnvOverrideMissingFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv(envKeyFile, filepath.Join(t.TempDir(), "does-not-exist"))

	_, err := LoadFromDir(root)
	if err == nil {
		t.Fatal("LoadFromDir with missing $RELA_KEY_FILE target should error")
	}
}

func TestResolveIdentityPath_ProjectWins(t *testing.T) {
	relaDir := t.TempDir()
	mustWrite(t, filepath.Join(relaDir, projectKeyFile), []byte("x"), 0o600)
	t.Setenv(envKeyFile, "")
	got, err := resolveIdentityPath(relaDir, func() (string, error) { return "", errors.New("no home") })
	if err != nil {
		t.Fatalf("resolveIdentityPath: %v", err)
	}
	if got != filepath.Join(relaDir, projectKeyFile) {
		t.Errorf("got %q, want project path", got)
	}
}

func TestResolveIdentityPath_HomeFallback(t *testing.T) {
	relaDir := t.TempDir() // empty, no .rela/key
	home := t.TempDir()
	mustMkdir(t, filepath.Join(home, ".config", userConfigSubdir))
	mustWrite(t, filepath.Join(home, ".config", userConfigSubdir, projectKeyFile), []byte("x"), 0o600)
	t.Setenv(envKeyFile, "")

	got, err := resolveIdentityPath(relaDir, func() (string, error) { return home, nil })
	if err != nil {
		t.Fatalf("resolveIdentityPath: %v", err)
	}
	want := filepath.Join(home, ".config", userConfigSubdir, projectKeyFile)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveIdentityPath_NothingConfigured(t *testing.T) {
	relaDir := t.TempDir()
	t.Setenv(envKeyFile, "")
	got, err := resolveIdentityPath(relaDir, func() (string, error) { return "", errors.New("no home") })
	if err != nil {
		t.Fatalf("resolveIdentityPath: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

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
