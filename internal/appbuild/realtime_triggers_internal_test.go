package appbuild

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/worlds"
)

const triggerSchema = `version: "1.0"
entities:
  ticket:
    label: Ticket
    id_type: manual
    properties:
      status: {type: string}
  page:
    label: Page
    id_type: manual
    properties:
      title: {type: string}
    faces:
      draft: {}
      final: {}
`

const triggerRealtimeYAML = `version: 1
channel_types:
  room:
    attach_to: [ticket]
    roles:
      member: { from: attached }
    message_types:
      text:
        properties: { body: { type: string, required: true } }
        post: [member]
  pagechat:
    attach_to: [page]
    anchor: face
    roles:
      member: { from: attached }
    message_types:
      text:
        properties: { body: { type: string, required: true } }
        post: [member]
triggers:
  opened:
    entity_type: ticket
    on: enter
    when: "entity.status == 'open'"
    channel: room
    post: { type: text, props: { body: "Opened" } }
  titled:
    entity_type: page
    on: enter
    when: "entity.title == 'ready'"
    channel: pagechat
    post: { type: text, props: { body: "Ready" } }
`

type triggerFixture struct {
	meta *metamodel.Metamodel
	cs   channels.Store
	st   store.Store
	src  entitySource
	kv   *mapKV
	cfg  realtimeConfigSource
}

func newTriggerFixture(t *testing.T) triggerFixture {
	t.Helper()
	meta, err := metamodel.Parse([]byte(triggerSchema))
	require.NoError(t, err)
	cfg, err := realtimecfg.Parse([]byte(triggerRealtimeYAML), meta, nil)
	require.NoError(t, err)
	w, err := worlds.Compile(meta)
	require.NoError(t, err)
	_, cs, st := newChannelAliases(t)
	h := &RealtimeConfig{}
	h.cur.Store(&RealtimeSnapshot{Config: cfg})
	src, err := newEntitySource(st, w.DefaultWorld())
	require.NoError(t, err)
	return triggerFixture{
		meta: meta, cs: cs, st: st, src: src,
		kv: &mapKV{m: map[string][]byte{}}, cfg: realtimeConfigSource{h: h},
	}
}

func (f triggerFixture) engine(t *testing.T) *realtime.Triggers {
	t.Helper()
	local, err := realtime.NewLocal(time.Now)
	require.NoError(t, err)
	t.Cleanup(local.Stop)
	trg, err := realtime.NewTriggers(realtime.TriggerDeps{
		Store: f.cs, Config: f.cfg, Local: local, Meta: f.meta, Rows: f.src, Seen: f.kv,
	})
	require.NoError(t, err)
	return trg
}

func (f triggerFixture) write(t *testing.T, typ, id string, face entity.Face, props map[string]any) {
	t.Helper()
	ctx := context.Background()
	e := &entity.Entity{ID: id, Type: typ, Face: face, Properties: props}
	if _, err := f.st.GetEntity(ctx, e.Ref()); err == nil {
		require.NoError(t, f.st.UpdateEntity(ctx, e))
		return
	}
	require.NoError(t, f.st.CreateEntity(ctx, e))
}

func (f triggerFixture) bodies(t *testing.T, k channels.Key) []string {
	t.Helper()
	msgs, err := f.cs.Messages(context.Background(), k, channels.Page{Limit: 50})
	if err != nil {
		return nil
	}
	var out []string
	for _, m := range msgs {
		out = append(out, m.Props["body"].(string))
	}
	return out
}

func TestEntitySource_ReadsTheFaceTheChannelShows(t *testing.T) {
	f := newTriggerFixture(t)
	ctx := context.Background()
	f.write(t, "page", "P-1", "draft", map[string]any{"title": "d"})
	f.write(t, "page", "P-1", "final", map[string]any{"title": "f"})
	each, err := f.src.Rows(ctx, "page", []string{"P-1"}, realtime.FaceEach)
	require.NoError(t, err)
	require.Len(t, each, 2)
	prime, err := f.src.Rows(ctx, "page", []string{"P-1"}, realtime.FacePrime)
	require.NoError(t, err)
	require.Len(t, prime, 1, "a family rule reads one face")
	var n int
	require.NoError(t, f.src.Scan(ctx, "page", realtime.FaceEach, func(rows []realtime.EntityRow) error {
		n += len(rows)
		return nil
	}))
	require.Equal(t, 2, n)
	none, err := f.src.Rows(ctx, "page", nil, realtime.FaceEach)
	require.NoError(t, err)
	require.Empty(t, none)
}

