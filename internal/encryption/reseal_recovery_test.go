package encryption

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/userstate"
)

// setupTestRepo creates an encrypted repo on disk with one sealed
// entity at version fromVersion, returns (root, keyring, svc). The
// keyring is loaded from the resulting state so ResumeInterruptedRotation
// sees a real parsed keyring, not a hand-built one.
func setupTestRepo(t *testing.T, fromVersion int) (string, *Keyring, Identity, userstate.FSService) {
	t.Helper()
	root := t.TempDir()
	svc := userstate.NewForTest(t.TempDir())

	id := newTestIdentity(t)
	priv, err := MarshalIdentity(id)
	if err != nil {
		t.Fatal(err)
	}
	// Install identity via the user-state service — LoadFromDir
	// reads <us-root>/key.
	if err = os.WriteFile(svc.Path(identityKey), []byte(priv+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Seal one entity file under the OLD recipient set at fromVersion.
	entityDir := filepath.Join(root, "entities", "tickets")
	if err = os.MkdirAll(entityDir, 0o755); err != nil {
		t.Fatal(err)
	}
	entityPath := filepath.Join(entityDir, "T-1.md")
	body := []byte("---\nid: T-1\ntype: ticket\n---\ncontent\n")
	sealed, err := sealWithHeader(root, entityPath, body, []Recipient{id.PublicRecipient()}, fromVersion)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(entityPath, sealed, 0o644); err != nil {
		t.Fatal(err)
	}
	// Write recipients.age at fromVersion.
	repoID, err := NewRepoID()
	if err != nil {
		t.Fatal(err)
	}
	rf := &RecipientsFile{
		Version:    fromVersion,
		RepoID:     repoID,
		Recipients: map[string]string{"alice": id.PublicRecipient().String()},
	}
	if err = WriteRecipientsFile(filepath.Join(root, RecipientsFileName), rf); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RELA_KEY_FILE", "")
	kr, err := LoadFromDir(root, svc)
	if err != nil {
		t.Fatal(err)
	}
	return root, kr, id, svc
}

func TestResumeInterruptedRotation_NoSentinelIsNoOp(t *testing.T) {
	root, kr, _, svc := setupTestRepo(t, 1)
	resumed, err := ResumeInterruptedRotation(root, kr, svc)
	if err != nil {
		t.Fatalf("ResumeInterruptedRotation: %v", err)
	}
	if resumed {
		t.Error("resumed = true when no sentinel exists")
	}
}

func TestResumeInterruptedRotation_CompletedButUncleanedSentinel(t *testing.T) {
	// Scenario: a prior rela run completed the rotation (recipients.age
	// is at ToVersion) but crashed before DeleteResealSentinel. Recovery
	// should just delete the sentinel.
	root, kr, id, svc := setupTestRepo(t, 7)
	sentinel := &ResealSentinel{
		FromVersion:   6, // any older value — the rotation already finished at 7
		ToVersion:     7,
		RepoRoot:      root,
		NewRecipients: map[string]string{"alice": id.PublicRecipient().String()},
		Operation:     "keys add alice",
	}
	if err := WriteResealSentinel(svc, sentinel); err != nil {
		t.Fatal(err)
	}

	resumed, err := ResumeInterruptedRotation(root, kr, svc)
	if err != nil {
		t.Fatalf("ResumeInterruptedRotation: %v", err)
	}
	if resumed {
		t.Error("resumed = true but rotation was already complete")
	}
	if _, err := ReadResealSentinel(svc); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("sentinel not cleaned up, got err=%v", err)
	}
}

func TestResumeInterruptedRotation_MidFlightCompletes(t *testing.T) {
	// Scenario: a prior rela run wrote the sentinel, started the walk,
	// crashed before recipients.age was rewritten. On recovery, the
	// walk must finish and recipients.age must land at ToVersion.
	root, kr, aliceID, svc := setupTestRepo(t, 3)
	bobID := newTestIdentity(t)
	// New recipient set = {alice, bob}; new version = 4.
	sentinel := &ResealSentinel{
		FromVersion: 3,
		ToVersion:   4,
		RepoRoot:    root,
		NewRecipients: map[string]string{
			"alice": aliceID.PublicRecipient().String(),
			"bob":   bobID.PublicRecipient().String(),
		},
		Operation: "keys add bob",
	}
	if err := WriteResealSentinel(svc, sentinel); err != nil {
		t.Fatal(err)
	}

	resumed, err := ResumeInterruptedRotation(root, kr, svc)
	if err != nil {
		t.Fatalf("ResumeInterruptedRotation: %v", err)
	}
	if !resumed {
		t.Fatal("resumed = false but rotation was mid-flight")
	}

	// After recovery: recipients.age reflects to_version and the new
	// recipient set; the entity file decrypts for Bob.
	rf, err := ReadRecipientsFile(filepath.Join(root, RecipientsFileName), aliceID)
	if err != nil {
		t.Fatalf("ReadRecipientsFile: %v", err)
	}
	if rf.Version != 4 {
		t.Errorf("recipients.age version = %d, want 4", rf.Version)
	}
	if _, ok := rf.Recipients["bob"]; !ok {
		t.Error("bob missing from recipients after recovery")
	}

	entityPath := filepath.Join(root, "entities", "tickets", "T-1.md")
	rawSealed, err := os.ReadFile(entityPath)
	if err != nil {
		t.Fatal(err)
	}
	plaintextForBob, err := Unseal(rawSealed, bobID)
	if err != nil {
		t.Errorf("bob cannot read re-sealed entity: %v", err)
	}
	h, body, err := ParseHeader(plaintextForBob)
	if err != nil {
		t.Fatal(err)
	}
	if h.Version != 4 {
		t.Errorf("header version on recovered file = %d, want 4", h.Version)
	}
	if string(body) != "---\nid: T-1\ntype: ticket\n---\ncontent\n" {
		t.Errorf("body not preserved: %q", body)
	}

	// Sentinel gone.
	if _, err := ReadResealSentinel(svc); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("sentinel not cleaned up after recovery, got err=%v", err)
	}
}

