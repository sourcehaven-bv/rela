package storage

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func newTestRooted(t *testing.T) *RootedFS {
	t.Helper()
	mem := NewMemFS()
	rfs, err := NewRootedFS(mem, "/root")
	if err != nil {
		t.Fatalf("NewRootedFS: %v", err)
	}
	if err := mem.MkdirAll("/root", 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	return rfs
}

func TestNewRootedFS_RejectsEmptyRoot(t *testing.T) {
	_, err := NewRootedFS(NewMemFS(), "")
	if err == nil {
		t.Fatal("expected error for empty root")
	}
}

func TestNewRootedFS_RejectsNilFS(t *testing.T) {
	_, err := NewRootedFS(nil, "/root")
	if err == nil {
		t.Fatal("expected error for nil fs")
	}
}

func TestNewRootedFS_CleansRoot(t *testing.T) {
	rfs, err := NewRootedFS(NewMemFS(), "/a/../b/./c")
	if err != nil {
		t.Fatalf("NewRootedFS: %v", err)
	}
	if rfs.root != "/b/c" {
		t.Fatalf("root = %q, want /b/c", rfs.root)
	}
}

func TestNewRootedFS_ResolvesRelativeRoot(t *testing.T) {
	rfs, err := NewRootedFS(NewMemFS(), "rel/dir")
	if err != nil {
		t.Fatalf("NewRootedFS: %v", err)
	}
	if !filepath.IsAbs(rfs.root) {
		t.Fatalf("root %q should be absolute", rfs.root)
	}
	if !strings.HasSuffix(rfs.root, "/rel/dir") {
		t.Fatalf("root %q should end with /rel/dir", rfs.root)
	}
}

func TestRootedFS_Resolve_Accepts(t *testing.T) {
	rfs := newTestRooted(t)
	keys := []string{
		"cache.json",
		"user-defaults.yaml",
		"ui-state.json",
		"palette.yaml",
		"documents/render-abc123.html",
		"a/b/c/d.txt",
		"file.with.dots.txt",
		"héllo.txt", // unicode
	}
	for _, k := range keys {
		t.Run(k, func(t *testing.T) {
			full, err := rfs.resolve(k)
			if err != nil {
				t.Fatalf("expected ok for %q, got %v", k, err)
			}
			want := filepath.Join(rfs.root, k)
			if full != want {
				t.Fatalf("resolve(%q) = %q, want %q", k, full, want)
			}
		})
	}
}

func TestRootedFS_Resolve_Rejects(t *testing.T) {
	rfs := newTestRooted(t)
	cases := []struct {
		key  string
		want string
	}{
		{"", "empty"},
		{"..", "traversal"},
		{".", "traversal"},
		{"/etc/passwd", "relative"},
		{"a\\b.yaml", "backslash"},
		{"../escape.yaml", "traversal"},
		{"sub/../escape.yaml", "traversal"},
		{"with\x00nul.yaml", "control character"},
		{"with\x01ctrl.yaml", "control character"},
		{"with\x7fdel.yaml", "control character"},
		{"a//b.yaml", "empty segment"},
		{"c:file.yaml", "colon"},
		{"has:colon.yaml", "colon"},
		{"CON.txt", "Windows reserved"},
		{"sub/nul", "Windows reserved"},
		{"com1", "Windows reserved"},
		{"LpT9.log", "Windows reserved"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			_, err := rfs.resolve(tc.key)
			if err == nil {
				t.Fatalf("expected error for %q", tc.key)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q should mention %q", err.Error(), tc.want)
			}
		})
	}
}

func TestRootedFS_ResolvedPath_StaysInsideRoot(t *testing.T) {
	rfs := newTestRooted(t)
	// Any accepted key must resolve to a descendant of root.
	keys := []string{"a", "a/b", "a/b/c/d.txt", "deep/nested/path/file.txt"}
	for _, k := range keys {
		full, err := rfs.resolve(k)
		if err != nil {
			t.Fatalf("resolve(%q): %v", k, err)
		}
		rel, err := filepath.Rel(rfs.root, full)
		if err != nil {
			t.Fatalf("rel: %v", err)
		}
		if strings.HasPrefix(rel, "..") {
			t.Fatalf("resolved %q escaped root (rel=%q)", k, rel)
		}
	}
}

