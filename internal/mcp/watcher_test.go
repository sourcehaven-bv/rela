package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
)

func TestIsRelevantFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"entities/requirement/REQ-001.md", true},
		{"metamodel.yaml", true},
		{"config.yml", true},
		{"main.go", false},
		{"README.txt", false},
		{"file.json", false},
		{"", false},
		{"path/to/file.MD", false}, // case sensitive
	}

	for _, tt := range tests {
		got := isRelevantFile(tt.path)
		if got != tt.want {
			t.Errorf("isRelevantFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestWatchDirRecursive(t *testing.T) {
	// Create a directory structure
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	subSubDir := filepath.Join(subDir, "deep")
	if err := os.MkdirAll(subSubDir, 0o755); err != nil {
		t.Fatal(err)
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer fsw.Close()

	if err := watchDirRecursive(fsw, tmpDir); err != nil {
		t.Fatalf("watchDirRecursive returned error: %v", err)
	}

	// Verify all directories are being watched by checking the watcher list
	watchList := fsw.WatchList()
	dirs := map[string]bool{tmpDir: false, subDir: false, subSubDir: false}
	for _, w := range watchList {
		if _, ok := dirs[w]; ok {
			dirs[w] = true
		}
	}
	for dir, found := range dirs {
		if !found {
			t.Errorf("directory %s is not being watched", dir)
		}
	}
}

func TestWatchDirRecursive_NonExistentDir(t *testing.T) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer fsw.Close()

	// Should not panic; Walk skips errors via SkipDir, so no error returned
	err = watchDirRecursive(fsw, "/nonexistent/path/that/does/not/exist")
	if err != nil {
		t.Errorf("expected nil error for nonexistent directory, got: %v", err)
	}
	// No directories should be watched
	if len(fsw.WatchList()) != 0 {
		t.Errorf("expected empty watch list, got %d entries", len(fsw.WatchList()))
	}
}

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
