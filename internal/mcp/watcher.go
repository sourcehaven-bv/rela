// coverage-ignore: file watcher - requires filesystem events
package mcp

import (
	"path/filepath"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/migration"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// Watcher watches entity and relation files for changes and notifies MCP clients.
type Watcher struct {
	server  *Server
	watcher *storage.Watcher
}

// NewWatcher creates a new file watcher for the rela project.
func NewWatcher(s *Server) (*Watcher, error) {
	viewsPath := filepath.Join(s.projectCtx.Root, "views.yaml")

	w := &Watcher{server: s}

	sw, err := storage.NewWatcher(storage.WatchConfig{
		Dirs:       []string{s.projectCtx.EntitiesDir, s.projectCtx.RelationsDir},
		Files:      []string{s.projectCtx.MetamodelPath, viewsPath},
		Extensions: []string{".md", ".yaml", ".yml"},
		Debounce:   200 * time.Millisecond,
		OnChange: func(events []storage.ChangeEvent) {
			for _, e := range events {
				s.logger.Printf("File changed: %s (%s)", e.Path, e.Op)
			}
			w.syncAndNotify()
		},
	})
	if err != nil {
		return nil, err
	}

	w.watcher = sw
	return w, nil
}

// Start begins watching for file changes. Should be called in a goroutine.
func (w *Watcher) Start() {
	w.watcher.Start()
}

// Stop stops the file watcher.
func (w *Watcher) Stop() {
	w.watcher.Stop()
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
