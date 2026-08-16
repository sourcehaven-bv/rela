// Package memstore provides an in-memory implementation of store.Store.
//
// # Concurrency
//
// All state is protected by a single [sync.RWMutex]. Go maps are not safe
// for concurrent access — even a read during a concurrent write on a
// different key causes a runtime panic. The mutex therefore guards every
// map access, not individual entries.
//
// Write methods acquire mu.Lock; read methods acquire mu.RLock.
//
// Iterator methods (ListEntities, ListRelations, Search) snapshot matching
// results into a slice under the read lock, release the lock, then yield
// from the snapshot. This keeps lock duration proportional to the scan,
// not to whatever the caller does in the loop body.
//
// Event emission (emit) is called under mu.Lock. Subscribers receive
// events on buffered channels via non-blocking sends.
package memstore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"iter"
	"maps"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// MemStore is an in-memory store implementation.
//
// TODO(TKT-N0IKN9): the exported surface (28) is the mandated store.Store
// interface (incl. Tx, TKT-GXHI8), which consumers depend on directly by
// design. This is a required-interface exception, not accreted public API —
// it tracks the interface size and ratchets only if store.Store itself is
// narrowed.
//
// Total methods crossed the 40 line with the Tx contract (TKT-GXHI8):
// each store.Store write method split into an exported txMu wrapper +
// unexported core so Tx callbacks can re-enter. Interface-driven, not
// accreted sprawl; ratchets with the interface.
//
// +1 for ListEntityHeaders (TKT-1ESTYJ): the store.HeaderReader capability,
// a content-free ListEntities. Implemented natively rather than left to the
// generic fallback because the fallback would clone each entity — bodies
// included — only to drop the body immediately, which is the cost the
// capability exists to remove. Interface-driven like the Tx split above.
//
//plimsoll:max-methods=43
//plimsoll:max-exported-methods=29
type MemStore struct {
	// txMu serializes an open Tx against ordinary writers: Tx holds it
	// for the whole callback, every exported write method takes it
	// briefly before mu (lock order: txMu → mu; see tx.go). Readers
	// never take it.
	txMu          sync.Mutex
	mu            sync.RWMutex
	entities      map[string]*entity.Entity   // ID -> entity
	entityOrder   []string                    // sorted entity IDs
	relations     map[string]*entity.Relation // key -> relation
	relationOrder []string                    // sorted relation keys
	attachments   map[string]*attachment      // "entityID/property" -> data
	subscribers   map[int]chan store.Event
	nextSubID     int
	observers     []store.EntityObserver // notified synchronously on entity writes
}

type attachment struct {
	entityID string
	property string
	fileName string
	data     []byte
}

// New creates a new in-memory store.
// Option configures a MemStore.
type Option func(*MemStore)

// WithObserver adds an entity observer that is notified on writes.
// A nil observer is dropped silently so callers can pass the result
// of an optional construction (e.g. a search-backend factory that
// returns nil on failure) without an extra nil guard at every call
// site. Matches the [app.FSFactory.AddObserver] contract.
func WithObserver(o store.EntityObserver) Option {
	return func(m *MemStore) {
		if o == nil {
			return
		}
		m.observers = append(m.observers, o)
	}
}

