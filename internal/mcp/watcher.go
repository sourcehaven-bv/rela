// coverage-ignore: file watcher - requires filesystem events
package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/migration"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/repository"
)

// Watcher watches entity and relation files for changes and notifies MCP clients.
type Watcher struct {
	server *Server
	stop   func()
}

// NewWatcher creates a new file watcher for the rela project using the
// Store's Watch method.
func NewWatcher(s *Server) (*Watcher, error) {
	w := &Watcher{server: s}

	stop, err := s.repo.Watch(repository.WatchOptions{}, func(events []model.ChangeEvent) {
		for _, e := range events {
			s.logger.Printf("File changed: %s (%s)", e.Path, e.Op)
		}
		w.syncAndNotify()
	})
	if err != nil {
		return nil, err
	}

	w.stop = stop
	return w, nil
}

// Stop stops the file watcher.
func (w *Watcher) Stop() {
	w.stop()
}

func (w *Watcher) syncAndNotify() {
	// Reload metamodel in case it changed
	newMeta, err := w.server.repo.LoadMetamodel()
	if err != nil {
		if migration.IsMigrationError(err) {
			w.server.logger.Printf("Metamodel needs migration, skipping reload: run 'rela migrate'")
		} else {
			w.server.logger.Printf("Metamodel reload error (keeping previous version): %v", err)
		}
	} else {
		w.server.setMeta(newMeta)
	}

	_, err = w.server.repo.Sync(w.server.getMeta(), w.server.graph)
	if err != nil {
		w.server.logger.Printf("Sync error: %v", err)
		return
	}

	w.server.saveCache()
	w.server.logger.Println("Graph re-synced from file changes")

	// Notify MCP clients that resources have changed
	if w.server.mcp != nil {
		w.server.mcp.SendNotificationToAllClients(
			mcp.MethodNotificationResourcesListChanged, nil,
		)
	}
}
