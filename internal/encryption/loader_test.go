package encryption

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeRepoSetup seals a single-recipient recipients.age at
// <root>/recipients.age and writes the matching identity file at
// identityPath. Used by LoadFromDir tests that need a working
// encrypted repo on disk.
func writeRepoSetup(t *testing.T, root string, alice Identity, identityPath string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(identityPath))
	mustWrite(t, identityPath, []byte(alice.(*hybridIdentity).i.String()+"\n"), 0o600)

	repoID, err := NewRepoID()
	if err != nil {
		t.Fatal(err)
	}
	rf := &RecipientsFile{
		Version:    1,
		RepoID:     repoID,
		Recipients: map[string]string{"alice": alice.PublicRecipient().String()},
	}
	if err := WriteRecipientsFile(filepath.Join(root, RecipientsFileName), rf); err != nil {
		t.Fatal(err)
	}
}

func TestLoadFromDir_AllPresent(t *testing.T) {
	root := t.TempDir()
	alice := newTestIdentity(t)
	writeRepoSetup(t, root, alice, filepath.Join(root, projectRelaDir, projectKeyFile))

	// Clear env so $RELA_KEY_FILE doesn't interfere.
	t.Setenv(envKeyFile, "")

	kr, err := LoadFromDir(root)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if kr.LocalName() != "alice" {
		t.Errorf("LocalName = %q, want alice", kr.LocalName())
	}
	if kr.Version() != 1 {
		t.Errorf("Version = %d, want 1", kr.Version())
	}
}

func TestLoadFromDir_NoIdentityIsError(t *testing.T) {
	// recipients.age is encrypted, so loading without an identity
	// cannot succeed — surface it as a hard error rather than a
	// silently-crippled keyring.
	root := t.TempDir()
	alice := newTestIdentity(t)
	repoID, _ := NewRepoID()
	rf := &RecipientsFile{
		Version:    1,
		RepoID:     repoID,
		Recipients: map[string]string{"alice": alice.PublicRecipient().String()},
	}
	if err := WriteRecipientsFile(filepath.Join(root, RecipientsFileName), rf); err != nil {
		t.Fatal(err)
	}

	t.Setenv(envKeyFile, "")
	t.Setenv("HOME", t.TempDir()) // empty home, no ~/.config/rela/key

	_, err := LoadFromDir(root)
	if !errors.Is(err, ErrNoPrivateKey) {
		t.Errorf("expected ErrNoPrivateKey, got %v", err)
	}
}

func TestLoadFromDir_EnvOverride(t *testing.T) {
	root := t.TempDir()
	alice := newTestIdentity(t)

	// Put identity at an arbitrary path and point $RELA_KEY_FILE at it.
	custom := filepath.Join(t.TempDir(), "custom.key")
	writeRepoSetup(t, root, alice, custom)
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

func TestIsEnabled(t *testing.T) {
	t.Run("no recipients file", func(t *testing.T) {
		root := t.TempDir()
		got, err := IsEnabled(root)
		if err != nil {
			t.Fatalf("IsEnabled: %v", err)
		}
		if got {
			t.Error("IsEnabled = true, want false (no recipients.age)")
		}
	})

	t.Run("recipients file present", func(t *testing.T) {
		root := t.TempDir()
		// Contents don't matter — IsEnabled is a presence check.
		mustWrite(t, filepath.Join(root, RecipientsFileName), []byte("dummy"), 0o644)
		got, err := IsEnabled(root)
		if err != nil {
			t.Fatalf("IsEnabled: %v", err)
		}
		if !got {
			t.Error("IsEnabled = false, want true")
		}
	})
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, perm); err != nil {
		t.Fatal(err)
	}
}
