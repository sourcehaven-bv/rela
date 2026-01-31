package dataentry

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// coverage-ignore: filesystem watcher

const watchDebounce = 500 * time.Millisecond

// FileWatcher watches entities/ and relations/ directories for changes
// and triggers graph rebuilds and auto-commits.
type FileWatcher struct {
	watcher   *fsnotify.Watcher
	stopCh    chan struct{}
	mu        sync.Mutex
	timer     *time.Timer
	onChanged func() // called after debounce when files change
}

// NewFileWatcher creates a file watcher for the given project directories.
// onChanged is called (debounced) whenever entity or relation files change.
func NewFileWatcher(dirs []string, onChanged func()) (*FileWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	fw := &FileWatcher{
		watcher:   w,
		stopCh:    make(chan struct{}),
		onChanged: onChanged,
	}

	// Add directories recursively
	for _, dir := range dirs {
		if err := fw.addRecursive(dir); err != nil {
			w.Close()
			return nil, err
		}
	}

	go fw.loop()
	return fw, nil
}

// addRecursive adds a directory and all subdirectories to the watcher.
func (fw *FileWatcher) addRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip hidden directories (like .git)
			if strings.HasPrefix(info.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			if addErr := fw.watcher.Add(path); addErr != nil {
				log.Printf("watcher: failed to add %s: %v", path, addErr)
			}
		}
		return nil
	})
}

// loop is the main event loop for the file watcher.
func (fw *FileWatcher) loop() {
	for {
		select {
		case <-fw.stopCh:
			return

		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			// Only react to markdown file changes
			if !isMarkdownFile(event.Name) {
				// If a new directory was created, watch it
				if event.Has(fsnotify.Create) {
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						_ = fw.watcher.Add(event.Name)
					}
				}
				continue
			}

			// Debounce: reset timer on each change
			fw.mu.Lock()
			if fw.timer != nil {
				fw.timer.Stop()
			}
			fw.timer = time.AfterFunc(watchDebounce, func() {
				if fw.onChanged != nil {
					fw.onChanged()
				}
			})
			fw.mu.Unlock()

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("watcher error: %v", err)
		}
	}
}

// isMarkdownFile returns true if the file path ends in .md.
func isMarkdownFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".md")
}

// Stop stops the file watcher and releases resources.
func (fw *FileWatcher) Stop() {
	close(fw.stopCh)
	fw.mu.Lock()
	if fw.timer != nil {
		fw.timer.Stop()
	}
	fw.mu.Unlock()
	fw.watcher.Close()
}
