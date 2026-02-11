// coverage-ignore: file watcher - requires filesystem events
package storage

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/Sourcehaven-BV/rela/internal/model"
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
	OnChange func(events []model.ChangeEvent)
}

// Watcher watches files and directories for changes, debouncing events
// and delivering them in batches via the OnChange callback.
type Watcher struct {
	cfg       WatchConfig
	fsWatcher *fsnotify.Watcher
	done      chan struct{}
	mu        sync.Mutex
	pending   []model.ChangeEvent
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
			w.pending = append(w.pending, model.ChangeEvent{
				Path: event.Name,
				Op:   toChangeOp(event.Op),
			})
			w.mu.Unlock()
			timer.Reset(w.cfg.Debounce)

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

func toChangeOp(op fsnotify.Op) model.ChangeOp {
	switch {
	case op.Has(fsnotify.Create):
		return model.OpCreate
	case op.Has(fsnotify.Remove):
		return model.OpDelete
	case op.Has(fsnotify.Rename):
		return model.OpRename
	default:
		return model.OpModify
	}
}
