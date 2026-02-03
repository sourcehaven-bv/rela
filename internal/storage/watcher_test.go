package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

func TestWatcher_NewAndStop(t *testing.T) {
	dir := t.TempDir()

	w, err := NewWatcher(WatchConfig{
		Dirs:     []string{dir},
		Debounce: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
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

func TestWatcher_WatchesDirectoriesRecursively(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	deep := filepath.Join(sub, "deep")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	w, err := NewWatcher(WatchConfig{
		Dirs:     []string{dir},
		Debounce: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer w.Stop()

	watchList := w.WatchList()
	dirs := map[string]bool{dir: false, sub: false, deep: false}
	for _, p := range watchList {
		if _, ok := dirs[p]; ok {
			dirs[p] = true
		}
	}
	for d, found := range dirs {
		if !found {
			t.Errorf("directory %s is not being watched", d)
		}
	}
}

func TestWatcher_WatchesIndividualFiles(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(filePath, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	w, err := NewWatcher(WatchConfig{
		Files:    []string{filePath},
		Debounce: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer w.Stop()

	watchList := w.WatchList()
	found := false
	for _, p := range watchList {
		if p == filePath {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("file %s is not being watched", filePath)
	}
}

func TestWatcher_SkipHidden(t *testing.T) {
	dir := t.TempDir()
	hiddenDir := filepath.Join(dir, ".hidden")
	visibleDir := filepath.Join(dir, "visible")
	if err := os.MkdirAll(hiddenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(visibleDir, 0o755); err != nil {
		t.Fatal(err)
	}

	w, err := NewWatcher(WatchConfig{
		Dirs:       []string{dir},
		SkipHidden: true,
		Debounce:   100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer w.Stop()

	watchList := w.WatchList()
	for _, p := range watchList {
		if p == hiddenDir {
			t.Errorf("hidden directory %s should not be watched", hiddenDir)
		}
	}

	foundVisible := false
	for _, p := range watchList {
		if p == visibleDir {
			foundVisible = true
			break
		}
	}
	if !foundVisible {
		t.Errorf("visible directory %s should be watched", visibleDir)
	}
}

func TestWatcher_NonExistentDir(t *testing.T) {
	w, err := NewWatcher(WatchConfig{
		Dirs:     []string{"/nonexistent/path/that/does/not/exist"},
		Debounce: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewWatcher() should not fail for nonexistent dir: %v", err)
	}
	defer w.Stop()

	if len(w.WatchList()) != 0 {
		t.Errorf("expected empty watch list, got %d entries", len(w.WatchList()))
	}
}

func TestIsRelevantFile(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		extensions []string
		want       bool
	}{
		{"md file with md ext", "test.md", []string{".md"}, true},
		{"yaml file with yaml ext", "config.yaml", []string{".yaml", ".yml"}, true},
		{"yml file with yaml ext", "config.yml", []string{".yaml", ".yml"}, true},
		{"go file with md ext", "main.go", []string{".md"}, false},
		{"no extensions matches all", "anything.go", nil, true},
		{"empty extensions matches all", "anything.go", []string{}, true},
		{"nested path", "path/to/file.md", []string{".md"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &Watcher{cfg: WatchConfig{Extensions: tt.extensions}}
			if got := w.isRelevantFile(tt.path); got != tt.want {
				t.Errorf("isRelevantFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestToChangeOp(t *testing.T) {
	tests := []struct {
		op   fsnotify.Op
		want model.ChangeOp
	}{
		{fsnotify.Create, model.OpCreate},
		{fsnotify.Write, model.OpModify},
		{fsnotify.Remove, model.OpDelete},
		{fsnotify.Rename, model.OpRename},
	}

	for _, tt := range tests {
		got := toChangeOp(tt.op)
		if got != tt.want {
			t.Errorf("toChangeOp(%v) = %v, want %v", tt.op, got, tt.want)
		}
	}
}

func TestChangeOp_String(t *testing.T) {
	tests := []struct {
		op   model.ChangeOp
		want string
	}{
		{model.OpCreate, "CREATE"},
		{model.OpModify, "MODIFY"},
		{model.OpDelete, "DELETE"},
		{model.OpRename, "RENAME"},
		{model.ChangeOp(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.op.String(); got != tt.want {
			t.Errorf("ChangeOp(%d).String() = %q, want %q", tt.op, got, tt.want)
		}
	}
}
