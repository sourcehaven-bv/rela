package realtime

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/filechannels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

const testSchema = `
version: "1.0"
types:
  severity:
    values: [low, high]
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    properties:
      title: {type: string}
      owner: {type: string}
      watchers: {type: string, list: true}
      status: {type: string}
  doc:
    label: Doc
    id_prefix: DOC
    properties:
      title: {type: string}
  page:
    label: Page
    id_prefix: PAGE
    properties:
      title: {type: string}
    faces:
      draft: {}
      published: {}
`

const testRealtime = `
version: 1
limits:
  page_limit: 50
  channels_per_entity: 3
channel_types:
  room:
    label: Room
    attach_to: [ticket]
    cardinality: one
    read: { from: attached }
    roles:
      member:  { from: attached, label: member }
      poster:  { permission: chat.post }
      owner:   { from: entity.owner }
      watcher: { from: entity.watchers }
      agent:   { principal_type: agent }
      mod:     { permission: chat.moderate, label: moderator }
    message_types:
      text:
        properties: { body: { type: markdown, required: true, max: 100 } }
        post: [member]
      perm-text:
        properties: { body: { type: string } }
        post: [poster]
      owner-text:
        properties: { body: { type: string } }
        post: [owner]
      watcher-text:
        properties: { body: { type: string } }
        post: [watcher]
      agent-text:
        properties: { body: { type: string } }
        post: [agent]
      note:
        properties:
          level: { type: integer, required: true }
          about: { type: ref, to: [ticket] }
          also:  { type: ref, to: [ticket, doc] }
          on:    { type: date }
          at:    { type: datetime }
          ok:    { type: boolean }
          pick:  { type: severity }
        when: "message.level > 0"
        post: [member]
      typing: { persist: ephemeral, ttl: 5s, post: [member] }
      cursor:
        persist: ephemeral
        properties: { at: { type: ref, to: [ticket, doc] } }
        post: [member]
    message_ops:
      edit_own:   { roles: [member], within: 15m }
      delete_own: { roles: [member], from: open }
      remove:     { roles: [mod], reason_required: true }
  threads:
    attach_to: [ticket, doc]
    cardinality: many
    read: { creator: true }
    create: { roles: [member] }
    roles:
      member: { from: attached }
    message_types:
      text:
        properties: { body: { type: string } }
        post: [member]
  topics:
    attach_to: [ticket]
    cardinality: many
    read: { permission: chat.read }
    create: { roles: [member] }
    roles:
      member: { from: attached }
    message_types:
      text:
        properties: { body: { type: string } }
        post: [member]
  facechat:
    attach_to: [page]
    anchor: face
    cardinality: one
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      text:
        properties: { body: { type: string } }
        post: [member]
`

func testMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	meta, err := metamodel.Parse([]byte(testSchema))
	require.NoError(t, err)
	return meta
}

func testCompiled(t *testing.T, yaml string) *realtimecfg.Compiled {
	t.Helper()
	meta := testMeta(t)
	c, err := realtimecfg.Parse([]byte(yaml), meta, nil)
	require.NoError(t, err)
	return c
}

// fakeAccess is a configurable Access.
type fakeAccess struct {
	p       principal.Principal
	q       predicatefns.QueryIdentity
	perms   map[string]bool
	ceiling bool
}

func (a *fakeAccess) Principal() principal.Principal            { return a.p }
func (a *fakeAccess) QueryIdentity() predicatefns.QueryIdentity { return a.q }
func (a *fakeAccess) HoldsPermissionFor(_ context.Context, _, perm string) bool {
	return a.perms[perm]
}
func (a *fakeAccess) CeilingAdmitsUpdate(string) bool { return a.ceiling }

// accessKey carries the fakeAccess of a test call on the context.
type accessKey struct{}

func as(a *fakeAccess) context.Context {
	return context.WithValue(context.Background(), accessKey{}, a)
}

// accessErrKey carries an error the fake access source fails with.
type accessErrKey struct{}

// failingAccess is a context whose access source fails with err.
func failingAccess(err error) context.Context {
	return context.WithValue(context.Background(), accessErrKey{}, err)
}

type fakeAccessSource struct{}

