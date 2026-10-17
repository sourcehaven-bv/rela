package realtime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/filechannels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

const sweepRealtime = `
version: 1
channel_types:
  room:
    attach_to: [ticket]
    detached_retention: 90d
    roles:
      member: { from: attached }
    message_types:
      text:
        properties: { body: { type: string } }
        post: [member]
        actions:
          accept: { roles: [member] }
    lifecycle:
      close:   { roles: [member], auto: { after: 1d } }
      archive: { roles: [member], from: closed, auto: { after: 1mo } }
  purge:
    attach_to: [doc]
    on_entity_delete: delete
    roles:
      member: { from: attached }
    message_types:
      text:
        properties: { body: { type: string } }
        post: [member]
  held:
    attach_to: [doc]
    roles:
      member: { from: attached }
    message_types:
      text:
        properties: { body: { type: string } }
        post: [member]
    lifecycle:
      close:  { roles: [member] }
      delete: { roles: [member], retention: 1y }
`

// sweepT0 is a 31 January, so a one-month period lands on 3 March.
var sweepT0 = time.Date(2026, 1, 31, 12, 0, 0, 0, time.UTC)

// fakeExistence is a settable AnchorExistence that counts its calls.
type fakeExistence struct {
	mu    sync.Mutex
	faces map[string][]entity.Face
	calls int
	sizes []int
	err   error
}

func (f *fakeExistence) Faces(_ context.Context, ids []string) (map[string][]entity.Face, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.sizes = append(f.sizes, len(ids))
	if f.err != nil {
		return nil, f.err
	}
	out := map[string][]entity.Face{}
	for _, id := range ids {
		if fs, ok := f.faces[id]; ok {
			out[id] = fs
		}
	}
	return out, nil
}

func (f *fakeExistence) set(id string, faces ...entity.Face) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if faces == nil {
		faces = []entity.Face{""}
	}
	f.faces[id] = faces
}

func (f *fakeExistence) drop(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.faces, id)
}

// recordingMarker is a ChangeMarker that keeps what it was told.
type recordingMarker struct {
	mu   sync.Mutex
	keys []channels.Key
}

func (m *recordingMarker) Mark(keys ...channels.Key) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keys = append(m.keys, keys...)
}

func (m *recordingMarker) marked() []channels.Key {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]channels.Key(nil), m.keys...)
}

type sweepHarness struct {
	store   channels.Store
	cfg     *fakeConfig
	anchors *fakeExistence
	marker  *recordingMarker
	clock   *testClock
	sweeper *Sweeper
}

func newSweepHarness(t *testing.T) *sweepHarness {
	t.Helper()
	st, err := filechannels.New(storage.NewMemFS(), "/channels", filelock.ProcessPrivate{})
	require.NoError(t, err)
	h := &sweepHarness{
		store:   st,
		cfg:     &fakeConfig{snap: Snapshot{Config: testCompiled(t, sweepRealtime)}},
		anchors: &fakeExistence{faces: map[string][]entity.Face{}},
		marker:  &recordingMarker{},
		clock:   &testClock{now: sweepT0},
	}
	h.sweeper, err = NewSweeper(SweepDeps{
		Store: st, Config: h.cfg, Anchors: h.anchors, Marker: h.marker, Clock: h.clock.Now,
	})
	require.NoError(t, err)
	return h
}

func (h *sweepHarness) post(t *testing.T, k channels.Key, at time.Time) channels.Message {
	t.Helper()
	m, err := h.store.Append(context.Background(), k, channels.NewMessage{
		Type: "text", Author: channels.Author{User: "alice"}, At: at, Props: map[string]any{"body": "hi"},
	})
	require.NoError(t, err)
	h.anchors.set(k.ID)
	return m
}

func (h *sweepHarness) head(t *testing.T, k channels.Key) channels.Channel {
	t.Helper()
	c, err := h.store.Head(context.Background(), k)
	require.NoError(t, err)
	return c
}

func sweepKey(id, typ string) channels.Key {
	return channels.Key{Anchor: channels.Anchor{ID: id}, Type: typ}
}

