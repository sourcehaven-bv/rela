package mcp

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func TestWatcherStop(t *testing.T) {
	s := makeTestServer(t)

	// Create required directories so Watch doesn't fail
	root := s.projectCtx.Root
	entitiesDir := filepath.Join(root, "entities")
	relationsDir := filepath.Join(root, "relations")
	metamodelPath := filepath.Join(root, "metamodel.yaml")
	if err := os.MkdirAll(entitiesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(relationsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metamodelPath, []byte("entities: {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := &project.Context{
		Root:          root,
		EntitiesDir:   entitiesDir,
		RelationsDir:  relationsDir,
		MetamodelPath: metamodelPath,
	}
	s.repo = repository.New(storage.NewOsFS(), ctx)
	s.projectCtx = ctx

	w, err := NewWatcher(s)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}

	// Stop should shut down the watcher started by Watch
	w.Stop()
}

func TestWatcherFileChange(t *testing.T) {
	s := makeTestServer(t)

	// Create required directories and files
	root := s.projectCtx.Root
	entitiesDir := filepath.Join(root, "entities")
	relationsDir := filepath.Join(root, "relations")
	metamodelPath := filepath.Join(root, "metamodel.yaml")
	cacheDir := filepath.Join(root, ".rela")

	if err := os.MkdirAll(entitiesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(relationsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metamodelPath, []byte("entities: {}\nrelations: {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := &project.Context{
		Root:          root,
		EntitiesDir:   entitiesDir,
		RelationsDir:  relationsDir,
		MetamodelPath: metamodelPath,
		CacheDir:      cacheDir,
	}
	s.repo = repository.New(storage.NewOsFS(), ctx)
	s.projectCtx = ctx

	w, err := NewWatcher(s)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer w.Stop()

	// Wait for watcher to be ready
	time.Sleep(100 * time.Millisecond)

	// Trigger a file change event by modifying a watched file
	if err := os.WriteFile(metamodelPath, []byte("entities: {}\nrelations: {}\n# updated"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Give time for the debounced callback to fire
	time.Sleep(400 * time.Millisecond)
}
