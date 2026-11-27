package fswatch

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

	w, err := New(Config{
		Dirs:     []string{dir},
		Debounce: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
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

	w, err := New(Config{
		Dirs:     []string{dir},
		Debounce: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
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

	w, err := New(Config{
		Files:    []string{filePath},
		Debounce: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer w.Stop()

	watchList := w.WatchList()
	found := slices.Contains(watchList, filePath)
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

	w, err := New(Config{
		Dirs:       []string{dir},
		SkipHidden: true,
		Debounce:   100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer w.Stop()

	watchList := w.WatchList()
	for _, p := range watchList {
		if p == hiddenDir {
			t.Errorf("hidden directory %s should not be watched", hiddenDir)
		}
	}

	foundVisible := slices.Contains(watchList, visibleDir)
	if !foundVisible {
		t.Errorf("visible directory %s should be watched", visibleDir)
	}
}

func TestWatcher_NonExistentDir(t *testing.T) {
	w, err := New(Config{
		Dirs:     []string{"/nonexistent/path/that/does/not/exist"},
		Debounce: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() should not fail for nonexistent dir: %v", err)
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
			w := &Watcher{cfg: Config{Extensions: tt.extensions}}
			if got := w.isRelevantFile(tt.path); got != tt.want {
				t.Errorf("isRelevantFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestToChangeOp(t *testing.T) {
	tests := []struct {
		op   fsnotify.Op
		want Op
	}{
		{fsnotify.Create, OpCreate},
		{fsnotify.Write, OpModify},
		{fsnotify.Remove, OpDelete},
		{fsnotify.Rename, OpRename},
	}

	for _, tt := range tests {
		got := toOp(tt.op)
		if got != tt.want {
			t.Errorf("toOp(%v) = %v, want %v", tt.op, got, tt.want)
		}
	}
}

func TestChangeOp_String(t *testing.T) {
	tests := []struct {
		op   Op
		want string
	}{
		{OpCreate, "CREATE"},
		{OpModify, "MODIFY"},
		{OpDelete, "DELETE"},
		{OpRename, "RENAME"},
		{Op(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.op.String(); got != tt.want {
			t.Errorf("Op(%d).String() = %q, want %q", tt.op, got, tt.want)
		}
	}
}

// eventTimeout is the outer deadline for every wait in this file. It is not a
// latency budget: each wait returns as soon as its condition holds, so the
// bound only decides how long a genuinely missing event takes to fail. It is
// sized for -race, coverage instrumentation and a contended CI runner rather
// than for an uncontended local run.
const eventTimeout = 30 * time.Second

// waitForEvent reads batches from events until one holds an event for which
// match returns true, failing the test after eventTimeout. Batches without a
// match are skipped, so a stray or split debounce batch cannot fail the wait.
// It returns every event it read, including those of skipped batches, so a
// caller can also assert that some event did NOT arrive before the match.
func waitForEvent(t *testing.T, events <-chan []Event, what string, match func(Event) bool) []Event {
	t.Helper()
	deadline := time.After(eventTimeout)
	var seen []Event
	for {
		select {
		case batch := <-events:
			seen = append(seen, batch...)
			if slices.ContainsFunc(batch, match) {
				return seen
			}
		case <-deadline:
			t.Fatalf("timeout after %s waiting for %s; events seen: %v", eventTimeout, what, seen)
		}
	}
}

// waitWatched polls until path shows up in the watch list, failing the
// test after eventTimeout. Used where the event loop registers
// watches asynchronously (auto-watching newly created directories).
func waitWatched(t *testing.T, w *Watcher, path string) {
	t.Helper()
	deadline := time.Now().Add(eventTimeout)
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

	eventsChan := make(chan []Event, 10)

	w, err := New(Config{
		Dirs:       []string{dir},
		Extensions: []string{".md"},
		Debounce:   50 * time.Millisecond,
		OnChange: func(events []Event) {
			eventsChan <- events
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	go w.Start()
	defer w.Stop()

	// No startup wait needed: New registers the fsnotify watches
	// synchronously, and events queue until Start drains them.

	// Create a file
	testFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(testFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	waitForEvent(t, eventsChan, "file create event", func(e Event) bool {
		return e.Path == testFile
	})
}

func TestWatcher_IgnoresNonMatchingExtensions(t *testing.T) {
	dir := t.TempDir()

	eventsChan := make(chan []Event, 10)

	w, err := New(Config{
		Dirs:       []string{dir},
		Extensions: []string{".md"},
		Debounce:   50 * time.Millisecond,
		OnChange: func(events []Event) {
			eventsChan <- events
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
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

	seen := waitForEvent(t, eventsChan, "sentinel .md event", func(e Event) bool {
		return e.Path == mdFile
	})
	for _, e := range seen {
		if e.Path == txtFile {
			t.Errorf("should not receive event for non-matching extension: %v", e)
		}
	}
}

func TestWatcher_AutoWatchesNewDirectories(t *testing.T) {
	dir := t.TempDir()

	eventsChan := make(chan []Event, 10)

	w, err := New(Config{
		Dirs:       []string{dir},
		Extensions: []string{".md"},
		Debounce:   50 * time.Millisecond,
		OnChange: func(events []Event) {
			eventsChan <- events
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
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

	waitForEvent(t, eventsChan, "event from new subdirectory", func(e Event) bool {
		return e.Path == testFile
	})
}

func TestWatcher_FileModifyAndDelete(t *testing.T) {
	dir := t.TempDir()

	// Create file before starting watcher
	testFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(testFile, []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}

	eventsChan := make(chan []Event, 10)

	w, err := New(Config{
		Dirs:       []string{dir},
		Extensions: []string{".md"},
		Debounce:   50 * time.Millisecond,
		OnChange: func(events []Event) {
			eventsChan <- events
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	go w.Start()
	defer w.Stop()

	// Modify the file
	if err := os.WriteFile(testFile, []byte("modified"), 0o644); err != nil {
		t.Fatal(err)
	}

	waitForEvent(t, eventsChan, "file modify event", func(e Event) bool {
		return e.Path == testFile && e.Op == OpModify
	})

	// Delete the file
	if err := os.Remove(testFile); err != nil {
		t.Fatal(err)
	}

	waitForEvent(t, eventsChan, "file delete event", func(e Event) bool {
		return e.Path == testFile && e.Op == OpDelete
	})
}

func TestWatcher_PauseResumeIsPaused(t *testing.T) {
	dir := t.TempDir()
	w, err := New(Config{
		Dirs:     []string{dir},
		Debounce: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
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

	w, err := New(Config{
		Dirs:     []string{dir},
		Debounce: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer w.Stop()

	// Adding the file (even though its dir is already watched) must not error.
	if err := w.AddFile(path); err != nil {
		t.Errorf("AddFile(%q): %v", path, err)
	}
}
