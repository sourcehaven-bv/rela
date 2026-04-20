package encryption

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func newTestSentinel(t *testing.T) *ResealSentinel {
	t.Helper()
	alice := newTestIdentity(t)
	return &ResealSentinel{
		FromVersion: 3,
		ToVersion:   4,
		RepoRoot:    "/tmp/some-repo",
		NewRecipients: map[string]string{
			"alice": alice.PublicRecipient().String(),
		},
		Operation: "keys add bob",
	}
}

func TestResealSentinel_RoundTrip(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	s := newTestSentinel(t)
	if err := WriteResealSentinel("repo-1", s); err != nil {
		t.Fatalf("WriteResealSentinel: %v", err)
	}
	got, err := ReadResealSentinel("repo-1")
	if err != nil {
		t.Fatalf("ReadResealSentinel: %v", err)
	}
	if got.FromVersion != 3 || got.ToVersion != 4 {
		t.Errorf("version round-trip: got %+v", got)
	}
	if got.RepoRoot != "/tmp/some-repo" {
		t.Errorf("RepoRoot round-trip: got %q", got.RepoRoot)
	}
	if got.Operation != "keys add bob" {
		t.Errorf("Operation round-trip: got %q", got.Operation)
	}
	if len(got.NewRecipients) != 1 {
		t.Errorf("NewRecipients len = %d, want 1", len(got.NewRecipients))
	}
}

func TestResealSentinel_MissingIsNotExist(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	_, err := ReadResealSentinel("nobody-home")
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
}

func TestResealSentinel_Delete(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	s := newTestSentinel(t)
	if err := WriteResealSentinel("repo-D", s); err != nil {
		t.Fatal(err)
	}
	if err := DeleteResealSentinel("repo-D"); err != nil {
		t.Fatalf("DeleteResealSentinel: %v", err)
	}
	if _, err := ReadResealSentinel("repo-D"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("sentinel not deleted, got %v", err)
	}
	// Idempotent on a second delete.
	if err := DeleteResealSentinel("repo-D"); err != nil {
		t.Errorf("second delete should be idempotent, got %v", err)
	}
}

func TestResealSentinel_ValidateRejectsBadState(t *testing.T) {
	cases := []struct {
		name string
		mut  func(s *ResealSentinel)
	}{
		{"zero from", func(s *ResealSentinel) { s.FromVersion = 0 }},
		{"to not greater", func(s *ResealSentinel) { s.ToVersion = s.FromVersion }},
		{"to less than from", func(s *ResealSentinel) { s.ToVersion = s.FromVersion - 1 }},
		{"relative repo root", func(s *ResealSentinel) { s.RepoRoot = "relative/bad" }},
		{"empty recipients", func(s *ResealSentinel) { s.NewRecipients = nil }},
		{"empty operation", func(s *ResealSentinel) { s.Operation = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestSentinel(t)
			tc.mut(s)
			if err := s.Validate(); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestResealSentinel_NewRecipientListIsSorted(t *testing.T) {
	alice := newTestIdentity(t)
	bob := newTestIdentity(t)
	carol := newTestIdentity(t)
	s := &ResealSentinel{
		FromVersion: 1,
		ToVersion:   2,
		RepoRoot:    "/tmp/r",
		Operation:   "keys add",
		NewRecipients: map[string]string{
			"carol": carol.PublicRecipient().String(),
			"alice": alice.PublicRecipient().String(),
			"bob":   bob.PublicRecipient().String(),
		},
	}
	list, err := s.NewRecipientList()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		alice.PublicRecipient().String(),
		bob.PublicRecipient().String(),
		carol.PublicRecipient().String(),
	}
	for i, r := range list {
		if r.String() != want[i] {
			t.Errorf("sort order: [%d] = %s, want %s", i, r.String(), want[i])
		}
	}
}

func TestResealSentinel_MalformedFileRejected(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	// Create the directory and plant a broken YAML file at the
	// expected path; Read must surface an error rather than
	// silently accepting it.
	state, err := NewLocalState("repo-M")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(state.root, stateDirPerm); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(
		filepath.Join(state.root, resealSentinelFile),
		[]byte("not: valid: yaml: [\n"), stateFilePerm,
	); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadResealSentinel("repo-M"); err == nil {
		t.Error("expected parse error for malformed sentinel")
	}
}

func TestResealSentinel_RejectedOnWriteIfInvalid(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	s := newTestSentinel(t)
	s.ToVersion = s.FromVersion // invalid
	if err := WriteResealSentinel("bad-repo", s); err == nil {
		t.Error("expected validation error on write")
	}
}