func New(opts ...Option) *MemStore {
	m := &MemStore{
		entities:    make(map[string]*entity.Entity),
		relations:   make(map[string]*entity.Relation),
		attachments: make(map[string]*attachment),
		subscribers: make(map[int]chan store.Event),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// LastModified scans stored entities and relations for the newest UpdatedAt.
func (m *MemStore) LastModified(_ context.Context) (time.Time, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var newest time.Time
	for _, e := range m.entities {
		if e.UpdatedAt.After(newest) {
			newest = e.UpdatedAt
		}
	}
	for _, r := range m.relations {
		if r.UpdatedAt.After(newest) {
			newest = r.UpdatedAt
		}
	}
	return newest, nil
}

func (m *MemStore) notifyPut(e *entity.Entity) {
	for _, o := range m.observers {
		_ = o.EntityPut(e)
	}
}

func (m *MemStore) notifyDelete(id string) {
	for _, o := range m.observers {
		_ = o.EntityDelete(id)
	}
}

// notifyRenamed fans out a rename to all observers. The rename code
// path emits this INSTEAD OF the EntityDelete(oldID)+EntityPut(renamed)
// pair — see store.EntityObserver.EntityRenamed.
func (m *MemStore) notifyRenamed(oldID string, renamed *entity.Entity) {
	for _, o := range m.observers {
		_ = o.EntityRenamed(oldID, renamed)
	}
}

// compile-time interface check
var _ store.Store = (*MemStore)(nil)

// Delegate validation and sorted-slice helpers to storeutil.
var (
	validateID       = storeutil.ValidateID
	validateProperty = storeutil.ValidateProperty
	sortedInsert     = storeutil.SortedInsert
	sortedRemove     = storeutil.SortedRemove
)

// idTaken reports whether any key of index case-folds to the same identity as
// id. except is skipped so a rename can ask "does any OTHER entity claim this
// identity?" without self-colliding.
//
// Derived from the entities map on each call rather than kept as a second
// index: entity IDs are written at five sites here, and a parallel map would
// silently drift out of sync at whichever one a future change forgets. The
// scan is O(n) but runs only on create and rename, never on a read path.
//
// Callers must hold m.mu.
// A free function rather than a method — MemStore is at its plimsoll
// max-methods line, and this needs no receiver state beyond the index.
func idTaken(index map[string]*entity.Entity, id, except string) bool {
	folded := storeutil.FoldID(id)
	for existing := range index {
		if existing == except {
			continue
		}
		if storeutil.FoldID(existing) == folded {
			return true
		}
	}
	return false
}

// --- EntityReader ---

func (m *MemStore) GetEntity(_ context.Context, id string) (*entity.Entity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	e, ok := m.entities[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return e.Clone(), nil
}

func (m *MemStore) ListEntities(_ context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	m.mu.RLock()
	idSet := entityIDSet(q.IDs)

	snapshot := make([]*entity.Entity, 0)
	for _, id := range m.entityOrder {
		e := m.entities[id]
		if !matchEntityQuery(e, q, idSet) {
			continue
		}
		snapshot = append(snapshot, e.Clone())
	}
	m.mu.RUnlock()

	return func(yield func(*entity.Entity, error) bool) {
		for _, e := range snapshot {
			if !yield(e, nil) {
				return
			}
		}
	}
}

// ListEntityHeaders implements store.HeaderReader. Like ListEntities it
// snapshots under the read lock (so the iterator never holds it), but copies
// only the header fields — the body string is not carried into the snapshot,
// and Properties are cloned so a caller cannot mutate stored state through a
// header.
//
// The generic fallback would clone each ENTITY (bodies included) just to
// discard the content a moment later; this keeps a whole-store scan
// proportional to ids and properties rather than to markdown.
func (m *MemStore) ListEntityHeaders(
	_ context.Context, q store.EntityQuery,
) iter.Seq2[store.EntityHeader, error] {
	m.mu.RLock()
	idSet := entityIDSet(q.IDs)

	snapshot := make([]store.EntityHeader, 0)
	for _, id := range m.entityOrder {
		e := m.entities[id]
		if !matchEntityQuery(e, q, idSet) {
			continue
		}
		snapshot = append(snapshot, store.EntityHeader{
			ID:         e.ID,
			Type:       e.Type,
			Properties: maps.Clone(e.Properties),
			UpdatedAt:  e.UpdatedAt,
		})
	}
	m.mu.RUnlock()

	return func(yield func(store.EntityHeader, error) bool) {
		for _, h := range snapshot {
			if !yield(h, nil) {
				return
			}
		}
	}
}

func (m *MemStore) ListEntitiesPage(_ context.Context, q store.EntityQuery) (store.Page[*entity.Entity], error) {
	cursorKey, err := storeutil.DecodeCursor(q.Cursor)
	if err != nil {
		return store.Page[*entity.Entity]{}, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	idSet := entityIDSet(q.IDs)
	keys := storeutil.PaginateSortedKeys(m.entityOrder, cursorKey, q.Limit, func(id string) bool {
		return matchEntityQuery(m.entities[id], q, idSet)
	})

	items := make([]*entity.Entity, 0, len(keys.Keys))
	for _, id := range keys.Keys {
		if e, ok := m.entities[id]; ok {
			items = append(items, e.Clone())
		}
	}
	return store.Page[*entity.Entity]{Items: items, NextCursor: keys.NextCursor}, nil
}

// entityIDSet builds the ID-lookup set used by ListEntities and its page
// variant. Returns nil when ids is empty so callers can test with len().
func entityIDSet(ids []string) map[string]bool {
	if len(ids) == 0 {
		return nil
	}
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// matchEntityQuery reports whether e satisfies q's Type and IDs filters.
// idSet must be pre-computed from q.IDs (see entityIDSet).
func matchEntityQuery(e *entity.Entity, q store.EntityQuery, idSet map[string]bool) bool {
	if e == nil {
		return false
	}
	if q.Type != "" && e.Type != q.Type {
		return false
	}
	if len(idSet) > 0 && !idSet[e.ID] {
		return false
	}
	return true
}

func (m *MemStore) CountEntities(_ context.Context, q store.EntityQuery) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	idSet := make(map[string]bool, len(q.IDs))
	for _, id := range q.IDs {
		idSet[id] = true
	}

	count := 0
	for _, e := range m.entities {
		if q.Type != "" && e.Type != q.Type {
			continue
		}
		if len(idSet) > 0 && !idSet[e.ID] {
			continue
		}
		count++
	}
	return count, nil
}

func (m *MemStore) HighestID(_ context.Context, prefix string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	highest := 0
	pfx := prefix + "-"
	for id := range m.entities {
		if !strings.HasPrefix(id, pfx) {
			continue
		}
		suffix := id[len(pfx):]
		var n int
		if _, err := fmt.Sscanf(suffix, "%d", &n); err == nil && n > highest {
			highest = n
		}
	}
	return highest, nil
}

func (m *MemStore) PropertyValues(_ context.Context, property string, limit int) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	counts := make(map[string]int)
	for _, e := range m.entities {
		if v, ok := e.Properties[property]; ok {
			s := fmt.Sprintf("%v", v)
			if s != "" {
				counts[s]++
			}
		}
	}

	type vc struct {
		value string
		count int
	}
	sorted := make([]vc, 0, len(counts))
	for v, c := range counts {
		sorted = append(sorted, vc{v, c})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].count != sorted[j].count {
			return sorted[i].count > sorted[j].count
		}
		return sorted[i].value < sorted[j].value
	})

	result := make([]string, 0, limit)
	for i := 0; i < len(sorted) && (limit == 0 || i < limit); i++ {
		result = append(result, sorted[i].value)
	}
	return result, nil
}

// --- EntityWriter ---

func (m *MemStore) createEntity(_ context.Context, e *entity.Entity) error {
	if err := validateID(e.ID); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Case-folded so "ABC" conflicts with an existing "abc" — on fsstore they
	// are one file, and the backends must agree on identity (BUG-3RCWNS).
	if idTaken(m.entities, e.ID, "") {
		return store.ErrConflict
	}

	stored := e.Clone()
	stored.Redacted = nil // per-reader ACL artifact, never content (RR-KBWJPV)
	stored.UpdatedAt = time.Now()
	m.entities[e.ID] = stored
	m.entityOrder = sortedInsert(m.entityOrder, e.ID)
	m.notifyPut(stored)

	m.emit(store.Event{
		Op:         store.EventEntityCreated,
		EntityType: e.Type,
		EntityID:   e.ID,
	})
	return nil
}

func (m *MemStore) updateEntity(_ context.Context, e *entity.Entity) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.entities[e.ID]; !exists {
		return store.ErrNotFound
	}

	stored := e.Clone()
	stored.Redacted = nil // per-reader ACL artifact, never content (RR-KBWJPV)
	stored.UpdatedAt = time.Now()
	m.entities[e.ID] = stored
	m.notifyPut(stored)

	m.emit(store.Event{
		Op:         store.EventEntityUpdated,
		EntityType: e.Type,
		EntityID:   e.ID,
	})
	return nil
}

