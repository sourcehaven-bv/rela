package realtime

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
	"github.com/Sourcehaven-BV/rela/internal/testutil/logtest"
)

// triggerRealtime is testRealtime with triggers on tickets and pages.
const triggerRealtime = testRealtime + `
triggers:
  opened:
    entity_type: ticket
    on: enter
    when: "entity.status == 'open'"
    channel: room
    post: { type: text, props: { body: "Opened" } }
  shut:
    entity_type: ticket
    on: leave
    when: "entity.status == 'open'"
    channel: room
    post: { type: text, props: { body: "No longer open" } }
  retitled:
    entity_type: ticket
    on: change
    when: "entity.status == 'open' and entity.title ~= ''"
    channel: room
    post: { type: text, props: { body: "Changed" } }
  drafted:
    entity_type: page
    on: enter
    when: "entity.title == 'ready'"
    channel: facechat
    post: { type: text, props: { body: "Ready" } }
`

// fakeSource is an [EntitySource] over rows held in memory. A row with a
// face is a faced type's: [FacePrime] reads the published face.
type fakeSource struct {
	mu    sync.Mutex
	rows  map[string]map[string][]EntityRow // type -> id -> faces
	scans atomic.Int64
	reads atomic.Int64
	fail  error
}

func newFakeSource() *fakeSource {
	return &fakeSource{rows: map[string]map[string][]EntityRow{}}
}

// set stores props for one face of id; nil props deletes the face.
func (f *fakeSource) set(typ, id string, face entity.Face, props map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rows[typ] == nil {
		f.rows[typ] = map[string][]EntityRow{}
	}
	faces := slices.DeleteFunc(f.rows[typ][id], func(r EntityRow) bool { return r.Face == face })
	if props != nil {
		faces = append(faces, EntityRow{ID: id, Face: face, Props: maps.Clone(props)})
	}
	if len(faces) == 0 {
		delete(f.rows[typ], id)
		return
	}
	f.rows[typ][id] = faces
}

func (f *fakeSource) pick(faces []EntityRow, m FaceMode) []EntityRow {
	if m == FaceEach {
		return slices.Clone(faces)
	}
	for _, r := range faces {
		if r.Face == "" || r.Face == "published" {
			return []EntityRow{r}
		}
	}
	return nil
}

func (f *fakeSource) Rows(_ context.Context, typ string, ids []string, m FaceMode) ([]EntityRow, error) {
	f.reads.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return nil, f.fail
	}
	var out []EntityRow
	for _, id := range ids {
		out = append(out, f.pick(f.rows[typ][id], m)...)
	}
	return out, nil
}

func (f *fakeSource) Scan(_ context.Context, typ string, m FaceMode, page func([]EntityRow) error) error {
	f.scans.Add(1)
	f.mu.Lock()
	if f.fail != nil {
		f.mu.Unlock()
		return f.fail
	}
	ids := slices.Sorted(maps.Keys(f.rows[typ]))
	var out []EntityRow
	for _, id := range ids {
		out = append(out, f.pick(f.rows[typ][id], m)...)
	}
	f.mu.Unlock()
	if len(out) == 0 {
		return nil
	}
	return page(out)
}

// memKV is a [KV] in memory that can fail its writes.
type memKV struct {
	mu      sync.Mutex
	data    map[string][]byte
	failPut bool
	puts    atomic.Int64
	gets    atomic.Int64
	bytes   atomic.Int64
}

func newMemKV() *memKV { return &memKV{data: map[string][]byte{}} }

func (k *memKV) Get(_ context.Context, key string) ([]byte, error) {
	k.gets.Add(1)
	k.mu.Lock()
	defer k.mu.Unlock()
	v, ok := k.data[key]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return slices.Clone(v), nil
}

func (k *memKV) Put(_ context.Context, key string, data []byte) error {
	k.puts.Add(1)
	k.bytes.Add(int64(len(data)))
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.failPut {
		return errors.New("kv down")
	}
	k.data[key] = slices.Clone(data)
	return nil
}

func (k *memKV) Delete(_ context.Context, key string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.failPut {
		return errors.New("kv down")
	}
	delete(k.data, key)
	return nil
}

func (k *memKV) keys() []string {
	k.mu.Lock()
	defer k.mu.Unlock()
	return slices.Sorted(maps.Keys(k.data))
}

type triggerHarness struct {
	*harness
	trg *Triggers
	src *fakeSource
	kv  *memKV
}

func newTriggerHarness(t *testing.T, yaml string) *triggerHarness {
	t.Helper()
	h := newHarness(t, withYAML(yaml))
	th := &triggerHarness{harness: h, src: newFakeSource(), kv: newMemKV()}
	th.trg = th.engine(t, th.kv)
	return th
}

// engine builds another engine over the harness's stores and KV.
func (th *triggerHarness) engine(t *testing.T, kv KV) *Triggers {
	t.Helper()
	trg, err := NewTriggers(TriggerDeps{
		Store: th.store, Config: th.cfg, Local: th.local, Meta: testMeta(t), Rows: th.src, Seen: kv, DueWake: th.wake,
	})
	require.NoError(t, err)
	return trg
}

// ticket sets TKT-<n>'s properties and notes the change.
func (th *triggerHarness) ticket(n int, props map[string]any) {
	id := fmt.Sprintf("TKT-%d", n)
	th.src.set("ticket", id, "", props)
	th.trg.Note(Change{EntityType: "ticket", ID: id})
}

func (th *triggerHarness) round(t *testing.T) TriggerReport {
	t.Helper()
	rep, err := th.trg.RunRound(context.Background())
	require.NoError(t, err)
	return rep
}

func (th *triggerHarness) reconcile(t *testing.T) TriggerReport {
	t.Helper()
	rep, err := th.trg.RunReconcile(context.Background())
	require.NoError(t, err)
	return rep
}

// bodies returns the bodies posted in the room of id, oldest first.
func (th *triggerHarness) bodies(t *testing.T, id string) []string {
	t.Helper()
	return th.bodiesIn(t, channels.Key{Anchor: channels.Anchor{ID: id}, Type: "room"})
}

func (th *triggerHarness) bodiesIn(t *testing.T, k channels.Key) []string {
	t.Helper()
	msgs, err := th.store.Store.Messages(context.Background(), k, channels.Page{Limit: 100})
	if errors.Is(err, channels.ErrNotFound) {
		return nil
	}
	require.NoError(t, err)
	var out []string
	for _, m := range msgs {
		out = append(out, fmt.Sprint(m.Props["body"]))
	}
	return out
}