func (fakeAccessSource) For(ctx context.Context) (context.Context, Access, error) {
	if err, ok := ctx.Value(accessErrKey{}).(error); ok {
		return nil, nil, err
	}
	a, ok := ctx.Value(accessKey{}).(*fakeAccess)
	if !ok {
		return nil, nil, fmt.Errorf("%w: unstamped", ErrNoPrincipal)
	}
	return ctx, a, nil
}

// user returns a stamped caller with the given permissions whose client
// ceiling admits updates.
func user(name string, perms ...string) *fakeAccess {
	m := map[string]bool{}
	for _, p := range perms {
		m[p] = true
	}
	return &fakeAccess{
		p:       principal.Principal{User: name, Tool: principal.ToolDataEntry},
		q:       predicatefns.QueryIdentity{Raw: name},
		perms:   m,
		ceiling: true,
	}
}

// fakeAnchors serves anchor views. With meta set it binds the record a
// lifecycle condition reads from the returned properties, as the wiring
// site does.
type fakeAnchors struct {
	meta    *metamodel.Metamodel
	mu      sync.Mutex
	views   map[string]AnchorView // by "ID" (served face) and "ID@face"
	calls   atomic.Int64
	onRead  func()
	lastReq []string
}

func (f *fakeAnchors) Anchor(_ context.Context, addr entity.Address, props []string) (AnchorView, error) {
	f.calls.Add(1)
	if f.onRead != nil {
		f.onRead()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastReq = props
	v, ok := f.views[addr.String()]
	if !ok {
		return AnchorView{}, ErrAnchorNotFound
	}
	out := v
	out.Props = map[string]any{}
	for _, p := range props {
		if val, ok := v.Props[p]; ok {
			out.Props[p] = val
		}
	}
	if def, ok := f.entityDef(v.Type); ok {
		out.Record = predicatefns.EntityRecord(f.meta, def, v.ID, v.Type, out.Props)
	}
	return out, nil
}

// setStatus sets the status of TKT-1; nil removes it.
func (f *fakeAnchors) setStatus(v any) {
	const addr, prop = "TKT-1", "status"
	f.mu.Lock()
	defer f.mu.Unlock()
	view := f.views[addr]
	props := map[string]any{}
	maps.Copy(props, view.Props)
	if v == nil {
		delete(props, prop)
	} else {
		props[prop] = v
	}
	view.Props = props
	f.views[addr] = view
}

func (f *fakeAnchors) entityDef(typ string) (*metamodel.EntityDef, bool) {
	if f.meta == nil {
		return nil, false
	}
	return f.meta.GetEntityDef(typ)
}

type fakeRefs struct {
	mu    sync.Mutex
	views map[string]RefView
	calls atomic.Int64
}

func (f *fakeRefs) Resolve(_ context.Context, ids []string) (map[string]RefView, error) {
	f.calls.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]RefView{}
	for _, id := range ids {
		if v, ok := f.views[id]; ok {
			out[id] = v
		}
	}
	return out, nil
}

type fakeConfig struct {
	mu   sync.Mutex
	snap Snapshot
}

func (f *fakeConfig) Load() Snapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.snap
}

func (f *fakeConfig) set(s Snapshot) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.snap = s
}

// testClock is a settable clock.
type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// countingStore counts the channel store calls a test cares about.
type countingStore struct {
	channels.Store
	heads    atomic.Int64
	appends  atomic.Int64
	records  atomic.Int64
	messages atomic.Int64
	typeRecs atomic.Int64
	onHead   func()
	// onRecord runs before RecordTypes.
	onRecord func()
	// onDelete runs before Delete.
	onDelete func()
}

func (s *countingStore) Delete(ctx context.Context, k channels.Key, d channels.Deletion) error {
	if s.onDelete != nil {
		s.onDelete()
	}
	return s.Store.Delete(ctx, k, d)
}

func (s *countingStore) Head(ctx context.Context, k channels.Key) (channels.Channel, error) {
	s.heads.Add(1)
	if s.onHead != nil {
		s.onHead()
	}
	return s.Store.Head(ctx, k)
}

func (s *countingStore) Messages(ctx context.Context, k channels.Key, p channels.Page) ([]channels.Message, error) {
	s.messages.Add(1)
	return s.Store.Messages(ctx, k, p)
}

func (s *countingStore) TypeRecords(ctx context.Context) (map[string]channels.TypeRecord, error) {
	s.typeRecs.Add(1)
	return s.Store.TypeRecords(ctx)
}