func (m *MemStore) deleteEntity(_ context.Context, id string, cascade bool) (*store.DeleteResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	e, ok := m.entities[id]
	if !ok {
		return nil, store.ErrNotFound
	}

	// Find related relations
	var related []*entity.Relation
	for _, r := range m.relations {
		if r.From == id || r.To == id {
			related = append(related, r)
		}
	}

	if !cascade && len(related) > 0 {
		return nil, fmt.Errorf("%w: entity %s has %d relation(s)", store.ErrHasRelations, id, len(related))
	}

	result := &store.DeleteResult{
		DeletedEntities: []*entity.Entity{e.Clone()},
	}

	delete(m.entities, id)
	m.entityOrder = sortedRemove(m.entityOrder, id)
	m.notifyDelete(id)

	// Cascade attachments — 1:1 ownership.
	for key, a := range m.attachments {
		if a.entityID == id {
			delete(m.attachments, key)
		}
	}

	for _, r := range related {
		result.DeletedRelations = append(result.DeletedRelations, r.Clone())
		key := r.Key()
		delete(m.relations, key)
		m.relationOrder = sortedRemove(m.relationOrder, key)
	}

	m.emit(store.Event{
		Op:         store.EventEntityDeleted,
		EntityType: e.Type,
		EntityID:   id,
	})
	for _, r := range related {
		m.emit(store.Event{
			Op:           store.EventRelationDeleted,
			RelationType: r.Type,
			From:         r.From,
			To:           r.To,
		})
	}

	return result, nil
}

