package schema

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// StoreCounter adapts a store.Store to the TypeCounter interface.
type StoreCounter struct {
	Store store.Store
}

func (sc *StoreCounter) CountByEntityType(entityType string) int {
	n, _ := sc.Store.CountEntities(context.Background(), store.EntityQuery{Type: entityType})
	return n
}

func (sc *StoreCounter) CountByRelationType(relationType string) int {
	n, _ := sc.Store.CountRelations(context.Background(), store.RelationQuery{Type: relationType})
	return n
}
