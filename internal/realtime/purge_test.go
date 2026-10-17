package realtime

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// A delete batch holding a delete type and a detach type erases the
// delete type at once and keeps the other for its retention.
func TestFilingFor_MixedBatch(t *testing.T) {
	h := newSweepHarness(t)
	cfg := h.cfg.Load().Config
	chs := []channels.Channel{{Key: sweepKey("X-1", "purge")}, {Key: sweepKey("X-1", "room")}}
	f := FilingFor(cfg, channels.PolicyDelete, sweepT0, chs, false)
	require.Equal(t, sweepT0.Add(90*24*time.Hour), f.PurgeAfter)
	require.Equal(t, map[string]time.Time{"purge": sweepT0}, f.TypeAfter)

	f = FilingFor(cfg, channels.PolicyDelete, sweepT0, chs[:1], false)
	require.Equal(t, sweepT0, f.PurgeAfter)
	require.Empty(t, f.TypeAfter, "a single type needs no early time")
}

func TestSweep_MixedBatchPurgesEachTypeOnTime(t *testing.T) {
	for _, run := range []struct {
		name string
		run  func(*Sweeper, context.Context) (RetentionReport, error)
	}{
		{"RunPurge", (*Sweeper).RunPurge},
		{"RunRetention", (*Sweeper).RunRetention},
	} {
		t.Run(run.name, func(t *testing.T) {
			ctx := context.Background()
			h := newSweepHarness(t)
			purge, room := sweepKey("DOC-1", "purge"), sweepKey("DOC-1", "room")
			h.post(t, purge, sweepT0.Add(-time.Hour))
			h.post(t, room, sweepT0.Add(-time.Hour))
			h.anchors.drop("DOC-1")
			filedAt := sweepT0.Add(-time.Second)
			_, err := h.store.Tombstone(ctx, "DOC-1", "", filedAt,
				Filer(h.cfg.Load().Config, channels.PolicyDelete, filedAt, false))
			require.NoError(t, err)

			rep, err := run.run(h.sweeper, ctx)
			require.NoError(t, err)
			require.Equal(t, 1, rep.Purged)
			batches, err := h.store.Tombstones(ctx, h.clock.Now())
			require.NoError(t, err)
			require.Len(t, batches, 1)
			require.Len(t, batches[0].Channels, 1)
			require.Equal(t, "room", batches[0].Channels[0].Key.Type)
			require.Empty(t, batches[0].Filing.TypeAfter)

			rep, err = run.run(h.sweeper, ctx)
			require.NoError(t, err)
			require.Zero(t, rep.Purged, "the room type waits for its retention")

			h.clock.Add(90 * 24 * time.Hour)
			rep, err = run.run(h.sweeper, ctx)
			require.NoError(t, err)
			require.Equal(t, 1, rep.Purged)
			batches, err = h.store.Tombstones(ctx, h.clock.Now())
			require.NoError(t, err)
			require.Empty(t, batches)
		})
	}
}

// A pending batch that cannot go back because its anchor has live
// channels again is confirmed and erased when due, not left forever.
func TestSweep_OccupiedPendingIsConfirmed(t *testing.T) {
	ctx := context.Background()
	h := newSweepHarness(t)
	k := sweepKey("DOC-1", "purge")
	h.post(t, k, sweepT0.Add(-time.Hour))
	filedAt := sweepT0.Add(-time.Second)
	_, err := h.store.Tombstone(ctx, "DOC-1", "", filedAt,
		Filer(h.cfg.Load().Config, channels.PolicyDelete, filedAt, true))
	require.NoError(t, err)
	h.post(t, k, sweepT0) // the delete failed and someone posted again

	h.clock.Add(realtimelimits.OrphanGrace)
	rep, err := h.sweeper.RunRetention(ctx)
	require.NoError(t, err)
	require.Zero(t, rep.Restored)
	require.Equal(t, 1, rep.Confirmed)
	require.Equal(t, 1, rep.Purged)
	batches, err := h.store.Tombstones(ctx, h.clock.Now())
	require.NoError(t, err)
	require.Empty(t, batches)
	require.Equal(t, channels.StateOpen, h.head(t, k).State, "the live channel stays")
}