// oneLock is a worker lock that one worker holds at a time.
type oneLock struct{ held atomic.Bool }

func (l *oneLock) take(context.Context) (lock heldLock, ok bool, err error) {
	if !l.held.CompareAndSwap(false, true) {
		return heldLock{}, false, nil
	}
	return heldLock{release: func() { l.held.Store(false) }}, true, nil
}

// testWorker is a worker of its own process over the fixture's stores.
type testWorker struct {
	w   *realtimeWorker
	src *countingSource
}

func (f triggerFixture) worker(t *testing.T, lock leaderLock) testWorker {
	t.Helper()
	tombs, err := newTombstoneConfirmer(f.cs)
	require.NoError(t, err)
	local, err := realtime.NewLocal(time.Now)
	require.NoError(t, err)
	t.Cleanup(local.Stop)
	sweeper, err := realtime.NewSweeper(realtime.SweepDeps{
		Store: f.cs, Config: f.cfg, Anchors: anchorExistence{st: f.st}, Marker: local, Clock: time.Now,
	})
	require.NoError(t, err)
	src := &countingSource{inner: f.src}
	trg, err := realtime.NewTriggers(realtime.TriggerDeps{
		Store: f.cs, Config: f.cfg, Local: local, Meta: f.meta, Rows: src, Seen: f.kv,
	})
	require.NoError(t, err)
	w, err := newRealtimeWorker(realtimeWorkerParts{
		sweeper: sweeper, triggers: trg, changes: f.st, lock: lock, wake: tombs.wake,
	})
	require.NoError(t, err)
	w.takeover = 20 * time.Millisecond
	return testWorker{w: w, src: src}
}

// Two workers share one lock. The leader notes store events and posts;
// a change stream costs it no reconcile. The other does no trigger work
// at all, and when the leader stops it takes over, reconciles and posts
// what changed in between.
func TestRealtimeWorker_TwoProcessesOneLeader(t *testing.T) {
	f := newTriggerFixture(t)
	f.write(t, "ticket", "T-1", "", map[string]any{"status": "new"})
	lock := &oneLock{}
	a := f.worker(t, lock.take)
	ctxA, stopA := context.WithCancel(context.Background())
	doneA := make(chan struct{})
	go func() { a.w.run(ctxA); close(doneA) }()
	t.Cleanup(func() { stopA(); <-doneA })
	require.Eventually(t, func() bool {
		idx, err := f.kv.Get(context.Background(), "realtime/seen/index")
		return err == nil && len(idx) > 0
	}, 5*time.Second, 10*time.Millisecond, "the start reconcile baselines")

	b := f.worker(t, lock.take)
	done := RunRealtimeWorker(context.Background(), &Services{realtimeWorker: b.w})
	t.Cleanup(func() { b.w.stop(); <-done })

	room := channels.Key{Anchor: channels.Anchor{ID: "T-1"}, Type: "room"}
	f.write(t, "ticket", "T-1", "", map[string]any{"status": "open"})
	require.Eventually(t, func() bool { return len(f.bodies(t, room)) == 1 },
		5*time.Second, 10*time.Millisecond, "a store event posts")
	// The start reconcile keeps scanning after it writes the index; the
	// leader posts only after it, so the count is stable from here.
	scans := a.src.scans.Load()
	for i := range 5 {
		f.write(t, "ticket", "T-1", "", map[string]any{"status": "open", "n": i})
	}
	require.Never(t, func() bool { return b.src.scans.Load()+b.src.rows.Load() > 0 },
		3*realtime.TriggerTick, 10*time.Millisecond, "a follower reads nothing")
	require.Equal(t, scans, a.src.scans.Load(), "changes cost the leader no reconcile")

	stopA()
	<-doneA
	f.write(t, "ticket", "T-2", "", map[string]any{"status": "open"})
	room2 := channels.Key{Anchor: channels.Anchor{ID: "T-2"}, Type: "room"}
	require.Eventually(t, func() bool { return len(f.bodies(t, room2)) == 1 },
		5*time.Second, 10*time.Millisecond, "the follower takes over")
	require.Positive(t, b.src.scans.Load(), "taking over reconciles")
	require.Equal(t, []string{"Opened"}, f.bodies(t, room))
}

