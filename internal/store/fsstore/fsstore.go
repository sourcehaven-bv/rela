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
	"errors"
	"log/slog"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// recentHashCapacity bounds how many recently-written file hashes are
// kept in memory for self-echo filtering. Large enough to cover a full
// bulk import of a sizable project; still small enough to stay cheap.
const recentHashCapacity = 4096

// Config holds the configuration for creating a new FSStore.
//
// Rooted is the primary write/read surface — every data I/O flows
// through it, so path validation sits in one auditable place.
// FS remains for raw operations that legitimately need absolute
// paths (watcher self-echo reads from fsnotify events).
//
// EntitiesKey/RelationsKey/AttachmentsKey/CacheKey are root-relative
// forward-slash keys (e.g. "entities", ".rela"), not absolute paths.
// They feed RootedFS directly.
type Config struct {
	FS             storage.FS
	Rooted         *storage.RootedFS
	EntitiesKey    string
	RelationsKey   string
	AttachmentsKey string
	CacheKey       string // for fsstore-index.json
	// Schemas maps every entity type the metamodel declares to its
	// storage-relevant configuration (plural directory name + property
	// order). fsstore relies on this map being complete: directories
	// whose plural does not resolve to a known type are skipped at scan
	// time, and inaccessible-entity shells use the schema's
	// PropertyOrder to enumerate fields. Empty maps are rejected by
	// [New].
	//
	// In production this is built by app.buildSchemas from the loaded
	// metamodel.
	Schemas map[string]store.EntityTypeSchema
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
//
// TODO(TKT-N0IKN9): FSStore is over the 40-method load line (89 methods).
// Decompose; ratchet this number down as responsibilities move out.
// (84 → 95 with the Tx contract, TKT-GXHI8: each write method split into
// an exported txMu wrapper + unexported core so Tx callbacks can re-enter;
// 95 → 89 with the fileLayout extraction, TKT-Y683LJ.)
//
// The exported surface (33) is mostly the mandated store.Store interface (~28
// methods incl. Tx) plus the Formatter and watcher side-interfaces; consumers
// depend on store.Store directly by design (see ./CLAUDE.md). Ratchet toward
// the interface size as the non-interface public methods
// (FormatEntity/Relation, Start/StopWatching) move to composed helpers.
//
//plimsoll:max-methods=89
//plimsoll:max-exported-methods=33
type FSStore struct {
	// rooted is the validated-key I/O surface. Every read, write,
	// directory op, and remove that operates on files under the
	// project root flows through it — the RootedFS.resolve() barrier
	// sits between callers and the underlying FS.
	rooted *storage.RootedFS

	// rawReader is the watcher's window into on-disk bytes for
	// self-echo hashing. Receives fsnotify absolute paths, so it
	// cannot use RootedFS (which takes keys).
	rawReader storage.FS

	// streamingSupported caches rooted.SupportsStreaming() to avoid
	// walking the decorator chain per attachment write.
	streamingSupported bool

	// layout resolves file keys, plural directory names, and property
	// order from the immutable path/key/schema configuration.
	layout fileLayout

	// Keys (root-relative forward-slash) for the non-layout subtrees.
	attachKey string
	cacheKey  string

	// txMu serializes an open Tx against ordinary writers: Tx holds it
	// for the whole callback, every exported write method takes it
	// briefly before mu (lock order: txMu → mu; see tx.go). Readers
	// never take it.
	txMu sync.Mutex

	// in-memory index
	mu            sync.RWMutex
	entities      map[string]entityMeta
	entityOrder   []string
	relations     map[string]relationMeta // key (from--type--to) → meta
	relationOrder []string
	attachments   map[string]attachMeta     // "entityID/property" → meta
	propCache     map[string]map[string]int // property → value → count

	// observers notified synchronously on entity writes
	observers []store.EntityObserver

	// event subscribers
	subscribers map[int]chan store.Event
	nextSubID   int

	// fs watcher (external-change detection). nil when not started.
	extWatcher *storage.Watcher

	// echoes tracks the hash of bytes most recently written by
	// this store so the external-change watcher can distinguish
	// its own writes (self-echoes) from genuine external edits.
	// Fed by SafeFS.OnPostWrite with the bytes that landed on disk.
	echoes *echoTracker
}

// compile-time interface check
var _ store.Store = (*FSStore)(nil)

// RecordWrite is the post-write observer entry point. Typically
// wired via SafeFS.OnPostWrite(store.RecordWrite). The watcher
// uses the recorded hash to suppress self-echoes.
//
// Signature matches storage.WriteObserver.
func (s *FSStore) RecordWrite(path string, content []byte) {
	s.echoes.Recorded(path, content)
}

// New creates a new filesystem-backed store. It scans the entities and
// relations directories to build the in-memory index, and loads or rebuilds
// the property value cache.
func New(cfg Config) (*FSStore, error) {
	if cfg.Rooted == nil {
		return nil, errors.New("fsstore: Config.Rooted must not be nil")
	}
	if cfg.FS == nil {
		return nil, errors.New("fsstore: Config.FS must not be nil")
	}
	if cfg.EntitiesKey == "" {
		return nil, errors.New("fsstore: Config.EntitiesKey must not be empty")
	}
	if cfg.RelationsKey == "" {
		return nil, errors.New("fsstore: Config.RelationsKey must not be empty")
	}
	// Schemas must be populated from the loaded metamodel. Without it,
	// fsstore cannot map plural directory names back to entity types
	// reliably, and inaccessible-entity shells (for git-crypt encrypted
	// files) cannot enumerate their schema-declared properties. An
	// empty map always indicates a bootstrap-ordering bug.
	if len(cfg.Schemas) == 0 {
		return nil, errors.New("fsstore: Config.Schemas must be populated from the metamodel; an empty map indicates a bootstrap-ordering bug")
	}

	s := &FSStore{
		rooted:             cfg.Rooted,
		rawReader:          cfg.FS,
		streamingSupported: cfg.Rooted.SupportsStreaming(),
		layout: fileLayout{
			entitiesKey:  cfg.EntitiesKey,
			relationsKey: cfg.RelationsKey,
			schemas:      cfg.Schemas,
			rooted:       cfg.Rooted,
		},
		attachKey:   cfg.AttachmentsKey,
		cacheKey:    cfg.CacheKey,
		observers:   cfg.Observers,
		entities:    make(map[string]entityMeta),
		relations:   make(map[string]relationMeta),
		attachments: make(map[string]attachMeta),
		propCache:   make(map[string]map[string]int),
		subscribers: make(map[int]chan store.Event),
		echoes:      newEchoTracker(recentHashCapacity),
	}

	s.cleanupTempFiles()

	if err := s.syncIndex(); err != nil {
		return nil, err
	}
	s.loadAttachmentsIndex()

	return s, nil
}

// loadAttachmentsIndex walks the attachments directory and populates
// in-memory metadata. Missing directory and read errors are swallowed
// — a partial index is preferable to failing the open.
//
// Size comes from fs.DirEntry.Info() so we never read the file
// contents during index load. Previously used s.bytes.ReadFile which
// pulled every attachment into memory on every store open.
func (s *FSStore) loadAttachmentsIndex() {
	if s.attachKey == "" {
		return
	}

	if _, err := s.rooted.Stat(s.attachKey); err != nil {
		return
	}

	entries, err := s.rooted.ReadDir(s.attachKey)
	if err != nil {
		return
	}

	for _, entityEntry := range entries {
		if !entityEntry.IsDir() {
			continue
		}
		entityID := entityEntry.Name()
		propEntries, err := s.rooted.ReadDir(path.Join(s.attachKey, entityID))
		if err != nil {
			continue
		}
		for _, propEntry := range propEntries {
			if !propEntry.IsDir() {
				continue
			}
			s.loadPropertyAttachments(entityID, propEntry.Name())
		}
	}
}

// loadPropertyAttachments indexes every attachment file under one
// entity/property directory. A property may hold multiple files; each is
// keyed by (entityID, property, fileName). Leftover temp files from an
// interrupted write (attachTempPrefix) are skipped — that marker can't
// collide with a real attachment name (which never starts with a dot).
// Must be called with s.mu held.
func (s *FSStore) loadPropertyAttachments(entityID, prop string) {
	fileEntries, err := s.rooted.ReadDir(path.Join(s.attachKey, entityID, prop))
	if err != nil {
		return
	}
	for _, fileEntry := range fileEntries {
		if fileEntry.IsDir() {
			continue
		}
		name := fileEntry.Name()
		if strings.HasPrefix(name, attachTempPrefix) {
			continue
		}
		info, err := fileEntry.Info()
		if err != nil {
			continue
		}
		s.attachments[attachmentKey(entityID, prop, name)] = attachMeta{
			entityID: entityID,
			property: prop,
			fileName: name,
			size:     info.Size(),
		}
	}
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

// notifyRenamed fans out a rename to all observers. The rename code
// path emits this INSTEAD OF the EntityDelete(oldID)+EntityPut(renamed)
// pair — see store.EntityObserver.EntityRenamed.
func (s *FSStore) notifyRenamed(oldID string, renamed *entity.Entity) {
	for _, o := range s.observers {
		_ = o.EntityRenamed(oldID, renamed)
	}
}

// cleanupTempFiles removes orphaned temp files left by interrupted
// writes. Two suffixes are swept: ".new" (legacy direct fsstore
// atomic-write) and ".tmp" (SafeFS atomic-write). Both are produced by
// writeFile → rename paths and are never expected to survive a normal
// process shutdown.
func (s *FSStore) cleanupTempFiles() {
	for _, dirKey := range []string{s.layout.entitiesKey, s.layout.relationsKey} {
		if dirKey == "" {
			continue
		}
		var toRemove []string
		if err := s.rooted.Walk(dirKey, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil //nolint:nilerr // walker continuation on error is intentional
			}
			if strings.HasSuffix(p, ".new") || strings.HasSuffix(p, ".tmp") {
				toRemove = append(toRemove, p)
			}
			return nil
		}); err != nil {
			slog.Warn("fsstore: temp-file cleanup walk failed", "dir", dirKey, "err", err)
		}
		for _, p := range toRemove {
			_ = s.rooted.Remove(p)
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
