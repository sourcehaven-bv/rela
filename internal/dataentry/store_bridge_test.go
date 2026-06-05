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

// waitForSSE reads from the broker channel until it finds an event of the given
// type, or fails after sseWaitTimeout. Returns the event.
func waitForSSE(t *testing.T, ch <-chan sseEvent, eventType string) sseEvent {
	t.Helper()
	deadline := time.After(sseWaitTimeout)
	for {
		select {
		case ev := <-ch:
			if ev.Type == eventType {
				return ev
			}
		case <-deadline:
			t.Fatalf("timed out waiting for SSE event %q", eventType)
			return sseEvent{}
		}
	}
}

// TestStoreEventBridgeMapsEntityEvents verifies the store-event -> SSE bridge
// broadcasts entity create/update/delete (and only those) to connected SSE
// clients. Uses a memstore so the test is backend-agnostic and needs no DB —
// the bridge consumes the standard store.Watcher, which every backend (incl.
// pgstore's cross-process feed) delivers through.
func TestStoreEventBridgeMapsEntityEvents(t *testing.T) {
	st := memstore.New()
	a := &App{store: st, broker: newEventBroker()}
	a.startStoreEventBridge()
	t.Cleanup(a.StopWatching)

	ch := a.broker.subscribe()
	defer a.broker.unsubscribe(ch)

	ctx := context.Background()

	// Create -> entity:created
	require.NoError(t, st.CreateEntity(ctx, entity.New("TKT-1", "ticket")))
	ev := waitForSSE(t, ch, "entity:created")
	require.Contains(t, ev.Data, "TKT-1")

	// Update -> entity:updated
	upd := entity.New("TKT-1", "ticket")
	upd.SetString("title", "changed")
	require.NoError(t, st.UpdateEntity(ctx, upd))
	waitForSSE(t, ch, "entity:updated")

	// Delete -> entity:deleted
	_, err := st.DeleteEntity(ctx, "TKT-1", false)
	require.NoError(t, err)
	waitForSSE(t, ch, "entity:deleted")
}

// TestStoreEventBridgeIgnoresRelations verifies relation events are NOT
// broadcast to SSE (matching the prior inline-broadcast behavior — relations
// are not part of the live feed).
func TestStoreEventBridgeIgnoresRelations(t *testing.T) {
	st := memstore.New()
	a := &App{store: st, broker: newEventBroker()}
	a.startStoreEventBridge()
	t.Cleanup(a.StopWatching)

	ch := a.broker.subscribe()
	defer a.broker.unsubscribe(ch)

	ctx := context.Background()
	require.NoError(t, st.CreateEntity(ctx, entity.New("A", "ticket")))
	require.NoError(t, st.CreateEntity(ctx, entity.New("B", "ticket")))
	// Drain the two entity:created events.
	waitForSSE(t, ch, "entity:created")
	waitForSSE(t, ch, "entity:created")

	// A relation write must NOT produce any SSE broadcast.
	_, err := st.CreateRelation(ctx, "A", "depends_on", "B", nil)
	require.NoError(t, err)

	select {
	case ev := <-ch:
		t.Fatalf("relation write produced an SSE broadcast (should be ignored): %+v", ev)
	case <-time.After(500 * time.Millisecond):
		// good — no broadcast for relations
	}
}

// TestStoreEventBridgeLocalWriteSingleBroadcast guards the de-dup: a single
// entity write produces exactly ONE SSE broadcast (the bridge is the sole
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
			if ev.Type == "entity:created" {
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
