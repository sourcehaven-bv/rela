//go:build postgres

package pgstore_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// personSpec is the (type, property) unique rule used across these tests.
var personSpec = []store.DerivedObjectSpec{
	{Kind: store.DerivedUnique, Type: "person", Property: "email"},
}

// newReconciledStore builds a store on a fresh migrated schema, reconciles the
// person.email unique rule, publishes the specs, and returns the store. It
// asserts the index was created (no pre-existing dupes).
func newReconciledStore(t *testing.T) *pgstore.Store {
	t.Helper()
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	outcomes, err := s.Reconcile(context.Background(), personSpec, store.ReconcileOptions{})
	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	require.Equal(t, store.DerivedCreated, outcomes[0].State)
	s.SetUniqueSpecProvider(personSpec)
	return s
}

func person(id, email string) *entity.Entity {
	return &entity.Entity{ID: id, Type: "person", Properties: map[string]any{"email": email}}
}

// AC1 + AC2: a second insert of the same unique value fails atomically with a
// property-named UniquePropertyError, and only one row survives.
func TestDerivedUnique_DuplicateInsertRejected(t *testing.T) {
	s := newReconciledStore(t)
	ctx := context.Background()

	require.NoError(t, s.CreateEntity(ctx, person("P-1", "a@x.com")))

	err := s.CreateEntity(ctx, person("P-2", "a@x.com"))
	var up store.UniquePropertyError
	require.ErrorAs(t, err, &up, "want UniquePropertyError, got %v", err)
	require.Equal(t, "email", up.Property)
	// It is also a conflict for the generic callers.
	require.ErrorIs(t, err, store.ErrConflict)

	// Exactly one row.
	got, err := s.GetEntity(ctx, "P-1")
	require.NoError(t, err)
	require.Equal(t, "a@x.com", got.GetString("email"))
	_, err = s.GetEntity(ctx, "P-2")
	require.ErrorIs(t, err, store.ErrNotFound)
}

// AC2 regression: an entity-ID collision is still a plain ErrConflict
// (ErrEntityAlreadyExists at the manager layer), NOT UniquePropertyError.
func TestDerivedUnique_IDCollisionStillPlainConflict(t *testing.T) {
	s := newReconciledStore(t)
	ctx := context.Background()

	require.NoError(t, s.CreateEntity(ctx, person("P-1", "a@x.com")))
	err := s.CreateEntity(ctx, person("P-1", "b@x.com")) // same ID, different email
	require.ErrorIs(t, err, store.ErrConflict)
	var up store.UniquePropertyError
	require.NotErrorAs(t, err, &up, "ID collision must NOT be UniquePropertyError")
}

// AC1: concurrent inserts of the same value yield exactly one row.
func TestDerivedUnique_ConcurrentInsertsSingleRow(t *testing.T) {
	s := newReconciledStore(t)
	ctx := context.Background()

	const n = 8
	var wg sync.WaitGroup
	var okCount, conflictCount int
	var mu sync.Mutex
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := s.CreateEntity(ctx, person("P-"+string(rune('a'+i)), "race@x.com"))
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				okCount++
			case errors.Is(err, store.ErrConflict):
				conflictCount++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}(i)
	}
	wg.Wait()
	require.Equal(t, 1, okCount, "exactly one insert should win")
	require.Equal(t, n-1, conflictCount, "the rest should conflict")
}

// RR-AROZJY: empty and absent values are exempt — the index must NOT collide
// them, matching the application scan's semantics.
func TestDerivedUnique_EmptyAndAbsentExempt(t *testing.T) {
	s := newReconciledStore(t)
	ctx := context.Background()

	// Two entities with an explicit empty email: allowed (both).
	require.NoError(t, s.CreateEntity(ctx, person("E-1", "")))
	require.NoError(t, s.CreateEntity(ctx, person("E-2", "")))

	// Two entities with NO email property at all: allowed (both).
	require.NoError(t, s.CreateEntity(ctx, &entity.Entity{ID: "A-1", Type: "person"}))
	require.NoError(t, s.CreateEntity(ctx, &entity.Entity{ID: "A-2", Type: "person"}))
}

// AC4: reconcile is idempotent (second run: enforced, no error), and dropping
// the declaration drops the owned index.
func TestDerivedUnique_ReconcileIdempotentAndDrop(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	ctx := context.Background()

	// First run creates.
	out, err := s.Reconcile(ctx, personSpec, store.ReconcileOptions{})
	require.NoError(t, err)
	require.Equal(t, store.DerivedCreated, out[0].State)

	// Second run: enforced (no-op).
	out, err = s.Reconcile(ctx, personSpec, store.ReconcileOptions{})
	require.NoError(t, err)
	require.Equal(t, store.DerivedEnforced, out[0].State)

	// Remove the declaration -> the owned index is dropped.
	out, err = s.Reconcile(ctx, nil, store.ReconcileOptions{})
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, store.DerivedDropped, out[0].State)

	// And now duplicates are allowed again (no DB constraint).
	require.NoError(t, s.CreateEntity(ctx, person("P-1", "a@x.com")))
	require.NoError(t, s.CreateEntity(ctx, person("P-2", "a@x.com")))
}

