package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/encryption"
)

func newTestIdentity(t *testing.T) encryption.Identity {
	t.Helper()
	id, err := encryption.GenerateIdentity()
	if err != nil {
		t.Fatalf("GenerateIdentity: %v", err)
	}
	return id
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// writeRecipientsAndIdentity writes <root>/keys/<name>.pub for each
// entry in pubs, writes local identity to <root>/.rela/key, and
// returns a keyring loaded from that layout.
func writeRecipientsAndIdentity(
	t *testing.T, root string,
	pubs map[string]encryption.Identity, local encryption.Identity,
) *encryption.Keyring {
	t.Helper()
	keysDir := filepath.Join(root, "keys")
	if err := os.MkdirAll(keysDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, id := range pubs {
		pubPath := filepath.Join(keysDir, name+".pub")
		if err := os.WriteFile(pubPath, []byte(id.PublicRecipient().String()+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idPath := filepath.Join(root, ".rela", "key")
	if err := os.MkdirAll(filepath.Dir(idPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(idPath, []byte(encryption.MarshalIdentity(local)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	kr, err := encryption.LoadKeyring(keysDir, idPath)
	if err != nil {
		t.Fatal(err)
	}
	return kr
}

func TestValidateRecipientName(t *testing.T) {
	cases := map[string]bool{
		"alice":   true,
		"alice-1": true,
		"alice_1": true,
		"ALICE":   true,
		"":        false,
		"alice/x": false,
		"alice.x": false,
		"alice x": false,
		"../etc":  false,
	}
	for name, ok := range cases {
		err := validateRecipientName(name)
		if (err == nil) != ok {
			t.Errorf("validateRecipientName(%q) err=%v; want ok=%v", name, err, ok)
		}
	}
}

// setupEncCLIProject creates a minimal project tree for the walk*
// helpers: a few entity and relation markdown files under root.
// Returns the root plus the paths of the created files.
func setupEncCLIProject(t *testing.T) (root string, filePaths []string) {
	t.Helper()
	root = t.TempDir()
	entry := func(rel, content string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		filePaths = append(filePaths, p)
	}
	entry("entities/tickets/TKT-001.md", "---\nid: TKT-001\ntype: ticket\n---\nhello\n")
	entry("entities/tickets/TKT-002.md", "---\nid: TKT-002\ntype: ticket\n---\nworld\n")
	entry("relations/TKT-001--blocks--TKT-002.md",
		"---\nfrom: TKT-001\nrelation: blocks\nto: TKT-002\n---\n")
	entry("attachments/TKT-001/spec/note.txt", "attached body")
	return root, filePaths
}

func TestSealAllFiles_AndUnseal(t *testing.T) {
	root, paths := setupEncCLIProject(t)
	id := newTestIdentity(t)

	if err := sealAllFiles(root, []encryption.Recipient{id.PublicRecipient()}); err != nil {
		t.Fatalf("sealAllFiles: %v", err)
	}
	for _, p := range paths {
		raw := mustReadFile(t, p)
		if !encryption.LooksSealed(raw) {
			t.Errorf("file %s not sealed after sealAllFiles", p)
		}
	}

	kr := writeRecipientsAndIdentity(t, root, map[string]encryption.Identity{"alice": id}, id)

	if err := unsealAllFiles(root, kr); err != nil {
		t.Fatalf("unsealAllFiles: %v", err)
	}
	for _, p := range paths {
		raw := mustReadFile(t, p)
		if encryption.LooksSealed(raw) {
			t.Errorf("file %s still sealed after unsealAllFiles", p)
		}
	}
}

func TestReencryptAll_AddsNewRecipient(t *testing.T) {
	root, paths := setupEncCLIProject(t)
	alice := newTestIdentity(t)
	bob := newTestIdentity(t)

	if err := sealAllFiles(root, []encryption.Recipient{alice.PublicRecipient()}); err != nil {
		t.Fatal(err)
	}

	kr := writeRecipientsAndIdentity(t, root,
		map[string]encryption.Identity{"alice": alice, "bob": bob},
		alice)

	if err := reencryptAll(root, kr); err != nil {
		t.Fatalf("reencryptAll: %v", err)
	}

	for _, p := range paths {
		raw := mustReadFile(t, p)
		if _, err := encryption.Unseal(raw, bob); err != nil {
			t.Errorf("bob could not read %s after re-encrypt: %v", p, err)
		}
	}
}

func TestWriteEncryptionConfig(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(root, ".rela")
	if err := writeEncryptionConfig(cacheDir, []string{"alice", "bob"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(cacheDir, "encryption.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("alice")) || !bytes.Contains(got, []byte("bob")) {
		t.Errorf("encryption.yaml missing recipients: %s", got)
	}
}

func TestEnsureKeyGitignored(t *testing.T) {
	t.Run("appends to existing gitignore", func(t *testing.T) {
		root := t.TempDir()
		gi := filepath.Join(root, ".gitignore")
		if err := os.WriteFile(gi, []byte("*.log\nbin/\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := ensureKeyGitignored(root); err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(gi)
		if !bytes.Contains(got, []byte("\n.rela/key\n")) {
			t.Errorf(".rela/key not appended: %s", got)
		}
		if !bytes.Contains(got, []byte("# rela encryption")) {
			t.Errorf("section header missing: %s", got)
		}
	})

	t.Run("idempotent when pattern already present", func(t *testing.T) {
		root := t.TempDir()
		gi := filepath.Join(root, ".gitignore")
		initial := "*.log\n.rela/key\nbin/\n"
		if err := os.WriteFile(gi, []byte(initial), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := ensureKeyGitignored(root); err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(gi)
		if string(got) != initial {
			t.Errorf("gitignore modified when pattern already present:\n%s", got)
		}
	})

	t.Run("creates new gitignore when missing", func(t *testing.T) {
		root := t.TempDir()
		if err := ensureKeyGitignored(root); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(root, ".gitignore"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(got, []byte(".rela/key")) {
			t.Errorf(".rela/key missing from fresh gitignore: %s", got)
		}
	})
}

func TestReadRecipientFromFile(t *testing.T) {
	t.Run("reads and parses a hybrid public key", func(t *testing.T) {
		id := newTestIdentity(t)
		path := filepath.Join(t.TempDir(), "alice.pub")
		if err := os.WriteFile(path, []byte(id.PublicRecipient().String()+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := readRecipientFromFile(path)
		if err != nil {
			t.Fatalf("readRecipientFromFile: %v", err)
		}
		if got.String() != id.PublicRecipient().String() {
			t.Errorf("recipient mismatch: got %q, want %q", got.String(), id.PublicRecipient().String())
		}
	})

	t.Run("empty path errors", func(t *testing.T) {
		if _, err := readRecipientFromFile(""); err == nil {
			t.Error("expected error for empty path")
		}
	})

	t.Run("missing file errors", func(t *testing.T) {
		if _, err := readRecipientFromFile(filepath.Join(t.TempDir(), "nope.pub")); err == nil {
			t.Error("expected error for missing file")
		}
	})

	t.Run("garbage contents errors", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "junk.pub")
		if err := os.WriteFile(path, []byte("not-an-age-key\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := readRecipientFromFile(path); err == nil {
			t.Error("expected parse error for garbage contents")
		}
	})
}

func TestWalkDataFiles_SkipsTempAndDotfiles(t *testing.T) {
	root := t.TempDir()
	// regular file
	if err := os.MkdirAll(filepath.Join(root, "entities", "tickets"), 0o755); err != nil {
		t.Fatal(err)
	}
	must := func(p, content string) {
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must(filepath.Join(root, "entities", "tickets", "TKT-1.md"), "real")
	must(filepath.Join(root, "entities", "tickets", "TKT-1.md.new"), "temp")
	must(filepath.Join(root, "entities", "tickets", "TKT-1.md.bak"), "backup")
	must(filepath.Join(root, "entities", "tickets", ".DS_Store"), "macos")
	must(filepath.Join(root, "entities", "tickets", "editor~"), "emacs")

	var visited []string
	if err := walkDataFiles(root, func(p string) error {
		visited = append(visited, filepath.Base(p))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(visited) != 1 || visited[0] != "TKT-1.md" {
		t.Errorf("visited = %v, want exactly [TKT-1.md]", visited)
	}
}
