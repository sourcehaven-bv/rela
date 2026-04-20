package encryption

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/userstate"
)

func TestLocalState_TOFU_ReadsZero(t *testing.T) {
	svc := userstate.NewForTest(t.TempDir())
	s, err := NewLocalState(svc)
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
	svc := userstate.NewForTest(t.TempDir())
	s, err := NewLocalState(svc)
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
	// version state. Scoping happens at the userstate layer now —
	// each project gets its own FSService rooted at a distinct
	// path, so a typical caller never shares one.
	svcA := userstate.NewForTest(t.TempDir())
	svcB := userstate.NewForTest(t.TempDir())
	sA, err := NewLocalState(svcA)
	if err != nil {
		t.Fatal(err)
	}
	sB, err := NewLocalState(svcB)
	if err != nil {
		t.Fatal(err)
	}
	if sErr := sA.StoreVersion(42); sErr != nil {
		t.Fatal(sErr)
	}

	got, err := sB.LoadVersion()
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Errorf("repo-B saw repo-A's state: got %d, want 0", got)
	}
}

func TestLocalState_CorruptedFileErrors(t *testing.T) {
	root := t.TempDir()
	svc := userstate.NewForTest(root)
	s, err := NewLocalState(svc)
	if err != nil {
		t.Fatal(err)
	}
	// Plant garbage where the version file should be.
	if err = os.WriteFile(filepath.Join(root, versionKey),
		[]byte("not-an-integer\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = s.LoadVersion()
	if !errors.Is(err, ErrCorruptedLocalState) {
		t.Errorf("expected ErrCorruptedLocalState, got %v", err)
	}
}

func TestLocalState_NegativeVersionRejected(t *testing.T) {
	root := t.TempDir()
	svc := userstate.NewForTest(root)
	s, err := NewLocalState(svc)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, versionKey),
		[]byte("-5\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = s.LoadVersion()
	if !errors.Is(err, ErrCorruptedLocalState) {
		t.Errorf("expected ErrCorruptedLocalState for negative version, got %v", err)
	}
}

func TestLocalState_NilServiceRejected(t *testing.T) {
	if _, err := NewLocalState(nil); err == nil {
		t.Error("NewLocalState with nil service should error")
	}
}

func TestLocalState_AtomicWrite(t *testing.T) {
	// A crash mid-write (no rename) must not leave a partial
	// version file where LoadVersion would see zero-length content.
	// We can't kill the process mid-StoreVersion; instead verify
	// the tmp-file + rename pattern by observing no .tmp is left
	// behind on successful write.
	root := t.TempDir()
	svc := userstate.NewForTest(root)
	s, err := NewLocalState(svc)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.StoreVersion(1); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == versionKey+".tmp" {
			t.Errorf("orphan .tmp left behind after successful StoreVersion")
		}
	}
}
