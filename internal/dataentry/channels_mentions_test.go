package dataentry

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/filechannels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// Mention route tests (TKT-6RXAC0). acl.yaml maps logins to person
// entities, so a principal is its person id (USR-1 and so on) and a
// mention names a person by id.

// mentionTestRealtime: a room on a ticket that anyone who reads the
// ticket reads, and a case channel whose read needs inbox.read, which
// the officer-of relation confers per ticket.
const mentionTestRealtime = `
version: 1
channel_types:
  room:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      text: { properties: { body: { type: markdown } }, post: [member] }
    message_ops:
      delete_own: { roles: [member] }
  case:
    attach_to: [ticket]
    read: { permission: inbox.read }
    roles:
      member: { from: attached }
    message_types:
      text: { properties: { body: { type: markdown } }, post: [member] }
`

func mentionTestMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Label: "Ticket", IDPrefix: "TKT-", Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string", Required: true},
			}, PropertyOrder: []string{"title"}},
			"feature": {Label: "Feature", IDPrefix: "FEAT-", Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string", Required: true},
			}, PropertyOrder: []string{"title"}},
			"person": {Label: "Person", IDPrefix: "USR-", Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string"}, "login": {Type: "string"},
			}, PropertyOrder: []string{"title", "login"}},
		},
		Relations: map[string]metamodel.RelationDef{
			"officer-of": {Label: "officer of", From: []string{"person"}, To: []string{"ticket"}},
		},
	}
}

// mentionPolicy: USR-1 to USR-3 read tickets and people; nobody reads
// features; inbox.read comes from officer-of.
func mentionPolicy() *acl.Policy {
	return &acl.Policy{
		UserEntityType: "person", PrincipalProperty: "login",
		Roles: map[string]acl.RoleDef{
			"editor":  {Read: []string{"ticket", "person"}, Update: []string{"ticket"}},
			"officer": {Permissions: []string{"inbox.read"}},
		},
		RoleRelations: map[string]acl.RoleRelationDef{"officer-of": {Confers: "officer"}},
		Assignments:   map[string]string{"USR-1": "editor", "USR-2": "editor", "USR-3": "editor"},
	}
}

// mentionEnv is an app with a channel service whose config is compiled
// under mentionPolicy, read through counters.
type mentionEnv struct {
	app      *App
	d        *acl.Declarative
	svc      *realtime.Service
	store    channels.Store
	counting *storetest.Counting
	chans    *countingChannels
}

// newMentionEnv seeds persons USR-1 to USR-persons and tickets TKT-1 and
// any seed adds.
func newMentionEnv(t *testing.T, persons int, seed func(app *App)) *mentionEnv {
	t.Helper()
	meta := mentionTestMeta()
	app := newAppFromParts(&Config{}, meta, newFixture())
	for i := 1; i <= persons; i++ {
		id := fmt.Sprintf("USR-%d", i)
		seedEntity(app, &entity.Entity{ID: id, Type: "person", Properties: map[string]any{
			"title": "Person " + id, "login": fmt.Sprintf("user%d", i),
		}})
	}
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "one"}})
	if seed != nil {
		seed(app)
	}
	counting := storetest.NewCounting(app.store)
	policy := mentionPolicy()
	d, err := acl.NewDeclarative(policy, aclstore.MustNewGraph(counting), counting,
		acl.WithPrincipalLookup(aclstore.MustNewPrincipalLookup(app.store)))
	require.NoError(t, err)
	setTestACL(t, app, d)
	cs, err := filechannels.New(storage.NewMemFS(), "/channels", filelock.ProcessPrivate{})
	require.NoError(t, err)
	chans := newCountingChannels(cs)
	compiled, err := realtimecfg.Parse([]byte(mentionTestRealtime), meta, policy)
	require.NoError(t, err)
	require.Equal(t, "person", compiled.MentionUserType())
	p := &atomic.Pointer[realtimecfg.Compiled]{}
	p.Store(compiled)
	bundle := appbuildtest.New(meta, appbuildtest.WithStore(counting), appbuildtest.WithDeclarative(d),
		appbuildtest.WithChannelsLocal(chans, testRealtimeConfig{p: p, degraded: &atomic.Bool{}}, nil),
		appbuildtest.WithChannelOutputs(&fakeOutputs{}))
	t.Cleanup(func() { _ = bundle.Close() })
	svc := appbuild.ChannelService(bundle)
	require.NotNil(t, svc)
	require.NoError(t, SetChannels(app, svc))
	return &mentionEnv{app: app, d: d, svc: svc, store: cs, counting: counting, chans: chans}
}

