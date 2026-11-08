package appbuild_test

import (
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
)

func TestDiscoverCacheDir(t *testing.T) {
	root := writeMinimalProject(t)
	sub := filepath.Join(root, "entities")

	got, err := appbuild.DiscoverCacheDir(sub)
	if err != nil {
		t.Fatalf("DiscoverCacheDir: %v", err)
	}
	want, err := filepath.EvalSymlinks(filepath.Join(root, ".rela"))
	if err != nil {
		t.Fatal(err)
	}
	if resolved, err := filepath.EvalSymlinks(got); err != nil || resolved != want {
		t.Errorf("DiscoverCacheDir = %q (%v), want %q", got, err, want)
	}
}

func TestDiscoverCacheDir_NoProject(t *testing.T) {
	if _, err := appbuild.DiscoverCacheDir(t.TempDir()); err == nil {
		t.Error("DiscoverCacheDir outside a project: got nil error")
	}
}
