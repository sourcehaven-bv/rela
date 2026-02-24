// coverage-ignore: file watcher - requires filesystem events
package storage

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// WatchConfig configures a Watcher.
type WatchConfig struct {
	// Dirs lists directories to watch recursively.
	Dirs []string
	// Files lists individual files to watch.
	Files []string
	// Extensions lists file extensions to react to (e.g., ".md", ".yaml").
	// An empty list means all files are relevant.
	Extensions []string
	// Debounce is the debounce interval for batching change events.
	Debounce time.Duration
	// SkipHidden skips hidden directories (names starting with ".").
	SkipHidden bool
	// OnChange is called after debounce with the batched change events.
	OnChange func(events []ChangeEvent)
}

// Watcher watches files and directories for changes, debouncing events
// and delivering them in batches via the OnChange callback.
type Watcher struct {
	cfg       WatchConfig
	fsWatcher *fsnotify.Watcher
	done      chan struct{}
	mu        sync.Mutex
	pending   []ChangeEvent
	paused    bool
}

// NewWatcher creates a file watcher with the given configuration.
// Call Start in a goroutine to begin the event loop.
func NewWatcher(cfg WatchConfig) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		cfg:       cfg,
		fsWatcher: fsw,
		done:      make(chan struct{}),
	}

	// Watch directories recursively
	for _, dir := range cfg.Dirs {
		w.addRecursive(dir)
	}

	// Watch individual files
	for _, file := range cfg.Files {
		_ = fsw.Add(file)
	}

	return w, nil
}

// Start begins the event loop. Should be called in a goroutine.
// It blocks until Stop is called or the underlying watcher is closed.
func (w *Watcher) Start() {
	timer := time.NewTimer(w.cfg.Debounce)
	timer.Stop()

	for {
		select {
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}

			// Auto-watch new directories
			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					w.addRecursive(event.Name)
					continue
				}
			}

			if !w.isRelevantFile(event.Name) {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}

			w.mu.Lock()
			if !w.paused {
				w.pending = append(w.pending, ChangeEvent{
					Path: event.Name,
					Op:   toChangeOp(event.Op),
				})
				timer.Reset(w.cfg.Debounce)
			}
			w.mu.Unlock()

		case <-timer.C:
			w.mu.Lock()
			events := w.pending
			w.pending = nil
			w.mu.Unlock()

			if len(events) > 0 && w.cfg.OnChange != nil {
				w.cfg.OnChange(events)
			}

		case _, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}

		case <-w.done:
			return
		}
	}
}

// Stop stops the watcher and releases resources.
func (w *Watcher) Stop() {
	close(w.done)
	_ = w.fsWatcher.Close()
}

// WatchList returns the list of currently watched paths.
// Useful for testing.
func (w *Watcher) WatchList() []string {
	return w.fsWatcher.WatchList()
}

// Pause temporarily stops processing file change events.
// Events that occur while paused are discarded.
// Use Resume to re-enable event processing.
func (w *Watcher) Pause() {
	w.mu.Lock()
	w.paused = true
	w.pending = nil // Discard any pending events
	w.mu.Unlock()
}

// Resume re-enables event processing after a Pause.
func (w *Watcher) Resume() {
	w.mu.Lock()
	w.paused = false
	w.mu.Unlock()
}

// IsPaused returns whether the watcher is currently paused.
func (w *Watcher) IsPaused() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.paused
}

func (w *Watcher) addRecursive(root string) {
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return filepath.SkipDir
		}
		if info.IsDir() {
			if w.cfg.SkipHidden && strings.HasPrefix(info.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			_ = w.fsWatcher.Add(path)
		}
		return nil
	})
}

func (w *Watcher) isRelevantFile(path string) bool {
	if len(w.cfg.Extensions) == 0 {
		return true
	}
	ext := filepath.Ext(path)
	for _, e := range w.cfg.Extensions {
		if ext == e {
			return true
		}
	}
	return false
}

func toChangeOp(op fsnotify.Op) ChangeOp {
	switch {
	case op.Has(fsnotify.Create):
		return OpCreate
	case op.Has(fsnotify.Remove):
		return OpDelete
	case op.Has(fsnotify.Rename):
		return OpRename
	default:
		return OpModify
	}
}
