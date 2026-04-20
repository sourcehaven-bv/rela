package encryption

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeRecipients writes a RecipientsFile at <root>/recipients.age
// sealed to the given identities (mapped by name), and returns the
// path. Convenience helper matching the old writeKeyring shape.
func writeRecipients(
	t *testing.T, root string,
	pubs map[string]Identity, version int, repoID string,
) string {
	t.Helper()
	rf := &RecipientsFile{
		Version:    version,
		RepoID:     repoID,
		Recipients: map[string]string{},
	}
	for name, id := range pubs {
		rf.Recipients[name] = id.PublicRecipient().String()
	}
	path := filepath.Join(root, RecipientsFileName)
	if err := WriteRecipientsFile(path, rf); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadKeyring_Basic(t *testing.T) {
	alice := newTestIdentity(t)
	bob := newTestIdentity(t)
	root := t.TempDir()
	repoID, _ := NewRepoID()

	path := writeRecipients(t, root, map[string]Identity{
		"alice": alice,
		"bob":   bob,
	}, 3, repoID)

	kr, err := LoadKeyring(path, alice)
	if err != nil {
		t.Fatalf("LoadKeyring: %v", err)
	}
	if !kr.HasIdentity() {
		t.Fatal("HasIdentity = false, want true")
	}
	if kr.LocalName() != "alice" {
		t.Errorf("LocalName = %q, want alice", kr.LocalName())
	}
	if got := kr.RecipientNames(); len(got) != 2 || got[0] != "alice" || got[1] != "bob" {
		t.Errorf("RecipientNames = %v, want [alice bob] sorted", got)
	}
	if kr.Version() != 3 {
		t.Errorf("Version = %d, want 3", kr.Version())
	}
	if kr.RepoID() != repoID {
		t.Errorf("RepoID = %q, want %q", kr.RepoID(), repoID)
	}
}

func TestLoadKeyring_NoIdentityRejected(t *testing.T) {
	// LoadKeyring without an identity is nonsensical — the file is
	// encrypted and needs a key to decrypt. Must fail fast.
	_, err := LoadKeyring("any/path", nil)
	if !errors.Is(err, ErrNoPrivateKey) {
		t.Errorf("expected ErrNoPrivateKey, got %v", err)
	}
}

func TestLoadKeyring_OrphanIdentity(t *testing.T) {
	// Identity is not in the recipient list; LoadKeyring should
	// reject at the age layer (can't decrypt).
	alice := newTestIdentity(t)
	orphan := newTestIdentity(t)
	root := t.TempDir()
	repoID, _ := NewRepoID()
	path := writeRecipients(t, root, map[string]Identity{"alice": alice}, 1, repoID)

	_, err := LoadKeyring(path, orphan)
	if err == nil {
		t.Fatal("orphan identity should not be able to decrypt recipients.age")
	}
	if !IsNoMatchingKey(err) {
		t.Errorf("expected IsNoMatchingKey, got %v", err)
	}
}

func TestLoadKeyring_MissingFile(t *testing.T) {
	alice := newTestIdentity(t)
	_, err := LoadKeyring(filepath.Join(t.TempDir(), "nope.age"), alice)
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing file should return os.ErrNotExist, got %v", err)
	}
}

func TestLoadKeyring_RecipientAccessors(t *testing.T) {
	alice := newTestIdentity(t)
	bob := newTestIdentity(t)
	root := t.TempDir()
	repoID, _ := NewRepoID()
	path := writeRecipients(t, root, map[string]Identity{
		"alice": alice,
		"bob":   bob,
	}, 1, repoID)

	kr, err := LoadKeyring(path, alice)
	if err != nil {
		t.Fatal(err)
	}

	// Recipient(name) returns the named recipient.
	if r, ok := kr.Recipient("alice"); !ok || r.String() != alice.PublicRecipient().String() {
		t.Errorf("Recipient(alice) = (%v, %v), want alice's pubkey", r, ok)
	}
	if _, ok := kr.Recipient("mallory"); ok {
		t.Error("Recipient(mallory) = true, want false (not in list)")
	}

	// Recipients() returns the full sorted list.
	list := kr.Recipients()
	if len(list) != 2 {
		t.Errorf("len(Recipients) = %d, want 2", len(list))
	}
}
