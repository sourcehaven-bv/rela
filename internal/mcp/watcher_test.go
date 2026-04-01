package mcp

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

func TestWatcherStop(t *testing.T) {
	root := t.TempDir()
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
	if err := os.WriteFile(metamodelPath, []byte("entities:\n  item:\n    label: Item\n    id_type: sequential\n    id_prefix: ITEM-\n    properties:\n      title:\n        type: string\n        required: true\nrelations: {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := &project.Context{
		Root:          root,
		EntitiesDir:   entitiesDir,
		RelationsDir:  relationsDir,
		MetamodelPath: metamodelPath,
		CacheDir:      cacheDir,
		CachePath:     filepath.Join(cacheDir, "cache.json"),
	}
	repo := repository.New(storage.NewOsFS(), ctx)
	ws, err := workspace.New(repo, workspace.NopScriptExecutor)
	if err != nil {
		t.Fatalf("workspace.New failed: %v", err)
	}

	if err := ws.StartWatching(workspace.WatchOptions{}); err != nil {
		t.Fatalf("StartWatching failed: %v", err)
	}

	// Stop should shut down the watcher
	ws.StopWatching()
}

func TestWatcherFileChange(t *testing.T) {
	root := t.TempDir()
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
	if err := os.WriteFile(metamodelPath, []byte("entities:\n  item:\n    label: Item\n    id_type: sequential\n    id_prefix: ITEM-\n    properties:\n      title:\n        type: string\n        required: true\nrelations: {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := &project.Context{
		Root:          root,
		EntitiesDir:   entitiesDir,
		RelationsDir:  relationsDir,
		MetamodelPath: metamodelPath,
		CacheDir:      cacheDir,
		CachePath:     filepath.Join(cacheDir, "cache.json"),
	}
	repo := repository.New(storage.NewOsFS(), ctx)
	ws, err := workspace.New(repo, workspace.NopScriptExecutor)
	if err != nil {
		t.Fatalf("workspace.New failed: %v", err)
	}

	if err := ws.StartWatching(workspace.WatchOptions{}); err != nil {
		t.Fatalf("StartWatching failed: %v", err)
	}
	defer ws.StopWatching()

	// Wait for watcher to be ready
	time.Sleep(100 * time.Millisecond)

	// Trigger a file change event by modifying a watched file
	if err := os.WriteFile(metamodelPath, []byte("entities:\n  item:\n    label: Item\n    id_type: sequential\n    id_prefix: ITEM-\n    properties:\n      title:\n        type: string\n        required: true\nrelations: {}\n# updated"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Give time for the debounced callback to fire
	time.Sleep(400 * time.Millisecond)
}
