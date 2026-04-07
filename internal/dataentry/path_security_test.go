package dataentry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestContainedProjectPath_AllowsInsideRoot(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "entities", "ticket.md")
	if err := os.MkdirAll(filepath.Dir(inside), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := containedProjectPath(root, "entities/ticket.md")
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	wantResolved, _ := filepath.EvalSymlinks(inside)
	if got != wantResolved {
		t.Fatalf("got %q, want %q", got, wantResolved)
	}
}

func TestContainedProjectPath_RejectsTraversal(t *testing.T) {
	root := t.TempDir()

	cases := []struct {
		name string
		path string
	}{
		{"absolute outside", "/etc/passwd"},
		{"relative dotdot", "../../../../../../etc/passwd"},
		{"absolute under tmp not root", filepath.Join(t.TempDir(), "x")},
		{"NUL byte", "ent\x00ities/x.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := containedProjectPath(root, tc.path)
			if err == nil {
				t.Fatalf("expected error for %q", tc.path)
			}
		})
	}
}

func TestContainedProjectPath_RejectsSymlinkOut(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(target, []byte("s"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "escape")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	_, err := containedProjectPath(root, "escape")
	if err == nil {
		t.Fatal("expected error for symlink escaping project root")
	}
}

func TestValidateOpenURL(t *testing.T) {
	ok := []string{
		"https://example.com",
		"http://localhost:8080/path?q=1",
		"mailto:user@example.com",
	}
	bad := []string{
		"file:///etc/passwd",
		"javascript:alert(1)",
		"data:text/html,<script>",
		"ftp://example.com",
		"",
	}
	for _, u := range ok {
		t.Run("ok/"+u, func(t *testing.T) {
			if err := validateOpenURL(u); err != nil {
				t.Fatalf("expected ok, got %v", err)
			}
		})
	}
	for _, u := range bad {
		t.Run("bad/"+u, func(t *testing.T) {
			if err := validateOpenURL(u); err == nil {
				t.Fatalf("expected error for %q", u)
			}
		})
	}
}