func (m *MemStore) renameEntity(_ context.Context, oldID, newID string) (*store.RenameResult, error) {
	if err := validateID(newID); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	e, ok := m.entities[oldID]
	if !ok {
		return nil, store.ErrNotFound
	}
	// except=oldID: renaming an entity to a different casing of its own ID
	// (abc -> ABC) is legitimate and must not self-collide.
	if idTaken(m.entities, newID, oldID) {
		return nil, store.ErrConflict
	}

	// Update entity — clone to avoid mutating stored object
	renamed := e.Clone()
	renamed.ID = newID
	renamed.UpdatedAt = time.Now()
	m.entities[newID] = renamed
	delete(m.entities, oldID)
	m.entityOrder = sortedRemove(m.entityOrder, oldID)
	m.entityOrder = sortedInsert(m.entityOrder, newID)
	m.notifyRenamed(oldID, renamed)

	// Update relations — clone each affected relation
	relationsUpdated := 0
	toRemove := make([]string, 0)
	toAdd := make([]*entity.Relation, 0)
	for key, r := range m.relations {
		if r.From != oldID && r.To != oldID {
			continue
		}
		clone := r.Clone()
		if clone.From == oldID {
			clone.From = newID
		}
		if clone.To == oldID {
			clone.To = newID
		}
		toRemove = append(toRemove, key)
		toAdd = append(toAdd, clone)
		relationsUpdated++
	}
	for _, key := range toRemove {
		delete(m.relations, key)
		m.relationOrder = sortedRemove(m.relationOrder, key)
	}
	for _, r := range toAdd {
		newKey := r.Key()
		m.relations[newKey] = r
		m.relationOrder = sortedInsert(m.relationOrder, newKey)
	}

	// Re-key attachments owned by the renamed entity.
	reKey := make(map[string]*attachment)
	for key, a := range m.attachments {
		if a.entityID != oldID {
			continue
		}
		delete(m.attachments, key)
		a.entityID = newID
		reKey[attachmentKey(newID, a.property, a.fileName)] = a
	}
	maps.Copy(m.attachments, reKey)

	m.emit(store.Event{
		Op:         store.EventEntityUpdated,
		EntityType: renamed.Type,
		EntityID:   newID,
	})

	return &store.RenameResult{RelationsUpdated: relationsUpdated}, nil
}

