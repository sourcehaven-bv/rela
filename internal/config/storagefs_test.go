package config_test

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func newStorageFS(t *testing.T) (view *config.StorageFS, root string) {
	t.Helper()
	mem := storage.NewMemFS()
	root = "/proj"
	for name, content := range map[string]string{
		"schema.yaml":     "version: \"1.0\"\n",
		"scripts/a.lua":   "return 1\n",
		"scripts/b.lua":   "return 2\n",
		"../secrets.yaml": "nope\n",
	} {
		path := filepath.Join(root, name)
		if err := mem.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := mem.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	view, err := config.NewStorageFS(context.Background(), config.NewFSLoader(mem, root), root)
	if err != nil {
		t.Fatal(err)
	}
	return view, root
}

func TestStorageFS_Reads(t *testing.T) {
	view, root := newStorageFS(t)

	got, err := view.ReadFile(filepath.Join(root, "schema.yaml"))
	if err != nil || string(got) != "version: \"1.0\"\n" {
		t.Fatalf("ReadFile = %q, %v", got, err)
	}
	info, err := view.Stat(filepath.Join(root, "scripts", "a.lua"))
	if err != nil || info.Size() != int64(len("return 1\n")) || info.IsDir() {
		t.Fatalf("Stat = %v, %v", info, err)
	}
	if rootInfo, rootErr := view.Stat(root); rootErr != nil || !rootInfo.IsDir() {
		t.Fatalf("Stat(root) = %v, %v; want a directory", rootInfo, rootErr)
	}
	entries, err := view.ReadDir(filepath.Join(root, "scripts"))
	if err != nil || len(entries) != 2 || entries[0].Name() != "a.lua" {
		t.Fatalf("ReadDir = %v, %v", entries, err)
	}
}

// A path outside the root is absent, never read from the underlying source.
func TestStorageFS_OutsideRootIsAbsent(t *testing.T) {
	view, root := newStorageFS(t)
	for _, path := range []string{
		filepath.Join(root, "..", "secrets.yaml"),
		"/secrets.yaml",
	} {
		if _, err := view.ReadFile(path); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("ReadFile(%q) err = %v, want ErrNotExist", path, err)
		}
	}
}

func TestStorageFS_IsReadOnly(t *testing.T) {
	view, root := newStorageFS(t)
	p := filepath.Join(root, "schema.yaml")
	for name, err := range map[string]error{
		"WriteFile": view.WriteFile(p, nil, 0o644),
		"Remove":    view.Remove(p),
		"Rename":    view.Rename(p, p+".bak"),
		"MkdirAll":  view.MkdirAll(root, 0o755),
	} {
		if err == nil {
			t.Errorf("%s succeeded on a read-only view", name)
		}
	}
}

func TestNewStorageFS_RejectsNil(t *testing.T) {
	if _, err := config.NewStorageFS(context.Background(), nil, "/"); err == nil {
		t.Error("nil loader accepted")
	}
	//nolint:staticcheck // SA1012: a nil context is what is under test.
	if _, err := config.NewStorageFS(nil, config.NewFSLoader(storage.NewMemFS(), "/"), "/"); err == nil {
		t.Error("nil context accepted")
	}
}

func TestStorageFS_OpenGetwdWalk(t *testing.T) {
	view, root := newStorageFS(t)

	rc, err := view.Open(filepath.Join(root, "scripts", "b.lua"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil || string(data) != "return 2\n" {
		t.Fatalf("Open read %q, %v", data, err)
	}
	if _, err := view.Open(filepath.Join(root, "absent.yaml")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Open(absent) err = %v, want ErrNotExist", err)
	}

	if wd, err := view.Getwd(); err != nil || wd != root {
		t.Errorf("Getwd = %q, %v; want %q", wd, err, root)
	}

	visited := 0
	if err := view.Walk(root, func(string, fs.DirEntry, error) error { visited++; return nil }); err != nil {
		t.Fatal(err)
	}
	if visited != 0 {
		t.Errorf("Walk visited %d entries; a Loader cannot enumerate, so it must visit none", visited)
	}
}

func TestStorageFS_MissingAndOutside(t *testing.T) {
	view, root := newStorageFS(t)
	outside := filepath.Join(root, "..", "secrets.yaml")
	if _, err := view.Stat(filepath.Join(root, "absent.yaml")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat(absent) err = %v", err)
	}
	if _, err := view.Stat(outside); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat(outside) err = %v", err)
	}
	if _, err := view.ReadDir(outside); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadDir(outside) err = %v", err)
	}
	if entries, err := view.ReadDir(filepath.Join(root, "nodir")); err != nil || len(entries) != 0 {
		t.Errorf("ReadDir(absent dir) = %v, %v; want empty", entries, err)
	}
}

func TestStorageFS_FileInfo(t *testing.T) {
	view, root := newStorageFS(t)
	info, err := view.Stat(filepath.Join(root, "schema.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Name() != "schema.yaml" || info.Mode().IsDir() || !info.ModTime().IsZero() || info.Sys() != nil {
		t.Errorf("file info = %+v", info)
	}
	dir, err := view.Stat(root)
	if err != nil || !dir.Mode().IsDir() {
		t.Errorf("root info = %+v, %v", dir, err)
	}
	entries, err := view.ReadDir(filepath.Join(root, "scripts"))
	if err != nil {
		t.Fatal(err)
	}
	if !entries[0].Type().IsRegular() {
		t.Errorf("ReadDir entry type = %v, want regular", entries[0].Type())
	}
}
