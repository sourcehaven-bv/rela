package schema

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TypeCounts is the counting capability [NewStoreCounter] adapts. Declared at
// this call site rather than taking `store.Store`, so a caller may supply an
// ACL-scoped reader; `store.Store` satisfies it structurally.
type TypeCounts interface {
	CountEntities(ctx context.Context, q store.EntityQuery) (int, error)
	CountRelations(ctx context.Context, q store.RelationQuery) (int, error)
}

// StoreCounter adapts a counting store to the [TypeCounter] interface.
//
// TypeCounter's methods take no context — it is a pure metamodel-usage report,
// and threading a ctx through it would churn every implementation for one
// caller. The request context is therefore captured in these closures at
// construction. The fields are unexported and closure-based on purpose: a
// composite literal that forgets the context would, for an ACL-scoped
// counter, silently count the WHOLE graph instead of the principal's slice.
//
// TypeCounter's methods cannot return an error, so a failed count reads as
// zero there and is kept for [StoreCounter.Err]. A caller must check Err
// after the report: a zero from a failed count would list a live type as
// unused.
type StoreCounter struct {
	countEntities  func(entityType string) int
	countRelations func(relationType string) int

	mu  sync.Mutex
	err error
}

// NewStoreCounter builds a [StoreCounter] whose counts run on ctx.
//
// The entity count is in entities, not face rows: it reads in families, the
// worlds.Compiled.Families scope, which selects one row per entity whichever
// of its declared faces it stores. So a family stored at three faces counts
// one, and a faced-only type does not read as unused, which a count in a
// declared world could report. An entity stored only at a face its type does
// not declare, or at the implicit face of a faced type, is not counted.
//
// families is required: an unset scope is rejected, because every count
// under it would fail.
func NewStoreCounter(ctx context.Context, st TypeCounts, families store.WorldScope) (*StoreCounter, error) {
	if st == nil {
		return nil, errors.New("schema: NewStoreCounter: nil store")
	}
	if !families.IsSet() {
		return nil, errors.New("schema: NewStoreCounter: families scope is unset (worlds.Compiled.Families)")
	}
	sc := &StoreCounter{}
	sc.countEntities = func(entityType string) int {
		n, err := st.CountEntities(ctx, store.EntityQuery{Type: entityType, Faces: store.InWorld(families)})
		sc.record(err, "entity type", entityType)
		return n
	}
	sc.countRelations = func(relationType string) int {
		n, err := st.CountRelations(ctx, store.RelationQuery{Type: relationType})
		sc.record(err, "relation type", relationType)
		return n
	}
	return sc, nil
}

func (sc *StoreCounter) record(err error, kind, name string) {
	if err == nil {
		return
	}
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if sc.err == nil {
		sc.err = fmt.Errorf("schema: count %s %s: %w", kind, name, err)
	}
}

// Err returns the first count error, or nil when every count succeeded.
func (sc *StoreCounter) Err() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.err
}

// CountByEntityType counts entityType in families.
func (sc *StoreCounter) CountByEntityType(entityType string) int {
	return sc.countEntities(entityType)
}

// CountByRelationType counts relationType.
func (sc *StoreCounter) CountByRelationType(relationType string) int {
	return sc.countRelations(relationType)
}
