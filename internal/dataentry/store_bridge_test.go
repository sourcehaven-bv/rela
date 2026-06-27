package dataentry

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// sseWaitTimeout bounds how long waitForSSE blocks. Generous enough for the
// cross-process pg path (a LISTEN/NOTIFY round-trip) and trivial for memstore.
const sseWaitTimeout = 5 * time.Second

// waitForEntityChange reads from the broker channel until it finds an
// entity-change event for the given type, or fails after sseWaitTimeout.
// entityType varies across callers (incl. the postgres-tagged bridge
// test, invisible to the default-build linter).
//
//nolint:unparam // entityType varies in the postgres-tagged caller
func waitForEntityChange(t *testing.T, ch <-chan sseEvent, entityType string) sseEvent {
	t.Helper()
	deadline := time.After(sseWaitTimeout)
	for {
		select {
		case ev := <-ch:
			if ev.EntityType == entityType {
				return ev
			}
		case <-deadline:
			t.Fatalf("timed out waiting for entity change %q", entityType)
			return sseEvent{}
		}
	}
}

// TestStoreEventBridgeMapsEntityEvents verifies the store-event -> SSE bridge
// surfaces entity create/update/delete as type-scoped change signals (no id,
// no op — all three collapse to "type T changed", TKT-POT9GQ). Uses a memstore
// so the test is backend-agnostic and needs no DB — the bridge consumes the
// standard store.Watcher every backend delivers through.
func TestStoreEventBridgeMapsEntityEvents(t *testing.T) {
	st := memstore.New()
	a := &App{store: st, broker: newEventBroker()}
	a.startStoreEventBridge()
	t.Cleanup(a.StopWatching)

	ch := a.broker.subscribe()
	defer a.broker.unsubscribe(ch)

	ctx := context.Background()

	// Create, update, delete all surface as an entity change of the type,
	// carrying no id.
	require.NoError(t, st.CreateEntity(ctx, entity.New("TKT-1", "ticket")))
	ev := waitForEntityChange(t, ch, "ticket")
	require.Empty(t, ev.Data, "entity change must carry no pre-rendered data/id")

	upd := entity.New("TKT-1", "ticket")
	upd.SetString("title", "changed")
	require.NoError(t, st.UpdateEntity(ctx, upd))
	waitForEntityChange(t, ch, "ticket")

	_, err := st.DeleteEntity(ctx, "TKT-1", false)
	require.NoError(t, err)
	waitForEntityChange(t, ch, "ticket")
}

// TestStoreEventBridgeRelationSignalsVerdictInvalidation verifies relation
// writes produce a RelationChange marker (which clears cached per-connection
// read verdicts, RR-K2WKEJ) but NEVER a wire-bound entity change — relations
// are still not part of the browser live feed.
func TestStoreEventBridgeRelationSignalsVerdictInvalidation(t *testing.T) {
	st := memstore.New()
	a := &App{store: st, broker: newEventBroker()}
	a.startStoreEventBridge()
	t.Cleanup(a.StopWatching)

	ch := a.broker.subscribe()
	defer a.broker.unsubscribe(ch)

	ctx := context.Background()
	require.NoError(t, st.CreateEntity(ctx, entity.New("A", "ticket")))
	require.NoError(t, st.CreateEntity(ctx, entity.New("B", "ticket")))
	waitForEntityChange(t, ch, "ticket")
	waitForEntityChange(t, ch, "ticket")

	_, err := st.CreateRelation(ctx, "A", "depends_on", "B", nil)
	require.NoError(t, err)

	// A relation write produces a RelationChange marker, never an
	// EntityType wire frame.
	deadline := time.After(sseWaitTimeout)
	for {
		select {
		case ev := <-ch:
			require.True(t, ev.RelationChange, "relation write must produce a RelationChange marker, got %+v", ev)
			require.Empty(t, ev.EntityType, "relation change must not carry an entity type")
			return
		case <-deadline:
			t.Fatal("timed out waiting for the relation-change marker")
		}
	}
}

// TestStoreEventBridgeLocalWriteSingleBroadcast guards the de-dup: a single
// entity write produces exactly ONE broker event (the bridge is the sole
// source now that the inline handler broadcasts were removed).
func TestStoreEventBridgeLocalWriteSingleBroadcast(t *testing.T) {
	st := memstore.New()
	a := &App{store: st, broker: newEventBroker()}
	a.startStoreEventBridge()
	t.Cleanup(a.StopWatching)

	ch := a.broker.subscribe()
	defer a.broker.unsubscribe(ch)

	require.NoError(t, st.CreateEntity(context.Background(), entity.New("TKT-1", "ticket")))

	count := 0
	deadline := time.After(800 * time.Millisecond)
	for {
		select {
		case ev := <-ch:
			if ev.EntityType == "ticket" {
				count++
			}
		case <-deadline:
			require.Equal(t, 1, count, "a local write must broadcast exactly once")
			return
		}
	}
}

// compile-time guard that memstore satisfies the store.Store the bridge needs.
var _ store.Store = (*memstore.MemStore)(nil)
