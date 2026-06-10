package config_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
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
