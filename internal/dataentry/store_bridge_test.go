package dataentry

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/changefeed"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// sseWaitTimeout bounds how long waitForEntityChange blocks. Generous enough
// for the cross-process pg path (a LISTEN/NOTIFY round-trip) and trivial for
// memstore.
const sseWaitTimeout = 5 * time.Second

// bridgeApp returns an App on st with its change feed started, as
// StartWatching starts it.
func bridgeApp(t *testing.T, st store.Store) *App {
	t.Helper()
	feed, err := changefeed.New()
	require.NoError(t, err)
	a := &App{store: st, broker: newEventBroker(), feed: feed}
	a.live = testLiveHub(a)
	a.startStoreEventBridge()
	t.Cleanup(a.StopWatching)
	return a
}

// waitForEntityChange takes from sub until a batch reports a change to
// entityType, or fails after sseWaitTimeout. It returns that batch.
//
// entityType looks constant to the default-build linter — every caller in this
// file passes "ticket" — but the postgres-tagged bridge test passes "feature".
// Neither `unparam` nor `nolintlint` can see both builds at once: suppressing
// unparam leaves an "unused directive" under -tags postgres, and dropping the
// directive fails unparam on the default build. The parameter is genuinely
// varying, so it is left in place and the suppression is applied to both
// linters, whichever one the current build actually raises.
//
//nolint:unparam,nolintlint // entityType varies in the postgres-tagged caller; see above
func waitForEntityChange(t *testing.T, sub *changefeed.Subscription, entityType string) changefeed.Batch {
	t.Helper()
	deadline := time.After(sseWaitTimeout)
	for {
		select {
		case <-sub.Wake():
			if b := sub.Take(); slices.Contains(b.Types, entityType) {
				return b
			}
		case <-deadline:
			t.Fatalf("timed out waiting for entity change %q", entityType)
			return changefeed.Batch{}
		}
	}
}

// TestStoreEventBridgeMapsEntityEvents verifies the store-event -> change feed
// bridge surfaces entity create/update/delete as type-scoped changes (no id,
// no op — all three collapse to "type T changed", TKT-POT9GQ). Uses a memstore
// so the test is backend-agnostic and needs no DB — the bridge consumes the
// standard store.Watcher every backend delivers through.
func TestStoreEventBridgeMapsEntityEvents(t *testing.T) {
	st := memstore.New()
	a := bridgeApp(t, st)
	sub := a.feed.Subscribe("")
	defer sub.Close()

	ctx := context.Background()
	require.NoError(t, st.CreateEntity(ctx, entity.New("TKT-1", "ticket")))
	waitForEntityChange(t, sub, "ticket")

	upd := entity.New("TKT-1", "ticket")
	upd.SetString("title", "changed")
	require.NoError(t, st.UpdateEntity(ctx, upd))
	waitForEntityChange(t, sub, "ticket")

	_, err := st.DeleteFamily(ctx, "TKT-1", false)
	require.NoError(t, err)
	waitForEntityChange(t, sub, "ticket")
}

// TestStoreEventBridgeRelationSignalsVerdictInvalidation verifies relation
// writes are reported as a relation change (which clears cached
// per-connection read verdicts, RR-K2WKEJ) and never as an entity type —
// relations are still not part of the browser live feed.
func TestStoreEventBridgeRelationSignalsVerdictInvalidation(t *testing.T) {
	st := memstore.New()
	a := bridgeApp(t, st)
	ctx := context.Background()
	require.NoError(t, st.CreateEntity(ctx, entity.New("A", "ticket")))
	require.NoError(t, st.CreateEntity(ctx, entity.New("B", "ticket")))

	sub := a.feed.Subscribe("")
	defer sub.Close()
	_, err := st.CreateRelation(ctx, entity.RelationKey{From: "A", Type: "depends_on", To: "B"}, nil)
	require.NoError(t, err)

	// The creates may still be in flight, so take until the relation shows;
	// no batch may ever report it as an entity type.
	deadline := time.After(sseWaitTimeout)
	for {
		select {
		case <-sub.Wake():
			b := sub.Take()
			require.NotContains(t, b.Types, "depends_on", "relation change must not carry an entity type")
			if b.Relation {
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for the relation change")
		}
	}
}

// TestStoreEventBridgeLocalWriteCountedOnce guards the de-dup: a single
// entity write reaches the feed once (the bridge is the sole source; the
// write handlers do not broadcast).
func TestStoreEventBridgeLocalWriteCountedOnce(t *testing.T) {
	st := memstore.New()
	a := bridgeApp(t, st)
	sub := a.feed.Subscribe("")
	defer sub.Close()

	require.NoError(t, st.CreateEntity(context.Background(), entity.New("TKT-1", "ticket")))
	b := waitForEntityChange(t, sub, "ticket")
	require.Equal(t, sub.Start()+1, b.Seq, "a local write must be counted exactly once")
	time.Sleep(100 * time.Millisecond)
	require.Equal(t, b.Seq, sub.Take().Seq, "no second event may follow the write")
}

// compile-time guard that memstore satisfies the store.Store the bridge needs.
var _ store.Store = (*memstore.MemStore)(nil)