func (e *mentionEnv) ctx(t *testing.T, user string) context.Context {
	t.Helper()
	return gateCtxFor(principalCtx(user), t, e.d)
}

var mentionRoom = realtime.RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "room"}

// serve serves method path with body as user and returns the response and
// what it cost.
func (e *mentionEnv) serve(t *testing.T, user, method, path, body string) (*httptest.ResponseRecorder, channelCost) {
	t.Helper()
	e.counting.Reset()
	e.chans.reset()
	rec := httptest.NewRecorder()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, http.NoBody)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := e.ctx(t, user)
	if strings.HasPrefix(path, inboxPath) {
		e.app.channels.serveInbox(rec, req.WithContext(ctx), e.app.State().Meta)
	} else {
		e.app.handleV1DynamicRoutes(rec, req.WithContext(ctx))
	}
	return rec, channelCost{entity: e.counting.Calls(), channel: e.chans.snapshot()}
}

func (e *mentionEnv) inbox(t *testing.T, user string) v1.Inbox {
	t.Helper()
	rec, _ := e.serve(t, user, http.MethodGet, inboxPath, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeAs[v1.Inbox](t, rec.Body.Bytes())
}

// spanList is n code spans `USR-from` onwards.
func spanList(from, n int) string {
	var b strings.Builder
	for i := from; i < from+n; i++ {
		fmt.Fprintf(&b, "`USR-%d` ", i)
	}
	return b.String()
}

// TestChannelsBudget_PostMentions pins TKT-6RXAC0 AC 13: a post that
// mentions one person costs what a post that mentions ten costs, and the
// ten are stored.
func TestChannelsBudget_PostMentions(t *testing.T) {
	cost := func(n int) channelCost {
		e := newMentionEnv(t, 12, nil)
		body := fmt.Sprintf(`{"type":"text","props":{"body":%q}}`, spanList(2, n))
		rec, c := e.serve(t, "USR-1", http.MethodPost, roomPath+"/messages", body)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		got := decodeAs[v1.Message](t, rec.Body.Bytes())
		require.Len(t, got.Mentions, n)
		m, err := e.store.Get(context.Background(), channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "room"}, 1)
		require.NoError(t, err)
		require.Len(t, m.Mentions, n)
		return c
	}
	one, ten := cost(1), cost(10)
	require.True(t, one.equal(ten), "1 mention: %s\n10 mentions: %s", one, ten)
}

// TestChannelsBudget_PostTooManyMentions pins the 422 for 11 people.
func TestChannelsBudget_PostTooManyMentions(t *testing.T) {
	e := newMentionEnv(t, 12, nil)
	body := fmt.Sprintf(`{"type":"text","props":{"body":%q}}`, spanList(2, 11))
	rec, _ := e.serve(t, "USR-1", http.MethodPost, roomPath+"/messages", body)
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
	require.Equal(t, realtime.TypeTooManyMentions, problemType(t, rec.Body.Bytes()))
}

// TestChannelsBudget_MessagesMentions pins TKT-6RXAC0 AC 13: a page of
// messages that each mention two people costs the same at 10 as at 50,
// and each message lists both as entity refs.
func TestChannelsBudget_MessagesMentions(t *testing.T) {
	cost := func(n int) channelCost {
		e := newMentionEnv(t, 3, nil)
		key := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "room"}
		for range n {
			_, err := e.store.Append(context.Background(), key, channels.NewMessage{
				Type: "text", Author: channels.Author{User: "USR-1", Tool: principal.ToolDataEntry}, At: time.Now(),
				Props: map[string]any{"body": "`USR-2` `USR-3`"},
				Mentions: []channels.Mention{
					{Target: "USR-2", Form: channels.FormUser}, {Target: "USR-3", Form: channels.FormUser},
				},
			})
			require.NoError(t, err)
		}
		rec, c := e.serve(t, "USR-1", http.MethodGet, fmt.Sprintf("%s/messages?limit=%d", roomPath, n), "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		page := decodeAs[v1.MessagePage](t, rec.Body.Bytes())
		require.Len(t, page.Messages, n)
		for _, m := range page.Messages {
			require.Len(t, m.Mentions, 2)
			for i, mm := range m.Mentions {
				id := fmt.Sprintf("USR-%d", i+2)
				require.Equal(t, v1.MessageMention{Text: id, Kind: "user", Ref: &v1.RefProp{
					ID: id, Type: "person", Title: mm.Ref.Title,
				}}, mm)
				require.NotEmpty(t, mm.Ref.Title)
			}
		}
		return c
	}
	small, large := cost(10), cost(50)
	require.True(t, small.equal(large), "10: %s\n50: %s", small, large)
}

