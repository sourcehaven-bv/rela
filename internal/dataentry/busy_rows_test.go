package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/filechannels"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// Busy state on list rows (TKT-21G6JS). A list asks for it with
// ?busy=<channel type>, and each row then carries `_busy`.

// busyRowsRealtime declares the session types the list tests seed:
//
//   - run: the agent-runner PoC shape, which works only in planning, with
//     typing and the owner's own edits and removals;
//   - instant: starts in a working state, so a seeded channel is busy at
//     once, and only the ticket's owner may read it;
//   - facerun: instant on each face of a page;
//   - chat: no session;
//   - job: one per ticket, starts working, so its first post creates a busy
//     channel without moving it; the owner closes, reopens and deletes it.
//     Its delete has no retention: a delete under retention needs a closed
//     channel, which is not busy, so it could not move `_busy`;
//   - task: many per ticket, created explicitly, starts working.
const busyRowsRealtime = `
version: 1
channel_types:
  run:
    label: Agent run
    attach_to: [ticket]
    read: { from: attached }
    typing: true
    roles:
      owner:  { from: entity.owner }
      runner: { from: entity.runner }
    message_ops:
      edit_own:   { roles: [owner] }
      delete_own: { roles: [owner] }
    message_types:
      go:      { properties: { note: { type: string } }, post: [owner] }
      claim:   { properties: { worktree: { type: string } }, post: [runner] }
      plan:    { properties: { plan: { type: string } }, post: [runner] }
      approve: { properties: { note: { type: string } }, post: [owner] }
    session:
      initial: new
      states:
        new:         { label: Not started, waiting_for: [owner], on: { go: { to: queued } } }
        queued:      { label: Queued, waiting_for: [runner], on: { claim: { to: planning } } }
        planning:    { label: Planning, status: working, on: { plan: { to: plan-review } } }
        plan-review: { label: Plan review, waiting_for: [owner], on: { approve: { to: planning } } }
  instant:
    label: Instant run
    attach_to: [ticket]
    read: { from: entity.owner }
    roles: &roles
      owner: { from: entity.owner }
    message_types: &types
      note: { properties: { body: { type: string } }, post: [owner] }
    session: &session
      initial: working
      states:
        working: { label: Working, status: working, on: { note: { to: done } } }
        done:    { label: Done, final: true }
  facerun:
    label: Face run
    attach_to: [page]
    anchor: face
    read: { from: entity.owner }
    roles: *roles
    message_types: *types
    session: *session
  chat:
    attach_to: [ticket]
    read: { from: attached }
    roles: *roles
    message_types: *types
  job:
    attach_to: [ticket]
    read: { from: attached }
    roles: { owner: { from: entity.owner } }
    message_types:
      log:    { properties: { body: { type: string } }, post: [owner] }
      finish: { properties: { body: { type: string } }, post: [owner] }
    session:
      initial: working
      states:
        working: { label: Working, status: working, on: { finish: { to: done } } }
        done:    { label: Done, final: true }
    lifecycle:
      close:  { roles: [owner] }
      reopen: { roles: [owner] }
      delete: { roles: [owner] }
  task:
    attach_to: [ticket]
    cardinality: many
    read: { from: attached }
    create: { roles: [owner] }
    roles: { owner: { from: entity.owner } }
    message_types:
      log:    { properties: { body: { type: string } }, post: [owner] }
      finish: { properties: { body: { type: string } }, post: [owner] }
    session:
      initial: working
      states:
        working: { label: Working, status: working, on: { finish: { to: done } } }
        done:    { label: Done, final: true }
`

// busyRowsMeta has tickets, pages with a draft and a published face, and
// people.
func busyRowsMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	m, err := metamodel.Parse([]byte(`
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    properties:
      title:  { type: string }
      owner:  { type: string }
      runner: { type: string }
  page:
    label: Page
    id_prefix: PAGE
    faces:
      draft: {}
      published: { label: Published }
    properties:
      title: { type: string }
      owner: { type: string }
  person:
    label: Person
    id_type: manual
    properties:
      title: { type: string }
`))
	require.NoError(t, err)
	return m
}

// busyPolicy: alice, bob and the runner read tickets, pages in the
// published world and people.
func busyPolicy() *acl.Policy {
	return &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"reader": {Read: []string{"ticket", "page", "person"}, Worlds: []string{"published"}},
		},
		Assignments: map[string]string{"alice": "reader", "bob": "reader", "runner": "reader"},
	}
}