func TestNewSweeper_RejectsNil(t *testing.T) {
	h := newSweepHarness(t)
	full := SweepDeps{Store: h.store, Config: h.cfg, Anchors: h.anchors, Marker: h.marker, Clock: h.clock.Now}
	cases := []struct {
		name string
		edit func(*SweepDeps)
	}{
		{"Store", func(d *SweepDeps) { d.Store = nil }},
		{"Config", func(d *SweepDeps) { d.Config = nil }},
		{"Anchors", func(d *SweepDeps) { d.Anchors = nil }},
		{"Marker", func(d *SweepDeps) { d.Marker = nil }},
		{"Clock", func(d *SweepDeps) { d.Clock = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := full
			tc.edit(&d)
			_, err := NewSweeper(d)
			require.ErrorContains(t, err, tc.name)
		})
	}
}

// Auto close is exact for a fixed period and auto archive for a calendar
// one; both record who and which rule moved the channel.
func TestSweep_AutoLifecycle(t *testing.T) {
	ctx := context.Background()
	h := newSweepHarness(t)
	k := sweepKey("TKT-1", "room")
	h.post(t, k, sweepT0.Add(-24*time.Hour+time.Minute))
	h.post(t, sweepKey("DOC-1", "held"), sweepT0.Add(-48*time.Hour))

	rep, err := h.sweeper.RunLifecycle(ctx)
	require.NoError(t, err)
	require.Zero(t, rep.Closed, "a day has not passed yet")
	require.Equal(t, channels.StateOpen, h.head(t, k).State)

	h.clock.Add(time.Minute)
	rep, err = h.sweeper.RunLifecycle(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, rep.Closed, "only the type with auto.after closes")
	c := h.head(t, k)
	require.Equal(t, channels.StateClosed, c.State)
	require.Equal(t, h.clock.Now(), c.ClosedAt)
	require.Equal(t, []channels.Key{k}, h.marker.marked())
	trail, err := h.store.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	last := trail[len(trail)-1]
	require.Equal(t, systemAuthor(), last.By)
	require.Equal(t, "auto:room/close", last.TriggeredBy)

	// Closed on 31 January at 12:01: one month later is 3 March, 12:01.
	h.clock.Add(time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC).Sub(h.clock.Now()))
	rep, err = h.sweeper.RunLifecycle(ctx)
	require.NoError(t, err)
	require.Zero(t, rep.Archived)
	h.clock.Add(time.Minute)
	rep, err = h.sweeper.RunLifecycle(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, rep.Archived)
	require.Equal(t, channels.StateArchived, h.head(t, k).State)
}

// A reopened channel is closed again only once auto.after has run from the
// reopen, not from its creation.
func TestSweep_ReopenRestartsAutoClose(t *testing.T) {
	ctx := context.Background()
	h := newSweepHarness(t)
	k := sweepKey("TKT-1", "room")
	h.post(t, k, sweepT0.Add(-72*time.Hour))
	rep, err := h.sweeper.RunLifecycle(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, rep.Closed)

	_, err = h.store.SetLifecycle(ctx, k, channels.StateClosed, channels.Transition{
		To: channels.StateOpen, By: channels.Author{User: "alice"}, At: h.clock.Now(),
	})
	require.NoError(t, err)
	h.clock.Add(time.Minute)
	rep, err = h.sweeper.RunLifecycle(ctx)
	require.NoError(t, err)
	require.Zero(t, rep.Closed, "the reopen restarts the day")
	require.Equal(t, channels.StateOpen, h.head(t, k).State)

	h.clock.Add(24 * time.Hour)
	rep, err = h.sweeper.RunLifecycle(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, rep.Closed, "a day after the reopen it closes again")
}

// A degraded config runs no automatic step and a missing one runs nothing.
func TestSweep_LifecycleConfigStates(t *testing.T) {
	ctx := context.Background()
	h := newSweepHarness(t)
	k := sweepKey("TKT-1", "room")
	h.post(t, k, sweepT0.Add(-48*time.Hour))
	cfg := h.cfg.Load().Config

	for _, snap := range []Snapshot{{Config: cfg, Degraded: true}, {}} {
		h.cfg.set(snap)
		rep, err := h.sweeper.RunLifecycle(ctx)
		require.NoError(t, err)
		require.Zero(t, rep.Closed)
		require.Equal(t, channels.StateOpen, h.head(t, k).State)
	}
}