// TestInboxMention_ACL pins TKT-6RXAC0 AC 11 through the deployed wiring:
// mentions of USR-1 on a ticket she may not read the case channel of (no
// officer relation) and on a feature she may not read change no byte of
// her inbox.
func TestInboxMention_ACL(t *testing.T) {
	at := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	mention := func(t *testing.T, e *mentionEnv, id string, at time.Time) {
		t.Helper()
		_, err := e.store.Append(context.Background(), channels.Key{Anchor: channels.Anchor{ID: id}, Type: "case"},
			channels.NewMessage{
				Type: "text", Author: channels.Author{User: "USR-2", Tool: principal.ToolDataEntry}, At: at,
				Props: map[string]any{"body": "`USR-1`"}, Mentions: []channels.Mention{{Target: "USR-1", Form: channels.FormUser}},
			})
		require.NoError(t, err)
	}
	build := func(hidden bool) *mentionEnv {
		e := newMentionEnv(t, 3, func(app *App) {
			seedEntity(app, &entity.Entity{ID: "TKT-100", Type: "ticket", Properties: map[string]any{"title": "shown"}})
			seedRelation(app, entity.NewRelation("USR-1", "officer-of", "TKT-100"))
			if hidden {
				seedEntity(app, &entity.Entity{ID: "TKT-500", Type: "ticket", Properties: map[string]any{"title": "no"}})
				seedEntity(app, &entity.Entity{ID: "FEAT-100", Type: "feature", Properties: map[string]any{"title": "f"}})
			}
		})
		if hidden {
			mention(t, e, "TKT-500", at)
			mention(t, e, "FEAT-100", at)
		}
		mention(t, e, "TKT-100", at.Add(time.Second))
		return e
	}
	plain, mixed := build(false), build(true)
	want, _ := plain.serve(t, "USR-1", http.MethodGet, inboxPath, "")
	got, _ := mixed.serve(t, "USR-1", http.MethodGet, inboxPath, "")
	require.Equal(t, want.Body.String(), got.Body.String())
	inbox := decodeAs[v1.Inbox](t, got.Body.Bytes())
	require.Equal(t, 1, *inbox.MentionCount)
	require.Equal(t, "/api/v1/tickets/TKT-100/_channels/case/messages/1", inbox.Mentions[0].Links.Message)
}

// TestInbox_MentionReadByMarker pins TKT-6RXAC0 AC 12: a mention leaves the
// inbox when the target's marker reaches it, comes back for a new mention
// at a higher seq, and goes when the author removes the message.
func TestInbox_MentionReadByMarker(t *testing.T) {
	e := newMentionEnv(t, 3, nil)
	post := func(body string) int64 {
		t.Helper()
		res, err := e.svc.Post(e.ctx(t, "USR-1"), mentionRoom, realtime.PostInput{
			Type: "text", Props: map[string]any{"body": body},
		})
		require.NoError(t, err)
		return res.Message.Seq
	}
	first := post("`USR-2` please look")
	got := e.inbox(t, "USR-2")
	require.Equal(t, 1, *got.MentionCount)
	require.Equal(t, first, got.Mentions[0].Seq)
	require.Equal(t, "mention", got.Mentions[0].Kind)
	require.Equal(t, 0, *e.inbox(t, "USR-3").MentionCount, "a mention addresses only its target")

	rec, _ := e.serve(t, "USR-2", http.MethodPut, roomPath+"/_read", fmt.Sprintf(`{"seq":%d}`, first))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, 0, *e.inbox(t, "USR-2").MentionCount)

	second := post("`USR-2` again")
	got = e.inbox(t, "USR-2")
	require.Equal(t, 1, *got.MentionCount)
	require.Equal(t, second, got.Mentions[0].Seq)

	_, err := e.svc.Delete(e.ctx(t, "USR-1"), mentionRoom, second, "")
	require.NoError(t, err)
	require.Equal(t, 0, *e.inbox(t, "USR-2").MentionCount)
}
