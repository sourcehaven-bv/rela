package memstore

import (
	"context"
	"sort"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// markedFamily is a family taken out of the live maps by MarkDeleted. The
// entities and relations are held exactly as stored, keyed as they were, so
// Unmark puts them back unchanged.
type markedFamily struct {
	at        time.Time
	by        string
	entities  map[string]*entity.Entity   // state key -> entity
	relations map[string]*entity.Relation // relation key -> relation
}

// markedTaken reports whether a marked family holds id, case-folded like
// idTaken. except skips one family, for a rename that only changes casing.
func markedTaken(marked map[string]*markedFamily, id, except string) bool {
	folded := storeutil.FoldID(id)
	for markedID := range marked {
		if except != "" && storeutil.FoldID(markedID) == storeutil.FoldID(except) {
			continue
		}
		if storeutil.FoldID(markedID) == folded {
			return true
		}
	}
	return false
}

// SoftDelete implements [store.SoftDeleteProvider].
func (m *MemStore) SoftDelete() store.SoftDeleter { return softDeleter{m: m} }

// SoftDelete on the transaction view skips txMu, which the open Tx holds.
func (t txStore) SoftDelete() store.SoftDeleter { return softDeleter{m: t.MemStore, inTx: true} }

// softDeleter implements [store.SoftDeleter] over the live maps. The marked
// families live on MemStore.marked, under mu like everything else.
type softDeleter struct {
	m    *MemStore
	inTx bool
}

func (d softDeleter) lock() func() {
	if d.inTx {
		return func() {}
	}
	return d.m.lockTx()
}

func (d softDeleter) MarkDeleted(_ context.Context, id, by string) (*store.DeleteResult, error) {
	defer d.lock()()
	m := d.m
	m.mu.Lock()
	defer m.mu.Unlock()

	fam := &markedFamily{
		at:        time.Now(),
		by:        by,
		entities:  map[string]*entity.Entity{},
		relations: map[string]*entity.Relation{},
	}
	result := &store.DeleteResult{}
	for key, e := range m.entities {
		if e.ID == id {
			fam.entities[key] = e
		}
	}
	if len(fam.entities) == 0 {
		return nil, store.ErrNotFound
	}
	for key, r := range m.relations {
		if r.From == id || r.To == id {
			fam.relations[key] = r
		}
	}

	faces := sortedEntities(fam.entities)
	for _, e := range faces {
		key := entity.FormatStateRef(e.ID, e.Face)
		delete(m.entities, key)
		m.entityOrder = entityRemove(m.entityOrder, key)
		result.DeletedEntities = append(result.DeletedEntities, e.Clone())
		m.notifyFaceDelete(id, e.Face)
	}
	m.notifyLastFaceDelete(id)
	rels := sortedRelations(fam.relations)
	for _, r := range rels {
		key := r.Key()
		delete(m.relations, key)
		m.relationOrder = sortedRemove(m.relationOrder, key)
		result.DeletedRelations = append(result.DeletedRelations, r.Clone())
	}
	if m.marked == nil {
		m.marked = map[string]*markedFamily{}
	}
	m.marked[id] = fam

	for _, e := range faces {
		m.emit(store.Event{Op: store.EventEntityDeleted, EntityType: e.Type, EntityID: id, Face: e.Face})
	}
	for _, r := range rels {
		m.emit(store.Event{
			Op: store.EventRelationDeleted, RelationType: r.Type, From: r.From, To: r.To, Face: r.FromFace,
		})
	}
	return result, nil
}

func (d softDeleter) Unmark(_ context.Context, id string) (*store.DeleteResult, error) {
	defer d.lock()()
	m := d.m
	m.mu.Lock()
	defer m.mu.Unlock()

	fam, ok := m.marked[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	delete(m.marked, id)

	result := &store.DeleteResult{}
	faces := sortedEntities(fam.entities)
	for _, e := range faces {
		key := entity.FormatStateRef(e.ID, e.Face)
		m.entities[key] = e
		m.entityOrder = entityInsert(m.entityOrder, key)
		result.DeletedEntities = append(result.DeletedEntities, e.Clone())
		m.notifyPut(e)
	}
	var back []*entity.Relation
	for _, r := range sortedRelations(fam.relations) {
		// An edge whose other end is still marked stays hidden and moves to
		// that family, so it comes back when that one does.
		if other, stillMarked := m.marked[otherEnd(r, id)]; stillMarked {
			other.relations[r.Key()] = r
			continue
		}
		key := r.Key()
		m.relations[key] = r
		m.relationOrder = sortedInsert(m.relationOrder, key)
		result.DeletedRelations = append(result.DeletedRelations, r.Clone())
		back = append(back, r)
	}

	for _, e := range faces {
		m.emit(store.Event{Op: store.EventEntityCreated, EntityType: e.Type, EntityID: id, Face: e.Face})
	}
	for _, r := range back {
		m.emit(store.Event{
			Op: store.EventRelationCreated, RelationType: r.Type, From: r.From, To: r.To, Face: r.FromFace,
		})
	}
	return result, nil
}

func (d softDeleter) ListMarked(_ context.Context) ([]store.MarkedEntity, error) {
	m := d.m
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]store.MarkedEntity, 0, len(m.marked))
	for id, fam := range m.marked {
		me := store.MarkedEntity{ID: id, DeletedAt: fam.at, DeletedBy: fam.by}
		for _, e := range sortedEntities(fam.entities) {
			me.Entities = append(me.Entities, e.Clone())
		}
		out = append(out, me)
	}
	sortMarked(out)
	return out, nil
}

