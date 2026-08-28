package schema

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TypeCounts is the counting capability [NewStoreCounter] adapts. Declared at
// this call site rather than taking `store.Store`, so a caller may supply an
// ACL-scoped reader; `store.Store` satisfies it structurally.
type TypeCounts interface {
	CountEntities(ctx context.Context, q store.EntityQuery) (int, error)
	CountRelations(ctx context.Context, q store.RelationQuery) (int, error)
}

// storeCounter adapts a counting store to the [TypeCounter] interface.
//
// TypeCounter's methods take no context — it is a pure metamodel-usage report,
// and threading a ctx through it would churn every implementation for one
// caller. The request context is therefore captured in these closures at
// construction. It is unexported and closure-based on purpose: a struct with
// an exported Ctx field invites a composite literal that forgets to set it,
// which for an ACL-scoped counter means silently counting the WHOLE graph
// instead of the principal's slice — a leak that still compiles and still
// returns plausible numbers.
type storeCounter struct {
	countEntities  func(entityType string) int
	countRelations func(relationType string) int
}

// NewStoreCounter builds a [TypeCounter] whose counts run on ctx.
func NewStoreCounter(ctx context.Context, st TypeCounts) TypeCounter {
	return &storeCounter{
		countEntities: func(entityType string) int {
			n, _ := st.CountEntities(ctx, store.EntityQuery{Type: entityType})
			return n
		},
		countRelations: func(relationType string) int {
			n, _ := st.CountRelations(ctx, store.RelationQuery{Type: relationType})
			return n
		},
	}
}

func (sc *storeCounter) CountByEntityType(entityType string) int {
	return sc.countEntities(entityType)
}

func (sc *storeCounter) CountByRelationType(relationType string) int {
	return sc.countRelations(relationType)
}
