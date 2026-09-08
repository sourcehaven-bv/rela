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

// TestLayered_DiskWinsOverBakedConfig is the end-to-end shape of the feature:
// a project with files on disk AND config baked into a second source behaves
// exactly as it did before the second source existed.
//
// mapLoader stands in for the store-backed loader here rather than the real
// one, because internal/config must not depend on a storage backend — the
// property under test is the LAYERING, and the backend has its own tests.
func TestLayered_DiskWinsOverBakedConfig(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	// On disk: an edited data-entry.yaml and one script.
	if err := os.WriteFile(filepath.Join(root, "data-entry.yaml"), []byte("from: disk\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "edited.lua"), []byte("-- disk"), 0o644); err != nil {
		t.Fatal(err)
	}

	baked := mapLoader{
		"data-entry.yaml":    []byte("from: database\n"),
		"schema.yaml":        []byte("from: database\n"),
		"scripts/edited.lua": []byte("-- database"),
		"scripts/baked.lua":  []byte("-- database"),
	}

	loader, err := config.NewLayered(config.NewFSLoader(storage.NewOsFS(), root), baked)
	if err != nil {
		t.Fatalf("NewLayered: %v", err)
	}
	ctx := context.Background()

	// The file the operator edited wins.
	got, err := loader.Load(ctx, "data-entry.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !bytes.Equal(got, []byte("from: disk\n")) {
		t.Errorf("Load = %q, want the file on disk to win", got)
	}

	// A file that exists ONLY in the database is still reachable — that is
	// what makes a shipped single file work at all.
	got, err = loader.Load(ctx, "schema.yaml")
	if err != nil {
		t.Fatalf("Load baked-only: %v", err)
	}
	if !bytes.Equal(got, []byte("from: database\n")) {
		t.Errorf("Load baked-only = %q", got)
	}

	// A directory unions both layers, so a baked script is not hidden by an
	// edited one sitting beside it.
	names, err := loader.List(ctx, "scripts")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if want := []string{"baked.lua", "edited.lua"}; !slices.Equal(names, want) {
		t.Errorf("List = %v, want %v", names, want)
	}

	// And the unioned name still resolves to the disk copy.
	got, err = loader.Load(ctx, "scripts/edited.lua")
	if err != nil {
		t.Fatalf("Load unioned: %v", err)
	}
	if !bytes.Equal(got, []byte("-- disk")) {
		t.Errorf("Load unioned = %q, want the disk copy", got)
	}
}