func TestRootedFS_WriteFile_ReadFile(t *testing.T) {
	rfs := newTestRooted(t)
	if err := rfs.WriteFile("a.txt", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := rfs.ReadFile("a.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("got %q, want %q", got, "hello")
	}
}

func TestRootedFS_WriteFile_InvalidKey_DoesNotTouchFS(t *testing.T) {
	spy := &callSpyFS{FS: NewMemFS()}
	rfs, err := NewRootedFS(spy, "/root")
	if err != nil {
		t.Fatalf("NewRootedFS: %v", err)
	}
	if err := rfs.WriteFile("../escape", []byte("x"), 0o644); err == nil {
		t.Fatal("expected error for invalid key")
	}
	if spy.writes != 0 {
		t.Fatalf("underlying FS was called %d times; should be 0", spy.writes)
	}
}

func TestRootedFS_ReadFile_InvalidKey(t *testing.T) {
	rfs := newTestRooted(t)
	if _, err := rfs.ReadFile(".."); err == nil {
		t.Fatal("expected error")
	}
}

func TestRootedFS_Remove(t *testing.T) {
	rfs := newTestRooted(t)
	if err := rfs.WriteFile("a.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := rfs.Remove("a.txt"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := rfs.ReadFile("a.txt"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected ErrNotExist after remove, got %v", err)
	}
}

func TestRootedFS_Rename_BothArgsValidated(t *testing.T) {
	rfs := newTestRooted(t)
	if err := rfs.WriteFile("a.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := rfs.Rename("a.txt", "b.txt"); err != nil {
		t.Fatalf("valid rename: %v", err)
	}
	got, err := rfs.ReadFile("b.txt")
	if err != nil || string(got) != "x" {
		t.Fatalf("after rename: got=%q err=%v", got, err)
	}

	if err := rfs.Rename("b.txt", "../escape"); err == nil {
		t.Fatal("expected error for invalid newKey")
	}
	if err := rfs.Rename("../escape", "c.txt"); err == nil {
		t.Fatal("expected error for invalid oldKey")
	}
}