func TestNewTriggers_RejectsNil(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	full := TriggerDeps{
		Store: th.store, Config: th.cfg, Local: th.local, Meta: testMeta(t), Rows: th.src, Seen: th.kv,
		DueWake: NewDueWake(),
	}
	for _, f := range []string{"Store", "Config", "Local", "Meta", "Rows", "Seen", "DueWake"} {
		t.Run(f, func(t *testing.T) {
			d := full
			switch f {
			case "Store":
				d.Store = nil
			case "Config":
				d.Config = nil
			case "Local":
				d.Local = nil
			case "Meta":
				d.Meta = nil
			case "Rows":
				d.Rows = nil
			case "Seen":
				d.Seen = nil
			case "DueWake":
				d.DueWake = nil
			}
			_, err := NewTriggers(d)
			require.ErrorContains(t, err, f)
		})
	}
}

func TestTriggers_EnterLeaveChange(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	th.src.set("ticket", "TKT-1", "", map[string]any{"status": "new", "title": "A"})
	rep := th.reconcile(t)
	require.Equal(t, 4, rep.Baselined)
	require.Zero(t, rep.Posted)

	th.ticket(1, map[string]any{"status": "open", "title": "A"})
	require.True(t, th.trg.Pending())
	require.Equal(t, 1, th.round(t).Posted)
	require.False(t, th.trg.Pending())

	th.clock.Add(realtimelimits.TriggerCooldown)
	th.ticket(1, map[string]any{"status": "open", "title": "B"})
	require.Equal(t, 1, th.round(t).Posted)

	th.clock.Add(realtimelimits.TriggerCooldown)
	th.ticket(1, map[string]any{"status": "done", "title": "B"})
	require.Equal(t, 1, th.round(t).Posted)

	require.Equal(t, []string{"Opened", "Changed", "No longer open"}, th.bodies(t, "TKT-1"))
	msgs, err := th.store.Store.Messages(context.Background(), roomKey(), channels.Page{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, "trigger:opened", msgs[0].TriggeredBy)
	require.Equal(t, systemAuthor(), msgs[0].Author)

	page, err := th.svc.Read(as(user("alice")), room, ReadRequest{})
	require.NoError(t, err)
	require.Equal(t, "trigger:opened", page.Messages[0].TriggeredBy)
	require.True(t, page.Messages[0].Author.System, "a client tells a trigger's message from a person's")
}

func TestTriggers_ChangeIgnoresUnreadFields(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	th.src.set("ticket", "TKT-1", "", map[string]any{"status": "open", "title": "A"})
	th.reconcile(t)
	th.ticket(1, map[string]any{"status": "open", "title": "A", "owner": "bob"})
	require.Zero(t, th.round(t).Posted, "owner is not read by any rule")
	require.Empty(t, th.bodies(t, "TKT-1"))
}

func TestTriggers_NewRuleBaselinesSilently(t *testing.T) {
	th := newTriggerHarness(t, testRealtime)
	th.src.set("ticket", "TKT-1", "", map[string]any{"status": "open"})
	th.ticket(1, map[string]any{"status": "open"})
	require.Zero(t, th.round(t).Posted, "no rules")

	th.cfg.set(Snapshot{Config: testCompiled(t, triggerRealtime)})
	th.ticket(1, map[string]any{"status": "open"})
	rep := th.round(t)
	require.Equal(t, 4, rep.Baselined)
	require.Zero(t, rep.Posted, "an entity that matches when the rule appears does not post")
	require.Empty(t, th.bodies(t, "TKT-1"))
}

func TestTriggers_EditedRuleRebaselines(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	th.src.set("ticket", "TKT-1", "", map[string]any{"status": "review"})
	th.reconcile(t)
	edited := strings.Replace(triggerRealtime, `when: "entity.status == 'open'"
    channel: room
    post: { type: text, props: { body: "Opened" } }`, `when: "entity.status == 'review'"
    channel: room
    post: { type: text, props: { body: "Opened" } }`, 1)
	require.NotEqual(t, triggerRealtime, edited)
	th.cfg.set(Snapshot{Config: testCompiled(t, edited)})
	rep := th.reconcile(t)
	require.Equal(t, 1, rep.Baselined)
	require.Zero(t, rep.Posted)
	idx, err := seenStore{kv: th.kv}.loadIndex(context.Background())
	require.NoError(t, err)
	require.Len(t, idx.Rules, 4, "the old rule's seen-set is removed")
}

func TestTriggers_CoalescesWithinTick(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	th.src.set("ticket", "TKT-1", "", map[string]any{"status": "new"})
	th.reconcile(t)
	th.ticket(1, map[string]any{"status": "open"})
	th.ticket(1, map[string]any{"status": "new"})
	require.Zero(t, th.round(t).Posted, "open and back within one tick is no change")
	th.ticket(1, map[string]any{"status": "open"})
	th.ticket(1, map[string]any{"status": "open", "title": "x"})
	require.Equal(t, 1, th.round(t).Posted, "one enter; the change is part of it")
	require.Equal(t, []string{"Opened"}, th.bodies(t, "TKT-1"))
}

func TestTriggers_CooldownCoalesces(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	th.src.set("ticket", "TKT-1", "", map[string]any{"status": "open", "title": "A"})
	th.reconcile(t)
	th.ticket(1, map[string]any{"status": "open", "title": "B"})
	require.Equal(t, 1, th.round(t).Posted)
	th.ticket(1, map[string]any{"status": "open", "title": "C"})
	require.Zero(t, th.round(t).Posted, "within the cooldown")
	th.ticket(1, map[string]any{"status": "open", "title": "D"})
	require.Zero(t, th.round(t).Posted)
	require.False(t, th.trg.Pending(), "held back until the cooldown ends")
	th.clock.Add(realtimelimits.TriggerCooldown)
	require.True(t, th.trg.Pending())
	require.Equal(t, 1, th.round(t).Posted, "the held changes post once")
	require.Equal(t, []string{"Changed", "Changed"}, th.bodies(t, "TKT-1"))
}

func TestTriggers_RoundCapSharedByRules(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	n := realtimelimits.MaxSystemPostsPerRound
	for i := range n {
		th.src.set("ticket", fmt.Sprintf("TKT-%d", i), "", map[string]any{"status": "new"})
		th.src.set("page", fmt.Sprintf("PAGE-%d", i), "draft", map[string]any{"title": "wip"})
	}
	th.reconcile(t)
	for i := range n {
		th.ticket(i, map[string]any{"status": "open"})
		id := fmt.Sprintf("PAGE-%d", i)
		th.src.set("page", id, "draft", map[string]any{"title": "ready"})
		th.trg.Note(Change{EntityType: "page", ID: id})
	}
	rep := th.round(t)
	require.Equal(t, n, rep.Posted)
	pages := 0
	for i := range n {
		pages += len(th.bodiesIn(t, channels.Key{
			Anchor: channels.Anchor{ID: fmt.Sprintf("PAGE-%d", i), Face: "draft"}, Type: "facechat",
		}))
	}
	require.Equal(t, n/2, pages, "the cap is shared by the rules in turn")
	require.True(t, th.trg.Pending())
	require.Equal(t, n, th.round(t).Posted, "the rest posts next round")
	require.Zero(t, th.round(t).Posted)
}

func TestTriggers_RuleOrder(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	th.src.set("ticket", "TKT-1", "", map[string]any{"status": "new", "title": "A"})
	th.reconcile(t)
	th.ticket(1, map[string]any{"status": "open", "title": "A"})
	th.round(t)
	th.clock.Add(realtimelimits.TriggerCooldown)
	th.ticket(1, map[string]any{"status": "new", "title": "B"})
	require.Equal(t, 1, th.round(t).Posted, "leave posts; change does not, because the rule stopped matching")
	require.Equal(t, []string{"Opened", "No longer open"}, th.bodies(t, "TKT-1"))
}

func TestTriggers_FaceEachPostsPerFace(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	th.src.set("page", "PAGE-1", "draft", map[string]any{"title": "wip"})
	th.src.set("page", "PAGE-1", "published", map[string]any{"title": "ready"})
	th.reconcile(t)
	th.src.set("page", "PAGE-1", "draft", map[string]any{"title": "ready"})
	th.trg.Note(Change{EntityType: "page", ID: "PAGE-1"})
	require.Equal(t, 1, th.round(t).Posted, "only the draft face entered")
	draft := channels.Key{Anchor: channels.Anchor{ID: "PAGE-1", Face: "draft"}, Type: "facechat"}
	require.Equal(t, []string{"Ready"}, th.bodiesIn(t, draft))
}

func TestTriggers_DeleteAndRestore(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	th.src.set("ticket", "TKT-1", "", map[string]any{"status": "open"})
	th.reconcile(t)
	th.ticket(1, nil)
	require.Zero(t, th.round(t).Posted, "a deleted entity does not leave")
	th.clock.Add(realtimelimits.TriggerCooldown)
	th.ticket(1, map[string]any{"status": "open"})
	require.Equal(t, 1, th.round(t).Posted, "a restore enters again")
	require.Equal(t, []string{"Opened"}, th.bodies(t, "TKT-1"))
}

func TestTriggers_DegradedAndOffAreIdle(t *testing.T) {
	for _, tc := range []struct {
		name string
		snap func(t *testing.T) Snapshot
	}{
		{"off", func(*testing.T) Snapshot { return Snapshot{} }},
		{"degraded", func(t *testing.T) Snapshot {
			t.Helper()
			return Snapshot{Config: testCompiled(t, triggerRealtime), Degraded: true}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			th := newTriggerHarness(t, triggerRealtime)
			th.src.set("ticket", "TKT-1", "", map[string]any{"status": "new"})
			th.reconcile(t)
			before := th.kv.keys()
			th.cfg.set(tc.snap(t))
			th.ticket(1, map[string]any{"status": "open"})
			require.Zero(t, th.round(t).Posted)
			require.Zero(t, th.reconcile(t).Posted)
			require.Equal(t, before, th.kv.keys(), "the seen-set is kept")

			th.cfg.set(Snapshot{Config: testCompiled(t, triggerRealtime)})
			require.Equal(t, 1, th.reconcile(t).Posted, "the reconcile after recovery catches up")
		})
	}
}

func TestTriggers_ReconcileCatchesDroppedEvent(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	th.src.set("ticket", "TKT-1", "", map[string]any{"status": "new"})
	th.src.set("ticket", "TKT-2", "", map[string]any{"status": "new"})
	th.reconcile(t)
	th.src.set("ticket", "TKT-2", "", map[string]any{"status": "open"}) // no Note
	require.False(t, th.trg.Pending())
	require.Equal(t, 1, th.reconcile(t).Posted)
	require.Equal(t, []string{"Opened"}, th.bodies(t, "TKT-2"))
	require.Zero(t, th.reconcile(t).Posted, "a second reconcile finds nothing")
}

func TestTriggers_TodayRuleViaReconcile(t *testing.T) {
	y := testRealtime + `
triggers:
  due:
    entity_type: ticket
    on: enter
    when: "days_between(today(), entity.due) == 0"
    channel: room
    post: { type: text, props: { body: "Due" } }
`
	th := newTriggerHarness(t, y)
	th.src.set("ticket", "TKT-1", "", map[string]any{"due": time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)})
	th.reconcile(t)
	require.Empty(t, th.bodies(t, "TKT-1"))
	th.clock.Add(24 * time.Hour)
	rep := th.reconcile(t)
	require.Equal(t, 1, rep.Posted, "%+v", rep)
	require.Equal(t, []string{"Due"}, th.bodies(t, "TKT-1"))
}

