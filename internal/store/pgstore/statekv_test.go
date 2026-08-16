package pgstore_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/state/statetest"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestStateKV_Conformance runs the shared state.KV contract against the
// PostgreSQL backend. FSKV runs the identical suite in internal/state, which is
// the point: the two backends are interchangeable only if one suite holds both.
func TestStateKV_Conformance(t *testing.T) {
	statetest.RunAll(t, func(tb testing.TB) state.KV {
		tb.Helper()
		pool := newScopedPool(tb)
		raw, err := pgstore.NewStateKV(pool)
		if err != nil {
			tb.Fatalf("NewStateKV: %v", err)
		}
		// Wrapped exactly as the wiring site does: key validation is the state
		// package's contract, not pgstore's, so the conformance suite must
		// exercise the same composition production uses.
		kv, err := state.NewValidatedKV(raw)
		if err != nil {
			tb.Fatalf("NewValidatedKV: %v", err)
		}
		return kv
	})
}

func TestNewStateKV_RejectsNilHandle(t *testing.T) {
	_, err := pgstore.NewStateKV(nil)
	require.Error(t, err, "a nil handle must fail at construction, not at first use")
}

// TestStateKV_SharedAcrossConnections is the property the whole ticket exists
// for: two independently-constructed KVs against the SAME schema observe each
// other's writes. On FSKV in a multi-process deployment they would not — each
// node has its own .rela/ — which is why an operator's logo upload is currently
// invisible to every other node.
//
// Two separate pools, not two KVs over one pool: sharing a pool would pass even
// if the implementation cached in memory, which is exactly the failure this
// guards against.
func TestStateKV_SharedAcrossConnections(t *testing.T) {
	schema := freshFeedSchema(t)
	ctx := context.Background()

	writer, err := pgstore.NewStateKV(poolForSchema(t, schema))
	require.NoError(t, err)
	reader, err := pgstore.NewStateKV(poolForSchema(t, schema))
	require.NoError(t, err)

	require.NoError(t, writer.Put(ctx, "theme/logo", []byte("PNGDATA")))

	got, err := reader.Get(ctx, "theme/logo")
	require.NoError(t, err, "a second connection must see the first's write")
	require.Equal(t, "PNGDATA", string(got))

	// And a delete propagates the same way.
	require.NoError(t, writer.Delete(ctx, "theme/logo"))
	_, err = reader.Get(ctx, "theme/logo")
	require.Error(t, err, "a second connection must see the first's delete")
}

// TestStateKV_IsolatedAcrossSchemas pins the multi-tenant property: state lives
// in the store's schema, so the same key in two schemas is two values. This is
// what makes schema-per-tenant give per-tenant state for free — without it the
// document render cache would serve one tenant's HTML to another.
func TestStateKV_IsolatedAcrossSchemas(t *testing.T) {
	ctx := context.Background()
	a, err := pgstore.NewStateKV(poolForSchema(t, freshFeedSchema(t)))
	require.NoError(t, err)
	b, err := pgstore.NewStateKV(poolForSchema(t, freshFeedSchema(t)))
	require.NoError(t, err)

	const key = "documents/DOC-1-samehash.html"
	require.NoError(t, a.Put(ctx, key, []byte("tenant-a")))
	require.NoError(t, b.Put(ctx, key, []byte("tenant-b")))

	gotA, err := a.Get(ctx, key)
	require.NoError(t, err)
	gotB, err := b.Get(ctx, key)
	require.NoError(t, err)

	require.Equal(t, "tenant-a", string(gotA))
	require.Equal(t, "tenant-b", string(gotB), "one schema's state must not overwrite another's")
}

// TestStateKV_RejectsOversizeValue pins the cap. A value over the limit is
// refused rather than truncated: a silently truncated cached render would be
// served as though it were a complete document.
func TestStateKV_RejectsOversizeValue(t *testing.T) {
	kv, err := pgstore.NewStateKV(newScopedPool(t))
	require.NoError(t, err)
	oversize := make([]byte, pgstore.MaxStateValueBytesForTest+1)
	err = kv.Put(context.Background(), "big", oversize)
	require.Error(t, err)
	require.Contains(t, err.Error(), "over the")
}