// --- RelationReader ---

func (m *MemStore) GetRelation(_ context.Context, from, relType, to string) (*entity.Relation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := from + "--" + relType + "--" + to
	r, ok := m.relations[key]
	if !ok {
		return nil, store.ErrNotFound
	}
	return r.Clone(), nil
}

func (m *MemStore) ListRelations(_ context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error] {
	m.mu.RLock()
	snapshot := make([]*entity.Relation, 0)
	for _, key := range m.relationOrder {
		r := m.relations[key]
		if !matchRelation(r, q) {
			continue
		}
		snapshot = append(snapshot, r.Clone())
	}
	m.mu.RUnlock()

	return func(yield func(*entity.Relation, error) bool) {
		for _, r := range snapshot {
			if !yield(r, nil) {
				return
			}
		}
	}
}

func (m *MemStore) ListRelationsPage(_ context.Context, q store.RelationQuery) (store.Page[*entity.Relation], error) {
	cursorKey, err := storeutil.DecodeCursor(q.Cursor)
	if err != nil {
		return store.Page[*entity.Relation]{}, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := storeutil.PaginateSortedKeys(m.relationOrder, cursorKey, q.Limit, func(key string) bool {
		return matchRelation(m.relations[key], q)
	})

	items := make([]*entity.Relation, 0, len(keys.Keys))
	for _, key := range keys.Keys {
		if r, ok := m.relations[key]; ok {
			items = append(items, r.Clone())
		}
	}
	return store.Page[*entity.Relation]{Items: items, NextCursor: keys.NextCursor}, nil
}

var matchRelation = storeutil.MatchRelation

func (m *MemStore) CountRelations(_ context.Context, q store.RelationQuery) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, key := range m.relationOrder {
		if matchRelation(m.relations[key], q) {
			count++
		}
	}
	return count, nil
}

// --- RelationWriter ---

func (m *MemStore) createRelation(
	_ context.Context, from, relType, to string, data *store.RelationData,
) (*entity.Relation, error) {
	for _, id := range []string{from, to} {
		if err := validateID(id); err != nil {
			return nil, err
		}
	}
	if err := storeutil.ValidateRelationType(relType); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	r := entity.NewRelation(from, relType, to)
	r.UpdatedAt = time.Now()
	if data != nil {
		r.Content = data.Content
		if data.Properties != nil {
			r.Properties = make(map[string]any, len(data.Properties))
			for k, v := range data.Properties {
				r.Properties[k] = entity.CloneValue(v)
			}
		}
	}

	key := r.Key()
	if _, exists := m.relations[key]; exists {
		return nil, store.ErrConflict
	}

	m.relations[key] = r
	m.relationOrder = sortedInsert(m.relationOrder, key)

	m.emit(store.Event{
		Op:           store.EventRelationCreated,
		RelationType: relType,
		From:         from,
		To:           to,
	})
	return r.Clone(), nil
}

func (m *MemStore) updateRelation(
	_ context.Context, from, relType, to string, data store.RelationData,
) (*entity.Relation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := from + "--" + relType + "--" + to
	r, ok := m.relations[key]
	if !ok {
		return nil, store.ErrNotFound
	}

	updated := r.Clone()
	updated.Content = data.Content
	if data.Properties != nil {
		updated.Properties = make(map[string]any, len(data.Properties))
		for k, v := range data.Properties {
			updated.Properties[k] = entity.CloneValue(v)
		}
	} else {
		updated.Properties = nil
	}
	updated.UpdatedAt = time.Now()
	m.relations[key] = updated

	m.emit(store.Event{
		Op:           store.EventRelationUpdated,
		RelationType: relType,
		From:         from,
		To:           to,
	})
	return updated.Clone(), nil
}