func TestTriggers_TwoEnginesOnePost(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	other := th.engine(t, th.kv)
	th.src.set("ticket", "TKT-1", "", map[string]any{"status": "new"})
	th.reconcile(t)
	th.ticket(1, map[string]any{"status": "open"})
	other.Note(Change{EntityType: "ticket", ID: "TKT-1"})
	require.Equal(t, 1, th.round(t).Posted)
	rep, err := other.RunRound(context.Background())
	require.NoError(t, err)
	require.Zero(t, rep.Posted, "the second engine reads what the first saved")
	require.Equal(t, []string{"Opened"}, th.bodies(t, "TKT-1"))
}

func TestTriggers_FailingSaveDoesNotRepost(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	th.src.set("ticket", "TKT-1", "", map[string]any{"status": "new"})
	th.reconcile(t)
	th.kv.failPut = true
	th.ticket(1, map[string]any{"status": "open"})
	require.Equal(t, 1, th.round(t).Posted)
	failed := th.kv.puts.Load()
	th.ticket(1, map[string]any{"status": "open"})
	th.round(t)
	require.Equal(t, failed, th.kv.puts.Load(), "a round that reads the shard still waits for its retry delay")
	for range 3 {
		th.clock.Add(realtimelimits.TriggerRetryMaxDelay)
		th.ticket(1, map[string]any{"status": "open"})
		require.Zero(t, th.round(t).Posted, "the unsaved entry is kept in memory")
	}
	puts := th.kv.puts.Load()
	th.kv.failPut = false
	th.clock.Add(realtimelimits.TriggerRetryMaxDelay)
	th.ticket(1, map[string]any{"status": "open"})
	th.round(t)
	require.Greater(t, th.kv.puts.Load(), puts)
	require.Empty(t, th.trg.lead.dirty, "the shard is saved once the KV is back")
	require.Equal(t, []string{"Opened"}, th.bodies(t, "TKT-1"))
}

