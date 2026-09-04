package storage

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestValidatedPath_ZeroValueIsEmpty(t *testing.T) {
	var v ValidatedPath
	if v.String() != "" {
		t.Fatalf("zero ValidatedPath = %q, want empty", v.String())
	}
}

func TestValidatedFS_UnwrapsToUnderlyingFS(t *testing.T) {
	dir := t.TempDir()
	rfs, err := NewRootedFS(NewOsFS(), dir)
	if err != nil {
		t.Fatalf("NewRootedFS: %v", err)
	}
	full, err := rfs.resolve("a/b.txt")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if err := rfs.vfs.MkdirAll(rfs.parent(full), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := rfs.vfs.WriteFile(full, []byte("hi"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "a", "b.txt"))
	if err != nil {
		t.Fatalf("os.ReadFile: %v", err)
	}
	if string(got) != "hi" {
		t.Fatalf("content = %q, want %q", got, "hi")
	}
}

// TestParent_StaysInsideRoot pins the claim in parent's doc comment: the
// directory of a validated path is itself under the root, so widening the
// invariant to it is sound. A key like "a/b.txt" must not have "." or the
// root's own parent as its parent.
func TestParent_StaysInsideRoot(t *testing.T) {
	dir := t.TempDir()
	rfs, err := NewRootedFS(NewOsFS(), dir)
	if err != nil {
		t.Fatalf("NewRootedFS: %v", err)
	}
	for _, key := range []string{"a.txt", "a/b.txt", "a/b/c/d.txt"} {
		t.Run(key, func(t *testing.T) {
			full, err := rfs.resolve(key)
			if err != nil {
				t.Fatalf("resolve(%q): %v", key, err)
			}
			p := rfs.parent(full).String()
			if p != rfs.root && !strings.HasPrefix(p, rfs.root+string(filepath.Separator)) {
				t.Fatalf("parent(%q) = %q, outside root %q", key, p, rfs.root)
			}
		})
	}
}

// rawFSCall matches a call on RootedFS's retained raw FS handle — the one
// field that still holds an unvalidated-path interface.
var rawFSCall = regexp.MustCompile(`r\.fs\.[A-Z]`)

// directOSCall matches a direct os/filepath filesystem call. These belong in
// validated.go (which unwraps ValidatedPath deliberately) and nowhere else in
// the rooted path.
var directOSCall = regexp.MustCompile(`\bos\.(ReadFile|WriteFile|OpenFile|Open|Remove|RemoveAll|Rename|Mkdir|MkdirAll|ReadDir|Stat|Lstat)\(`)

// TestRootedFS_ReachesFSOnlyThroughValidatedFS is a guard test in the style of
// internal/acl's ceilingguard_test: it scans rooted.go's source for the two
// shapes that would bypass the ValidatedPath barrier.
//
// The barrier is a type, but Go cannot express "this struct field may only be
// read by that method". RootedFS keeps a raw `fs FS` for SupportsStreaming's
// backend sniff, and a future edit could reach an I/O method on it with a bare
// string — reintroducing exactly the taint path this change closed. Likewise a
// direct os.* call would skip validatedFS entirely, which is what
// OpenForWrite used to do.
//
// Both are cheap to write by accident and invisible in review, so they are
// checked mechanically rather than left to prose.
func TestRootedFS_ReachesFSOnlyThroughValidatedFS(t *testing.T) {
	src, err := os.ReadFile("rooted.go")
	if err != nil {
		t.Fatalf("read rooted.go: %v", err)
	}
	for i, line := range strings.Split(string(src), "\n") {
		code, _, _ := strings.Cut(line, "//")
		if m := rawFSCall.FindString(code); m != "" {
			t.Errorf("rooted.go:%d calls the raw FS directly (%q); go through r.vfs so the path carries ValidatedPath: %s",
				i+1, m, strings.TrimSpace(line))
		}
		if m := directOSCall.FindString(code); m != "" {
			t.Errorf("rooted.go:%d calls %s directly, bypassing validatedFS; put the unwrap in validated.go: %s",
				i+1, m, strings.TrimSpace(line))
		}
	}
}
