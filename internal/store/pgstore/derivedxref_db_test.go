//go:build postgres

package pgstore_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

var xrefSpec = []store.DerivedObjectSpec{
	{Kind: store.DerivedExternalRefUnique, Type: "ticket", Property: "basecamp"},
}

func refTicket(id, extID string) *entity.Entity {
	return &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{
		"basecamp": map[string]any{"id": extID, "url": "https://x.test/" + extID},
	}}
}

func newXrefStore(t *testing.T) *pgstore.Store {
	t.Helper()
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	out, err := s.Reconcile(context.Background(), xrefSpec, store.ReconcileOptions{})
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, store.DerivedCreated, out[0].State)
	s.SetUniqueSpecProvider(xrefSpec)
	return s
}

// AC5: a duplicate (system, id) within a type is refused by the index and
// attributed to the property.
func TestDerivedXref_DuplicateRejected(t *testing.T) {
	s := newXrefStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateEntity(ctx, refTicket("T-1", "42")))

	err := s.CreateEntity(ctx, refTicket("T-2", "42"))
	var up store.UniquePropertyError
	require.ErrorAs(t, err, &up)
	require.Equal(t, "basecamp", up.Property)
	require.ErrorIs(t, err, store.ErrConflict)

	require.NoError(t, s.CreateEntity(ctx, refTicket("T-3", "43")))
	// A non-string id is not indexed (the write path refuses it anyway).
	num := &entity.Entity{ID: "T-4", Type: "ticket", Properties: map[string]any{"basecamp": map[string]any{"id": 42}}}
	require.NoError(t, s.CreateEntity(ctx, num))
}

func TestDerivedXref_FacesOfOneEntityMayShare(t *testing.T) {
	s := newXrefStore(t)
	ctx := context.Background()
	draft, err := entity.ParseFace("draft")
	require.NoError(t, err)
	require.NoError(t, s.CreateEntity(ctx, refTicket("T-1", "42")))
	faced := refTicket("T-1", "42")
	faced.Face = draft
	require.NoError(t, s.CreateEntity(ctx, faced))
}

func TestDerivedXref_ConcurrentInsertsSingleRow(t *testing.T) {
	s := newXrefStore(t)
	ctx := context.Background()
	const n = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok, conflicts := 0, 0
	for i := range n {
		wg.Go(func() {
			err := s.CreateEntity(ctx, refTicket(fmt.Sprintf("T-%d", i), "race"))
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
			case errors.Is(err, store.ErrConflict):
				conflicts++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
	wg.Wait()
	require.Equal(t, 1, ok)
	require.Equal(t, n-1, conflicts)
}

func TestDerivedXref_PreexistingDuplicatesDegrade(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, s.CreateEntity(ctx, refTicket("T-1", "42")))
	require.NoError(t, s.CreateEntity(ctx, refTicket("T-2", "42")))

	out, err := s.Reconcile(ctx, xrefSpec, store.ReconcileOptions{DryRun: true})
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, store.DerivedUnenforced, out[0].State)
	require.Equal(t, 1, out[0].BlockingCount)

	out, err = s.Reconcile(ctx, xrefSpec, store.ReconcileOptions{})
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, store.DerivedUnenforced, out[0].State)
	require.Equal(t, 1, out[0].BlockingCount)

	// Dropping the declaration drops nothing it did not create, and an
	// empty desired set reports no external-ref object.
	out, err = s.Reconcile(ctx, nil, store.ReconcileOptions{})
	require.NoError(t, err)
	require.Empty(t, out)
}

// AC6: a PropKeyEqual lookup is an index scan on the derived index.
func TestGraphQueryExplainKeyEqualUsesXrefIndex(t *testing.T) {
	const n = 5000
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	ctx := context.Background()
	_, err = s.Reconcile(ctx, xrefSpec, store.ReconcileOptions{})
	require.NoError(t, err)
	for i := range n {
		require.NoError(t, s.CreateEntity(ctx, refTicket(fmt.Sprintf("T-%06d", i), strconv.Itoa(7000000+i))))
	}
	_, err = pool.Exec(ctx, "ANALYZE entities")
	require.NoError(t, err)

	plan := explainGraphQuery(t, pool, store.GraphQuery{
		EntityType: "ticket",
		Props:      []store.PropPredicate{{Property: "basecamp", Op: store.PropKeyEqual, Key: "id", Value: "7000123"}},
		Faces:      store.AllFaces(),
	})
	t.Logf("plan:\n%s", plan)
	require.Contains(t, plan, "rela_derived_xref__", "lookup must use the derived index")
}