func TestTriggers_ResetForgetsLeaderState(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	th.src.set("ticket", "TKT-1", "", map[string]any{"status": "new", "title": "A"})
	th.reconcile(t)
	th.kv.failPut = true
	th.ticket(1, map[string]any{"status": "open", "title": "A"})
	require.Equal(t, 1, th.round(t).Posted)
	th.ticket(1, map[string]any{"status": "open", "title": "B"})
	require.Equal(t, 1, th.round(t).Posted)
	th.ticket(1, map[string]any{"status": "open", "title": "C"})
	th.round(t) // held back by the cooldown
	th.trg.Note(Change{EntityType: "ticket", ID: "TKT-2"})
	require.NotEmpty(t, th.trg.lead.dirty)
	require.NotEmpty(t, th.trg.lead.deferred)
	require.NotEmpty(t, th.trg.lead.lastPost)

	th.trg.Reset()
	require.Empty(t, th.trg.lead.dirty, "a stale unsaved shard is never saved over the next leader's")
	require.Empty(t, th.trg.lead.deferred)
	require.Empty(t, th.trg.lead.lastPost)
	require.Empty(t, th.trg.lead.failures)
	require.Zero(t, th.trg.lead.queued)
	require.False(t, th.trg.Pending())
	th.kv.failPut = false
	require.Equal(t, 1, th.reconcile(t).Posted, "the reconcile finds what was dropped: at least once")
}

// failingAppend fails every Append with err, or a transient error when
// err is nil.
type failingAppend struct {
	channels.Store
	err   error
	calls atomic.Int64
}

func (s *failingAppend) Append(context.Context, channels.Key, channels.NewMessage) (channels.Message, error) {
	s.calls.Add(1)
	if s.err != nil {
		return channels.Message{}, s.err
	}
	return channels.Message{}, errors.New("store down")
}

func (th *triggerHarness) withStore(t *testing.T, st channels.Store) {
	t.Helper()
	trg, err := NewTriggers(TriggerDeps{
		Store: st, Config: th.cfg, Local: th.local, Meta: testMeta(t), Rows: th.src, Seen: th.kv, DueWake: NewDueWake(),
	})
	require.NoError(t, err)
	th.trg = trg
}

func TestTriggers_FailingPostRetriesForAWindow(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	fa := &failingAppend{Store: th.store}
	th.withStore(t, fa)
	th.src.set("ticket", "TKT-1", "", map[string]any{"status": "new"})
	th.reconcile(t)
	th.ticket(1, map[string]any{"status": "open"})
	require.Zero(t, th.round(t).Failed)
	require.Equal(t, int64(1), fa.calls.Load())
	require.False(t, th.trg.Pending(), "the retry waits")
	th.ticket(1, map[string]any{"status": "open"})
	th.round(t)
	require.Equal(t, int64(1), fa.calls.Load(), "a new change does not retry before the delay")

	failed := 0
	start := th.clock.Now()
	for th.clock.Now().Sub(start) <= realtimelimits.TriggerRetryWindow+realtimelimits.TriggerRetryMaxDelay {
		th.clock.Add(realtimelimits.TriggerRetryMaxDelay)
		failed += th.round(t).Failed
	}
	require.Equal(t, 1, failed, "given up once the window passed")
	calls := fa.calls.Load()
	require.Greater(t, calls, int64(3))
	require.Less(t, calls, int64(20), "the delay grows")
	th.clock.Add(realtimelimits.TriggerRetryMaxDelay)
	th.round(t)
	require.Equal(t, calls, fa.calls.Load(), "the change is counted as seen")
	require.False(t, th.trg.Pending())
}

func TestTriggers_PermanentFailureGivesUpAtOnce(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	th.withStore(t, &failingAppend{Store: th.store, err: channels.ErrTooManyChannels})
	th.src.set("ticket", "TKT-1", "", map[string]any{"status": "new"})
	th.reconcile(t)
	th.ticket(1, map[string]any{"status": "open"})
	require.Equal(t, 1, th.round(t).Failed)
	require.False(t, th.trg.Pending())
}

func TestTriggers_FailingStoreStopsTheRound(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	fa := &failingAppend{Store: th.store}
	th.withStore(t, fa)
	for i := range 10 {
		th.src.set("ticket", fmt.Sprintf("TKT-%d", i), "", map[string]any{"status": "new"})
	}
	th.reconcile(t)
	for i := range 10 {
		th.ticket(i, map[string]any{"status": "open"})
	}
	th.round(t)
	require.Equal(t, int64(1), fa.calls.Load(), "one failure stops the round")
	require.True(t, th.trg.Pending(), "the rest wait in the queue")
	th.round(t)
	require.Equal(t, int64(2), fa.calls.Load())
}

func TestTriggers_CapCountsAttempts(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	n := realtimelimits.MaxSystemPostsPerRound + 10
	for i := range n {
		th.src.set("ticket", fmt.Sprintf("TKT-%d", i), "", map[string]any{"status": "new"})
	}
	th.reconcile(t)
	ctx := context.Background()
	for i := range n {
		k := channels.Key{Anchor: channels.Anchor{ID: fmt.Sprintf("TKT-%d", i)}, Type: "room"}
		_, err := th.store.Store.Append(ctx, k, channels.NewMessage{
			Type: "text", Author: systemAuthor(), At: th.clock.Now(), Props: map[string]any{"body": "first"},
		})
		require.NoError(t, err)
		_, err = th.store.SetLifecycle(ctx, k, channels.StateOpen, channels.Transition{
			To: channels.StateClosed, By: systemAuthor(), At: th.clock.Now(),
		})
		require.NoError(t, err)
		th.ticket(i, map[string]any{"status": "open"})
	}
	rep := th.round(t)
	require.Equal(t, realtimelimits.MaxSystemPostsPerRound, rep.Skipped, "a skipped post is an attempt")
	require.Equal(t, 10, th.round(t).Skipped)
}