// Stale pending actions are reported, never reverted, and only for types
// that declare actions.
func TestSweep_StalePending(t *testing.T) {
	ctx := context.Background()
	h := newSweepHarness(t)
	k := sweepKey("TKT-1", "room")
	m := h.post(t, k, sweepT0)
	_, err := h.store.SetAction(ctx, k, m.Seq, "accept", channels.ActionNone, channels.Action{
		Status: channels.ActionPending, By: channels.Author{User: "bob"}, At: sweepT0,
	})
	require.NoError(t, err)

	h.clock.Add(realtimelimits.PendingRelease - time.Second)
	rep, err := h.sweeper.RunLifecycle(ctx)
	require.NoError(t, err)
	require.Empty(t, rep.StalePending)

	h.clock.Add(2 * time.Second)
	rep, err = h.sweeper.RunLifecycle(ctx)
	require.NoError(t, err)
	require.Len(t, rep.StalePending, 1)
	require.Equal(t, k, rep.StalePending[0].Key)
	got, err := h.store.Get(ctx, k, m.Seq)
	require.NoError(t, err)
	require.Equal(t, channels.ActionPending, got.Action.Status, "the sweep never reverts")
}

func TestPurgeAfter(t *testing.T) {
	h := newSweepHarness(t)
	cfg := h.cfg.Load().Config
	at := sweepT0
	closed := at.Add(-24 * time.Hour)
	ch := func(typ string, closedAt time.Time) channels.Channel {
		return channels.Channel{Key: sweepKey("X-1", typ), ClosedAt: closedAt}
	}
	cases := []struct {
		name   string
		cfg    *realtimecfg.Compiled
		policy channels.TombstonePolicy
		chs    []channels.Channel
		want   time.Time
	}{
		{"delete policy erases at once", cfg, channels.PolicyDelete, []channels.Channel{ch("purge", time.Time{})}, at},
		{"delete batch keeps a detach type", cfg, channels.PolicyDelete,
			[]channels.Channel{ch("purge", time.Time{}), ch("room", time.Time{})}, at.Add(90 * 24 * time.Hour)},
		{"detach of a delete type waits", cfg, channels.PolicyDetach, []channels.Channel{ch("purge", time.Time{})},
			at.Add(realtimelimits.DetachedRetention)},
		{"type retention", cfg, channels.PolicyOrphan, []channels.Channel{ch("room", time.Time{})},
			at.Add(90 * 24 * time.Hour)},
		{"undeclared type", cfg, channels.PolicyDetach, []channels.Channel{ch("gone", time.Time{})},
			at.Add(realtimelimits.DetachedRetention)},
		{"realtime off", nil, channels.PolicyDelete, []channels.Channel{ch("purge", time.Time{})},
			at.Add(realtimelimits.DetachedRetention)},
		{"lifecycle retention from close", cfg, channels.PolicyDetach, []channels.Channel{ch("held", closed)},
			closed.AddDate(1, 0, 0)},
		{"lifecycle retention of an unclosed channel", cfg, channels.PolicyDetach,
			[]channels.Channel{ch("held", time.Time{})}, at.AddDate(1, 0, 0)},
		{"empty batch", cfg, channels.PolicyDetach, nil, at},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, PurgeAfter(tc.cfg, tc.policy, at, tc.chs))
		})
	}
	require.Equal(t, channels.PolicyDelete, DeletePolicy(cfg, []channels.Channel{ch("room", time.Time{}), ch("purge", time.Time{})}))
	require.Equal(t, channels.PolicyDetach, DeletePolicy(cfg, []channels.Channel{ch("room", time.Time{})}))
	require.Equal(t, channels.PolicyDetach, DeletePolicy(nil, []channels.Channel{ch("purge", time.Time{})}))
}

