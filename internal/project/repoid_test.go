package project

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveRepoID_GeneratesOnFirstAccess(t *testing.T) {
	root := t.TempDir()
	id, err := ResolveRepoID(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 32 {
		t.Errorf("id = %q (len %d), want 32", id, len(id))
	}
	// Second call returns the same id — stable across invocations.
	id2, err := ResolveRepoID(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if id2 != id {
		t.Errorf("second resolve = %q, want %q", id2, id)
	}
}

func TestResolveRepoID_WritesHeader(t *testing.T) {
	root := t.TempDir()
	id, err := ResolveRepoID(root, "")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".rela", RepoIDFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "DO NOT COMMIT") {
		t.Errorf("repo-id missing commit-warning header:\n%s", raw)
	}
	if !strings.Contains(string(raw), id) {
		t.Errorf("repo-id file missing actual id")
	}
}

func TestResolveRepoID_HonorsExistingFile(t *testing.T) {
	root := t.TempDir()
	existing := "deadbeefdeadbeefdeadbeefdeadbeef"
	if err := WriteRepoID(root, existing); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveRepoID(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != existing {
		t.Errorf("got %q, want %q", got, existing)
	}
}

func TestResolveRepoID_KeyringMatchPasses(t *testing.T) {
	root := t.TempDir()
	kr := "cafed00dcafed00dcafed00dcafed00d"
	// First call creates the file from the keyring value.
	got, err := ResolveRepoID(root, kr)
	if err != nil {
		t.Fatal(err)
	}
	if got != kr {
		t.Errorf("got %q, want %q", got, kr)
	}
	// Second call with same keyring value: no-op success.
	if _, err := ResolveRepoID(root, kr); err != nil {
		t.Errorf("matching keyring + existing file: unexpected error %v", err)
	}
}

func TestResolveRepoID_KeyringMismatchFails(t *testing.T) {
	root := t.TempDir()
	if err := WriteRepoID(root, "deadbeefdeadbeefdeadbeefdeadbeef"); err != nil {
		t.Fatal(err)
	}
	_, err := ResolveRepoID(root, "cafed00dcafed00dcafed00dcafed00d")
	if err == nil {
		t.Fatal("want mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "disagrees with keyring") {
		t.Errorf("wrong error: %v", err)
	}
}

func TestResolveRepoID_MalformedFileErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".rela"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".rela", RepoIDFile),
		[]byte("not-a-repo-id"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ResolveRepoID(root, "")
	if !errors.Is(err, ErrRepoIDMalformed) {
		t.Errorf("want ErrRepoIDMalformed, got %v", err)
	}
}

func TestResolveRepoID_MultipleContentLinesRejected(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".rela"), 0o700); err != nil {
		t.Fatal(err)
	}
	// Plant a file with two non-comment lines — the parser refuses.
	content := "# comment\ndeadbeefdeadbeefdeadbeefdeadbeef\nextra\n"
	if err := os.WriteFile(filepath.Join(root, ".rela", RepoIDFile),
		[]byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveRepoID(root, ""); !errors.Is(err, ErrRepoIDMalformed) {
		t.Errorf("want ErrRepoIDMalformed, got %v", err)
	}
}

func TestWriteRepoID_RejectsInvalid(t *testing.T) {
	root := t.TempDir()
	if err := WriteRepoID(root, "not-a-repo-id"); !errors.Is(err, ErrRepoIDMalformed) {
		t.Errorf("want ErrRepoIDMalformed, got %v", err)
	}
	if err := WriteRepoID(root, ""); !errors.Is(err, ErrRepoIDMalformed) {
		t.Errorf("want ErrRepoIDMalformed for empty, got %v", err)
	}
}

func TestResolveRepoID_GeneratesFromKeyring(t *testing.T) {
	// Encrypted-repo first open: .rela/repo-id missing, keyring id
	// passed in — must be adopted and written.
	root := t.TempDir()
	kr := "feedfacefeedfacefeedfacefeedface"
	id, err := ResolveRepoID(root, kr)
	if err != nil {
		t.Fatal(err)
	}
	if id != kr {
		t.Errorf("got %q, want %q", id, kr)
	}
	// File now exists with that content.
	raw, err := os.ReadFile(filepath.Join(root, ".rela", RepoIDFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), kr) {
		t.Errorf("repo-id file missing keyring id: %s", raw)
	}
}

func TestResolveRepoID_MalformedKeyringRejected(t *testing.T) {
	root := t.TempDir()
	_, err := ResolveRepoID(root, "not-a-valid-keyring-id")
	if err == nil {
		t.Fatal("want error for malformed keyring id")
	}
	if !strings.Contains(err.Error(), "keyring repo-id") {
		t.Errorf("error should mention keyring: %v", err)
	}
}

func TestResolveRepoID_RefusesTrackedByGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "t@t.invalid")
	run("config", "user.name", "t")
	if err := WriteRepoID(root, "deadbeefdeadbeefdeadbeefdeadbeef"); err != nil {
		t.Fatal(err)
	}
	run("add", ".rela/"+RepoIDFile)
	run("commit", "-m", "oops")

	_, err := ResolveRepoID(root, "")
	if err == nil {
		t.Fatal("want tracked-by-git error, got nil")
	}
	if !errors.Is(err, ErrRepoIDTracked) {
		t.Errorf("want ErrRepoIDTracked, got %v", err)
	}
}