func TestResumeInterruptedRotation_WrongRepoRootRejected(t *testing.T) {
	// Defense: a machine-local sentinel pointing at a different repo
	// must not be honored. This is belt-and-suspenders — repo_id keys
	// the sentinel's directory, so the wrong repo shouldn't normally
	// find a sentinel at all, but if state directories get copied
	// around we'd rather refuse than rotate the wrong tree.
	root, kr, id, svc := setupTestRepo(t, 1)
	bogus := &ResealSentinel{
		FromVersion:   1,
		ToVersion:     2,
		RepoRoot:      "/tmp/entirely-different-repo",
		NewRecipients: map[string]string{"alice": id.PublicRecipient().String()},
		Operation:     "keys add",
	}
	if err := WriteResealSentinel(svc, bogus); err != nil {
		t.Fatal(err)
	}

	_, err := ResumeInterruptedRotation(root, kr, svc)
	if err == nil {
		t.Error("expected error when sentinel.RepoRoot doesn't match")
	}
}

func TestResumeInterruptedRotation_StaleSentinelRejected(t *testing.T) {
	// A sentinel whose ToVersion is OLDER than the current
	// recipients.age version indicates local state is corrupt.
	// Refuse loudly rather than trying to recover.
	root, kr, id, svc := setupTestRepo(t, 10) // current version = 10
	stale := &ResealSentinel{
		FromVersion:   1,
		ToVersion:     2, // older than current (10)
		RepoRoot:      root,
		NewRecipients: map[string]string{"alice": id.PublicRecipient().String()},
		Operation:     "keys add",
	}
	if err := WriteResealSentinel(svc, stale); err != nil {
		t.Fatal(err)
	}

	_, err := ResumeInterruptedRotation(root, kr, svc)
	if err == nil {
		t.Error("expected error for stale sentinel")
	}
}