// pageWorld resolves a page to its published face, else its draft
// face, so a list in it shows pages on both faces.
type pageWorld struct{}

func (pageWorld) Lookup(name string) (store.WorldScope, bool) {
	switch name {
	case "published":
		return store.NewWorldScope(map[string]store.TypeResolution{
			"page": {Chain: []entity.Face{"published", "draft"}, Fallback: store.FallbackExclude},
		}), true
	case metamodel.DefaultWorldName:
		return store.TrivialScope(), true
	}
	return store.TrivialScope(), false
}

// busyEnv is an app whose channel service reads the entity store and the
// channel store through counters.
type busyEnv struct {
	app      *App
	d        *acl.Declarative
	ch       *channelService
	counting *storetest.Counting
	chans    *countingChannels
}

// newBusyEnv seeds alice, bob and the runner, runs seed, and wires a
// channel service with busyRowsRealtime under policy. wrap, when set,
// wraps the counting channel store the service uses.
func newBusyEnv(
	t *testing.T, policy *acl.Policy, seed func(app *App), wrap func(channels.Store) channels.Store,
) *busyEnv {
	t.Helper()
	return newBusyEnvLocal(t, policy, seed, wrap, nil)
}

// newBusyEnvLocal is newBusyEnv over the process-local state local. Nil: a
// new one on the real clock.
func newBusyEnvLocal(
	t *testing.T, policy *acl.Policy, seed func(app *App), wrap func(channels.Store) channels.Store,
	local *realtime.Local,
) *busyEnv {
	t.Helper()
	app := newAppFromParts(&Config{App: AppConfig{Name: "Test"}}, busyRowsMeta(t), newFixture())
	for _, id := range []string{"alice", "bob", "runner"} {
		seedEntity(app, &entity.Entity{ID: id, Type: "person", Properties: map[string]any{"title": id}})
	}
	if seed != nil {
		seed(app)
	}
	app.setWorlds(pageWorld{})
	counting := storetest.NewCounting(app.store)
	d := mustNewACL(t, policy, app.store)
	setTestACL(t, app, d)
	fs, err := filechannels.New(storage.NewMemFS(), "/channels", filelock.ProcessPrivate{})
	require.NoError(t, err)
	chans := newCountingChannels(fs)
	var cs channels.Store = chans
	if wrap != nil {
		cs = wrap(chans)
	}
	ch := newChannelServiceLocal(t, app.Meta(), counting, d, busyRowsRealtime, cs, local)
	require.NoError(t, SetChannels(app, ch.svc))
	return &busyEnv{app: app, d: d, ch: ch, counting: counting, chans: chans}
}

// ownedTicket is a ticket owned by alice, run by the runner.
func ownedTicket(id string) *entity.Entity {
	return &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{
		"title": id, "owner": "alice", "runner": "runner",
	}}
}

// seedChannel opens a channel of typ on anchor at at, with a message
// written straight to the store. A session type whose initial state works
// is then busy, since its session never moved.
func (e *busyEnv) seedChannel(t *testing.T, anchor channels.Anchor, typ string, at time.Time) {
	t.Helper()
	_, err := e.ch.store.Append(context.Background(), channels.Key{Anchor: anchor, Type: typ}, channels.NewMessage{
		Type: "note", Author: channels.Author{User: "alice", Tool: principal.ToolDataEntry}, At: at,
		Props: map[string]any{"body": "x"},
	})
	require.NoError(t, err)
}

// post posts a message of typ to the run channel of id as user.
func (e *busyEnv) post(t *testing.T, user, id, typ string, props map[string]any) {
	t.Helper()
	ctx := gateCtxFor(principalCtx(user), t, e.d)
	_, err := e.ch.svc.Post(ctx, realtime.RawAddress{EntityType: "ticket", Anchor: id, Type: "run"},
		realtime.PostInput{Type: typ, Props: props})
	require.NoError(t, err)
}

// list serves path as user, through the world gate, and returns the
// response and the raw body.
func (e *busyEnv) list(t *testing.T, user, path string) (resp v1.ListResponse, body string) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody).WithContext(gateCtxFor(principalCtx(user), t, e.d))
	attachWorld(http.HandlerFunc(e.app.handleV1DynamicRoutes), e.app).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "GET %s: %s", path, rec.Body)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp, rec.Body.String()
}

