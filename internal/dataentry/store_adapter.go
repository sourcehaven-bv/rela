package dataentry

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// StoreGraph adapts a store.Store to the EntityGraph interface, allowing
// the data entry app to be backed by any store implementation.
type StoreGraph struct {
	store store.Store
}

// NewStoreGraph creates an EntityGraph adapter backed by a store.Store.
func NewStoreGraph(s store.Store) *StoreGraph {
	return &StoreGraph{store: s}
}

func (sg *StoreGraph) GetNode(id string) (*model.Entity, bool) {
	e, err := sg.store.GetEntity(context.Background(), id)
	if err != nil {
		return nil, false
	}
	return model.EntityFromDomain(e), true
}

func (sg *StoreGraph) NodesByType(entityType string) []*model.Entity {
	var result []*model.Entity
	for e, err := range sg.store.ListEntities(context.Background(), store.EntityQuery{Type: entityType}) {
		if err != nil {
			break
		}
		result = append(result, model.EntityFromDomain(e))
	}
	return result
}

func (sg *StoreGraph) AllNodes() []*model.Entity {
	var result []*model.Entity
	for e, err := range sg.store.ListEntities(context.Background(), store.EntityQuery{}) {
		if err != nil {
			break
		}
		result = append(result, model.EntityFromDomain(e))
	}
	return result
}

func (sg *StoreGraph) AllIDs() []string {
	var ids []string
	for e, err := range sg.store.ListEntities(context.Background(), store.EntityQuery{}) {
		if err != nil {
			break
		}
		ids = append(ids, e.ID)
	}
	return ids
}

func (sg *StoreGraph) AllEdges() []*model.Relation {
	var result []*model.Relation
	for r, err := range sg.store.ListRelations(context.Background(), store.RelationQuery{}) {
		if err != nil {
			break
		}
		result = append(result, model.RelationFromDomain(r))
	}
	return result
}

func (sg *StoreGraph) OutgoingEdges(id string) []*model.Relation {
	var result []*model.Relation
	q := store.RelationQuery{EntityID: id, Direction: store.DirectionOutgoing}
	for r, err := range sg.store.ListRelations(context.Background(), q) {
		if err != nil {
			break
		}
		result = append(result, model.RelationFromDomain(r))
	}
	return result
}

func (sg *StoreGraph) IncomingEdges(id string) []*model.Relation {
	var result []*model.Relation
	q := store.RelationQuery{EntityID: id, Direction: store.DirectionIncoming}
	for r, err := range sg.store.ListRelations(context.Background(), q) {
		if err != nil {
			break
		}
		result = append(result, model.RelationFromDomain(r))
	}
	return result
}

func (sg *StoreGraph) FindOrphans() []*model.Entity {
	// An orphan is an entity with no relations (incoming or outgoing).
	var orphans []*model.Entity
	for e, err := range sg.store.ListEntities(context.Background(), store.EntityQuery{}) {
		if err != nil {
			break
		}
		q := store.RelationQuery{EntityID: e.ID, Direction: store.DirectionBoth}
		count, err := sg.store.CountRelations(context.Background(), q)
		if err != nil {
			continue
		}
		if count == 0 {
			orphans = append(orphans, model.EntityFromDomain(e))
		}
	}
	return orphans
}
