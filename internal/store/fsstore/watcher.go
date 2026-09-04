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

// Close shuts down the store, persists the index, closes all subscriber
// channels, and uninstalls the self-echo recorder from the FS so a
// store that outlives this one on the same FS is the only one fed.
func (s *FSStore) Close() error {
	s.StopWatching()
	if s.unwireEchoes != nil {
		s.unwireEchoes()
	}

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

// ErrWatchNeedsObservableFS is returned by StartWatching when the
// store's FS offered no post-write observer at New (see Config.FS).
// Without one the watcher cannot tell its own writes from external
// edits and would re-deliver every store write as an Updated event,
// so it refuses to start rather than run degraded (BUG-S24X52).
var ErrWatchNeedsObservableFS = errors.New(
	"fsstore: StartWatching requires an FS with a post-write observer (SafeFS or MemFS); " +
		"wrap the FS with storage.NewSafeFS at the entry point")

// StartWatching begins watching the entities and relations directories for
// external file changes (edits made outside the store API). Detected
// changes are reconciled into the in-memory index and re-emitted as
// store.Events. Self-writes are suppressed via the echoTracker, which
// New feeds from the FS's post-write observer; an FS without one makes
// this return [ErrWatchNeedsObservableFS].
//
// Calling StartWatching more than once is a no-op after the first call.
//
// coverage-ignore-func: requires real filesystem events via fsnotify
func (s *FSStore) StartWatching() error {
	if s.echoWiringErr != nil {
		return s.echoWiringErr
	}
	s.mu.Lock()
	if s.extWatcher != nil {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	var dirs []string
	if abs := s.layout.absPath(s.layout.entitiesKey); abs != "" {
		dirs = append(dirs, abs)
	}
	if abs := s.layout.absPath(s.layout.relationsKey); abs != "" {
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
	abs := s.layout.absPath(s.layout.entitiesKey)
	return abs != "" && hasPathPrefix(path, abs)
}

// isRelationPath reports whether path lives under the relations directory.
func (s *FSStore) isRelationPath(path string) bool {
	abs := s.layout.absPath(s.layout.relationsKey)
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
	// The filename stem is authoritative for the face (TKT-DOFYR1):
	// a state file's frontmatter carries only the bare id.
	if _, ptr, _, ok := s.entityIdentityFromPath(path); ok {
		e.Face = ptr
	}

	s.echoes.Recorded(path, rawData)

	key := stateKey(e.ID, e.Face)
	existing, known := s.entities[key]
	if known && existing.Face.IsDefault() {
		removed, loadErr := s.loadEntityMeta(existing)
		if loadErr == nil {
			removeEntityFromCache(s.propCache, removed)
		}
	}
	s.entities[key] = entityMeta{ID: e.ID, Type: e.Type, Face: e.Face}
	if !known {
		s.entityOrder = storeutil.SortedInsertFunc(s.entityOrder, key, storeutil.CompareStateKeys)
	}
	if e.Face.IsDefault() {
		addEntityToCache(s.propCache, e)
	}
	s.notifyPut(e)

	op := store.EventEntityUpdated
	if !known {
		op = store.EventEntityCreated
	}
	s.emit(store.Event{Op: op, EntityType: e.Type, EntityID: e.ID, Face: e.Face})
}

// handleEntityRemoval handles the disappearance of an entity file.
// Must be called under mu.Lock.
func (s *FSStore) handleEntityRemoval(path string) {
	s.echoes.Forget(path)

	stem, ok := s.entityStemFromPath(path)
	if !ok {
		return
	}
	// The stem IS the state key ("id" or "id@face").
	meta, known := s.entities[stem]
	if !known {
		return
	}

	if meta.Face.IsDefault() {
		if e, err := s.loadEntityMeta(meta); err == nil {
			removeEntityFromCache(s.propCache, e)
		}
	}
	delete(s.entities, stem)
	s.entityOrder = storeutil.SortedRemoveFunc(s.entityOrder, stem, storeutil.CompareStateKeys)
	// Face-aware observers can evict exactly the removed face. Bare-id
	// observers still cannot address one, so they hear a delete only for
	// the default face — evicting the whole entity because a sibling
	// state's file vanished would drop a document that is still live.
	s.notifyFaceDelete(meta.ID, meta.Face)
	if meta.Face.IsDefault() {
		s.notifyLastFaceDelete(meta.ID)
	}
	s.emit(store.Event{Op: store.EventEntityDeleted, EntityType: meta.Type, EntityID: meta.ID, Face: meta.Face})
}

// entityStemFromPath extracts the filename stem — the STATE KEY, "id"
// or "id@face" — from a file path under the entities directory:
// entitiesDir/<plural>/<stem>.md.
func (s *FSStore) entityStemFromPath(path string) (string, bool) {
	base := filepath.Base(path)
	if !strings.HasSuffix(base, ".md") {
		return "", false
	}
	return strings.TrimSuffix(base, ".md"), true
}

// entityIdentityFromPath extracts the bare entity ID, the state
// face, and the entity type from a file path under
// entitiesDir/<plural>/<stem>.md. The plural directory name is mapped
// back to the entity type via the configured schemas. Returns ok=false
// if the path doesn't have the expected shape, the stem doesn't parse
// as a state reference, or the plural doesn't map to a known type.
func (s *FSStore) entityIdentityFromPath(path string) (id string, ptr entity.Face, entityType string, ok bool) {
	stem, ok := s.entityStemFromPath(path)
	if !ok {
		return "", "", "", false
	}
	id, ptr, err := entity.ParseStateRef(stem)
	if err != nil {
		return "", "", "", false
	}
	parent := filepath.Base(filepath.Dir(path))
	if parent == "" {
		return "", "", "", false
	}
	pluralToType := s.layout.buildPluralToTypeMap()
	entityType = s.layout.resolveEntityType(parent, pluralToType)
	if entityType == "" {
		return "", "", "", false
	}
	return id, ptr, entityType, true
}

// reconcileRelationPath handles a change event for a relation file. Must
// be called under mu.Lock.
func (s *FSStore) reconcileRelationPath(path string) {
	base := filepath.Base(path)
	if !strings.HasSuffix(base, ".md") {
		return
	}
	fromSlot, relType, to := parseRelationFilename(strings.TrimSuffix(base, ".md"))
	if fromSlot == "" || relType == "" || to == "" {
		return
	}
	// The FROM slot may carry a tail face ("id@face", TKT-DOFYR1).
	from, fp, refErr := entity.ParseStateRef(fromSlot)
	if refErr != nil {
		return
	}
	rm := relationMeta{From: from, Type: relType, To: to, FromFace: fp}
	key := rm.key()

	data, readErr := s.rawReader.ReadFile(path)
	if readErr != nil {
		s.handleRelationRemoval(path, key, rm)
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
		s.relations[key] = rm
		s.relationOrder = storeutil.SortedInsert(s.relationOrder, key)
	}

	op := store.EventRelationUpdated
	if !known {
		op = store.EventRelationCreated
	}
	s.emit(store.Event{Op: op, RelationType: relType, From: from, To: to, Face: fp})
}

// handleRelationRemoval handles the disappearance of a relation file.
// Must be called under mu.Lock.
func (s *FSStore) handleRelationRemoval(path, key string, rm relationMeta) {
	s.echoes.Forget(path)

	if _, known := s.relations[key]; !known {
		return
	}
	delete(s.relations, key)
	s.relationOrder = storeutil.SortedRemove(s.relationOrder, key)
	s.emit(store.Event{
		Op: store.EventRelationDeleted, RelationType: rm.Type,
		From: rm.From, To: rm.To, Face: rm.FromFace,
	})
}

// parseEntityFromPath parses raw bytes from a watcher event into an
// entity. Encrypted files are recognized at this boundary and returned
// as inaccessible-entity shells, mirroring the regular read path
// (readEntityFile) so the watcher does not have to know about
// git-crypt directly.
func (s *FSStore) parseEntityFromPath(data []byte, path string) (*entity.Entity, error) {
	if isGitCryptEncrypted(data) {
		id, ptr, entityType, ok := s.entityIdentityFromPath(path)
		if !ok {
			return nil, errors.New("encrypted entity file: cannot derive id/type from path")
		}
		key := s.layout.entityFileKey(entityType, stateKey(id, ptr))
		shell := s.codec.buildInaccessibleEntity(key, id, entityType, entity.InaccessibleReasonGitCrypt)
		shell.Face = ptr
		return shell, nil
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
		if entity.IsEntityPropertyKey(key) {
			e.Properties[key] = entity.CloneValue(value)
		}
	}
	return e, nil
}