// busyOf returns each row's `_busy` by id.
func busyOf(resp v1.ListResponse) map[string][]v1.BusyChannel {
	out := map[string][]v1.BusyChannel{}
	for _, row := range resp.Data {
		out[row.ID] = row.Busy
	}
	return out
}

// TestListBusy_Rows pins AC 4 of TKT-21G6JS: every row carries `_busy`, an
// empty list for a row with nothing busy, and the entries otherwise. The
// first value of the parameter counts.
func TestListBusy_Rows(t *testing.T) {
	at := time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC)
	e := newBusyEnv(t, busyPolicy(), func(app *App) {
		for _, id := range []string{"TKT-1", "TKT-2", "TKT-3"} {
			seedEntity(app, ownedTicket(id))
		}
	}, nil)
	e.seedChannel(t, channels.Anchor{ID: "TKT-1"}, "instant", at)
	e.seedChannel(t, channels.Anchor{ID: "TKT-3"}, "instant", at)
	// The owner's note moves TKT-3's session to done, which is final.
	ctx := gateCtxFor(principalCtx("alice"), t, e.d)
	_, err := e.ch.svc.Post(ctx, realtime.RawAddress{EntityType: "ticket", Anchor: "TKT-3", Type: "instant"},
		realtime.PostInput{Type: "note", Props: map[string]any{"body": "done"}})
	require.NoError(t, err)

	want := map[string][]v1.BusyChannel{
		"TKT-1": {{
			Type: "instant", TypeLabel: "Instant run", State: "working", Label: "Working",
			Since: at.Format(time.RFC3339Nano),
		}},
		"TKT-2": {},
		"TKT-3": {},
	}
	for _, path := range []string{"/api/v1/tickets?busy=instant", "/api/v1/tickets?busy=instant&busy=chat"} {
		resp, body := e.list(t, "alice", path)
		require.Equal(t, want, busyOf(resp), path)
		require.Equal(t, 2, strings.Count(body, `"_busy":[]`), "an empty answer is on the wire as []: %s", body)
	}
}

// TestListBusy_NotRequested pins AC 4: without the parameter, or with an
// empty one, no row carries `_busy`.
func TestListBusy_NotRequested(t *testing.T) {
	e := newBusyEnv(t, busyPolicy(), func(app *App) { seedEntity(app, ownedTicket("TKT-1")) }, nil)
	e.seedChannel(t, channels.Anchor{ID: "TKT-1"}, "instant", time.Now())
	for _, path := range []string{"/api/v1/tickets", "/api/v1/tickets?busy="} {
		resp, body := e.list(t, "alice", path)
		require.Len(t, resp.Data, 1)
		require.NotContains(t, body, "_busy", path)
		require.Zero(t, e.chans.snapshot()["ListWaiting"], "%s read the channel store", path)
	}
}

// TestListBusy_NotServed pins AC 3 and AC 4: a busy read that cannot be
// served for a reason of configuration leaves `_busy` absent, answers 200
// and logs nothing. A request without a principal never reaches the list
// with a row: the ACL request binds the principal. TestBusy_NotServed in
// internal/realtime pins that case.
func TestListBusy_NotServed(t *testing.T) {
	cases := []struct {
		name  string
		path  string
		setup func(e *busyEnv)
	}{
		{name: "unknown type", path: "/api/v1/tickets?busy=nope"},
		{name: "no session", path: "/api/v1/tickets?busy=chat"},
		{name: "does not attach", path: "/api/v1/tickets?busy=facerun"},
		{name: "realtime off", path: "/api/v1/tickets?busy=instant", setup: func(e *busyEnv) { e.ch.cfg.Store(nil) }},
		{
			name: "degraded", path: "/api/v1/tickets?busy=instant",
			setup: func(e *busyEnv) { e.ch.degraded.Store(true) },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureWarnings(t)
			e := newBusyEnv(t, busyPolicy(), func(app *App) { seedEntity(app, ownedTicket("TKT-1")) }, nil)
			e.seedChannel(t, channels.Anchor{ID: "TKT-1"}, "instant", time.Now())
			if tc.setup != nil {
				tc.setup(e)
			}
			resp, body := e.list(t, "alice", tc.path)
			require.Len(t, resp.Data, 1)
			require.NotContains(t, body, "_busy")
			require.NotContains(t, logs.String(), "busy", "a busy read that is not served logs nothing")
		})
	}
}