// cancelAfter cancels its context after the first Append succeeds.
type cancelAfter struct {
	channels.Store
	cancel context.CancelFunc
	calls  atomic.Int64
}

func (s *cancelAfter) Append(ctx context.Context, k channels.Key, nm channels.NewMessage) (channels.Message, error) {
	if s.calls.Add(1) > 1 {
		return channels.Message{}, ctx.Err()
	}
	m, err := s.Store.Append(ctx, k, nm)
	s.cancel()
	return m, err
}

func TestTriggers_CanceledRoundSavesWhatItPosted(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	for i := range 3 {
		th.src.set("ticket", fmt.Sprintf("TKT-%d", i), "", map[string]any{"status": "new"})
	}
	th.reconcile(t)
	ctx, cancel := context.WithCancel(context.Background())
	th.withStore(t, &cancelAfter{Store: th.store, cancel: cancel})
	for i := range 3 {
		th.ticket(i, map[string]any{"status": "open"})
	}
	rep, err := th.trg.RunRound(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, rep.Posted)

	next := th.engine(t, th.kv) // the next leader
	require.Equal(t, 2, func() int {
		rep, err := next.RunReconcile(context.Background())
		require.NoError(t, err)
		return rep.Posted
	}(), "the post made before shutdown was saved, so only the other two post")
	for i := range 3 {
		require.Equal(t, []string{"Opened"}, th.bodies(t, fmt.Sprintf("TKT-%d", i)))
	}
}

func TestTriggers_QueueIsNotReadAgain(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	n := 3 * realtimelimits.MaxSystemPostsPerRound
	for i := range n {
		th.src.set("ticket", fmt.Sprintf("TKT-%d", i), "", map[string]any{"status": "new"})
	}
	th.reconcile(t)
	for i := range n {
		th.ticket(i, map[string]any{"status": "open"})
	}
	reads := th.src.reads.Load()
	posted := 0
	for th.trg.Pending() {
		posted += th.round(t).Posted
	}
	require.Equal(t, n, posted)
	require.Equal(t, reads+1, th.src.reads.Load(), "the backlog is read once, not once per round")
}

// A round's cost does not grow with the number of changed ids: one read
// per type, the index and the shards the ids fall in, and at most the
// size of those shards written.
func TestTriggers_RoundCostIsPerChange(t *testing.T) {
	type cost struct{ gets, puts, bytes, reads int64 }
	measure := func(changed int) (cost, int64) {
		th := newTriggerHarness(t, triggerRealtime)
		for i := range 500 {
			th.src.set("ticket", fmt.Sprintf("TKT-%d", i), "", map[string]any{"status": "open"})
		}
		th.reconcile(t)
		before := cost{th.kv.gets.Load(), th.kv.puts.Load(), th.kv.bytes.Load(), th.src.reads.Load()}
		for i := range changed {
			th.ticket(i, map[string]any{"status": "done"})
		}
		require.Equal(t, changed, th.round(t).Posted)
		c := cost{
			th.kv.gets.Load() - before.gets, th.kv.puts.Load() - before.puts,
			th.kv.bytes.Load() - before.bytes, th.src.reads.Load() - before.reads,
		}
		var seen int64
		for _, k := range th.kv.keys() {
			if k != seenIndexKey {
				v, err := th.kv.Get(context.Background(), k)
				require.NoError(t, err)
				seen += int64(len(v))
			}
		}
		return c, seen
	}
	c10, seen := measure(10)
	c50, _ := measure(50)
	require.Equal(t, c10.reads, c50.reads, "one read per type, whatever the number of ids")
	require.Equal(t, c10.gets, c50.gets, "the index and the shards the ids fall in")
	require.Equal(t, c10.puts, c50.puts)
	require.LessOrEqual(t, c10.gets, int64(1+3), "the index and one shard per ticket rule")
	require.LessOrEqual(t, c50.bytes, 2*seen, "a round writes the shards it touched, nothing more")
}

func TestTriggers_QueueIsFirstComeFirstServed(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	n := realtimelimits.MaxSystemPostsPerRound + 1
	for i := range n + 1 {
		th.src.set("ticket", fmt.Sprintf("TKT-%d", i+100), "", map[string]any{"status": "new"})
	}
	th.src.set("ticket", "TKT-0", "", map[string]any{"status": "new"})
	th.reconcile(t)
	for i := range n {
		th.ticket(i+100, map[string]any{"status": "open"})
	}
	require.Equal(t, realtimelimits.MaxSystemPostsPerRound, th.round(t).Posted)
	th.ticket(0, map[string]any{"status": "open"}) // sorts first, came last
	require.Equal(t, 2, th.round(t).Posted)
	last := fmt.Sprintf("TKT-%d", n-1+100)
	require.Equal(t, []string{"Opened"}, th.bodies(t, last), "the old leftover was not starved")
	require.Equal(t, []string{"Opened"}, th.bodies(t, "TKT-0"))
}

func TestTriggers_CorruptIndexStartsOver(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	th.src.set("ticket", "TKT-1", "", map[string]any{"status": "open"})
	th.reconcile(t)
	var opened string
	for _, tr := range th.cfg.Load().Config.Triggers() {
		if tr.Name == "opened" {
			opened = ruleID(tr, FacePrime)
		}
	}
	old := shardKey(opened, 4, 3) // a layout the lost index once recorded
	require.NoError(t, th.kv.Put(context.Background(), old, []byte(`{"TKT-9":"x"}`)))
	require.NoError(t, th.kv.Put(context.Background(), seenIndexKey, []byte("{")))
	th.ticket(1, map[string]any{"status": "open"})
	rep := th.round(t)
	require.NotContains(t, th.kv.keys(), old, "old layouts of current rules are deleted")
	require.Equal(t, 4, rep.Baselined, "every rule baselines again")
	require.Zero(t, rep.Posted, "silently")
	th.ticket(1, map[string]any{"status": "new"})
	require.Equal(t, 1, th.round(t).Posted, "and the rules work again")
}