func (d softDeleter) PurgeMarked(_ context.Context, id string) (*store.DeleteResult, error) {
	defer d.lock()()
	m := d.m
	m.mu.Lock()
	defer m.mu.Unlock()

	fam, ok := m.marked[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	delete(m.marked, id)

	result := &store.DeleteResult{}
	for _, e := range sortedEntities(fam.entities) {
		result.DeletedEntities = append(result.DeletedEntities, e.Clone())
	}
	for _, r := range sortedRelations(fam.relations) {
		result.DeletedRelations = append(result.DeletedRelations, r.Clone())
	}
	// A hidden edge to id may sit with another marked family; it must not
	// come back pointing at an entity that no longer exists.
	for _, other := range m.marked {
		for key, r := range other.relations {
			if r.From == id || r.To == id {
				result.DeletedRelations = append(result.DeletedRelations, r.Clone())
				delete(other.relations, key)
			}
		}
	}
	for key, a := range m.attachments {
		if a.entityID == id {
			delete(m.attachments, key)
		}
	}
	return result, nil
}

// revealedRelations returns the hidden relations of the entity ctx reveals
// (see [store.WithRevealed]) that satisfy match. Caller holds mu.
func revealedRelations(ctx context.Context, m *MemStore, match func(*entity.Relation) bool) []*entity.Relation {
	id := store.RevealedID(ctx)
	if id == "" {
		return nil
	}
	fam, ok := m.marked[id]
	if !ok {
		return nil
	}
	var out []*entity.Relation
	for _, r := range sortedRelations(fam.relations) {
		if match(r) {
			out = append(out, r.Clone())
		}
	}
	return out
}

// otherEnd returns the endpoint of r that is not id; for a self-loop, id.
func otherEnd(r *entity.Relation, id string) string {
	if r.From == id {
		return r.To
	}
	return r.From
}

func sortedEntities(in map[string]*entity.Entity) []*entity.Entity {
	out := make([]*entity.Entity, 0, len(in))
	for _, e := range in {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Face < out[j].Face })
	return out
}

func sortedRelations(in map[string]*entity.Relation) []*entity.Relation {
	out := make([]*entity.Relation, 0, len(in))
	for _, r := range in {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out
}

func sortMarked(out []store.MarkedEntity) {
	sort.Slice(out, func(i, j int) bool {
		if !out[i].DeletedAt.Equal(out[j].DeletedAt) {
			return out[i].DeletedAt.Before(out[j].DeletedAt)
		}
		return out[i].ID < out[j].ID
	})
}