// failingWaiting fails every ListWaiting call with err.
type failingWaiting struct {
	channels.Store
	err error
}

func (f failingWaiting) ListWaiting(context.Context, channels.WaitingQuery) ([]channels.Channel, error) {
	return nil, f.err
}

// TestListBusy_StoreErrorKeepsList pins AC 4: a failed busy read is logged
// once and leaves `_busy` absent on every row, and the list still answers.
// A row ceiling the store reaches fails the same way.
func TestListBusy_StoreErrorKeepsList(t *testing.T) {
	cases := []struct {
		name string
		wrap func(*countingChannels) channels.Store
	}{
		{name: "store error", wrap: func(c *countingChannels) channels.Store {
			return failingWaiting{Store: c, err: errors.New("disk on fire")}
		}},
		{name: "too many rows", wrap: func(c *countingChannels) channels.Store {
			c.maxRows = 1
			return c
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureWarnings(t)
			e := newBusyEnv(t, busyPolicy(), func(app *App) {
				seedEntity(app, ownedTicket("TKT-1"))
				seedEntity(app, ownedTicket("TKT-2"))
			}, func(s channels.Store) channels.Store {
				c, ok := s.(*countingChannels)
				require.True(t, ok)
				return tc.wrap(c)
			})
			e.seedChannel(t, channels.Anchor{ID: "TKT-1"}, "instant", time.Now())
			e.seedChannel(t, channels.Anchor{ID: "TKT-2"}, "instant", time.Now())
			resp, body := e.list(t, "alice", "/api/v1/tickets?busy=instant")
			require.Len(t, resp.Data, 2)
			require.NotContains(t, body, "_busy")
			require.Equal(t, 1, strings.Count(logs.String(), "reading busy channels failed"), logs.String())
		})
	}
}

// TestListBusy_HiddenRow pins AC 5: a row the caller may not read is not
// in the list, so its busy channel never shows.
func TestListBusy_HiddenRow(t *testing.T) {
	policy := &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"member":  {Read: []string{"person"}},
			"officer": {Read: []string{"ticket"}},
		},
		RoleRelations: map[string]acl.RoleRelationDef{"officer-of": {Confers: "officer"}},
		Assignments:   map[string]string{"alice": "member"},
	}
	e := newBusyEnv(t, policy, func(app *App) {
		seedEntity(app, ownedTicket("TKT-1"))
		seedEntity(app, ownedTicket("TKT-2"))
		seedRelation(app, entity.NewRelation("alice", "officer-of", "TKT-1"))
	}, nil)
	e.seedChannel(t, channels.Anchor{ID: "TKT-1"}, "instant", time.Now())
	e.seedChannel(t, channels.Anchor{ID: "TKT-2"}, "instant", time.Now())
	resp, body := e.list(t, "alice", "/api/v1/tickets?busy=instant")
	got := busyOf(resp)
	require.Len(t, got, 1)
	require.Len(t, got["TKT-1"], 1)
	require.NotContains(t, body, "TKT-2")
}

// TestListBusy_TwoPrincipals pins AC 5: two callers who both read the
// ticket get different answers on one URL. alice holds the channel's read
// role, as the ticket's owner; bob does not, and his row reads as not
// busy.
func TestListBusy_TwoPrincipals(t *testing.T) {
	e := newBusyEnv(t, busyPolicy(), func(app *App) { seedEntity(app, ownedTicket("TKT-1")) }, nil)
	e.seedChannel(t, channels.Anchor{ID: "TKT-1"}, "instant", time.Now())
	const path = "/api/v1/tickets?busy=instant"
	alice, _ := e.list(t, "alice", path)
	bob, _ := e.list(t, "bob", path)
	require.Len(t, busyOf(alice)["TKT-1"], 1)
	require.Equal(t, map[string][]v1.BusyChannel{"TKT-1": {}}, busyOf(bob))
}