// Purge follows the recorded filing, refuses a pending batch until its
// delete is settled, and needs a sound config for a batch without one.
func TestSweep_PurgeHonoursPolicyAndRetention(t *testing.T) {
	type step struct {
		advance  time.Duration
		snap     func(cfg *realtimecfg.Compiled) Snapshot
		purged   int
		restored int
	}
	// A batch is listed once filed before now, so it is filed a second
	// before the first run.
	filedAt := sweepT0.Add(-time.Second)
	healthy := func(cfg *realtimecfg.Compiled) Snapshot { return Snapshot{Config: cfg} }
	degraded := func(cfg *realtimecfg.Compiled) Snapshot { return Snapshot{Config: cfg, Degraded: true} }
	cases := []struct {
		name    string
		typ     string
		filing  func(cfg *realtimecfg.Compiled, chs []channels.Channel) channels.Filing
		present bool
		steps   []step
	}{
		{
			name: "delete policy purges at once", typ: "purge",
			filing: func(cfg *realtimecfg.Compiled, chs []channels.Channel) channels.Filing {
				return FilingFor(cfg, channels.PolicyDelete, filedAt, chs, false)
			},
			steps: []step{{snap: healthy, purged: 1}},
		},
		{
			name: "detach waits for the type's retention", typ: "room",
			filing: func(cfg *realtimecfg.Compiled, chs []channels.Channel) channels.Filing {
				return FilingFor(cfg, channels.PolicyDetach, filedAt, chs, false)
			},
			steps: []step{
				{advance: 90*24*time.Hour - 2*time.Second, snap: healthy},
				{advance: time.Second, snap: healthy, purged: 1},
			},
		},
		{
			name: "recorded filing ignores the config", typ: "purge",
			filing: func(cfg *realtimecfg.Compiled, chs []channels.Channel) channels.Filing {
				return FilingFor(cfg, channels.PolicyDelete, filedAt, chs, false)
			},
			steps: []step{{snap: func(*realtimecfg.Compiled) Snapshot { return Snapshot{} }, purged: 1}},
		},
		{
			name: "pending with the entity gone is confirmed and purged", typ: "purge",
			filing: func(cfg *realtimecfg.Compiled, chs []channels.Channel) channels.Filing {
				return FilingFor(cfg, channels.PolicyDelete, filedAt, chs, true)
			},
			steps: []step{{snap: healthy, purged: 1}},
		},
		{
			name: "pending with the entity present is restored after the grace", typ: "purge", present: true,
			filing: func(cfg *realtimecfg.Compiled, chs []channels.Channel) channels.Filing {
				return FilingFor(cfg, channels.PolicyDelete, filedAt, chs, true)
			},
			steps: []step{
				{advance: realtimelimits.OrphanGrace - 2*time.Second, snap: healthy},
				{advance: time.Second, snap: healthy, restored: 1},
			},
		},
		{
			name: "unrecorded needs a sound config", typ: "purge",
			filing: func(*realtimecfg.Compiled, []channels.Channel) channels.Filing { return channels.Filing{} },
			steps: []step{
				{advance: realtimelimits.DetachedRetention, snap: degraded},
				{snap: func(*realtimecfg.Compiled) Snapshot { return Snapshot{} }},
				{snap: healthy, purged: 1},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			h := newSweepHarness(t)
			cfg := h.cfg.Load().Config
			k := sweepKey("DOC-1", tc.typ)
			h.post(t, k, sweepT0.Add(-time.Hour))
			if !tc.present {
				h.anchors.drop("DOC-1")
			}
			list, err := h.store.ListEntity(ctx, "DOC-1")
			require.NoError(t, err)
			_, err = h.store.Tombstone(ctx, "DOC-1", "", filedAt, channels.FileAs(tc.filing(cfg, list)))
			require.NoError(t, err)
			for i, s := range tc.steps {
				h.clock.Add(s.advance)
				h.cfg.set(s.snap(cfg))
				rep, err := h.sweeper.RunRetention(ctx)
				require.NoError(t, err)
				require.Equal(t, s.purged, rep.Purged, "step %d purged", i)
				require.Equal(t, s.restored, rep.Restored, "step %d restored", i)
			}
		})
	}
}

// An unrecorded batch holding a type the config no longer declares is
// never purged.
func TestSweep_UnrecordedUndeclaredType(t *testing.T) {
	ctx := context.Background()
	h := newSweepHarness(t)
	h.post(t, sweepKey("DOC-1", "gone"), sweepT0)
	h.anchors.drop("DOC-1")
	_, err := h.store.Tombstone(ctx, "DOC-1", "", sweepT0, nil)
	require.NoError(t, err)
	h.clock.Add(10 * realtimelimits.DetachedRetention)
	rep, err := h.sweeper.RunRetention(ctx)
	require.NoError(t, err)
	require.Zero(t, rep.Purged)
}