// An orphan batch made final by a new entity reusing the id is not put
// back onto it; the sweep confirms it and erases it when due.
func TestSweep_FinalOrphanIsNotRestored(t *testing.T) {
	ctx := context.Background()
	h := newSweepHarness(t)
	k := sweepKey("TKT-1", "room")
	h.post(t, k, sweepT0)
	h.anchors.drop("TKT-1")
	_, err := h.sweeper.RunRetention(ctx)
	require.NoError(t, err)
	h.clock.Add(realtimelimits.OrphanGrace)
	rep, err := h.sweeper.RunRetention(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, rep.Detached)

	// A new entity is created with the id.
	require.NoError(t, h.store.ConfirmTombstones(ctx, channels.Anchor{ID: "TKT-1"}))
	h.anchors.set("TKT-1")
	h.clock.Add(time.Hour)
	rep, err = h.sweeper.RunRetention(ctx)
	require.NoError(t, err)
	require.Zero(t, rep.Restored)
	_, err = h.store.Head(ctx, k)
	require.ErrorIs(t, err, channels.ErrNotFound)
}

// walkCountingStore counts the store reads that walk every channel or batch.
type walkCountingStore struct {
	channels.Store
	mu    sync.Mutex
	calls map[string]int
}

func (c *walkCountingStore) count(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls[name]++
}

func (c *walkCountingStore) ListByLifecycle(
	ctx context.Context, typ string, state channels.State, before time.Time,
) ([]channels.Channel, error) {
	c.count("ListByLifecycle")
	return c.Store.ListByLifecycle(ctx, typ, state, before)
}

func (c *walkCountingStore) StalePending(ctx context.Context, before time.Time) ([]channels.PendingAction, error) {
	c.count("StalePending")
	return c.Store.StalePending(ctx, before)
}

func (c *walkCountingStore) Keys(ctx context.Context) ([]channels.Key, error) {
	c.count("Keys")
	return c.Store.Keys(ctx)
}

func (c *walkCountingStore) Tombstones(ctx context.Context, before time.Time) ([]channels.TombstoneBatch, error) {
	c.count("Tombstones")
	return c.Store.Tombstones(ctx, before)
}

func (c *walkCountingStore) DueTombstones(ctx context.Context, at time.Time) ([]channels.TombstoneBatch, error) {
	c.count("DueTombstones")
	return c.Store.DueTombstones(ctx, at)
}

// The per-minute runs read one indexed listing per auto step, and the
// purge poll one due listing; neither lists every channel or batch.
func TestSweep_Budget(t *testing.T) {
	ctx := context.Background()
	h := newSweepHarness(t)
	for _, id := range []string{"TKT-1", "TKT-2", "TKT-3"} {
		h.post(t, sweepKey(id, "room"), sweepT0)
	}
	cs := &walkCountingStore{Store: h.store, calls: map[string]int{}}
	sw, err := NewSweeper(SweepDeps{
		Store: cs, Config: h.cfg, Anchors: h.anchors, Marker: h.marker, Clock: h.clock.Now,
	})
	require.NoError(t, err)

	_, err = sw.RunLifecycle(ctx)
	require.NoError(t, err)
	// room: close from open, archive from closed; it declares actions.
	require.Equal(t, map[string]int{"ListByLifecycle": 2, "StalePending": 1}, cs.calls)

	cs.calls = map[string]int{}
	_, err = sw.RunPurge(ctx)
	require.NoError(t, err)
	require.Equal(t, map[string]int{"DueTombstones": 1}, cs.calls)
	require.Zero(t, h.anchors.calls, "the purge poll asks nothing of the entity store")
}
