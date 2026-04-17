// Package fsstore provides a filesystem-backed implementation of store.Store.
//
// # Architecture
//
// FSStore maintains a lightweight in-memory index (entity IDs+types, relation
// keys) and a property value cache. Full entity/relation data is loaded from
// disk on demand. Writes persist to the filesystem first, then update the
// in-memory index.
//
// # Concurrency
//
// All state is protected by a single [sync.RWMutex]. Write methods acquire
// mu.Lock; read methods acquire mu.RLock. File I/O happens under the lock
// to ensure index consistency with the filesystem.
//
// Event emission is called under mu.Lock. Subscribers receive events on
// buffered channels via non-blocking sends.
package fsstore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/cache"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// recentHashCapacity bounds how many recently-written file hashes are
// kept in memory for self-echo filtering. Large enough to cover a full
// bulk import of a sizable project; still small enough to stay cheap.
const recentHashCapacity = 4096

// Config holds the configuration for creating a new FSStore.
type Config struct {
	FS             storage.FS
	EntitiesDir    string
	RelationsDir   string
	AttachmentsDir string
	CacheDir       string                            // for property-cache.json
	Schemas        map[string]store.EntityTypeSchema // type → plural + property order
	// Observers are notified synchronously on entity writes (create, update,
	// delete, rename). They are NOT populated from existing entity files on
	// startup — callers that need that behavior can iterate ListEntities
	// after New returns and feed their observer directly.
	Observers []store.EntityObserver
}

// entityMeta is the lightweight in-memory representation of an entity.
type entityMeta struct {
	ID   string
	Type string
}

// relationMeta is the lightweight in-memory representation of a relation.
type relationMeta struct {
	From string
	Type string
	To   string
}

// attachMeta tracks attachment metadata in memory.
type attachMeta struct {
	entityID string
	property string
	fileName string
	size     int64
}

// FSStore is a filesystem-backed store implementation.
type FSStore struct {
	// filesystem
	fs           storage.FS
	entitiesDir  string
	relationsDir string
	attachDir    string
	cacheDir     string
	schemas      map[string]store.EntityTypeSchema

	// in-memory index
	mu            sync.RWMutex
	entities      map[string]entityMeta
	entityOrder   []string
	relations     map[string]relationMeta // key (from--type--to) → meta
	relationOrder []string
	attachments   map[string]attachMeta // "entityID/property" → meta
	propCache     map[string]map[string]int // property → value → count

	// observers notified synchronously on entity writes
	observers []store.EntityObserver

	// event subscribers
	subscribers map[int]chan store.Event
	nextSubID   int

	// fs watcher (external-change detection). nil when not started.
	extWatcher *storage.Watcher

	// recentHashes records the SHA256 of the last content written by this
	// store for each entity/relation file path. The external-change watcher
	// uses these to distinguish its own writes (self-echoes) from genuine
	// external edits: if the on-disk file hashes to the recorded value, the
	// event is a self-echo and gets dropped. Bounded-LRU so memory stays
	// bounded under large-project bulk writes.
	recentHashes *cache.LRU[string, string]
}

// compile-time interface check
var _ store.Store = (*FSStore)(nil)

// New creates a new filesystem-backed store. It scans the entities and
// relations directories to build the in-memory index, and loads or rebuilds
// the property value cache.
func New(cfg Config) (*FSStore, error) {
	if cfg.Schemas == nil {
		cfg.Schemas = make(map[string]store.EntityTypeSchema)
	}

	s := &FSStore{
		fs:           cfg.FS,
		entitiesDir:  cfg.EntitiesDir,
		relationsDir: cfg.RelationsDir,
		attachDir:    cfg.AttachmentsDir,
		cacheDir:     cfg.CacheDir,
		schemas:      cfg.Schemas,
		observers:    cfg.Observers,
		entities:     make(map[string]entityMeta),
		relations:    make(map[string]relationMeta),
		attachments:  make(map[string]attachMeta),
		propCache:    make(map[string]map[string]int),
		subscribers:  make(map[int]chan store.Event),
		recentHashes: cache.NewLRU[string, string](recentHashCapacity),
	}

	s.cleanupTempFiles()

	if err := s.syncIndex(); err != nil {
		return nil, err
	}
	if err := s.loadAttachmentsIndex(); err != nil {
		return nil, err
	}

	return s, nil
}

