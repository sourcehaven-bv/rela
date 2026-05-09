package fsstore

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// Subscribe registers a new event subscriber with the given buffer size.
// Events are delivered on a best-effort basis: if the subscriber's channel
// is full, events are dropped silently.
func (s *FSStore) Subscribe(bufSize int) (events <-chan store.Event, cancel func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan store.Event, bufSize)
	id := s.nextSubID
	s.nextSubID++
	s.subscribers[id] = ch

	cancel = func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if _, ok := s.subscribers[id]; ok {
			delete(s.subscribers, id)
			close(ch)
		}
	}
	return ch, cancel
}

// emit sends an event to all subscribers. Non-blocking: drops if full.
// Must be called under mu.Lock.
func (s *FSStore) emit(ev store.Event) {
	for _, ch := range s.subscribers {
		select {
		case ch <- ev:
		default:
		}
	}
}

// Close shuts down the store, persists the index, and closes all subscriber channels.
func (s *FSStore) Close() error {
	s.StopWatching()

	s.mu.Lock()
	defer s.mu.Unlock()

	_ = s.savePersistedIndex()

	for id, ch := range s.subscribers {
		close(ch)
		delete(s.subscribers, id)
	}
	return nil
}

// --- external-change watcher ---

// StartWatching begins watching the entities and relations directories for
// external file changes (edits made outside the store API). Detected
// changes are reconciled into the in-memory index and re-emitted as
// store.Events. Self-writes are suppressed via the echoTracker.
//
// Calling StartWatching more than once is a no-op after the first call.
//
// coverage-ignore: requires real filesystem events via fsnotify
func (s *FSStore) StartWatching() error {
	s.mu.Lock()
	if s.extWatcher != nil {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	var dirs []string
	if abs := s.absPath(s.entitiesKey); abs != "" {
		dirs = append(dirs, abs)
	}
	if abs := s.absPath(s.relationsKey); abs != "" {
		dirs = append(dirs, abs)
	}
	if len(dirs) == 0 {
		return nil
	}

	w, err := storage.NewWatcher(storage.WatchConfig{
		Dirs:       dirs,
		Extensions: []string{".md"},
		Debounce:   200 * time.Millisecond,
		SkipHidden: true,
		OnChange: func(events []storage.ChangeEvent) {
			s.handleExternalEvents(events)
		},
	})
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.extWatcher = w
	s.mu.Unlock()

	go w.Start()
	return nil
}

// StopWatching stops the external-change watcher if one is running.
func (s *FSStore) StopWatching() {
	s.mu.Lock()
	w := s.extWatcher
	s.extWatcher = nil
	s.mu.Unlock()
	if w != nil {
		w.Stop()
	}
}

// handleExternalEvents reconciles a batch of filesystem events against the
// in-memory index and emits store.Events for anything that isn't a
// self-echo of our own write.
func (s *FSStore) handleExternalEvents(events []storage.ChangeEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ev := range events {
		s.handleExternalEvent(ev)
	}
}

// handleExternalEvent dispatches a single event to the entity or relation
// path. Must be called under mu.Lock.
func (s *FSStore) handleExternalEvent(ev storage.ChangeEvent) {
	if !strings.HasSuffix(ev.Path, ".md") {
		return
	}
	switch {
	case s.isEntityPath(ev.Path):
		s.reconcileEntityPath(ev.Path)
	case s.isRelationPath(ev.Path):
		s.reconcileRelationPath(ev.Path)
	}
}

// isEntityPath reports whether path lives under the entities directory.
// path is absolute (from fsnotify); converted via absPath(entitiesKey).
func (s *FSStore) isEntityPath(path string) bool {
	abs := s.absPath(s.entitiesKey)
	return abs != "" && hasPathPrefix(path, abs)
}

// isRelationPath reports whether path lives under the relations directory.
func (s *FSStore) isRelationPath(path string) bool {
	abs := s.absPath(s.relationsKey)
	return abs != "" && hasPathPrefix(path, abs)
}

// hasPathPrefix reports whether path is inside dir (as a prefix, with
// a path separator boundary). Handles trailing separators in dir.
func hasPathPrefix(path, dir string) bool {
	dir = strings.TrimRight(dir, string(filepath.Separator))
	if !strings.HasPrefix(path, dir) {
		return false
	}
	rest := path[len(dir):]
	return rest != "" && rest[0] == filepath.Separator
}

// reconcileEntityPath handles a change event for an entity file. Must be
// called under mu.Lock.
func (s *FSStore) reconcileEntityPath(path string) {
	rawData, readErr := s.rawReader.ReadFile(path)
	if readErr != nil {
		s.handleEntityRemoval(path)
		return
	}

	// Self-echo detection compares the on-disk bytes against the
	// hash recorded by SafeFS.OnPostWrite when fsstore itself wrote
	// this path.
	if s.echoes.IsEcho(path, rawData) {
		return // self-echo
	}

	e, err := s.parseEntityFromPath(rawData, path)
	if err != nil {
		return
	}

	s.echoes.Recorded(path, rawData)

	existing, known := s.entities[e.ID]
	if known {
		removed, loadErr := s.loadEntity(existing.ID, existing.Type)
		if loadErr == nil {
			removeEntityFromCache(s.propCache, removed)
		}
	}
	s.entities[e.ID] = entityMeta{ID: e.ID, Type: e.Type}
	if !known {
		s.entityOrder = storeutil.SortedInsert(s.entityOrder, e.ID)
	}
	addEntityToCache(s.propCache, e)
	s.notifyPut(e)

	op := store.EventEntityUpdated
	if !known {
		op = store.EventEntityCreated
	}
	s.emit(store.Event{Op: op, EntityType: e.Type, EntityID: e.ID})
}

// handleEntityRemoval handles the disappearance of an entity file.
// Must be called under mu.Lock.
func (s *FSStore) handleEntityRemoval(path string) {
	s.echoes.Forget(path)

	id, ok := s.entityIDFromPath(path)
	if !ok {
		return
	}
	meta, known := s.entities[id]
	if !known {
		return
	}

	if e, err := s.loadEntity(meta.ID, meta.Type); err == nil {
		removeEntityFromCache(s.propCache, e)
	}
	delete(s.entities, id)
	s.entityOrder = storeutil.SortedRemove(s.entityOrder, id)
	s.notifyDelete(id)
	s.emit(store.Event{Op: store.EventEntityDeleted, EntityType: meta.Type, EntityID: id})
}

// entityIDFromPath extracts the entity ID from a file path under the
// entities directory: entitiesDir/<plural>/<id>.md.
func (s *FSStore) entityIDFromPath(path string) (string, bool) {
	base := filepath.Base(path)
	if !strings.HasSuffix(base, ".md") {
		return "", false
	}
	return strings.TrimSuffix(base, ".md"), true
}

// entityIdentityFromPath extracts both the entity ID and entity type
// from a file path under entitiesDir/<plural>/<id>.md. The plural
// directory name is mapped back to the entity type via the configured
// schemas. Returns ok=false if the path doesn't have the expected shape
// or the plural directory doesn't map to a known type.
func (s *FSStore) entityIdentityFromPath(path string) (id, entityType string, ok bool) {
	id, ok = s.entityIDFromPath(path)
	if !ok {
		return "", "", false
	}
	parent := filepath.Base(filepath.Dir(path))
	if parent == "" {
		return "", "", false
	}
	pluralToType := s.buildPluralToTypeMap()
	entityType = s.resolveEntityType(parent, pluralToType)
	if entityType == "" {
		return "", "", false
	}
	return id, entityType, true
}

// reconcileRelationPath handles a change event for a relation file. Must
// be called under mu.Lock.
func (s *FSStore) reconcileRelationPath(path string) {
	base := filepath.Base(path)
	if !strings.HasSuffix(base, ".md") {
		return
	}
	from, relType, to := parseRelationFilename(strings.TrimSuffix(base, ".md"))
	if from == "" || relType == "" || to == "" {
		return
	}
	key := from + "--" + relType + "--" + to

	data, readErr := s.rawReader.ReadFile(path)
	if readErr != nil {
		s.handleRelationRemoval(path, key, from, relType, to)
		return
	}

	if s.echoes.IsEcho(path, data) {
		return
	}
	// Encrypted relation files participate in the index by filename
	// but their bodies are unreadable; the reconcile path is otherwise
	// identical because we don't index per-property values for relations.
	s.echoes.Recorded(path, data)

	_, known := s.relations[key]
	if !known {
		s.relations[key] = relationMeta{From: from, Type: relType, To: to}
		s.relationOrder = storeutil.SortedInsert(s.relationOrder, key)
	}

	op := store.EventRelationUpdated
	if !known {
		op = store.EventRelationCreated
	}
	s.emit(store.Event{Op: op, RelationType: relType, From: from, To: to})
}

// handleRelationRemoval handles the disappearance of a relation file.
// Must be called under mu.Lock.
func (s *FSStore) handleRelationRemoval(path, key, from, relType, to string) {
	s.echoes.Forget(path)

	if _, known := s.relations[key]; !known {
		return
	}
	delete(s.relations, key)
	s.relationOrder = storeutil.SortedRemove(s.relationOrder, key)
	s.emit(store.Event{Op: store.EventRelationDeleted, RelationType: relType, From: from, To: to})
}

// parseEntityFromPath parses raw bytes from a watcher event into an
// entity. Encrypted files are recognized at this boundary and returned
// as inaccessible-entity shells, mirroring the regular read path
// (readEntityFile) so the watcher does not have to know about
// git-crypt directly.
func (s *FSStore) parseEntityFromPath(data []byte, path string) (*entity.Entity, error) {
	if isGitCryptEncrypted(data) {
		id, entityType, ok := s.entityIdentityFromPath(path)
		if !ok {
			return nil, errors.New("encrypted entity file: cannot derive id/type from path")
		}
		key := s.entityFileKey(entityType, id)
		return s.buildInaccessibleEntity(key, id, entityType, entity.InaccessibleReasonGitCrypt), nil
	}
	doc, err := parseDocument(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	id := doc.getString("id")
	entityType := doc.getString("type")
	if id == "" || entityType == "" {
		return nil, errors.New("entity file missing id or type")
	}
	e := entity.New(id, entityType)
	e.Content = doc.content
	for key, value := range doc.frontmatter {
		if key != "id" && key != "type" {
			e.Properties[key] = entity.CloneValue(value)
		}
	}
	return e, nil
}
