package config_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func TestFSLoader_Load_RoundTrip(t *testing.T) {
	t.Parallel()
	fs := storage.NewMemFS()
	if err := fs.MkdirAll("/project/sub", 0o755); err != nil {
		t.Fatal(err)
	}
	want := []byte("app:\n  name: Test\n")
	if err := fs.WriteFile("/project/data-entry.yaml", want, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fs.WriteFile("/project/sub/nested.yaml", []byte("x: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	l := config.NewFSLoader(fs, "/project")

	got, err := l.Load(context.Background(), "data-entry.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Load = %q, want %q", got, want)
	}

	// Relative subdirectory paths are allowed.
	if _, err := l.Load(context.Background(), "sub/nested.yaml"); err != nil {
		t.Errorf("Load(sub/nested.yaml): %v", err)
	}
}

func TestFSLoader_Load_MissingFileIsNotExist(t *testing.T) {
	t.Parallel()
	l := config.NewFSLoader(storage.NewMemFS(), "/project")

	_, err := l.Load(context.Background(), "absent.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	// The Loader contract promises an os.IsNotExist-compatible error so
	// consumers (dataentry, mcp) can treat absence as "no config".
	if !os.IsNotExist(err) {
		t.Errorf("error %v is not os.IsNotExist-compatible", err)
	}
}

func TestFSLoader_Load_RejectsUnsafeNames(t *testing.T) {
	t.Parallel()
	fs := storage.NewMemFS()
	if err := fs.WriteFile("/secret.yaml", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	l := config.NewFSLoader(fs, "/project")

	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"NUL", "a\x00b.yaml"},
		{"control character", "a\x1fb.yaml"},
		{"backslash", `sub\file.yaml`},
		{"absolute", "/secret.yaml"},
		{"parent traversal", "../secret.yaml"},
		{"embedded traversal", "sub/../../secret.yaml"},
		{"dot segment", "./file.yaml"},
		{"empty segment", "sub//file.yaml"},
		{"drive letter", `C:secret.yaml`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := l.Load(context.Background(), tt.input); err == nil {
				t.Errorf("Load(%q) should be rejected", tt.input)
			}
		})
	}
}

func TestFSLoader_Subscribe(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "data-entry.yaml")
	if err := os.WriteFile(path, []byte("a: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	l := config.NewFSLoader(storage.NewSafeFS(storage.NewOsFS()), dir)

	// Unsafe names are rejected before any watcher is created.
	if _, err := l.Subscribe(context.Background(), "../x.yaml", func() {}); err == nil {
		t.Error("Subscribe with traversal name should be rejected")
	}

	// A valid subscription returns a working stop function. Event
	// delivery itself is the watcher's contract, tested in
	// internal/storage — no sleeps here.
	stop, err := l.Subscribe(context.Background(), "data-entry.yaml", func() {})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	stop()
}

func TestFSLoader_List(t *testing.T) {
	t.Parallel()
	fs := storage.NewMemFS()
	for _, p := range []string{"/project/scripts", "/project/scripts/nested"} {
		if err := fs.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Deliberately written out of order: List sorts.
	for _, n := range []string{"zeta.lua", "alpha.lua", "mid.lua"} {
		if err := fs.WriteFile("/project/scripts/"+n, []byte("-- x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := fs.WriteFile("/project/scripts/nested/deep.lua", []byte("-- y"), 0o644); err != nil {
		t.Fatal(err)
	}

	l := config.NewFSLoader(fs, "/project")

	got, err := l.List(context.Background(), "scripts")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// Sorted, subdirectory entries dropped, and NOT recursive: deep.lua sits
	// one level down and must not appear under any name.
	want := []string{"alpha.lua", "mid.lua", "zeta.lua"}
	if !slices.Equal(got, want) {
		t.Errorf("List = %v, want %v (sorted, no subdirectories, non-recursive)", got, want)
	}
	for _, unwanted := range []string{"nested", "deep.lua", "nested/deep.lua"} {
		if slices.Contains(got, unwanted) {
			t.Errorf("List returned %q; directories and nested files must be excluded", unwanted)
		}
	}
}

func TestFSLoader_List_NotADirectoryIsAnError(t *testing.T) {
	t.Parallel()
	fs := storage.NewMemFS()
	if err := fs.MkdirAll("/project", 0o755); err != nil {
		t.Fatal(err)
	}
	// scripts exists, but as a regular file.
	if err := fs.WriteFile("/project/scripts", []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	l := config.NewFSLoader(fs, "/project")

	// Reporting this as "no scripts" would silently drop operator-authored
	// config — the case the absent-is-empty asymmetry deliberately excludes.
	if _, err := l.List(context.Background(), "scripts"); err == nil {
		t.Error("List of a non-directory should surface an error, not report empty")
	}
}

func TestFSLoader_List_SkipsSymlinks(t *testing.T) {
	t.Parallel()
	// Needs a real filesystem: MemFS has no symlink support.
	root := t.TempDir()
	scripts := filepath.Join(root, "scripts")
	if err := os.MkdirAll(scripts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "secret.yaml"), []byte("token: x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scripts, "real.lua"), []byte("-- x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "secret.yaml"), filepath.Join(scripts, "evil.lua")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	l := config.NewFSLoader(storage.NewOsFS(), root)
	got, err := l.List(context.Background(), "scripts")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// A symlink is not a directory, so without the regular-file check it
	// would be listed as an ordinary config file pointing outside scripts/.
	want := []string{"real.lua"}
	if !slices.Equal(got, want) {
		t.Errorf("List = %v, want %v (symlinks excluded)", got, want)
	}
}

func TestFSLoader_List_MissingDirIsEmpty(t *testing.T) {
	t.Parallel()
	fs := storage.NewMemFS()
	if err := fs.MkdirAll("/project", 0o755); err != nil {
		t.Fatal(err)
	}
	l := config.NewFSLoader(fs, "/project")

	// A project with no scripts/ at all is ordinary, not an error.
	got, err := l.List(context.Background(), "scripts")
	if err != nil {
		t.Fatalf("List of missing dir: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("List = %v, want empty", got)
	}
}

func TestFSLoader_List_RejectsUnsafeNames(t *testing.T) {
	t.Parallel()
	l := config.NewFSLoader(storage.NewMemFS(), "/project")
	for _, name := range []string{"", "../etc", "/etc", `sub\dir`, "sub/../../etc", "C:dir"} {
		if _, err := l.List(context.Background(), name); err == nil {
			t.Errorf("List(%q) should be rejected", name)
		}
	}
}
