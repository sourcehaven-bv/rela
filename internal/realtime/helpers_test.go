package realtime

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
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
  risk-level:
    values: [none, low, high]
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    properties:
      title: {type: string}
      owner: {type: string}
      watchers: {type: string, list: true}
      status: {type: string}
      due: {type: date}
  doc:
    label: Doc
    id_prefix: DOC
    properties:
      title: {type: string}
  person:
    label: Person
    id_prefix: USR
    properties:
      title: {type: string}
  page:
    label: Page
    id_prefix: PAGE
    properties:
      title: {type: string}
      owner: {type: string}
    faces:
      draft: {}
      published: {}
relations:
  about:
    from: [doc]
    to: [ticket]
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

// typingRealtime has channel types with the built-in typing indicator:
// chat (one), rooms (many), and plain without typing. Readers need
// chat.read; log posts need chat.post (text) or chat.lead (note); any
// attached reader may post the operator signal status.
const typingRealtime = `
version: 1
channel_types:
  chat:
    attach_to: [ticket]
    typing: true
    read: { permission: chat.read }
    roles:
      member: { from: attached }
      poster: { permission: chat.post }
      lead:   { permission: chat.lead }
    message_types:
      text:   { properties: { body: { type: string } }, post: [poster] }
      note:   { properties: { body: { type: string } }, post: [lead] }
      status: { persist: ephemeral, ttl: 30s, post: [member] }
    lifecycle:
      close:   { roles: [lead] }
      archive: { roles: [lead], from: closed }
  rooms:
    attach_to: [ticket]
    cardinality: many
    typing: true
    read: { permission: chat.read }
    create: { roles: [poster] }
    roles:
      poster: { permission: chat.post }
    message_types:
      text: { properties: { body: { type: string } }, post: [poster] }
  plain:
    attach_to: [ticket]
    read: { permission: chat.read }
    roles:
      poster: { permission: chat.post }
    message_types:
      text: { properties: { body: { type: string } }, post: [poster] }
`

func testMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	meta, err := metamodel.Parse([]byte(testSchema))
	require.NoError(t, err)
	return meta
}

func testCompiled(t *testing.T, yaml string) *realtimecfg.Compiled {
	t.Helper()
	return testCompiledWith(t, yaml, nil)
}

// testCompiledWith compiles yaml under policy.
//
// Nil: policy is accepted and means no acl.yaml.
func testCompiledWith(t *testing.T, yaml string, policy *acl.Policy) *realtimecfg.Compiled {
	t.Helper()
	c, err := realtimecfg.Parse([]byte(yaml), testMeta(t), policy)
	require.NoError(t, err)
	return c
}

// fakeAccess is a configurable Access.
type fakeAccess struct {
	p       principal.Principal
	q       predicatefns.QueryIdentity
	perms   map[string]bool
	ceiling bool
	// deniedPerms are permissions the client ceiling's permission axis
	// denies (deny_permissions): held, but not for this request.
	deniedPerms map[string]bool
	// permCalls counts HoldsPermissionFor calls.
	permCalls atomic.Int64
	// identityCalls counts QueryIdentity calls: one per entity-property
	// role check.
	identityCalls atomic.Int64
	// primeCalls counts Prime calls.
	primeCalls atomic.Int64
}

func (a *fakeAccess) Principal() principal.Principal { return a.p }
func (a *fakeAccess) QueryIdentity() predicatefns.QueryIdentity {
	a.identityCalls.Add(1)
	return a.q
}
func (a *fakeAccess) HoldsPermissionFor(_ context.Context, _, perm string) bool {
	a.permCalls.Add(1)
	return a.perms[perm] && !a.deniedPerms[perm]
}
func (a *fakeAccess) CeilingAdmitsUpdate(string) bool { return a.ceiling }
func (a *fakeAccess) Prime(ctx context.Context, _ []string) context.Context {
	a.primeCalls.Add(1)
	return ctx
}

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
	// hide names, per caller, the properties the caller's redacted view
	// leaves out.
	hide map[string][]string
	// fail, when set, is asked with the call's number (from 1); a non-nil
	// answer fails the read.
	fail func(n int64) error
	// anchorsCalls counts Anchors calls.
	anchorsCalls atomic.Int64
	// hideAt names, per view key, the properties every caller's view
	// leaves out, as a `visible:` rule on one entity does.
	hideAt map[string][]string
}

