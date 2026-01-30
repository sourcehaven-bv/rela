// coverage-ignore: file watcher - requires filesystem events
package mcp

import (
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/migration"
)

// Watcher watches entity and relation files for changes and notifies MCP clients.
type Watcher struct {
	server    *Server
	fsWatcher *fsnotify.Watcher
	done      chan struct{}
}

// NewWatcher creates a new file watcher for the rela project.
func NewWatcher(s *Server) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		server:    s,
		fsWatcher: fsw,
		done:      make(chan struct{}),
	}

	// Watch entities directory and subdirectories
	if err := watchDirRecursive(fsw, s.projectCtx.EntitiesDir); err != nil {
		s.logger.Printf("Warning: could not watch entities dir: %v", err)
	}

	// Watch relations directory
	if err := fsw.Add(s.projectCtx.RelationsDir); err != nil {
		s.logger.Printf("Warning: could not watch relations dir: %v", err)
	}

	// Watch metamodel
	if err := fsw.Add(s.projectCtx.MetamodelPath); err != nil {
		s.logger.Printf("Warning: could not watch metamodel: %v", err)
	}

	// Watch views file
	viewsPath := filepath.Join(s.projectCtx.Root, "views.yaml")
	if err := fsw.Add(viewsPath); err != nil {
		s.logger.Printf("Warning: could not watch views file: %v", err)
	}

	return w, nil
}

// Start begins watching for file changes. Should be called in a goroutine.
func (w *Watcher) Start() {
	const debounceInterval = 200 * time.Millisecond
	timer := time.NewTimer(debounceInterval)
	timer.Stop()

	pendingSync := false

	for {
		select {
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			// Only react to actual file changes on .md and .yaml files
			if !isRelevantFile(event.Name) {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}

			w.server.logger.Printf("File changed: %s (%s)", event.Name, event.Op)
			pendingSync = true
			timer.Reset(debounceInterval)

		case <-timer.C:
			if pendingSync {
				w.syncAndNotify()
				pendingSync = false
			}

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			w.server.logger.Printf("Watcher error: %v", err)

		case <-w.done:
			return
		}
	}
}

// Stop stops the file watcher.
func (w *Watcher) Stop() {
	close(w.done)
	_ = w.fsWatcher.Close()
}

func (w *Watcher) syncAndNotify() {
	// Reload metamodel in case it changed
	newMeta, err := metamodel.Load(w.server.projectCtx.MetamodelPath)
	if err != nil {
		if migration.IsMigrationError(err) {
			w.server.logger.Printf("Metamodel needs migration, skipping reload: run 'rela migrate'")
		} else {
			w.server.logger.Printf("Metamodel reload error (keeping previous version): %v", err)
		}
	} else {
		w.server.setMeta(newMeta)
	}

	_, err = markdown.SyncFromFiles(w.server.projectCtx, w.server.getMeta(), w.server.graph)
	if err != nil {
		w.server.logger.Printf("Sync error: %v", err)
		return
	}

	w.server.saveCache()
	w.server.logger.Println("Graph re-synced from file changes")

	// Notify MCP clients that resources have changed
	w.server.mcp.SendNotificationToAllClients(
		mcp.MethodNotificationResourcesListChanged, nil,
	)
}

func isRelevantFile(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".md" || ext == ".yaml" || ext == ".yml"
}

func watchDirRecursive(w *fsnotify.Watcher, dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return w.Add(path)
		}
		return nil
	})
}