// Two engines over one KV, run one after the other as the worker lock
// orders them, post a change once: the second reads what the first saved.
func TestTriggers_TakeoverPostsOnce(t *testing.T) {
	f := newTriggerFixture(t)
	ctx := context.Background()
	f.write(t, "page", "P-1", "draft", map[string]any{"title": "wip"})
	a, b := f.engine(t), f.engine(t)
	_, err := a.RunReconcile(ctx)
	require.NoError(t, err)
	f.write(t, "page", "P-1", "draft", map[string]any{"title": "ready"})
	c := realtime.Change{EntityType: "page", ID: "P-1"}
	a.Note(c)
	b.Note(c)
	rep, err := a.RunRound(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, rep.Posted)
	rep, err = b.RunRound(ctx)
	require.NoError(t, err)
	require.Zero(t, rep.Posted)
	rep, err = b.RunReconcile(ctx)
	require.NoError(t, err)
	require.Zero(t, rep.Posted)
	draft := channels.Key{Anchor: channels.Anchor{ID: "P-1", Face: "draft"}, Type: "pagechat"}
	require.Equal(t, []string{"Ready"}, f.bodies(t, draft))
}

// A round costs a bounded number of reads whatever the size of the type:
// one header batch per face mode for the changed ids, and no scan.
func TestTriggers_RoundReadsOnlyChangedIDs(t *testing.T) {
	f := newTriggerFixture(t)
	ctx := context.Background()
	for i := range 200 {
		f.write(t, "ticket", "T-"+string(rune('A'+i%26))+string(rune('a'+i/26)), "", map[string]any{"status": "new"})
	}
	cnt := &countingSource{inner: f.src}
	local, err := realtime.NewLocal(time.Now)
	require.NoError(t, err)
	t.Cleanup(local.Stop)
	trg, err := realtime.NewTriggers(realtime.TriggerDeps{
		Store: f.cs, Config: f.cfg, Local: local, Meta: f.meta, Rows: cnt, Seen: f.kv,
	})
	require.NoError(t, err)
	_, err = trg.RunReconcile(ctx)
	require.NoError(t, err)
	cnt.rows.Store(0)
	cnt.scans.Store(0)
	f.write(t, "ticket", "T-Aa", "", map[string]any{"status": "open"})
	trg.Note(realtime.Change{EntityType: "ticket", ID: "T-Aa"})
	rep, err := trg.RunRound(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, rep.Posted)
	require.Equal(t, int64(1), cnt.rows.Load())
	require.Zero(t, cnt.scans.Load())
}

type countingSource struct {
	inner       realtime.EntitySource
	rows, scans atomic.Int64
}

func (c *countingSource) Rows(
	ctx context.Context, typ string, ids []string, m realtime.FaceMode,
) ([]realtime.EntityRow, error) {
	c.rows.Add(1)
	return c.inner.Rows(ctx, typ, ids, m)
}

func (c *countingSource) Scan(
	ctx context.Context, typ string, m realtime.FaceMode, page func([]realtime.EntityRow) error,
) error {
	c.scans.Add(1)
	return c.inner.Scan(ctx, typ, m, page)
}

func TestChangeOf(t *testing.T) {
	c, ok := changeOf(store.Event{Op: store.EventEntityUpdated, EntityType: "ticket", EntityID: "T-1"})
	require.True(t, ok)
	require.Equal(t, realtime.Change{EntityType: "ticket", ID: "T-1"}, c)
	_, ok = changeOf(store.Event{Op: store.EventRelationCreated, From: "T-1", To: "T-2"})
	require.False(t, ok)
}