func TestTriggers_CorruptShardResetsOneRule(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	th.src.set("ticket", "TKT-1", "", map[string]any{"status": "new", "title": "A"})
	th.src.set("ticket", "TKT-2", "", map[string]any{"status": "open", "title": "A"})
	th.reconcile(t)
	var opened string
	for _, tr := range th.cfg.Load().Config.Triggers() {
		if tr.Name == "opened" {
			opened = ruleID(tr, FacePrime)
		}
	}
	require.NoError(t, th.kv.Put(context.Background(), shardKey(opened, 1, 0), []byte("[")))
	th.ticket(1, map[string]any{"status": "open", "title": "A"})
	th.ticket(2, map[string]any{"status": "new", "title": "A"})
	rep := th.round(t)
	require.Equal(t, 1, rep.Posted, "the other rules carry on: shut posts")
	require.Equal(t, []string{"No longer open"}, th.bodies(t, "TKT-2"))
	require.Empty(t, th.bodies(t, "TKT-1"), "the corrupt rule posts nothing")
	require.NotContains(t, th.kv.keys(), shardKey(opened, 1, 0), "the corrupt shard is deleted")
	require.True(t, th.trg.Pending())
	require.Equal(t, 1, th.round(t).Baselined, "and baselines again")
	th.clock.Add(realtimelimits.TriggerCooldown)
	th.ticket(2, map[string]any{"status": "open", "title": "A"})
	require.Equal(t, 1, th.round(t).Posted)
	require.Equal(t, []string{"Opened"}, th.bodies(t, "TKT-2")[1:])
}

func TestTriggers_PruneDropsStaleFailures(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	th.trg.lead.failures["k"] = &failure{first: th.clock.Now()}
	th.trg.lead.lastPost["k"] = th.clock.Now()
	th.clock.Add(2 * realtimelimits.TriggerRetryWindow)
	th.round(t)
	require.Empty(t, th.trg.lead.failures)
	require.Empty(t, th.trg.lead.lastPost)
}

func TestTriggers_WhenClosed(t *testing.T) {
	for _, tc := range []struct {
		name, mode string
		want       []string
	}{
		{"skip", "", []string{"first"}},
		{"post", "\n    when_closed: post", []string{"first", "Opened"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			y := testRealtime + `
triggers:
  opened:
    entity_type: ticket
    on: enter
    when: "entity.status == 'open'"
    channel: room` + tc.mode + `
    post: { type: text, props: { body: "Opened" } }
`
			th := newTriggerHarness(t, y)
			ctx := context.Background()
			_, err := th.store.Store.Append(ctx, roomKey(), channels.NewMessage{
				Type: "text", Author: systemAuthor(), At: th.clock.Now(), Props: map[string]any{"body": "first"},
			})
			require.NoError(t, err)
			_, err = th.store.SetLifecycle(ctx, roomKey(), channels.StateOpen, channels.Transition{
				To: channels.StateClosed, By: systemAuthor(), At: th.clock.Now(),
			})
			require.NoError(t, err)
			th.src.set("ticket", "TKT-1", "", map[string]any{"status": "new"})
			th.reconcile(t)
			th.ticket(1, map[string]any{"status": "open"})
			rep := th.round(t)
			require.Equal(t, tc.want, th.bodies(t, "TKT-1"))
			if tc.mode == "" {
				require.Equal(t, 1, rep.Skipped)
				th.ticket(1, map[string]any{"status": "open"})
				require.Zero(t, th.round(t).Posted, "a skipped post counts as seen")
			}
		})
	}
}

func TestTriggers_PendingOverflowComparesInFull(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	th.src.set("ticket", "TKT-1", "", map[string]any{"status": "new"})
	th.reconcile(t)
	th.src.set("ticket", "TKT-1", "", map[string]any{"status": "open"})
	for i := range realtimelimits.MaxTriggerPending + 1 {
		th.trg.Note(Change{EntityType: "ticket", ID: fmt.Sprintf("TKT-X%d", i)})
	}
	th.trg.Note(Change{EntityType: "doc", ID: "DOC-1"})
	require.True(t, th.trg.full["ticket"])
	require.Zero(t, th.trg.npending, "an untriggered type is dropped")
	scans := th.src.scans.Load()
	require.Equal(t, 1, th.round(t).Posted)
	require.Equal(t, scans+1, th.src.scans.Load(), "one full scan")
}

func TestTriggers_OverflowFlagsTheBusiestType(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	for i := range realtimelimits.MaxTriggerPending {
		th.trg.Note(Change{EntityType: "ticket", ID: fmt.Sprintf("TKT-X%d", i)})
	}
	th.trg.Note(Change{EntityType: "page", ID: "PAGE-1"})
	require.True(t, th.trg.full["ticket"], "the type holding the most changes is compared in full")
	require.False(t, th.trg.full["page"])
	require.Contains(t, th.trg.pending["page"], "PAGE-1", "the new change is kept")
}

func TestTriggers_ReadFailureRetries(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	th.src.set("ticket", "TKT-1", "", map[string]any{"status": "new"})
	th.reconcile(t)
	th.ticket(1, map[string]any{"status": "open"})
	th.src.fail = errors.New("db down")
	_, err := th.trg.RunRound(context.Background())
	require.Error(t, err)
	require.True(t, th.trg.Pending(), "the changes are kept for the next round")
	th.src.fail = nil
	require.Equal(t, 1, th.round(t).Posted)

	th.ticket(1, map[string]any{"status": "new"})
	th.src.fail = errors.New("db down")
	for range 3 {
		th.clock.Add(realtimelimits.TriggerRetryWindow / 2)
		_, err = th.trg.RunRound(context.Background())
		require.Error(t, err)
	}
	require.False(t, th.trg.Pending(), "after the window the changes are left to the reconcile")
}