func TestResumeInterruptedRotation_IdempotentOnRerun(t *testing.T) {
	// After successful recovery, a second recovery attempt on the
	// same state must be a no-op (no sentinel, nothing to do).
	root, kr, aliceID, svc := setupTestRepo(t, 1)
	bobID := newTestIdentity(t)
	sentinel := &ResealSentinel{
		FromVersion: 1,
		ToVersion:   2,
		RepoRoot:    root,
		NewRecipients: map[string]string{
			"alice": aliceID.PublicRecipient().String(),
			"bob":   bobID.PublicRecipient().String(),
		},
		Operation: "keys add bob",
	}
	if err := WriteResealSentinel(svc, sentinel); err != nil {
		t.Fatal(err)
	}

	// First recovery: resumed = true.
	resumed1, err := ResumeInterruptedRotation(root, kr, svc)
	if err != nil {
		t.Fatalf("first recovery: %v", err)
	}
	if !resumed1 {
		t.Fatal("first recovery should have resumed")
	}

	// Reload kr to reflect the post-recovery state.
	kr2, err := LoadFromDir(root, svc)
	if err != nil {
		t.Fatal(err)
	}

	// Second recovery: resumed = false, error = nil.
	resumed2, err := ResumeInterruptedRotation(root, kr2, svc)
	if err != nil {
		t.Fatalf("second recovery: %v", err)
	}
	if resumed2 {
		t.Error("second recovery resumed — state should be clean")
	}
}

func TestResumeInterruptedRotation_SkipsFilesAlreadyAtNewVersion(t *testing.T) {
	// Mid-flight state: some files are already at newVersion (the
	// walk had renamed them before the crash); others are still at
	// oldVersion. Recovery must re-seal only the stragglers.
	root, kr, aliceID, svc := setupTestRepo(t, 1)
	bobID := newTestIdentity(t)
	newRecipients := []Recipient{
		aliceID.PublicRecipient(),
		bobID.PublicRecipient(),
	}

	// Simulate: T-1.md already re-sealed at v=2 under the new set.
	newBody := []byte("---\nid: T-1\ntype: ticket\n---\ncontent\n")
	sealedNew, err := sealWithHeader(root,
		filepath.Join(root, "entities", "tickets", "T-1.md"),
		newBody, newRecipients, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(
		filepath.Join(root, "entities", "tickets", "T-1.md"),
		sealedNew, 0o644); err != nil {
		t.Fatal(err)
	}

	// Add a SECOND file still sealed at v=1 (the straggler).
	strBody := []byte("---\nid: T-2\ntype: ticket\n---\nstraggler\n")
	sealedOld, err := sealWithHeader(root,
		filepath.Join(root, "entities", "tickets", "T-2.md"),
		strBody, []Recipient{aliceID.PublicRecipient()}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(
		filepath.Join(root, "entities", "tickets", "T-2.md"),
		sealedOld, 0o644); err != nil {
		t.Fatal(err)
	}

	// Sentinel describes the rotation.
	sentinel := &ResealSentinel{
		FromVersion: 1,
		ToVersion:   2,
		RepoRoot:    root,
		NewRecipients: map[string]string{
			"alice": aliceID.PublicRecipient().String(),
			"bob":   bobID.PublicRecipient().String(),
		},
		Operation: "keys add bob",
	}
	if err = WriteResealSentinel(svc, sentinel); err != nil {
		t.Fatal(err)
	}

	resumed, err := ResumeInterruptedRotation(root, kr, svc)
	if err != nil {
		t.Fatalf("ResumeInterruptedRotation: %v", err)
	}
	if !resumed {
		t.Fatal("resumed = false but there was a sentinel")
	}

	// Both files must now be readable by Bob.
	for _, name := range []string{"T-1.md", "T-2.md"} {
		raw, err := os.ReadFile(filepath.Join(root, "entities", "tickets", name))
		if err != nil {
			t.Fatal(err)
		}
		plaintext, err := Unseal(raw, bobID)
		if err != nil {
			t.Errorf("bob cannot read %s after recovery: %v", name, err)
			continue
		}
		h, _, err := ParseHeader(plaintext)
		if err != nil {
			t.Fatal(err)
		}
		if h.Version != 2 {
			t.Errorf("%s version = %d, want 2", name, h.Version)
		}
	}
}
