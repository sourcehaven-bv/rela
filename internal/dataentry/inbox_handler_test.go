package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/filechannels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// Inbox route tests (TKT-ZZ4PNX). They run a real channel service, built
// with the production constructor, so the holder query, the anchor batch
// and the role prime are the deployed ones.

// inboxTestRealtime: case waits for a ticket's or a feature's owner or
// watchers; threads waits for its creator. Reading either needs
// inbox.read, which the officer-of relation confers per entity, so the
// read check walks a relation unless the inbox primes the roles.
const inboxTestRealtime = `
version: 1
channel_types:
  case:
    attach_to: [ticket, feature]
    read: { permission: inbox.read }
    roles:
      lead:    { from: entity.owner, label: the lead }
      watcher: { from: entity.watchers, label: a watcher }
      member:  { from: attached }
    message_types:
      chat:   { properties: { body: { type: string } }, post: [member] }
      answer: { properties: { note: { type: string } }, post: [lead, watcher] }
    session:
      initial: waiting
      states:
        waiting: { label: Waiting, waiting_for: [lead, watcher], on: { answer: { to: done } } }
        done: { final: true }
  threads:
    attach_to: [ticket]
    cardinality: many
    read: { permission: inbox.read }
    create: { roles: [member] }
    roles:
      starter: { creator: true, label: the starter }
      member:  { from: attached }
    message_types:
      wrap: { properties: { body: { type: string } }, post: [starter] }
    session:
      initial: asked
      states:
        asked: { waiting_for: [starter], on: { wrap: { to: done } } }
        done: { final: true }
`

// inboxTestMeta has tickets and features with an owner and watchers, and
// people who are officers of them.
func inboxTestMeta() *metamodel.Metamodel {
	props := map[string]metamodel.PropertyDef{
		"title":    {Type: "string", Required: true},
		"owner":    {Type: "string"},
		"watchers": {Type: "string", List: true},
	}
	order := []string{"title", "owner", "watchers"}
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket":  {Label: "Ticket", IDPrefix: "TKT-", Properties: props, PropertyOrder: order},
			"feature": {Label: "Feature", IDPrefix: "FEAT-", Properties: props, PropertyOrder: order},
			"person": {Label: "Person", Properties: map[string]metamodel.PropertyDef{"title": {Type: "string"}},
				PropertyOrder: []string{"title"}},
		},
		Relations: map[string]metamodel.RelationDef{
			"officer-of": {Label: "officer of", From: []string{"person"}, To: []string{"ticket", "feature"}},
		},
	}
}

// inboxEnv is an app whose channel service reads the entity store and the
// channel store through counters.
type inboxEnv struct {
	app      *App
	d        *acl.Declarative
	ch       *channelService
	counting *storetest.Counting
	chans    *countingChannels
}

func (c *countingChannels) ListWaiting(ctx context.Context, q channels.WaitingQuery) ([]channels.Channel, error) {
	c.hit("ListWaiting")
	c.mu.Lock()
	if c.maxRows > 0 {
		q.MaxRows = c.maxRows
	}
	c.anchors = slices.Clone(q.Anchors)
	c.mu.Unlock()
	return c.Store.ListWaiting(ctx, q)
}

// newInboxEnv wires a channel service over meta's store. alice reads and
// updates tickets but not features, and holds inbox.read on an entity she
// is an officer of.
func newInboxEnv(t *testing.T, seed func(app *App)) *inboxEnv {
	t.Helper()
	return newInboxEnvPolicy(t, seed, inboxPolicy())
}

// inboxPolicy: alice and bob read tickets and people and update tickets;
// inbox.read comes from the officer-of relation.
func inboxPolicy() *acl.Policy {
	return &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"editor":  {Read: []string{"ticket", "person"}, Update: []string{"ticket"}},
			"officer": {Permissions: []string{"inbox.read"}},
		},
		RoleRelations: map[string]acl.RoleRelationDef{"officer-of": {Confers: "officer"}},
		Assignments:   map[string]string{"alice": "editor", "bob": "editor"},
	}
}