func (s *countingStore) Append(ctx context.Context, k channels.Key, m channels.NewMessage) (channels.Message, error) {
	s.appends.Add(1)
	return s.Store.Append(ctx, k, m)
}

func (s *countingStore) RecordTypes(ctx context.Context, r map[string]channels.TypeRecord) error {
	s.records.Add(1)
	if s.onRecord != nil {
		s.onRecord()
	}
	return s.Store.RecordTypes(ctx, r)
}

type harness struct {
	svc     *Service
	store   *countingStore
	anchors *fakeAnchors
	refs    *fakeRefs
	cfg     *fakeConfig
	clock   *testClock
	local   *Local
}

type harnessOpt func(*harnessOpts)

type harnessOpts struct {
	yaml string
	lim  ephemeralLimits
	caps limitCaps
}

func withEphemeral(l ephemeralLimits) harnessOpt { return func(o *harnessOpts) { o.lim = l } }
func withBuckets(n int) harnessOpt               { return func(o *harnessOpts) { o.caps.buckets = n } }
func withLifecycleYAML() harnessOpt              { return func(o *harnessOpts) { o.yaml = lifecycleRealtime } }

func newHarness(t *testing.T, opts ...harnessOpt) *harness {
	t.Helper()
	o := harnessOpts{yaml: testRealtime, lim: defaultEphemeralLimits(), caps: defaultLimitCaps()}
	for _, f := range opts {
		f(&o)
	}
	fs, err := filechannels.New(storage.NewMemFS(), "/channels", filelock.ProcessPrivate{})
	require.NoError(t, err)
	st := &countingStore{Store: fs}
	clock := &testClock{now: time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)}
	local, err := newLocal(clock.Now, o.lim, o.caps)
	require.NoError(t, err)
	t.Cleanup(local.Stop)
	h := &harness{
		store: st, clock: clock, local: local,
		anchors: &fakeAnchors{meta: testMeta(t), views: map[string]AnchorView{
			"TKT-1": {ID: "TKT-1", Type: "ticket", Props: map[string]any{
				"owner": "alice", "watchers": []any{"carol", "dave"},
			}},
			"TKT-2":            {ID: "TKT-2", Type: "ticket"},
			"DOC-1":            {ID: "DOC-1", Type: "doc"},
			"PAGE-1":           {ID: "PAGE-1", Face: "published", Type: "page"},
			"PAGE-1@published": {ID: "PAGE-1", Face: "published", Type: "page"},
			"PAGE-1@draft":     {ID: "PAGE-1", Face: "draft", Type: "page"},
		}},
		refs: &fakeRefs{views: map[string]RefView{
			"TKT-2": {ID: "TKT-2", Type: "ticket", Title: "Second"},
			"DOC-1": {ID: "DOC-1", Type: "doc", Title: "A doc"},
		}},
		cfg: &fakeConfig{snap: Snapshot{Config: testCompiled(t, o.yaml)}},
	}
	h.svc, err = New(Deps{
		Store: st, Config: h.cfg, Access: fakeAccessSource{}, Anchors: h.anchors, Refs: h.refs, Local: local,
	})
	require.NoError(t, err)
	return h
}

// room is the cardinality-one room channel of TKT-1.
var room = RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "room"}

func roomKey() channels.Key {
	return channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "room"}
}

func text(body string) PostInput {
	return PostInput{Type: "text", Props: map[string]any{"body": body}}
}

// takeKeys returns the keys of a Take and fails on a resync.
func takeKeys(t *testing.T, sub *Subscription) []channels.Key {
	t.Helper()
	keys, resync := sub.Take()
	require.False(t, resync, "unexpected resync")
	return keys
}

// requireDenial asserts err is a *DenialError with status and type.
func requireDenial(t *testing.T, err error, status int, typ string) *DenialError {
	t.Helper()
	d, ok := errors.AsType[*DenialError](err)
	require.Truef(t, ok, "want a *DenialError, got %v", err)
	require.Equal(t, status, d.Status, d.Error())
	require.Equal(t, typ, d.Type, d.Error())
	return d
}

func (h *harness) post(t *testing.T, a *fakeAccess, in PostInput) MessageView {
	t.Helper()
	res, err := h.svc.Post(as(a), room, in)
	require.NoError(t, err)
	require.NotNil(t, res.Message)
	return *res.Message
}
