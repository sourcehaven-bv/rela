package mcp

import (
	"os"
	"path/filepath"
	"testing"

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