// newInboxEnvPolicy is newInboxEnv under policy.
func newInboxEnvPolicy(t *testing.T, seed func(app *App), policy *acl.Policy) *inboxEnv {
	t.Helper()
	app := newAppFromParts(&Config{}, inboxTestMeta(), newFixture())
	for _, id := range []string{"alice", "bob"} {
		seedEntity(app, &entity.Entity{ID: id, Type: "person", Properties: map[string]any{"title": id}})
	}
	if seed != nil {
		seed(app)
	}
	counting := storetest.NewCounting(app.store)
	d := mustNewACL(t, policy, counting)
	setTestACL(t, app, d)
	cs, err := filechannels.New(storage.NewMemFS(), "/channels", filelock.ProcessPrivate{})
	require.NoError(t, err)
	chans := newCountingChannels(cs)
	ch := newChannelServiceOver(t, app.Meta(), counting, d, inboxTestRealtime, chans)
	require.NoError(t, SetChannels(app, ch.svc))
	return &inboxEnv{app: app, d: d, ch: ch, counting: counting, chans: chans}
}

// get serves /api/v1/_inbox?query as alice and returns what it cost.
func (e *inboxEnv) get(t *testing.T, method, query string) (rec *httptest.ResponseRecorder, cost channelCost) {
	t.Helper()
	e.counting.Reset()
	e.chans.reset()
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(method, inboxPath+query, http.NoBody)
	e.app.channels.serveInbox(rec, req.WithContext(gateCtxFor(principalCtx("alice"), t, e.d)), e.app.State().Meta)
	return rec, channelCost{entity: e.counting.Calls(), channel: e.chans.snapshot()}
}

// waitOn seeds an open case channel on id, which waits on its owner and
// watchers.
func (e *inboxEnv) waitOn(t *testing.T, id string, at time.Time) {
	t.Helper()
	_, err := e.ch.store.Append(context.Background(), channels.Key{Anchor: channels.Anchor{ID: id}, Type: "case"},
		channels.NewMessage{
			Type: "chat", Author: channels.Author{User: "bob", Tool: principal.ToolDataEntry}, At: at,
			Props: map[string]any{"body": "x"},
		})
	require.NoError(t, err)
}

// thread seeds a threads channel on id opened by user.
func (e *inboxEnv) thread(t *testing.T, id, user string, at time.Time) {
	t.Helper()
	_, err := e.ch.store.CreateMany(context.Background(), channels.Anchor{ID: id}, channels.NewChannel{
		Type: "threads", By: channels.Author{User: user, Tool: principal.ToolDataEntry}, At: at,
	})
	require.NoError(t, err)
}

// seedInbox seeds n tickets that wait on alice, half through owner and
// half through watchers, n features that wait on her (she cannot read
// features), and n tickets that name her but whose channel she may not
// read. Two threads she opened wait on her too. alice's officer relation
// covers the n readable tickets.
func seedInbox(t *testing.T, n int) (env *inboxEnv, entries int) {
	t.Helper()
	ticket := func(i int) string { return fmt.Sprintf("TKT-%d", 100+i) }
	e := newInboxEnv(t, func(app *App) {
		for i := range n {
			props := map[string]any{"title": "t" + strconv.Itoa(i), "owner": "alice"}
			if i%2 == 1 {
				props = map[string]any{"title": "t" + strconv.Itoa(i), "owner": "bob", "watchers": []any{"carol", "alice"}}
			}
			seedEntity(app, &entity.Entity{ID: ticket(i), Type: "ticket", Properties: props})
			seedRelation(app, entity.NewRelation("alice", "officer-of", ticket(i)))
			seedEntity(app, &entity.Entity{ID: fmt.Sprintf("FEAT-%d", 100+i), Type: "feature",
				Properties: map[string]any{"title": "f", "owner": "alice"}})
			seedEntity(app, &entity.Entity{ID: fmt.Sprintf("TKT-%d", 500+i), Type: "ticket",
				Properties: map[string]any{"title": "no officer", "owner": "alice"}})
		}
	})
	at := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	for i := range n {
		e.waitOn(t, ticket(i), at.Add(time.Duration(i)*time.Second))
		e.waitOn(t, fmt.Sprintf("FEAT-%d", 100+i), at)
		e.waitOn(t, fmt.Sprintf("TKT-%d", 500+i), at)
	}
	e.thread(t, ticket(0), "alice", at)
	e.thread(t, ticket(1), "alice", at)
	e.thread(t, ticket(1), "bob", at)
	return e, n + 2
}