func (m *MemStore) deleteRelation(_ context.Context, from, relType, to string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := from + "--" + relType + "--" + to
	if _, ok := m.relations[key]; !ok {
		return store.ErrNotFound
	}
	delete(m.relations, key)
	m.relationOrder = sortedRemove(m.relationOrder, key)

	m.emit(store.Event{
		Op:           store.EventRelationDeleted,
		RelationType: relType,
		From:         from,
		To:           to,
	})
	return nil
}

// --- Attachments ---

// attachmentKey is the per-file index key: (entityID, property, fileName).
func attachmentKey(entityID, property, fileName string) string {
	return entityID + "/" + property + "/" + fileName
}

func (m *MemStore) attachFile(_ context.Context, entityID, property, fileName string, r io.Reader) error {
	if err := validateProperty(property); err != nil {
		return err
	}
	if err := store.ValidateFileName(fileName); err != nil {
		return err
	}

	// Backstop size guard (shared with fsstore): reject oversize bytes so
	// the in-memory backend is never unbounded. The API layer also caps
	// at ingress.
	data, err := io.ReadAll(storeutil.LimitAttachmentReader(r))
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.entities[entityID]; !ok {
		return store.ErrNotFound
	}

	// Append: key by (entityID, property, fileName). A same-name re-attach
	// replaces only that one file; sibling files on the property are
	// untouched. Cap/suffix policy lives in the write path, not here.
	m.attachments[attachmentKey(entityID, property, fileName)] = &attachment{
		entityID: entityID,
		property: property,
		fileName: fileName,
		data:     data,
	}
	return nil
}

func (m *MemStore) ReadAttachment(_ context.Context, entityID, property, fileName string) (io.ReadCloser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	a, ok := m.attachments[attachmentKey(entityID, property, fileName)]
	if !ok {
		return nil, store.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(a.data)), nil
}

func (m *MemStore) deleteAttachment(_ context.Context, entityID, property, fileName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := attachmentKey(entityID, property, fileName)
	if _, ok := m.attachments[key]; !ok {
		return store.ErrNotFound
	}
	delete(m.attachments, key)
	return nil
}

func (m *MemStore) ListAttachments(_ context.Context, entityID string) ([]store.AttachmentInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.entities[entityID]; !ok {
		return nil, store.ErrNotFound
	}

	var result []store.AttachmentInfo
	for _, a := range m.attachments {
		if a.entityID == entityID {
			result = append(result, store.AttachmentInfo{
				EntityID: a.entityID,
				Property: a.property,
				FileName: a.fileName,
				Size:     int64(len(a.data)),
			})
		}
	}
	return result, nil
}

// --- Watcher ---

func (m *MemStore) Subscribe(bufSize int) (events <-chan store.Event, cancel func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan store.Event, bufSize)
	id := m.nextSubID
	m.nextSubID++
	m.subscribers[id] = ch

	cancel = func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if _, ok := m.subscribers[id]; ok {
			delete(m.subscribers, id)
			close(ch)
		}
	}
	return ch, cancel
}

// emit sends an event to all subscribers. Non-blocking: drops if full.
// Must be called under mu.Lock.
func (m *MemStore) emit(ev store.Event) {
	for _, ch := range m.subscribers {
		select {
		case ch <- ev:
		default:
			// drop — subscriber is slow
		}
	}
}

// --- Lifecycle ---

func (m *MemStore) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, ch := range m.subscribers {
		close(ch)
		delete(m.subscribers, id)
	}
	return nil
}