// TestListBusy_PoCFlow runs the agent-runner PoC's run through the list
// handler on a test clock: new and queued are not busy, planning is since
// the claim, plan review, which waits on the owner, is not, and the
// approval makes the run busy again since the approval.
func TestListBusy_PoCFlow(t *testing.T) {
	clock := newFakeClock()
	local, err := realtime.NewLocal(clock.Now)
	require.NoError(t, err)
	e := newBusyEnvLocal(t, busyPolicy(), func(app *App) { seedEntity(app, ownedTicket("TKT-1")) }, nil, local)
	const path = "/api/v1/tickets?busy=run"
	busy := func() []v1.BusyChannel {
		t.Helper()
		resp, _ := e.list(t, "alice", path)
		return busyOf(resp)["TKT-1"]
	}
	step := func(user, typ string, props map[string]any) time.Time {
		t.Helper()
		clock.advance(time.Minute)
		e.post(t, user, "TKT-1", typ, props)
		return clock.Now()
	}
	require.Empty(t, busy(), "no channel yet")
	step("alice", "go", nil)
	require.Empty(t, busy(), "queued waits on the runner")
	claimed := step("runner", "claim", map[string]any{"worktree": "wt"})
	got := busy()
	require.Len(t, got, 1)
	require.Equal(t, "Planning", got[0].Label)
	require.Equal(t, claimed.Format(time.RFC3339Nano), got[0].Since)
	step("runner", "plan", map[string]any{"plan": "do it"})
	require.Empty(t, busy(), "plan review waits on the owner")
	approved := step("alice", "approve", nil)
	got = busy()
	require.Len(t, got, 1)
	require.Equal(t, "Planning", got[0].Label)
	require.Equal(t, approved.Format(time.RFC3339Nano), got[0].Since)
	require.True(t, approved.After(claimed))
}

// seedBusyPages seeds n pages and a busy face channel on each. Page i is on
// the published face for an even i and on the draft face for an odd one,
// and alice owns pages 0 and 1 of every four, so half of each face's
// channels deny her the read.
func seedBusyPages(t *testing.T, n int) *busyEnv {
	t.Helper()
	face := func(i int) entity.Face {
		if i%2 == 0 {
			return "published"
		}
		return "draft"
	}
	owner := func(i int) string {
		if i%4 < 2 {
			return "alice"
		}
		return "bob"
	}
	e := newBusyEnv(t, busyPolicy(), func(app *App) {
		for i := range n {
			seedEntity(app, &entity.Entity{ID: fmt.Sprintf("PAGE-%d", 100+i), Type: "page", Face: face(i),
				Properties: map[string]any{"title": fmt.Sprintf("p%d", i), "owner": owner(i)}})
		}
	}, nil)
	for i := range n {
		e.seedChannel(t, channels.Anchor{ID: fmt.Sprintf("PAGE-%d", 100+i), Face: face(i)}, "facerun", time.Now())
	}
	return e
}

// TestListBusy_ListIDScoped pins AC 9 of TKT-TU3XH4: a list view's request
// carries list_id, and the list's `condition:` bounds the busy read. A row
// the list drops is in neither the response nor the busy read's anchors.
func TestListBusy_ListIDScoped(t *testing.T) {
	e := newBusyEnv(t, busyPolicy(), func(app *App) {
		seedEntity(app, ownedTicket("TKT-1"))
		bobs := ownedTicket("TKT-2")
		bobs.Properties["owner"] = "bob"
		seedEntity(app, bobs)
		app.State().Cfg.Lists = map[string]dataentryconfig.List{"mine": {
			EntityType: "ticket",
			Columns:    []dataentryconfig.ListColumn{{Property: "title"}, {Busy: "instant"}},
			Condition:  "entity.owner == 'alice'",
		}}
	}, nil)
	wireRealViewConditions(t, e.app)
	e.seedChannel(t, channels.Anchor{ID: "TKT-1"}, "instant", time.Now())
	e.seedChannel(t, channels.Anchor{ID: "TKT-2"}, "instant", time.Now())

	resp, body := e.list(t, "alice", "/api/v1/tickets?list_id=mine&busy=instant")
	got := busyOf(resp)
	require.Len(t, got, 1, body)
	require.Len(t, got["TKT-1"], 1)
	require.NotContains(t, body, "TKT-2")
	e.chans.mu.Lock()
	anchors := slices.Clone(e.chans.anchors)
	e.chans.mu.Unlock()
	require.Equal(t, []channels.Anchor{{ID: "TKT-1"}}, anchors, "the busy read saw only the served row")

	// Control: without list_id both rows are served and read.
	resp, _ = e.list(t, "alice", "/api/v1/tickets?busy=instant")
	require.Len(t, busyOf(resp), 2)
	e.chans.mu.Lock()
	defer e.chans.mu.Unlock()
	require.Len(t, e.chans.anchors, 2)
}
