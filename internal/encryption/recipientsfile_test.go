package encryption

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRecipientsFile_RoundTrip(t *testing.T) {
	alice := newTestIdentity(t)
	bob := newTestIdentity(t)

	repoID, err := NewRepoID()
	if err != nil {
		t.Fatal(err)
	}
	rf := &RecipientsFile{
		Version: 1,
		RepoID:  repoID,
		Recipients: map[string]string{
			"alice": alice.PublicRecipient().String(),
			"bob":   bob.PublicRecipient().String(),
		},
	}

	path := filepath.Join(t.TempDir(), RecipientsFileName)
	if err = WriteRecipientsFile(path, rf); err != nil {
		t.Fatalf("WriteRecipientsFile: %v", err)
	}

	// Alice can decrypt.
	got, err := ReadRecipientsFile(path, alice)
	if err != nil {
		t.Fatalf("ReadRecipientsFile (alice): %v", err)
	}
	if got.Version != 1 || got.RepoID != repoID {
		t.Errorf("round-trip version/repoID mismatch: %+v", got)
	}
	if len(got.Recipients) != 2 {
		t.Errorf("got %d recipients, want 2", len(got.Recipients))
	}

	// Bob can also decrypt.
	if _, err := ReadRecipientsFile(path, bob); err != nil {
		t.Errorf("ReadRecipientsFile (bob): %v", err)
	}
}

func TestRecipientsFile_NonRecipientFailsDecrypt(t *testing.T) {
	alice := newTestIdentity(t)
	mallory := newTestIdentity(t) // not listed

	repoID, _ := NewRepoID()
	rf := &RecipientsFile{
		Version:    1,
		RepoID:     repoID,
		Recipients: map[string]string{"alice": alice.PublicRecipient().String()},
	}
	path := filepath.Join(t.TempDir(), RecipientsFileName)
	if err := WriteRecipientsFile(path, rf); err != nil {
		t.Fatal(err)
	}

	_, err := ReadRecipientsFile(path, mallory)
	if err == nil {
		t.Fatal("non-recipient should fail to decrypt")
	}
	if !IsNoMatchingKey(err) {
		t.Errorf("expected IsNoMatchingKey, got %v", err)
	}
}

func TestRecipientsFile_TamperFails(t *testing.T) {
	alice := newTestIdentity(t)
	repoID, _ := NewRepoID()
	rf := &RecipientsFile{
		Version:    1,
		RepoID:     repoID,
		Recipients: map[string]string{"alice": alice.PublicRecipient().String()},
	}
	path := filepath.Join(t.TempDir(), RecipientsFileName)
	if err := WriteRecipientsFile(path, rf); err != nil {
		t.Fatal(err)
	}

	// Flip a byte deep in the payload (past the header).
	data, _ := os.ReadFile(path)
	data[len(data)-3] ^= 0x01
	_ = os.WriteFile(path, data, 0o644)

	_, err := ReadRecipientsFile(path, alice)
	if err == nil {
		t.Fatal("tampered file should fail to decrypt")
	}
	if !IsCorrupted(err) {
		t.Errorf("expected IsCorrupted, got %v", err)
	}
}

func TestRecipientsFile_MissingFile(t *testing.T) {
	alice := newTestIdentity(t)
	_, err := ReadRecipientsFile(filepath.Join(t.TempDir(), "nope"), alice)
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing file should return os.ErrNotExist, got %v", err)
	}
}

func TestRecipientsFile_Validate(t *testing.T) {
	cases := []struct {
		name string
		rf   RecipientsFile
		want bool // true = valid
	}{
		{"valid", RecipientsFile{Version: 1, RepoID: "a", Recipients: map[string]string{"x": "y"}}, true},
		{"zero version", RecipientsFile{Version: 0, RepoID: "a", Recipients: map[string]string{"x": "y"}}, false},
		{"empty repo id", RecipientsFile{Version: 1, RepoID: "", Recipients: map[string]string{"x": "y"}}, false},
		{"no recipients", RecipientsFile{Version: 1, RepoID: "a"}, false},
		{"empty name", RecipientsFile{Version: 1, RepoID: "a", Recipients: map[string]string{"": "y"}}, false},
		{"empty pub", RecipientsFile{Version: 1, RepoID: "a", Recipients: map[string]string{"x": ""}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.rf.Validate()
			if tc.want && err != nil {
				t.Errorf("valid case returned error: %v", err)
			}
			if !tc.want && err == nil {
				t.Error("invalid case returned nil error")
			}
		})
	}
}

func TestRecipientList_SortedByName(t *testing.T) {
	// Names given in non-alphabetical order; RecipientList must sort
	// so age.Encrypt sees a stable stanza order across runs.
	alice := newTestIdentity(t)
	bob := newTestIdentity(t)
	carol := newTestIdentity(t)
	rf := &RecipientsFile{
		Version: 1,
		RepoID:  "r",
		Recipients: map[string]string{
			"carol": carol.PublicRecipient().String(),
			"alice": alice.PublicRecipient().String(),
			"bob":   bob.PublicRecipient().String(),
		},
	}
	list, err := rf.RecipientList()
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
			t.Errorf("list[%d] = %s, want %s", i, r.String(), want[i])
		}
	}
}

func TestNewRepoID_FormatAndUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := range 100 {
		id, err := NewRepoID()
		if err != nil {
			t.Fatal(err)
		}
		if len(id) != 32 {
			t.Errorf("repo id length = %d, want 32", len(id))
		}
		if seen[id] {
			t.Errorf("duplicate repo id at iteration %d", i)
		}
		seen[id] = true
	}
}