// TestInboxBudget_10vs50 pins AC 11: an inbox request costs the same
// entity-store and channel-store calls at 10 entries as at 50. The
// entries sit on distinct anchors over two waiting pairs plus the creator
// branch; as many anchors are hidden as shown, and the read role is a
// permission a relation confers per entity. alice also holds a marker with
// unread messages on every waiting channel, half of them hidden
// (TKT-2B2AIA AC 10), and n messages mention her on readable channels and
// n on hidden ones (TKT-6RXAC0 AC 13): one ListAddressed and no message
// read pays for them.
func TestInboxBudget_10vs50(t *testing.T) {
	cost := func(n int) channelCost {
		e, want := seedInbox(t, n)
		at := time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)
		for i := range n {
			e.markAndChat(t, fmt.Sprintf("TKT-%d", 100+i), at)
			e.markAndChat(t, fmt.Sprintf("FEAT-%d", 100+i), at)
			e.mentionAlice(t, fmt.Sprintf("TKT-%d", 100+i), at)
			e.mentionAlice(t, fmt.Sprintf("TKT-%d", 500+i), at)
		}
		rec, c := e.get(t, http.MethodGet, "?limit=200")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		got := decodeAs[v1.Inbox](t, rec.Body.Bytes())
		require.Len(t, got.Entries, want)
		require.Equal(t, want, *got.Count)
		for _, en := range got.Entries {
			require.True(t, strings.HasPrefix(en.Channel.Anchor, "TKT-1"), en.Channel.Anchor)
		}
		require.Equal(t, n, *got.UnreadCount)
		require.Equal(t, n, *got.MentionCount)
		require.Len(t, got.Mentions, n)
		return c
	}
	small, large := cost(10), cost(50)
	require.True(t, small.equal(large), "10 entries: %s\n50 entries: %s", small, large)
	require.Equal(t, 1, small.channel["ListWaiting"], small.String())
	require.Zero(t, small.channel["ListEntity"], small.String())
	require.Zero(t, small.channel["Head"], small.String())
	require.Zero(t, small.channel["List"], small.String())
	require.LessOrEqual(t, small.entity["ResolveHeaders"], 1, small.String())
	require.Equal(t, 1, small.channel["ListReads"], small.String())
	require.Equal(t, 1, small.channel["Reads"], small.String())
	require.Equal(t, 1, small.channel["ListAddressed"], small.String())
	require.Zero(t, small.channel["Get"], small.String())
}

// mentionAlice appends to the case channel on id a message by bob that
// mentions alice by login.
func (e *inboxEnv) mentionAlice(t *testing.T, id string, at time.Time) {
	t.Helper()
	_, err := e.ch.store.Append(context.Background(), channels.Key{Anchor: channels.Anchor{ID: id}, Type: "case"},
		channels.NewMessage{
			Type: "chat", Author: channels.Author{User: "bob", Tool: principal.ToolDataEntry}, At: at,
			Props: map[string]any{"body": "`@alice`"}, Mentions: []channels.Mention{{Target: "alice", Form: channels.FormLogin}},
		})
	require.NoError(t, err)
}

