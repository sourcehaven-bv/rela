package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWatcherStop(t *testing.T) {
	s := makeTestServer(t)

	// Create required directories so NewWatcher doesn't fail
	entitiesDir := filepath.Join(s.projectCtx.Root, "entities")
	relationsDir := filepath.Join(s.projectCtx.Root, "relations")
	metamodelPath := filepath.Join(s.projectCtx.Root, "metamodel.yaml")
	if err := os.MkdirAll(entitiesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(relationsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metamodelPath, []byte("entities: {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	s.projectCtx.EntitiesDir = entitiesDir
	s.projectCtx.RelationsDir = relationsDir
	s.projectCtx.MetamodelPath = metamodelPath

	w, err := NewWatcher(s)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}

	// Start in goroutine
	done := make(chan struct{})
	go func() {
		w.Start()
		close(done)
	}()

	// Stop should cause Start to return
	w.Stop()
	<-done
}