// loadAttachmentsIndex walks the attachments directory and populates metadata.
func (s *FSStore) loadAttachmentsIndex() error {
	if s.attachDir == "" {
		return nil
	}

	_, err := s.fs.Stat(s.attachDir)
	if err != nil {
		return nil // directory doesn't exist yet — no attachments
	}

	entries, err := s.fs.ReadDir(s.attachDir)
	if err != nil {
		return nil // ignore errors reading attachment dir
	}

	for _, entityEntry := range entries {
		if !entityEntry.IsDir() {
			continue
		}
		entityID := entityEntry.Name()
		propEntries, err := s.fs.ReadDir(filepath.Join(s.attachDir, entityID))
		if err != nil {
			continue
		}
		for _, propEntry := range propEntries {
			if !propEntry.IsDir() {
				continue
			}
			prop := propEntry.Name()
			fileEntries, err := s.fs.ReadDir(filepath.Join(s.attachDir, entityID, prop))
			if err != nil {
				continue
			}
			for _, fileEntry := range fileEntries {
				if fileEntry.IsDir() {
					continue
				}
				info, err := fileEntry.Info()
				if err != nil {
					continue
				}
				key := entityID + "/" + prop
				s.attachments[key] = attachMeta{
					entityID: entityID,
					property: prop,
					fileName: fileEntry.Name(),
					size:     info.Size(),
				}
				break // one file per property
			}
		}
	}
	return nil
}

// LastModified returns the newest mtime across all entity and relation
// files, also folding in the entities/ and relations/ directory mtimes so
// that deletions (which remove files without touching other files) are
// still observable.
func (s *FSStore) LastModified(_ context.Context) (time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	newest := s.newestEntityFileMtime()
	for _, t := range []time.Time{
		s.newestRelationFileMtime(),
		s.entitiesDirMtime(),
		s.relationsDirMtime(),
	} {
		if t.After(newest) {
			newest = t
		}
	}
	return newest, nil
}

// notifyPut notifies all observers that an entity was created or updated.
func (s *FSStore) notifyPut(e *entity.Entity) {
	for _, o := range s.observers {
		_ = o.EntityPut(e)
	}
}

// notifyDelete notifies all observers that an entity was removed.
func (s *FSStore) notifyDelete(id string) {
	for _, o := range s.observers {
		_ = o.EntityDelete(id)
	}
}

// entityFilePath returns the path for an entity file: entities/<plural>/<id>.md
func (s *FSStore) entityFilePath(entityType, id string) string {
	plural := entityType + "s"
	if schema, ok := s.schemas[entityType]; ok && schema.Plural != "" {
		plural = schema.Plural
	}
	return filepath.Join(s.entitiesDir, plural, id+".md")
}

// relationFilePath returns the path for a relation file.
func (s *FSStore) relationFilePath(from, relType, to string) string {
	return filepath.Join(s.relationsDir, from+"--"+relType+"--"+to+".md")
}

// propertyOrder returns the property order for an entity type, if configured.
func (s *FSStore) propertyOrder(entityType string) []string {
	if schema, ok := s.schemas[entityType]; ok {
		return schema.PropertyOrder
	}
	return nil
}

// cleanupTempFiles removes orphaned .new temp files left by interrupted writes.
func (s *FSStore) cleanupTempFiles() {
	for _, dir := range []string{s.entitiesDir, s.relationsDir} {
		if dir == "" {
			continue
		}
		var toRemove []string
		_ = s.fs.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if strings.HasSuffix(path, ".new") {
				toRemove = append(toRemove, path)
			}
			return nil
		})
		for _, path := range toRemove {
			_ = s.fs.Remove(path)
		}
	}
}

// sortStrings sorts a string slice in place.
func sortStrings(s []string) {
	if len(s) <= 1 {
		return
	}
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