// TestInbox_HiddenRowsChangeNothingRealACL pins AC 8 through the deployed
// wiring: the real declarative ACL and its field redactor. A ticket whose
// owner `visible:` hides from alice is not a holder anchor, so a project
// with such tickets, each with a waiting session, answers byte for byte
// what a project without them answers, on every page.
func TestInbox_HiddenRowsChangeNothingRealACL(t *testing.T) {
	policy := inboxPolicy()
	editor := policy.Roles["editor"]
	editor.Visible = map[string][]acl.FieldGrant{"ticket": {
		{Field: "title"}, {Field: "watchers"}, {Field: "owner", When: "entity.title == 'shown'"},
	}}
	policy.Roles["editor"] = editor
	at := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	build := func(hidden bool) *inboxEnv {
		e := newInboxEnvPolicy(t, func(app *App) {
			for i := range 7 {
				id := fmt.Sprintf("TKT-%d", 100+i)
				seedEntity(app, &entity.Entity{ID: id, Type: "ticket",
					Properties: map[string]any{"title": "shown", "owner": "alice"}})
				seedRelation(app, entity.NewRelation("alice", "officer-of", id))
				if hidden {
					id = fmt.Sprintf("TKT-%d", 150+i)
					seedEntity(app, &entity.Entity{ID: id, Type: "ticket",
						Properties: map[string]any{"title": "hidden", "owner": "alice"}})
					seedRelation(app, entity.NewRelation("alice", "officer-of", id))
				}
			}
		}, policy)
		for i := range 7 {
			e.waitOn(t, fmt.Sprintf("TKT-%d", 100+i), at.Add(time.Duration(i)*time.Second))
			if hidden {
				// Interleaved in time, so a leak would shift every page.
				e.waitOn(t, fmt.Sprintf("TKT-%d", 150+i), at.Add(time.Duration(i)*time.Second+time.Millisecond))
			}
		}
		return e
	}
	plain, mixed := build(false), build(true)

	// The holder query of the deployed wiring redacts before it counts: no
	// ticket with a hidden owner reaches the channel listing as an anchor.
	rec, _ := mixed.get(t, http.MethodGet, "?limit=200")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, 7, *decodeAs[v1.Inbox](t, rec.Body.Bytes()).Count)
	mixed.chans.mu.Lock()
	require.Len(t, mixed.chans.anchors, 7, "anchors %v", mixed.chans.anchors)
	mixed.chans.mu.Unlock()

	cursor := ""
	for page := 0; ; page++ {
		q := "?limit=3"
		if cursor != "" {
			q += "&cursor=" + cursor
		}
		base, _ := plain.get(t, http.MethodGet, q)
		withHidden, _ := mixed.get(t, http.MethodGet, q)
		require.Equal(t, http.StatusOK, base.Code, base.Body.String())
		require.JSONEq(t, base.Body.String(), withHidden.Body.String(), "page %d", page)
		got := decodeAs[v1.Inbox](t, base.Body.Bytes())
		require.Equal(t, 7, *got.Count)
		require.False(t, *got.Truncated)
		if got.NextCursor == "" {
			break
		}
		cursor = got.NextCursor
	}
}

// TestInboxHandler_Shape pins AC 13: the wire shape, with links and
// RFC 3339 times, and that the route is served by the router.
func TestInboxHandler_Shape(t *testing.T) {
	e, _ := seedInbox(t, 2)
	rec, _ := e.get(t, http.MethodGet, "?limit=1")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var raw map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	require.ElementsMatch(t, []string{
		"notice", "entries", "count", "truncated", "degraded", "next_cursor", "unread_channels", "unread_count",
		"mentions", "mention_count",
	},
		jsonKeys(raw))
	got := decodeAs[v1.Inbox](t, rec.Body.Bytes())
	require.Equal(t, realtime.UntrustedNotice, got.Notice)
	require.Equal(t, 4, *got.Count)
	require.NotEmpty(t, got.NextCursor)
	require.Len(t, got.Entries, 1)
	en := got.Entries[0]
	require.Equal(t, "waiting", en.Kind)
	require.Equal(t, v1.InboxChannel{
		Anchor: "TKT-100", EntityType: "ticket", AnchorTitle: "t0", Type: "case", State: "open",
	}, en.Channel)
	require.Equal(t, "2026-10-27T09:00:00Z", en.Since)
	require.Equal(t, en.Since, en.Session.EnteredAt)
	require.Equal(t, []string{"lead"}, en.Roles)
	require.Equal(t, []string{"answer"}, en.Transitions)
	require.Equal(t, v1.InboxLinks{
		Entity: "/api/v1/tickets/TKT-100", Channel: "/api/v1/tickets/TKT-100/_channels/case",
	}, en.Links)
	require.Equal(t, []v1.ChannelSessionWaiting{
		{Role: "lead", Label: "the lead", Holders: []string{"alice"}}, {Role: "watcher", Label: "a watcher"},
	}, en.Session.WaitingFor)

	next, _ := e.get(t, http.MethodGet, "?limit=10&cursor="+got.NextCursor)
	page := decodeAs[v1.Inbox](t, next.Body.Bytes())
	require.Len(t, page.Entries, 3)
	require.Empty(t, page.NextCursor)
	named := page.Entries[0]
	require.Equal(t, "threads", named.Channel.Type)
	require.Equal(t, "TKT-100", named.Channel.Anchor)
	require.Equal(t, []string{"starter"}, named.Roles)
	require.Equal(t, "/api/v1/tickets/"+named.Channel.Anchor+"/_channels/threads/"+named.Channel.Name,
		named.Links.Channel)

	t.Run("routed", func(t *testing.T) {
		env := newChannelEnv(t)
		resp, body := env.do(t, "alice", http.MethodGet, inboxPath, "", nil)
		require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
		require.Empty(t, decodeAs[v1.Inbox](t, body).Entries)
	})
}

func jsonKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// stubInbox answers every inbox request with err.
type stubInbox struct{ err error }

func (s stubInbox) Inbox(context.Context, realtime.InboxRequest) (realtime.InboxView, error) {
	return realtime.InboxView{}, s.err
}

// TestInboxHandler_Errors pins AC 12 on REST: off and unwired are 404, no
// principal 403, a bad limit or cursor 400, another method 405, the row
// ceiling 503 inbox_too_large, and another fault a 500 without detail.
//
//nolint:paralleltest // the subtests share one seeded env
func TestInboxHandler_Errors(t *testing.T) {
	serve := func(h *channelsHandler, method, query string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.serveInbox(rec, httptest.NewRequest(method, inboxPath+query, http.NoBody), nil)
		return rec
	}
	requireProblem := func(t *testing.T, rec *httptest.ResponseRecorder, status int, typ string) v1.Error {
		t.Helper()
		require.Equal(t, status, rec.Code, rec.Body.String())
		require.Equal(t, typ, problemType(t, rec.Body.Bytes()))
		return decodeAs[v1.Error](t, rec.Body.Bytes())
	}

	t.Run("not wired", func(t *testing.T) {
		p := requireProblem(t, serve(nil, http.MethodGet, ""), http.StatusNotFound, "not_found")
		require.Equal(t, realtimeOffTitle, p.Title)
		plain := newLiveEnv(t, nil)
		resp, body := plain.do(t, "alice", http.MethodGet, inboxPath, "", nil)
		require.Equal(t, http.StatusNotFound, resp.StatusCode, string(body))
	})

	t.Run("off", func(t *testing.T) {
		e := newInboxEnv(t, nil)
		e.ch.cfg.Store(nil)
		rec, _ := e.get(t, http.MethodGet, "")
		p := requireProblem(t, rec, http.StatusNotFound, "not_found")
		require.Equal(t, realtimeOffTitle, p.Title)
		h := &channelsHandler{on: stubChannels{}, inbox: stubInbox{err: realtime.ErrRealtimeOff}}
		requireProblem(t, serve(h, http.MethodGet, ""), http.StatusNotFound, "not_found")
	})

	t.Run("no principal", func(t *testing.T) {
		h := &channelsHandler{on: stubChannels{}, inbox: stubInbox{err: realtime.ErrNoPrincipal}}
		requireProblem(t, serve(h, http.MethodGet, ""), http.StatusForbidden, realtime.TypeNoPrincipal)
		e := newInboxEnv(t, nil)
		rec := httptest.NewRecorder()
		e.app.channels.serveInbox(rec, httptest.NewRequest(http.MethodGet, inboxPath, http.NoBody), nil)
		requireProblem(t, rec, http.StatusForbidden, realtime.TypeNoPrincipal)
	})

	e, _ := seedInbox(t, 2)
	for _, q := range []string{"?limit=x", "?limit=0.5", "?limit=0", "?limit=-1", "?limit=201"} {
		t.Run("limit "+q, func(t *testing.T) {
			rec, _ := e.get(t, http.MethodGet, q)
			requireProblem(t, rec, http.StatusBadRequest, realtime.TypeInvalid)
		})
	}
	t.Run("bad cursor", func(t *testing.T) {
		rec, _ := e.get(t, http.MethodGet, "?cursor=%21%21")
		requireProblem(t, rec, http.StatusBadRequest, realtime.TypeInvalidCursor)
	})
	t.Run("method", func(t *testing.T) {
		rec, _ := e.get(t, http.MethodPost, "")
		require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
		require.Equal(t, http.MethodGet, rec.Header().Get("Allow"))
	})
	t.Run("row ceiling", func(t *testing.T) {
		capped, _ := seedInbox(t, 2)
		capped.chans.mu.Lock()
		capped.chans.maxRows = 2
		capped.chans.mu.Unlock()
		rec, _ := capped.get(t, http.MethodGet, "")
		requireProblem(t, rec, http.StatusServiceUnavailable, realtime.TypeInboxTooLarge)
	})
	t.Run("store fault", func(t *testing.T) {
		h := &channelsHandler{on: stubChannels{}, inbox: stubInbox{err: errors.New("disk on fire")}}
		rec := serve(h, http.MethodGet, "")
		require.Equal(t, http.StatusInternalServerError, rec.Code)
		require.NotContains(t, rec.Body.String(), "disk on fire")
	})
}