// AC5: pre-existing duplicate values do NOT fail reconcile — the constraint is
// reported unenforced with a blocking count, and the store stays usable.
func TestDerivedUnique_PreexistingDuplicatesDegrade(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	ctx := context.Background()

	// Seed duplicate values BEFORE any index exists.
	require.NoError(t, s.CreateEntity(ctx, person("P-1", "dup@x.com")))
	require.NoError(t, s.CreateEntity(ctx, person("P-2", "dup@x.com")))

	out, err := s.Reconcile(ctx, personSpec, store.ReconcileOptions{})
	require.NoError(t, err, "reconcile must not error on pre-existing duplicates")
	require.Len(t, out, 1)
	require.Equal(t, store.DerivedUnenforced, out[0].State)
	require.Equal(t, 1, out[0].BlockingCount, "one blocking value group")
	require.Empty(t, out[0].SampleValues, "values withheld without ShowValues")

	// With ShowValues, the blocking value is sampled.
	out, err = s.Reconcile(ctx, personSpec, store.ReconcileOptions{DryRun: true, ShowValues: true})
	require.NoError(t, err)
	require.Equal(t, store.DerivedUnenforced, out[0].State)
	require.Contains(t, out[0].SampleValues, "dup@x.com")
}

// AC6: a dry-run reports what WOULD happen without changing anything.
func TestDerivedUnique_DryRunNoMutation(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	ctx := context.Background()

	out, err := s.Reconcile(ctx, personSpec, store.ReconcileOptions{DryRun: true})
	require.NoError(t, err)
	require.Equal(t, store.DerivedCreated, out[0].State)
	require.True(t, out[0].WouldChange)

	// Nothing was created: a live reconcile still sees it as to-create.
	out, err = s.Reconcile(ctx, personSpec, store.ReconcileOptions{})
	require.NoError(t, err)
	require.Equal(t, store.DerivedCreated, out[0].State)
}

// RR-2HMGZJ: the reconciler only sees/drops its OWN schema's indexes. A second
// schema's owned index must survive a reconcile in the first schema.
func TestDerivedUnique_CrossSchemaIsolation(t *testing.T) {
	ctxA := context.Background()

	// Schema A: create the person.email index.
	poolA := newScopedPool(t)
	sA, err := pgstore.New(poolA)
	require.NoError(t, err)
	_, err = sA.Reconcile(ctxA, personSpec, store.ReconcileOptions{})
	require.NoError(t, err)

	// Schema B: create its own index.
	poolB := newScopedPool(t)
	sB, err := pgstore.New(poolB)
	require.NoError(t, err)
	_, err = sB.Reconcile(ctxA, personSpec, store.ReconcileOptions{})
	require.NoError(t, err)

	// Reconcile schema A with an EMPTY desired set: it must drop A's index but
	// leave B's intact.
	_, err = sA.Reconcile(ctxA, nil, store.ReconcileOptions{})
	require.NoError(t, err)

	// B still enforces uniqueness.
	require.NoError(t, sB.CreateEntity(ctxA, person("B-1", "b@x.com")))
	err = sB.CreateEntity(ctxA, person("B-2", "b@x.com"))
	require.ErrorIs(t, err, store.ErrConflict, "schema B's index must survive A's drop")
}

// A dry-run that would DROP an orphan index reports it without mutating.
func TestDerivedUnique_DryRunDrop(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	ctx := context.Background()

	// Create the index for real.
	_, err = s.Reconcile(ctx, personSpec, store.ReconcileOptions{})
	require.NoError(t, err)

	// Dry-run with an empty desired set: reports a would-drop, changes nothing.
	out, err := s.Reconcile(ctx, nil, store.ReconcileOptions{DryRun: true})
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, store.DerivedDropped, out[0].State)
	require.True(t, out[0].WouldChange)

	// The index is still present: a live reconcile still sees it as an orphan
	// to drop (i.e. the dry-run did not actually drop it).
	out, err = s.Reconcile(ctx, nil, store.ReconcileOptions{})
	require.NoError(t, err)
	require.Equal(t, store.DerivedDropped, out[0].State)
}

// A spec whose names are unsafe for DDL is reported unenforced, never
// interpolated (defense-in-depth over the metamodel's load-time validation).
func TestDerivedUnique_UnsafeNameRefused(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)

	bad := []store.DerivedObjectSpec{
		{Kind: store.DerivedUnique, Type: "person", Property: "ev'il"},
	}
	out, err := s.Reconcile(context.Background(), bad, store.ReconcileOptions{})
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, store.DerivedUnenforced, out[0].State)
	require.Contains(t, out[0].Reason, "unsafe")
}

// The blocking-group count is EXACT, not capped, so an operator sees the true
// scale of the dirty data (RR-78T6Q9).
func TestDerivedUnique_BlockingCountExact(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	ctx := context.Background()

	// Seed 3 distinct duplicated values (6 rows), plus a unique one.
	for i, v := range []string{"a@x.com", "b@x.com", "c@x.com"} {
		require.NoError(t, s.CreateEntity(ctx, person("D"+string(rune('a'+i))+"-1", v)))
		require.NoError(t, s.CreateEntity(ctx, person("D"+string(rune('a'+i))+"-2", v)))
	}
	require.NoError(t, s.CreateEntity(ctx, person("U-1", "unique@x.com")))

	out, err := s.Reconcile(ctx, personSpec, store.ReconcileOptions{})
	require.NoError(t, err)
	require.Equal(t, store.DerivedUnenforced, out[0].State)
	require.Equal(t, 3, out[0].BlockingCount, "exactly three blocking value groups")
}