// namesOf is the identities a role property value names: the string, or
// the strings of a list.
func namesOf(v any) []string {
	switch v := v.(type) {
	case string:
		return []string{v}
	case []string:
		return v
	case []any:
		var out []string
		for _, e := range v {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// fakeHolders is a HolderFinder over the views of a fakeAnchors: a bare id
// view is a family holder, an `ID@face` view a face holder. A property
// the caller's view hides names nobody, as the gated query answers.
type fakeHolders struct {
	anchors *fakeAnchors
	calls   atomic.Int64
	// more, when set, is reported for every query.
	more bool
	// err, when set, fails every query.
	err error
}

func (f *fakeHolders) Holding(ctx context.Context, q HolderQuery) ([]AnchorRef, bool, error) {
	f.calls.Add(1)
	if f.err != nil {
		return nil, false, f.err
	}
	f.anchors.mu.Lock()
	defer f.anchors.mu.Unlock()
	var out []AnchorRef
	for key, v := range f.anchors.views {
		faced := strings.Contains(key, "@")
		if faced != q.Face || v.Type != q.EntityType || slices.Contains(f.anchors.hidden(ctx, key), q.Property) {
			continue
		}
		if !slices.Contains(namesOf(v.Props[q.Property]), q.Identity) {
			continue
		}
		ref := AnchorRef{ID: v.ID}
		if q.Face {
			ref.Face = v.Face
		}
		out = append(out, ref)
	}
	slices.SortFunc(out, func(a, b AnchorRef) int {
		return cmp.Or(cmp.Compare(a.ID, b.ID), cmp.Compare(a.Face, b.Face))
	})
	more := f.more
	if len(out) > q.Limit {
		out, more = out[:q.Limit], true
	}
	return out, more, nil
}

func (f *fakeAnchors) Anchor(ctx context.Context, addr entity.Address, props []string) (AnchorView, error) {
	f.calls.Add(1)
	return f.read(ctx, addr, props)
}

func (f *fakeAnchors) read(ctx context.Context, addr entity.Address, props []string) (AnchorView, error) {
	n := f.calls.Load() + f.anchorsCalls.Load()
	if f.onRead != nil {
		f.onRead()
	}
	if f.fail != nil {
		if err := f.fail(n); err != nil {
			return AnchorView{}, err
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastReq = props
	v, ok := f.views[addr.String()]
	if !ok {
		return AnchorView{}, ErrAnchorNotFound
	}
	hidden := f.hidden(ctx, addr.String())
	out := v
	out.Props = map[string]any{}
	for _, p := range props {
		if val, ok := v.Props[p]; ok && !slices.Contains(hidden, p) {
			out.Props[p] = val
		}
	}
	if def, ok := f.entityDef(v.Type); ok {
		out.Record = predicatefns.EntityRecord(f.meta, def, v.ID, v.Type, out.Props)
	}
	return out, nil
}

// Anchors reads each address as Anchor does, as one call: anchorsCalls
// counts it and calls does not.
func (f *fakeAnchors) Anchors(
	ctx context.Context, addrs []entity.Address, props []string,
) (map[entity.Address]AnchorView, error) {
	f.anchorsCalls.Add(1)
	out := map[entity.Address]AnchorView{}
	for _, a := range addrs {
		v, err := f.read(ctx, a, props)
		if errors.Is(err, ErrAnchorNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out[a] = v
	}
	return out, nil
}

// setStatus sets the status of TKT-1; nil removes it.
func (f *fakeAnchors) setStatus(v any) { f.setProp("status", v) }

// setProp sets property prop of TKT-1; nil removes it.
func (f *fakeAnchors) setProp(prop string, v any) {
	const addr = "TKT-1"
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

// hidden names the properties the caller's view of the view at key leaves
// out: those hide names for the caller and those hideAt names for key.
func (f *fakeAnchors) hidden(ctx context.Context, key string) []string {
	var out []string
	if a, ok := ctx.Value(accessKey{}).(*fakeAccess); ok {
		out = append(out, f.hide[a.p.User]...)
	}
	return append(out, f.hideAt[key]...)
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
	// hide names, per caller, the ids the caller may not read.
	hide map[string][]string
}

func (f *fakeRefs) Resolve(ctx context.Context, ids []string) (map[string]RefView, error) {
	f.calls.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	var hidden []string
	if a, ok := ctx.Value(accessKey{}).(*fakeAccess); ok {
		hidden = f.hide[a.p.User]
	}
	out := map[string]RefView{}
	for _, id := range ids {
		if v, ok := f.views[id]; ok && !slices.Contains(hidden, id) {
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
	// beforeAppend runs once, before the next Append reaches the store.
	beforeAppend func()
	// onRecord runs before RecordTypes.
	onRecord func()
	// onDelete runs before Delete.
	onDelete func()
	// onSetAction runs before SetAction.
	onSetAction func()
	// failSetAction fails the next SetAction, once.
	failSetAction atomic.Bool
	// dueLists, lifecycleLists and setSessions count the calls the timer
	// loop makes.
	dueLists, lifecycleLists, setSessions atomic.Int64
	// onSetSession runs before SetSession; an error it returns fails the
	// call.
	onSetSession func(channels.Key) error
	// setActions and staleLists count SetAction and StalePending calls.
	setActions, staleLists atomic.Int64
	// entityLists counts ListEntity calls.
	entityLists atomic.Int64
	// waitingLists counts ListWaiting calls.
	waitingLists atomic.Int64
	// addressedLists counts ListAddressed calls.
	addressedLists atomic.Int64
	// reads, listReads and setReads count the read marker calls.
	reads, listReads, setReads atomic.Int64
	// beforeReads runs once, before the next Reads reaches the store.
	beforeReads func()
}

func (s *countingStore) Reads(
	ctx context.Context, r channels.Reader, keys []channels.Key,
) (map[channels.Key]channels.ReadState, error) {
	s.reads.Add(1)
	if f := s.beforeReads; f != nil {
		s.beforeReads = nil
		f()
	}
	return s.Store.Reads(ctx, r, keys)
}

func (s *countingStore) ListReads(ctx context.Context, r channels.Reader, limit int) ([]channels.ReadMarker, error) {
	s.listReads.Add(1)
	return s.Store.ListReads(ctx, r, limit)
}

func (s *countingStore) SetRead(
	ctx context.Context, k channels.Key, r channels.Reader, seq int64, at time.Time,
) (channels.ReadState, bool, error) {
	s.setReads.Add(1)
	return s.Store.SetRead(ctx, k, r, seq, at)
}

func (s *countingStore) ListAddressed(
	ctx context.Context, r channels.Reader, kind channels.AddressKind, limit int,
) ([]channels.Addressed, error) {
	s.addressedLists.Add(1)
	return s.Store.ListAddressed(ctx, r, kind, limit)
}

func (s *countingStore) ListWaiting(ctx context.Context, q channels.WaitingQuery) ([]channels.Channel, error) {
	s.waitingLists.Add(1)
	return s.Store.ListWaiting(ctx, q)
}

func (s *countingStore) ListEntity(ctx context.Context, id string) ([]channels.Channel, error) {
	s.entityLists.Add(1)
	return s.Store.ListEntity(ctx, id)
}

func (s *countingStore) StalePending(ctx context.Context, now time.Time) ([]channels.PendingAction, error) {
	s.staleLists.Add(1)
	return s.Store.StalePending(ctx, now)
}

func (s *countingStore) ListSessionsDue(
	ctx context.Context, types []string, until time.Time, limit int,
) ([]channels.Channel, error) {
	s.dueLists.Add(1)
	return s.Store.ListSessionsDue(ctx, types, until, limit)
}

func (s *countingStore) ListByLifecycle(
	ctx context.Context, typ string, state channels.State, before time.Time,
) ([]channels.Channel, error) {
	s.lifecycleLists.Add(1)
	return s.Store.ListByLifecycle(ctx, typ, state, before)
}

func (s *countingStore) SetSession(
	ctx context.Context, k channels.Key, at time.Time, step channels.SessionStep,
) (channels.Channel, error) {
	s.setSessions.Add(1)
	if s.onSetSession != nil {
		if err := s.onSetSession(k); err != nil {
			return channels.Channel{}, err
		}
	}
	return s.Store.SetSession(ctx, k, at, step)
}

func (s *countingStore) SetAction(
	ctx context.Context, k channels.Key, seq int64, name string, expect channels.ActionStatus, next channels.Action,
) (channels.Message, error) {
	s.setActions.Add(1)
	if s.onSetAction != nil {
		s.onSetAction()
	}
	if s.failSetAction.CompareAndSwap(true, false) {
		return channels.Message{}, errors.New("set action failed")
	}
	return s.Store.SetAction(ctx, k, seq, name, expect, next)
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
	if f := s.beforeAppend; f != nil {
		s.beforeAppend = nil
		f()
	}
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
	// wake is the DueWake the service notes due times on.
	wake *DueWake
	// holders answers the inbox's holder queries from anchors.
	holders *fakeHolders
}

type harnessOpt func(*harnessOpts)

type harnessOpts struct {
	yaml     string
	policy   *acl.Policy
	lim      ephemeralLimits
	caps     limitCaps
	promoter Promoter
	outputs  OutputWriter
}

func withEphemeral(l ephemeralLimits) harnessOpt { return func(o *harnessOpts) { o.lim = l } }
func withBuckets(n int) harnessOpt               { return func(o *harnessOpts) { o.caps.buckets = n } }
func withLifecycleYAML() harnessOpt              { return func(o *harnessOpts) { o.yaml = lifecycleRealtime } }
func withYAML(y string) harnessOpt               { return func(o *harnessOpts) { o.yaml = y } }
func withPromoter(p Promoter) harnessOpt         { return func(o *harnessOpts) { o.promoter = p } }
func withOutputs(w OutputWriter) harnessOpt      { return func(o *harnessOpts) { o.outputs = w } }

// withPersonMapping compiles the config under an acl.yaml that maps logins
// to person entities, so the mention user type is person.
func withPersonMapping() harnessOpt {
	return func(o *harnessOpts) { o.policy = personMappingPolicy() }
}

// personMappingPolicy is an acl.yaml that maps logins to person entities.
func personMappingPolicy() *acl.Policy {
	return &acl.Policy{
		UserEntityType: "person", PrincipalProperty: "login",
		Roles: map[string]acl.RoleDef{"secret-reader": {Permissions: []string{"chat.secret"}}},
	}
}

func newHarness(t *testing.T, opts ...harnessOpt) *harness {
	t.Helper()
	o := harnessOpts{
		yaml: testRealtime, lim: defaultEphemeralLimits(), caps: defaultLimitCaps(), promoter: NoPromoter{},
		outputs: NoOutputs{},
	}
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
		store: st, clock: clock, local: local, wake: NewDueWake(),
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
		cfg: &fakeConfig{snap: Snapshot{Config: testCompiledWith(t, o.yaml, o.policy)}},
	}
	h.holders = &fakeHolders{anchors: h.anchors}
	h.svc, err = New(Deps{
		Store: st, Config: h.cfg, Access: fakeAccessSource{}, Anchors: h.anchors, Refs: h.refs, Local: local,
		Promoter: o.promoter, DueWake: h.wake, Outputs: o.outputs, Holders: h.holders,
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
