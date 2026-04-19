package encryption

import (
	"os"
	"path/filepath"
	"testing"
)

// writeKeyring sets up a keysDir and (optionally) an identity file,
// returning their paths. Used by LoadKeyring tests.
func writeKeyring(t *testing.T, pubs map[string]Recipient, id Identity) (keysDir, idPath string) {
	t.Helper()
	dir := t.TempDir()
	for name, r := range pubs {
		if err := os.WriteFile(filepath.Join(dir, name+".pub"), []byte(r.String()+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if id != nil {
		idPath = filepath.Join(dir, "local.key")
		if err := os.WriteFile(idPath, []byte(id.(*x25519Identity).i.String()+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir, idPath
}

func TestLoadKeyring_Basic(t *testing.T) {
	alice := newTestIdentity(t)
	bob := newTestIdentity(t)
	dir, idPath := writeKeyring(t, map[string]Recipient{
		"alice": alice.PublicRecipient(),
		"bob":   bob.PublicRecipient(),
	}, alice)

	kr, err := LoadKeyring(dir, idPath)
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
}

func TestLoadKeyring_NoIdentity(t *testing.T) {
	alice := newTestIdentity(t)
	dir, _ := writeKeyring(t, map[string]Recipient{"alice": alice.PublicRecipient()}, nil)
	kr, err := LoadKeyring(dir, "")
	if err != nil {
		t.Fatalf("LoadKeyring: %v", err)
	}
	if kr.HasIdentity() {
		t.Error("HasIdentity = true, want false")
	}
	if kr.LocalName() != "" {
		t.Errorf("LocalName = %q, want empty", kr.LocalName())
	}
}

func TestLoadKeyring_OrphanIdentity(t *testing.T) {
	// Identity doesn't correspond to any listed recipient -> LocalName is "".
	alice := newTestIdentity(t)
	orphan := newTestIdentity(t)
	dir, idPath := writeKeyring(t, map[string]Recipient{"alice": alice.PublicRecipient()}, orphan)
	kr, err := LoadKeyring(dir, idPath)
	if err != nil {
		t.Fatalf("LoadKeyring: %v", err)
	}
	if !kr.HasIdentity() {
		t.Fatal("HasIdentity = false, want true")
	}
	if kr.LocalName() != "" {
		t.Errorf("LocalName = %q, want empty (orphan identity)", kr.LocalName())
	}
}

func TestLoadKeyring_MissingKeysDir(t *testing.T) {
	kr, err := LoadKeyring(filepath.Join(t.TempDir(), "nope"), "")
	if err != nil {
		t.Fatalf("LoadKeyring should tolerate missing keys dir: %v", err)
	}
	if n := len(kr.RecipientNames()); n != 0 {
		t.Errorf("got %d recipients, want 0", n)
	}
}

func TestLoadKeyring_MalformedPubFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bob.pub"), []byte("not-a-recipient\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadKeyring(dir, ""); err == nil {
		t.Fatal("LoadKeyring with malformed pub file should error")
	}
}

func TestLoadKeyring_MalformedIdentity(t *testing.T) {
	alice := newTestIdentity(t)
	dir, _ := writeKeyring(t, map[string]Recipient{"alice": alice.PublicRecipient()}, nil)
	idPath := filepath.Join(dir, "bad.key")
	if err := os.WriteFile(idPath, []byte("garbage\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadKeyring(dir, idPath); err == nil {
		t.Fatal("LoadKeyring with malformed identity should error")
	}
}

func TestLoadKeyring_PubFileWithCommentLine(t *testing.T) {
	alice := newTestIdentity(t)
	dir := t.TempDir()
	content := "# alice's pubkey\n" + alice.PublicRecipient().String() + "\n"
	if err := os.WriteFile(filepath.Join(dir, "alice.pub"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	kr, err := LoadKeyring(dir, "")
	if err != nil {
		t.Fatalf("LoadKeyring: %v", err)
	}
	if _, ok := kr.Recipient("alice"); !ok {
		t.Error("Recipient(alice) not found")
	}
}

func TestLoadKeyring_IgnoresNonPubFiles(t *testing.T) {
	alice := newTestIdentity(t)
	dir, _ := writeKeyring(t, map[string]Recipient{"alice": alice.PublicRecipient()}, nil)
	// Drop some red-herring files.
	_ = os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte("macos"), 0o644)
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	kr, err := LoadKeyring(dir, "")
	if err != nil {
		t.Fatalf("LoadKeyring: %v", err)
	}
	if n := len(kr.RecipientNames()); n != 1 {
		t.Errorf("got %d recipients, want 1", n)
	}
}
