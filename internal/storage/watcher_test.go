package storage

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
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
		want ChangeOp
	}{
		{fsnotify.Create, OpCreate},
		{fsnotify.Write, OpModify},
		{fsnotify.Remove, OpDelete},
		{fsnotify.Rename, OpRename},
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
		op   ChangeOp
		want string
	}{
		{OpCreate, "CREATE"},
		{OpModify, "MODIFY"},
		{OpDelete, "DELETE"},
		{OpRename, "RENAME"},
		{ChangeOp(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.op.String(); got != tt.want {
			t.Errorf("ChangeOp(%d).String() = %q, want %q", tt.op, got, tt.want)
		}
	}
}

// waitWatched polls until path shows up in the watch list, failing the
// test after a generous deadline. Used where the event loop registers
// watches asynchronously (auto-watching newly created directories).
func waitWatched(t *testing.T, w *Watcher, path string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if slices.Contains(w.WatchList(), path) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s to be watched", path)
}

func TestWatcher_FileChangeEvents(t *testing.T) {
	dir := t.TempDir()

	eventsChan := make(chan []ChangeEvent, 10)

	w, err := NewWatcher(WatchConfig{
		Dirs:       []string{dir},
		Extensions: []string{".md"},
		Debounce:   50 * time.Millisecond,
		OnChange: func(events []ChangeEvent) {
			eventsChan <- events
		},
	})
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	go w.Start()
	defer w.Stop()

	// No startup wait needed: NewWatcher registers the fsnotify watches
	// synchronously, and events queue until Start drains them.

	// Create a file
	testFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(testFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Wait for event
	select {
	case events := <-eventsChan:
		if len(events) == 0 {
			t.Error("expected at least one event")
		}
		found := false
		for _, e := range events {
			if e.Path == testFile {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected event for %s, got %v", testFile, events)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for file create event")
	}
}

func TestWatcher_IgnoresNonMatchingExtensions(t *testing.T) {
	dir := t.TempDir()

	eventsChan := make(chan []ChangeEvent, 10)

	w, err := NewWatcher(WatchConfig{
		Dirs:       []string{dir},
		Extensions: []string{".md"},
		Debounce:   50 * time.Millisecond,
		OnChange: func(events []ChangeEvent) {
			eventsChan <- events
		},
	})
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	go w.Start()
	defer w.Stop()

	// Create a non-matching file, then a matching sentinel. The sentinel's
	// arrival proves the watcher already processed (and filtered) the .txt
	// event — no quiet-window sleep needed.
	txtFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(txtFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	mdFile := filepath.Join(dir, "sentinel.md")
	if err := os.WriteFile(mdFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case events := <-eventsChan:
			for _, e := range events {
				if e.Path == txtFile {
					t.Errorf("should not receive event for non-matching extension: %v", e)
				}
			}
			for _, e := range events {
				if e.Path == mdFile {
					return // sentinel arrived; .txt was filtered
				}
			}
		case <-deadline:
			t.Fatal("timeout waiting for sentinel .md event")
		}
	}
}

func TestWatcher_AutoWatchesNewDirectories(t *testing.T) {
	dir := t.TempDir()

	eventsChan := make(chan []ChangeEvent, 10)

	w, err := NewWatcher(WatchConfig{
		Dirs:       []string{dir},
		Extensions: []string{".md"},
		Debounce:   50 * time.Millisecond,
		OnChange: func(events []ChangeEvent) {
			eventsChan <- events
		},
	})
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	go w.Start()
	defer w.Stop()

	// Create a new subdirectory
	subDir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// The event loop auto-watches new directories asynchronously; wait
	// until the watch is actually registered before writing into it.
	waitWatched(t, w, subDir)

	// Create a file in the new subdirectory
	testFile := filepath.Join(subDir, "test.md")
	if err := os.WriteFile(testFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Wait for event from the new subdirectory
	select {
	case events := <-eventsChan:
		found := false
		for _, e := range events {
			if e.Path == testFile {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected event for %s in new subdir, got %v", testFile, events)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for event from new subdirectory")
	}
}

func TestWatcher_FileModifyAndDelete(t *testing.T) {
	dir := t.TempDir()

	// Create file before starting watcher
	testFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(testFile, []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}

	eventsChan := make(chan []ChangeEvent, 10)

	w, err := NewWatcher(WatchConfig{
		Dirs:       []string{dir},
		Extensions: []string{".md"},
		Debounce:   50 * time.Millisecond,
		OnChange: func(events []ChangeEvent) {
			eventsChan <- events
		},
	})
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	go w.Start()
	defer w.Stop()

	// Modify the file
	if err := os.WriteFile(testFile, []byte("modified"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Wait for modify event
	select {
	case events := <-eventsChan:
		found := false
		for _, e := range events {
			if e.Path == testFile && e.Op == OpModify {
				found = true
				break
			}
		}
		if !found {
			t.Logf("received events: %v (looking for MODIFY)", events)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for file modify event")
	}

	// Delete the file
	if err := os.Remove(testFile); err != nil {
		t.Fatal(err)
	}

	// Wait for delete event
	select {
	case events := <-eventsChan:
		found := false
		for _, e := range events {
			if e.Path == testFile && e.Op == OpDelete {
				found = true
				break
			}
		}
		if !found {
			t.Logf("received events: %v (looking for DELETE)", events)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for file delete event")
	}
}

func TestWatcher_PauseResumeIsPaused(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWatcher(WatchConfig{
		Dirs:     []string{dir},
		Debounce: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer w.Stop()

	if w.IsPaused() {
		t.Error("new watcher should not be paused")
	}
	w.Pause()
	if !w.IsPaused() {
		t.Error("after Pause, IsPaused should be true")
	}
	w.Resume()
	if w.IsPaused() {
		t.Error("after Resume, IsPaused should be false")
	}
}

func TestWatcher_AddFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tracked.md")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	w, err := NewWatcher(WatchConfig{
		Dirs:     []string{dir},
		Debounce: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer w.Stop()

	// Adding the file (even though its dir is already watched) must not error.
	if err := w.AddFile(path); err != nil {
		t.Errorf("AddFile(%q): %v", path, err)
	}
}
