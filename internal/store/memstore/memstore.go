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
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// MemStore is an in-memory store implementation.
type MemStore struct {
	mu            sync.RWMutex
	entities      map[string]*entity.Entity   // ID -> entity
	entityOrder   []string                    // sorted entity IDs
	relations     map[string]*entity.Relation // key -> relation
	relationOrder []string                    // sorted relation keys
	attachments   map[string]*attachment      // "entityID/property" -> data
	subscribers   map[int]chan store.Event
	nextSubID     int
}

type attachment struct {
	entityID    string
	property    string
	fileName    string
	contentType string
	data        []byte
}

// New creates a new in-memory store.
func New() *MemStore {
	return &MemStore{
		entities:    make(map[string]*entity.Entity),
		relations:   make(map[string]*entity.Relation),
		attachments: make(map[string]*attachment),
		subscribers: make(map[int]chan store.Event),
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
	idSet := make(map[string]bool, len(q.IDs))
	for _, id := range q.IDs {
		idSet[id] = true
	}

	snapshot := make([]*entity.Entity, 0)
	for _, id := range m.entityOrder {
		e := m.entities[id]
		if q.Type != "" && e.Type != q.Type {
			continue
		}
		if len(idSet) > 0 && !idSet[e.ID] {
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

func (m *MemStore) CreateEntity(_ context.Context, e *entity.Entity) error {
	if err := validateID(e.ID); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.entities[e.ID]; exists {
		return store.ErrConflict
	}

	stored := e.Clone()
	stored.UpdatedAt = time.Now()
	m.entities[e.ID] = stored
	m.entityOrder = sortedInsert(m.entityOrder, e.ID)

	m.emit(store.Event{
		Op:         store.EventEntityCreated,
		EntityType: e.Type,
		EntityID:   e.ID,
	})
	return nil
}

func (m *MemStore) UpdateEntity(_ context.Context, e *entity.Entity) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.entities[e.ID]; !exists {
		return store.ErrNotFound
	}

	stored := e.Clone()
	stored.UpdatedAt = time.Now()
	m.entities[e.ID] = stored

	m.emit(store.Event{
		Op:         store.EventEntityUpdated,
		EntityType: e.Type,
		EntityID:   e.ID,
	})
	return nil
}

func (m *MemStore) DeleteEntity(_ context.Context, id string, cascade bool) (*store.DeleteResult, error) {
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

func (m *MemStore) RenameEntity(_ context.Context, oldID, newID string) (*store.RenameResult, error) {
	if err := validateID(newID); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	e, ok := m.entities[oldID]
	if !ok {
		return nil, store.ErrNotFound
	}
	if _, exists := m.entities[newID]; exists {
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

	// Update relations — clone each affected relation
	relationsUpdated := 0
	var toRemove []string
	var toAdd []*entity.Relation
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

func (m *MemStore) CreateRelation(_ context.Context, from, relType, to string, data *store.RelationData) (*entity.Relation, error) {
	for _, id := range []string{from, to} {
		if err := validateID(id); err != nil {
			return nil, err
		}
	}
	if strings.Contains(relType, "--") {
		return nil, fmt.Errorf("store: relation type %q contains consecutive dashes", relType)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if relType == "" {
		return nil, fmt.Errorf("store: empty relation type")
	}

	r := entity.NewRelation(from, relType, to)
	r.UpdatedAt = time.Now()
	if data != nil {
		r.Content = data.Content
		if data.Properties != nil {
			r.Properties = make(map[string]interface{}, len(data.Properties))
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

func (m *MemStore) UpdateRelation(_ context.Context, from, relType, to string, data store.RelationData) (*entity.Relation, error) {
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
		updated.Properties = make(map[string]interface{}, len(data.Properties))
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

func (m *MemStore) DeleteRelation(_ context.Context, from, relType, to string) error {
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

// --- Search ---

func (m *MemStore) Search(_ context.Context, q store.SearchQuery) iter.Seq2[*entity.Entity, error] {
	m.mu.RLock()
	typeSet := make(map[string]bool, len(q.Types))
	for _, t := range q.Types {
		typeSet[t] = true
	}

	snapshot := make([]*entity.Entity, 0)
	for _, id := range m.entityOrder {
		e := m.entities[id]
		if len(typeSet) > 0 && !typeSet[e.Type] {
			continue
		}
		if !matchFilters(e, q.Filters) {
			continue
		}
		if q.Text != "" && !matchText(e, q.Text) {
			continue
		}
		snapshot = append(snapshot, e.Clone())
	}
	m.mu.RUnlock()

	if q.Limit > 0 && len(snapshot) > q.Limit {
		snapshot = snapshot[:q.Limit]
	}

	return func(yield func(*entity.Entity, error) bool) {
		for _, e := range snapshot {
			if !yield(e, nil) {
				return
			}
		}
	}
}

var (
	matchFilters = storeutil.MatchFilters
	matchText    = storeutil.MatchText
)

// --- Attachments ---

func (m *MemStore) AttachFile(_ context.Context, entityID, property, fileName string, r io.Reader) error {
	if err := validateProperty(property); err != nil {
		return err
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.entities[entityID]; !ok {
		return store.ErrNotFound
	}

	key := entityID + "/" + property
	m.attachments[key] = &attachment{
		entityID: entityID,
		property: property,
		fileName: fileName,
		data:     data,
	}
	return nil
}

func (m *MemStore) ReadAttachment(_ context.Context, entityID, property string) (io.ReadCloser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := entityID + "/" + property
	a, ok := m.attachments[key]
	if !ok {
		return nil, store.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(a.data)), nil
}

func (m *MemStore) DeleteAttachment(_ context.Context, entityID, property string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := entityID + "/" + property
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

func (m *MemStore) Subscribe(bufSize int) (<-chan store.Event, func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan store.Event, bufSize)
	id := m.nextSubID
	m.nextSubID++
	m.subscribers[id] = ch

	cancel := func() {
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
