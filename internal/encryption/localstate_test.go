package encryption

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalState_TOFU_ReadsZero(t *testing.T) {
	// Point XDG at an empty tree so no prior state exists.
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	s, err := NewLocalState("abc123")
	if err != nil {
		t.Fatalf("NewLocalState: %v", err)
	}
	got, err := s.LoadVersion()
	if err != nil {
		t.Fatalf("LoadVersion: %v", err)
	}
	if got != 0 {
		t.Errorf("LoadVersion on empty state = %d, want 0 (TOFU)", got)
	}
}

func TestLocalState_StoreAndLoad(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	s, err := NewLocalState("repo-1")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.StoreVersion(7); err != nil {
		t.Fatalf("StoreVersion: %v", err)
	}
	got, err := s.LoadVersion()
	if err != nil {
		t.Fatal(err)
	}
	if got != 7 {
		t.Errorf("LoadVersion = %d, want 7", got)
	}
}

func TestLocalState_PerRepoIsolation(t *testing.T) {
	// Two repos on the same machine must not see each other's
	// version state. That's what repo_id is for.
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	s1, err := NewLocalState("repo-A")
	if err != nil {
		t.Fatal(err)
	}
	s2, err := NewLocalState("repo-B")
	if err != nil {
		t.Fatal(err)
	}
	if sErr := s1.StoreVersion(42); sErr != nil {
		t.Fatal(sErr)
	}

	got, err := s2.LoadVersion()
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Errorf("repo-B saw repo-A's state: got %d, want 0", got)
	}
}

func TestLocalState_CorruptedFileErrors(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	s, err := NewLocalState("repo-X")
	if err != nil {
		t.Fatal(err)
	}
	// Manually plant garbage where the version file should be.
	if err = os.MkdirAll(s.root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(s.root, versionFile),
		[]byte("not-an-integer\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = s.LoadVersion()
	if !errors.Is(err, ErrCorruptedLocalState) {
		t.Errorf("expected ErrCorruptedLocalState, got %v", err)
	}
}

func TestLocalState_NegativeVersionRejected(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	s, err := NewLocalState("repo-Y")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(s.root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(s.root, versionFile),
		[]byte("-5\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = s.LoadVersion()
	if !errors.Is(err, ErrCorruptedLocalState) {
		t.Errorf("expected ErrCorruptedLocalState for negative version, got %v", err)
	}
}

func TestLocalState_EmptyRepoIDRejected(t *testing.T) {
	if _, err := NewLocalState(""); err == nil {
		t.Error("NewLocalState with empty repo id should error")
	}
}

func TestLocalState_XDGOverride(t *testing.T) {
	// XDG_STATE_HOME takes precedence over the per-OS default.
	custom := t.TempDir()
	t.Setenv("XDG_STATE_HOME", custom)

	s, err := NewLocalState("repo-Z")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.StoreVersion(3); err != nil {
		t.Fatal(err)
	}
	// The file must land under $XDG_STATE_HOME/rela/repos/<id>/.
	want := filepath.Join(custom, "rela", "repos", "repo-Z", versionFile)
	if _, err := os.Stat(want); err != nil {
		t.Errorf("version file not at XDG location %s: %v", want, err)
	}
}

func TestLocalState_AtomicWrite(t *testing.T) {
	// A crash mid-write (no rename) must not leave a partial
	// version file where LoadVersion would see zero-length content.
	// We can't kill the process mid-StoreVersion; instead verify
	// the tmp-file + rename pattern by observing no .tmp is left
	// behind on successful write.
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	s, err := NewLocalState("repo-A")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.StoreVersion(1); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(s.root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == versionFile+".tmp" {
			t.Errorf("orphan .tmp left behind after successful StoreVersion")
		}
	}
}