// An orphan is detached only after OrphanGrace, and comes back when its
// entity does.
func TestSweep_OrphanGrace(t *testing.T) {
	ctx := context.Background()
	h := newSweepHarness(t)
	k := sweepKey("TKT-1", "room")
	h.post(t, k, sweepT0)
	h.post(t, sweepKey("TKT-2", "room"), sweepT0)
	h.anchors.drop("TKT-1")

	rep, err := h.sweeper.RunRetention(ctx)
	require.NoError(t, err)
	require.Zero(t, rep.Detached, "first seen missing")
	h.clock.Add(realtimelimits.OrphanGrace - time.Second)
	rep, err = h.sweeper.RunRetention(ctx)
	require.NoError(t, err)
	require.Zero(t, rep.Detached)

	h.clock.Add(time.Second)
	rep, err = h.sweeper.RunRetention(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, rep.Detached)
	_, err = h.store.Head(ctx, k)
	require.ErrorIs(t, err, channels.ErrNotFound)
	batches, err := h.store.Tombstones(ctx, h.clock.Now().Add(time.Second))
	require.NoError(t, err)
	require.Len(t, batches, 1)
	require.Equal(t, channels.PolicyOrphan, batches[0].Filing.Policy)
	require.Equal(t, h.clock.Now().Add(90*24*time.Hour), batches[0].Filing.PurgeAfter)

	h.anchors.set("TKT-1")
	h.clock.Add(time.Hour)
	rep, err = h.sweeper.RunRetention(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, rep.Restored)
	require.Equal(t, channels.StateOpen, h.head(t, k).State)
}

// An id that reappears within the grace starts over when it goes again.
func TestSweep_OrphanReappears(t *testing.T) {
	ctx := context.Background()
	h := newSweepHarness(t)
	h.post(t, sweepKey("TKT-1", "room"), sweepT0)
	h.anchors.drop("TKT-1")
	_, err := h.sweeper.RunRetention(ctx)
	require.NoError(t, err)
	h.anchors.set("TKT-1")
	h.clock.Add(30 * time.Minute)
	_, err = h.sweeper.RunRetention(ctx)
	require.NoError(t, err)
	h.anchors.drop("TKT-1")
	h.clock.Add(45 * time.Minute)
	rep, err := h.sweeper.RunRetention(ctx)
	require.NoError(t, err)
	require.Zero(t, rep.Detached, "the grace restarted when the id came back")
}

// Type records go only for undeclared, unused types, and never under a
// degraded config.
func TestSweep_TypeRecords(t *testing.T) {
	ctx := context.Background()
	h := newSweepHarness(t)
	rec := channels.TypeRecord{Anchor: channels.AnchorFamily, Cardinality: channels.CardinalityOne}
	require.NoError(t, h.store.RecordTypes(ctx, map[string]channels.TypeRecord{
		"room": rec, "old": rec, "busy": rec,
	}))
	h.post(t, sweepKey("TKT-1", "busy"), sweepT0)

	h.cfg.set(Snapshot{Config: h.cfg.Load().Config, Degraded: true})
	rep, err := h.sweeper.RunRetention(ctx)
	require.NoError(t, err)
	require.Zero(t, rep.TypesCleared)

	h.cfg.set(Snapshot{Config: h.cfg.Load().Config})
	rep, err = h.sweeper.RunRetention(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, rep.TypesCleared)
	got, err := h.store.TypeRecords(ctx)
	require.NoError(t, err)
	require.Contains(t, got, "room")
	require.Contains(t, got, "busy")
	require.NotContains(t, got, "old")
}

// The orphan check asks about at most anchorChunk ids per call.
func TestSweep_AnchorBatches(t *testing.T) {
	ctx := context.Background()
	h := newSweepHarness(t)
	for i := range anchorChunk + 1 {
		h.post(t, sweepKey("TKT-"+itoa(i+1), "room"), sweepT0)
	}
	_, err := h.sweeper.RunRetention(ctx)
	require.NoError(t, err)
	require.Equal(t, []int{anchorChunk, 1}, h.anchors.sizes)

	h.anchors.err = errors.New("store down")
	_, err = h.sweeper.RunRetention(ctx)
	require.ErrorContains(t, err, "store down")
}

func itoa(n int) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{digits[n%10]}, b...)
	}
	return string(b)
}