func TestTriggers_Reshard(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	n := realtimelimits.SeenPerShard + 1
	for i := range n {
		th.src.set("ticket", fmt.Sprintf("TKT-%d", i), "", map[string]any{"status": "open"})
	}
	th.reconcile(t)
	idx, err := seenStore{kv: th.kv}.loadIndex(context.Background())
	require.NoError(t, err)
	var id string
	for _, tr := range th.cfg.Load().Config.Triggers() {
		if tr.Name == "opened" {
			id = ruleID(tr, FacePrime)
		}
	}
	require.Contains(t, idx.Rules, id)
	require.Equal(t, 2, idx.Rules[id].Shards, "the baseline splits a large rule")

	for i := range n - 10 {
		th.src.set("ticket", fmt.Sprintf("TKT-%d", i), "", nil)
	}
	require.Zero(t, th.reconcile(t).Posted)
	idx, err = seenStore{kv: th.kv}.loadIndex(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, idx.Rules[id].Shards, "a shrunk rule is joined")
	for _, k := range th.kv.keys() {
		require.NotContains(t, k, id+"/2-", "the old shards are deleted")
	}
}

// A post waiting in the queue is worked out again when its entity is read
// in full, so a change undone by a lost event posts nothing.
func TestTriggers_StaleQueuedPostIsDropped(t *testing.T) {
	for _, tc := range []struct {
		name   string
		undo   func(th *triggerHarness, id string)
		repair func(t *testing.T, th *triggerHarness) TriggerReport
	}{
		// A revert still posts the leave of the shut trigger, whose
		// seen-set recorded the entity as open: only the stale enter goes.
		{"deleted, overflow", func(th *triggerHarness, id string) { th.src.set("ticket", id, "", nil) },
			func(t *testing.T, th *triggerHarness) TriggerReport {
				t.Helper()
				for i := range realtimelimits.MaxTriggerPending + 1 {
					th.trg.Note(Change{EntityType: "ticket", ID: fmt.Sprintf("TKT-X%d", i)})
				}
				require.True(t, th.trg.full["ticket"])
				return th.round(t)
			}},
		{"deleted, reconcile", func(th *triggerHarness, id string) { th.src.set("ticket", id, "", nil) },
			func(t *testing.T, th *triggerHarness) TriggerReport { t.Helper(); return th.reconcile(t) }},
		{"reverted, reconcile", func(th *triggerHarness, id string) {
			th.src.set("ticket", id, "", map[string]any{"status": "new"})
		}, func(t *testing.T, th *triggerHarness) TriggerReport { t.Helper(); return th.reconcile(t) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			th := newTriggerHarness(t, triggerRealtime)
			n := realtimelimits.MaxSystemPostsPerRound + 1
			for i := range n {
				th.src.set("ticket", fmt.Sprintf("TKT-%d", i), "", map[string]any{"status": "new"})
			}
			th.reconcile(t)
			for i := range n {
				th.ticket(i, map[string]any{"status": "open"})
			}
			require.Equal(t, n-1, th.round(t).Posted)
			var last string
			for i := range n {
				if id := fmt.Sprintf("TKT-%d", i); len(th.bodies(t, id)) == 0 {
					last = id
				}
			}
			require.NotEmpty(t, last, "one post waits")
			tc.undo(th, last) // the event is lost
			tc.repair(t, th)
			require.Zero(t, th.reconcile(t).Posted)
			require.NotContains(t, th.bodies(t, last), "Opened", "the stale enter is not posted")
			require.Zero(t, th.trg.lead.queued)
		})
	}
}

// An error about one post's key does not stop the round for the others.
func TestTriggers_PerKeyErrorIsPermanent(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	fa := &failingAppend{Store: th.store, err: channels.ErrStaleAnchor}
	th.withStore(t, fa)
	for i := range 2 {
		th.src.set("ticket", fmt.Sprintf("TKT-%d", i), "", map[string]any{"status": "new"})
	}
	th.reconcile(t)
	for i := range 2 {
		th.ticket(i, map[string]any{"status": "open"})
	}
	require.Equal(t, 2, th.round(t).Failed)
	require.Equal(t, int64(2), fa.calls.Load())
	require.False(t, th.trg.Pending())
}

// A post held back by its cooldown keeps its place in the queue, ahead of
// changes noted after it.
func TestTriggers_CooldownKeepsQueuePlace(t *testing.T) {
	th := newTriggerHarness(t, triggerRealtime)
	n := 2 * realtimelimits.MaxSystemPostsPerRound
	for i := range n + 1 {
		th.src.set("ticket", fmt.Sprintf("TKT-%d", i), "", map[string]any{"status": "open", "title": "A"})
	}
	th.reconcile(t)
	th.ticket(0, map[string]any{"status": "open", "title": "B"})
	require.Equal(t, 1, th.round(t).Posted)
	th.clock.Add(time.Second)
	th.ticket(0, map[string]any{"status": "open", "title": "C"})
	th.clock.Add(time.Second)
	for i := range n {
		th.ticket(i+1, map[string]any{"status": "open", "title": "B"})
	}
	require.Equal(t, realtimelimits.MaxSystemPostsPerRound, th.round(t).Posted)
	th.clock.Add(realtimelimits.TriggerCooldown)
	require.Equal(t, realtimelimits.MaxSystemPostsPerRound, th.round(t).Posted)
	require.Equal(t, []string{"Changed", "Changed"}, th.bodies(t, "TKT-0"), "it went before the later changes")
	waiting := 0
	for i := range n {
		if len(th.bodies(t, fmt.Sprintf("TKT-%d", i+1))) == 0 {
			waiting++
		}
	}
	require.Equal(t, 1, waiting, "one later change still waits")
}

// sessionTriggers is the incident session type with two triggers: one
// posting escalated (by: trigger) and one posting contained (roles: lead).
func sessionTriggers(whenClosed string) string {
	return sessionRealtime("detected", sessionStates) + `
triggers:
  escalate:
    entity_type: ticket
    on: enter
    when: "entity.status == 'escalated'"
    channel: incident` + whenClosed + `
    post: { type: escalated, props: { note: "Escalated" } }
  contain:
    entity_type: ticket
    on: enter
    when: "entity.status == 'contained'"
    channel: incident
    post: { type: contained, props: { note: "Contained" } }
`
}

func (th *triggerHarness) incidentSession(t *testing.T) channels.Session {
	t.Helper()
	c, err := th.store.Store.Head(context.Background(), incidentKey())
	require.NoError(t, err)
	return c.Session
}