func TestRootedFS_MkdirAll_ReadDir(t *testing.T) {
	rfs := newTestRooted(t)
	if err := rfs.MkdirAll("a/b/c", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := rfs.WriteFile("a/b/c/f.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	entries, err := rfs.ReadDir("a/b")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	if len(names) != 1 || names[0] != "c" {
		t.Fatalf("ReadDir entries = %v, want [c]", names)
	}
}

func TestRootedFS_Stat(t *testing.T) {
	rfs := newTestRooted(t)
	if err := rfs.WriteFile("a.txt", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	info, err := rfs.Stat("a.txt")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size() != 5 {
		t.Fatalf("size = %d, want 5", info.Size())
	}
}

func TestRootedFS_Open(t *testing.T) {
	rfs := newTestRooted(t)
	if err := rfs.WriteFile("a.txt", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	rc, err := rfs.Open("a.txt")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer rc.Close()
	buf := make([]byte, 5)
	n, err := rc.Read(buf)
	if err != nil || n != 5 || string(buf) != "hello" {
		t.Fatalf("read: n=%d err=%v buf=%q", n, err, buf)
	}
}

func TestRootedFS_Walk_ReturnsKeys(t *testing.T) {
	rfs := newTestRooted(t)
	if err := rfs.MkdirAll("sub/nested", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	for _, p := range []string{"sub/a.txt", "sub/b.txt", "sub/nested/c.txt"} {
		if err := rfs.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", p, err)
		}
	}
	var seen []string
	if err := rfs.Walk("sub", func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		seen = append(seen, path)
		return nil
	}); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	sort.Strings(seen)
	want := []string{"sub", "sub/a.txt", "sub/b.txt", "sub/nested", "sub/nested/c.txt"}
	if !stringSliceEqual(seen, want) {
		t.Fatalf("seen = %v, want %v", seen, want)
	}
	for _, p := range seen {
		if filepath.IsAbs(p) {
			t.Fatalf("callback received absolute path %q; expected key", p)
		}
	}
}

func TestRootedFS_WalkAll_FromRoot(t *testing.T) {
	rfs := newTestRooted(t)
	if err := rfs.MkdirAll("sub", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	for _, p := range []string{"a.txt", "sub/b.txt"} {
		if err := rfs.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", p, err)
		}
	}
	var seen []string
	if err := rfs.WalkAll(func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		seen = append(seen, path)
		return nil
	}); err != nil {
		t.Fatalf("WalkAll: %v", err)
	}
	sort.Strings(seen)
	want := []string{".", "a.txt", "sub", "sub/b.txt"}
	if !stringSliceEqual(seen, want) {
		t.Fatalf("seen = %v, want %v", seen, want)
	}
}

func TestRootedFS_Walk_InvalidKey(t *testing.T) {
	rfs := newTestRooted(t)
	err := rfs.Walk("..", func(_ string, _ fs.DirEntry, _ error) error { return nil })
	if err == nil {
		t.Fatal("expected error for invalid walk key")
	}
}

// callSpyFS wraps an FS and counts calls to WriteFile, used to assert
// that invalid keys short-circuit before touching the underlying FS.
type callSpyFS struct {
	FS
	writes int
}

func (s *callSpyFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	s.writes++
	return s.FS.WriteFile(path, data, perm)
}

func TestRootedFS_OpenForWrite_WritesAndReadsBack(t *testing.T) {
	// OS-backed: OpenForWrite uses os.OpenFile directly.
	dir := t.TempDir()
	rfs, err := NewRootedFS(NewOsFS(), dir)
	if err != nil {
		t.Fatalf("NewRootedFS: %v", err)
	}
	wc, err := rfs.OpenForWrite("stream.txt", 0o644)
	if err != nil {
		t.Fatalf("OpenForWrite: %v", err)
	}
	if _, writeErr := wc.Write([]byte("hello streaming")); writeErr != nil {
		t.Fatalf("Write: %v", writeErr)
	}
	if closeErr := wc.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}
	got, err := rfs.ReadFile("stream.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello streaming" {
		t.Fatalf("got %q", got)
	}
}

func TestRootedFS_OpenForWrite_RejectsBadKey(t *testing.T) {
	// Bad-key rejection happens in resolve, before os.OpenFile is reached,
	// so MemFS is fine here.
	rfs := newTestRooted(t)
	_, err := rfs.OpenForWrite("../escape", 0o644)
	if err == nil {
		t.Fatal("expected error for bad key")
	}
}

func TestRootedFS_OpenForWrite_CreatesParentDirs(t *testing.T) {
	// Needs OS-backed FS since we use os.OpenFile; skip on memfs.
	dir := t.TempDir()
	rfs, err := NewRootedFS(NewOsFS(), dir)
	if err != nil {
		t.Fatalf("NewRootedFS: %v", err)
	}
	wc, err := rfs.OpenForWrite("deep/nested/stream.txt", 0o644)
	if err != nil {
		t.Fatalf("OpenForWrite: %v", err)
	}
	if _, err := wc.Write([]byte("x")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := wc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestRootedFS_AbsPath(t *testing.T) {
	rfs := newTestRooted(t)
	full, err := rfs.AbsPath("sub/file.txt")
	if err != nil {
		t.Fatalf("AbsPath: %v", err)
	}
	want := filepath.Join(rfs.root, "sub", "file.txt")
	if full != want {
		t.Fatalf("AbsPath = %q, want %q", full, want)
	}
}

func TestRootedFS_AbsPath_RejectsBadKey(t *testing.T) {
	rfs := newTestRooted(t)
	_, err := rfs.AbsPath("..")
	if err == nil {
		t.Fatal("expected error for bad key")
	}
}

func TestRootedFS_WriteFile_CreatesParentDirs(t *testing.T) {
	rfs := newTestRooted(t)
	// No explicit MkdirAll — WriteFile should create parents itself.
	if err := rfs.WriteFile("deep/nested/path/file.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := rfs.ReadFile("deep/nested/path/file.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "x" {
		t.Fatalf("got %q, want %q", got, "x")
	}
}

// FuzzResolve checks two invariants for any key that resolve() accepts:
//  1. The resolved path is under the root (filepath.Rel has no ".." prefix).
//  2. The resolved path begins with root + separator (or equals root).
//
// This is the security-critical property: no key should ever resolve
// outside the rooted subtree, regardless of what bytes the caller sends.
func FuzzResolve(f *testing.F) {
	seeds := []string{
		"a.txt",
		"sub/b.txt",
		"",
		"..",
		".",
		"/abs",
		"a\\b",
		"with\x00",
		"c:file",
		"CON",
		"sub/../esc",
		"a//b",
		"héllo",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	rfs, err := NewRootedFS(NewMemFS(), "/root")
	if err != nil {
		f.Fatalf("NewRootedFS: %v", err)
	}
	f.Fuzz(func(t *testing.T, key string) {
		full, err := rfs.resolve(key)
		if err != nil {
			return // rejection is fine — we only assert on acceptances
		}
		rel, relErr := filepath.Rel(rfs.root, full)
		if relErr != nil {
			t.Fatalf("filepath.Rel(%q, %q): %v", rfs.root, full, relErr)
		}
		// rel must not be ".." and must not step up out of root.
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("resolved %q escapes root: rel=%q full=%q", key, rel, full)
		}
		// full must be root itself or a path inside root.
		sep := string(filepath.Separator)
		if full != rfs.root && !strings.HasPrefix(full, rfs.root+sep) {
			t.Fatalf("resolved %q not prefixed by root: full=%q root=%q", key, full, rfs.root)
		}
	})
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