func TestSessionTriggerPost(t *testing.T) {
	t.Run("by trigger moves", func(t *testing.T) {
		th := newTriggerHarness(t, sessionTriggers(""))
		th.src.set("ticket", "TKT-1", "", map[string]any{"status": "new"})
		th.reconcile(t)
		th.ticket(1, map[string]any{"status": "escalated"})
		require.Equal(t, 1, th.round(t).Posted)
		require.Equal(t, "contained", th.incidentSession(t).State)
		audit := th.sessionAudit(t)
		require.Len(t, audit, 1)
		require.Equal(t, systemAuthor(), audit[0].By)
		require.Equal(t, "trigger:escalate", audit[0].TriggeredBy)
		msgs, err := th.store.Store.Messages(context.Background(), incidentKey(), channels.Page{})
		require.NoError(t, err)
		require.Len(t, msgs, 2)
		require.Equal(t, "session:incident", msgs[1].TriggeredBy)
	})
	t.Run("roles never admit a trigger", func(t *testing.T) {
		th := newTriggerHarness(t, sessionTriggers(""))
		th.src.set("ticket", "TKT-1", "", map[string]any{"status": "new"})
		th.reconcile(t)
		th.ticket(1, map[string]any{"status": "contained"})
		require.Equal(t, 1, th.round(t).Posted)
		require.True(t, th.incidentSession(t).IsZero())
		require.Empty(t, th.sessionAudit(t))
	})
}

func TestSessionTriggerWhenClosed(t *testing.T) {
	th := newTriggerHarness(t, sessionTriggers("\n    when_closed: post"))
	ctx := context.Background()
	_, err := th.store.Store.Append(ctx, incidentKey(), channels.NewMessage{
		Type: "text", Author: systemAuthor(), At: th.clock.Now(), Props: map[string]any{"body": "first"},
	})
	require.NoError(t, err)
	_, err = th.store.SetLifecycle(ctx, incidentKey(), channels.StateOpen, channels.Transition{
		To: channels.StateClosed, By: systemAuthor(), At: th.clock.Now(),
	})
	require.NoError(t, err)
	th.src.set("ticket", "TKT-1", "", map[string]any{"status": "new"})
	th.reconcile(t)
	th.ticket(1, map[string]any{"status": "escalated"})
	require.Equal(t, 1, th.round(t).Posted)
	msgs, err := th.store.Store.Messages(ctx, incidentKey(), channels.Page{})
	require.NoError(t, err)
	require.Len(t, msgs, 2, "the post lands; no notice follows")
	require.Equal(t, "escalated", msgs[1].Type)
	require.True(t, th.incidentSession(t).IsZero())
}

// TestSessionTriggerPost_SetsDue pins that a trigger post on a timer type
// stores the session's due time and notes it on the DueWake.
func TestSessionTriggerPost_SetsDue(t *testing.T) {
	th := newTriggerHarness(t, timerRealtime+`
triggers:
  open:
    entity_type: ticket
    on: enter
    when: "entity.status == 'breached'"
    channel: breach-response
    post: { type: text, props: { body: "Breach reported" } }
`)
	th.src.set("ticket", "TKT-1", "", map[string]any{"status": "new"})
	th.reconcile(t)
	th.ticket(1, map[string]any{"status": "breached"})
	require.Equal(t, 1, th.round(t).Posted)
	c, err := th.store.Store.Head(context.Background(), breachKey("TKT-1"))
	require.NoError(t, err)
	require.Equal(t, t0.Add(24*time.Hour), c.Session.NextDueAt)
	require.Equal(t, t0.Add(24*time.Hour), th.wake.Take())
}

// pocTriggers is the PoC's ready trigger, which posts go (in: [new]).
func pocTriggers(whenClosed string) string {
	return pocRealtime(`
triggers:
  ready:
    entity_type: ticket
    on: enter
    when: "entity.status == 'ready'"
    channel: run` + whenClosed + `
    post: { type: go, props: { note: "Moved to Ready on the board." } }
`)
}

// TestSessionTriggerPost_MessageIn pins AC 6: a trigger whose message type
// has in: is skipped while the session is elsewhere: nothing is stored and
// Skipped counts it. In the type's state it posts and moves the session.
func TestSessionTriggerPost_MessageIn(t *testing.T) {
	t.Run("posts in its state", func(t *testing.T) {
		th := newTriggerHarness(t, pocTriggers(""))
		th.src.set("ticket", "TKT-1", "", map[string]any{"status": "new"})
		th.reconcile(t)
		th.ticket(1, map[string]any{"status": "ready"})
		rep := th.round(t)
		require.Equal(t, 1, rep.Posted)
		head, err := th.store.Store.Head(context.Background(), pocKey())
		require.NoError(t, err)
		require.Equal(t, "queued", head.Session.State)
	})
	t.Run("skipped elsewhere", func(t *testing.T) {
		th := newTriggerHarness(t, pocTriggers(""))
		th.pocDriveTo(t, "working")
		before, err := th.store.Store.Head(context.Background(), pocKey())
		require.NoError(t, err)
		th.src.set("ticket", "TKT-1", "", map[string]any{"status": "new"})
		th.reconcile(t)
		th.ticket(1, map[string]any{"status": "ready"})
		// The test is not parallel, so no other test logs meanwhile.
		logs := logtest.Capture(t, slog.LevelWarn)
		rep := th.round(t)
		require.Equal(t, 0, rep.Posted)
		require.Equal(t, 1, rep.Skipped)
		require.Zero(t, rep.Failed)
		require.Contains(t, logs(), "level=WARN")
		require.Contains(t, logs(), "in=channel_types.run.message_types.go.in")
		after, err := th.store.Store.Head(context.Background(), pocKey())
		require.NoError(t, err)
		require.Equal(t, before.LastSeq, after.LastSeq, "a skipped trigger stored a message")
		require.Equal(t, "working", after.Session.State)
	})
	t.Run("when_closed post into a final state outside in", func(t *testing.T) {
		th := newTriggerHarness(t, pocTriggers("\n    when_closed: post"))
		th.pocDriveTo(t, "done")
		_, err := th.svc.Lifecycle(as(pocOwner()), pocRun, realtimecfg.LifecycleClose)
		require.NoError(t, err)
		before, err := th.store.Store.Head(context.Background(), pocKey())
		require.NoError(t, err)
		th.src.set("ticket", "TKT-1", "", map[string]any{"status": "new"})
		th.reconcile(t)
		th.ticket(1, map[string]any{"status": "ready"})
		rep := th.round(t)
		require.Equal(t, 1, rep.Skipped)
		require.Zero(t, rep.Posted)
		after, err := th.store.Store.Head(context.Background(), pocKey())
		require.NoError(t, err)
		require.Equal(t, before.LastSeq, after.LastSeq)
	})
}
