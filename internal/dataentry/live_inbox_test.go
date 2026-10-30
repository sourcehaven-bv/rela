package dataentry

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/filechannels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// liveInboxRealtime has a waiting type (case: an ask moves the session to
// a state that waits on the ticket's owner, and a timer moves it on) and a
// type without a waiting state (room). Both need inbox.read, which the
// officer-of relation confers.
const liveInboxRealtime = `
version: 1
channel_types:
  case:
    attach_to: [ticket, feature]
    read: { permission: inbox.read }
    typing: true
    roles:
      lead:   { from: entity.owner, label: the lead }
      member: { from: attached }
    message_types:
      chat:   { properties: { body: { type: markdown } }, post: [member] }
      ask:    { properties: { body: { type: string } }, post: [member] }
      answer: { properties: { note: { type: string } }, post: [lead] }
      status: { persist: ephemeral, ttl: 30s, post: [member] }
    session:
      initial: open
      states:
        open: { label: Open, on: { ask: { to: asked } } }
        asked:
          label: Asked
          waiting_for: [lead]
          on: { answer: { to: open } }
          after: { lapse: { period: 1h, to: lapsed, label: Lapsed } }
        lapsed: { label: Lapsed, final: true }
  room:
    attach_to: [ticket, feature]
    read: { permission: inbox.read }
    roles:
      member: { from: attached }
    message_types:
      chat: { properties: { body: { type: markdown } }, post: [member] }
      progress: { properties: { body: { type: markdown } }, post: [member], unread: false }
`

const (
	inboxCase1 = "/api/v1/tickets/TKT-1/_channels/case"
	inboxRoom1 = "/api/v1/tickets/TKT-1/_channels/room"
	inboxRoom2 = "/api/v1/tickets/TKT-2/_channels/room"
)

// liveStepper drives the hub's round clock by hand: tick n runs only when
// the test sends it, so a test can name the tick a frame lands on.
type liveStepper struct {
	h    *liveHub
	c    chan time.Time
	base time.Time
	slot int // the wall-clock slot of the next tick sent
}

// manualClock replaces app's round clock with a liveStepper. Call it
// before the hub starts.
func manualClock(app *App) *liveStepper {
	s := &liveStepper{h: app.live, c: make(chan time.Time), base: time.Now()}
	app.live.ticker = func(time.Duration) (<-chan time.Time, func()) { return s.c, func() {} }
	return s
}

// step runs n ticks and waits until each has finished.
func (s *liveStepper) step(tb testing.TB, n int) {
	tb.Helper()
	for range n {
		if !s.tryStep() {
			tb.Fatal("the live worker took no tick, or did not finish it")
		}
	}
}

// tryStep runs one tick and waits until it has finished. It reports a
// failure instead of failing the test, so it may run off the test
// goroutine.
func (s *liveStepper) tryStep() bool {
	want := s.h.ticks.Load() + 1
	at := s.base.Add(time.Duration(s.slot) * s.h.window)
	select {
	case s.c <- at:
	case <-time.After(2 * time.Second):
		return false
	}
	s.slot++
	deadline := time.Now().Add(30 * time.Second)
	for s.h.ticks.Load() < want {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(time.Millisecond)
	}
	return true
}

// through runs every tick up to and including tick n.
func (s *liveStepper) through(tb testing.TB, n uint64) {
	tb.Helper()
	for s.h.ticks.Load() <= n {
		s.step(tb, 1)
	}
}

// drop skips n wall-clock slots without sending a tick, as a time.Ticker
// does while the worker is busy.
func (s *liveStepper) drop(n int) { s.slot += n }

// next is the number of the tick that runs next.
func (s *liveStepper) next() uint64 { return s.h.ticks.Load() }

// nextGrid is the first inbox grid tick at or after the next tick, for a
// stepper that dropped no slots.
func (s *liveStepper) nextGrid() uint64 {
	n := s.next()
	for n%s.h.inboxGrid != 0 {
		n++
	}
	return n
}

// principalReplays counts the hub's replays by principal and URL.
type principalReplays struct {
	inner http.Handler
	mu    sync.Mutex
	n     map[string]int
}

func countPrincipalReplays(t *testing.T, h *liveHub) *principalReplays {
	t.Helper()
	inner := h.replay.Load()
	require.NotNil(t, inner, "no replay handler installed")
	c := &principalReplays{inner: *inner, n: map[string]int{}}
	var hh http.Handler = c
	h.replay.Store(&hh)
	return c
}

func (c *principalReplays) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	c.n[principal.From(r.Context()).User+" "+r.URL.RequestURI()]++
	c.mu.Unlock()
	c.inner.ServeHTTP(w, r)
}

// count is the count of key, "user URL", since the last take.
func (c *principalReplays) count(key string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n[key]
}

// take returns the counts since the last take and clears them.
func (c *principalReplays) take() map[string]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := c.n
	c.n = map[string]int{}
	return out
}

// countingInbox counts Service.Inbox calls.
type countingInbox struct {
	inner inboxReader
	n     atomic.Int64
}

func (c *countingInbox) Inbox(ctx context.Context, req realtime.InboxRequest) (realtime.InboxView, error) {
	c.n.Add(1)
	return c.inner.Inbox(ctx, req)
}

// refusingInbox refuses the inbox of user with no_principal, so that
// user's inbox subscriptions learn no wake set.
type refusingInbox struct {
	inner inboxReader
	user  string
}

func (r refusingInbox) Inbox(ctx context.Context, req realtime.InboxRequest) (realtime.InboxView, error) {
	if principal.From(ctx).User == r.user {
		return realtime.InboxView{}, realtime.ErrNoPrincipal
	}
	return r.inner.Inbox(ctx, req)
}

// liveInboxClock is a settable clock for realtime.Local.
type liveInboxClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *liveInboxClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *liveInboxClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// inboxLiveEnv is a live env whose channel service runs liveInboxRealtime
// on a hand-driven round clock.
type inboxLiveEnv struct {
	*liveEnv
	ch    *channelService
	local *realtime.Local
	clock *liveInboxClock
	ticks *liveStepper
}

// inboxLivePolicy: alice, bob and carol edit tickets, dave also features;
// inbox.read comes from officer-of. extra users edit tickets too.
func inboxLivePolicy(extra ...string) *acl.Policy {
	p := &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"editor":  {Read: []string{"ticket", "person"}, Update: []string{"ticket"}},
			"admin":   {Read: []string{"ticket", "feature", "person"}, Update: []string{"ticket", "feature"}},
			"officer": {Permissions: []string{"inbox.read"}},
		},
		RoleRelations: map[string]acl.RoleRelationDef{"officer-of": {Confers: "officer"}},
		Assignments:   map[string]string{"alice": "editor", "bob": "editor", "carol": "editor", "dave": "admin"},
	}
	for _, u := range extra {
		p.Assignments[u] = "editor"
	}
	return p
}

// newInboxLiveEnv seeds TKT-1 (owned by alice) and TKT-2, both with alice,
// bob and carol as officers, TKT-9 with only bob and dave as officers, and
// FEAT-1, which only dave reads.
func newInboxLiveEnv(t *testing.T, policy *acl.Policy) *inboxLiveEnv {
	t.Helper()
	return newInboxLiveEnvGrid(t, policy, LiveInboxGrid)
}

// newInboxLiveEnvGrid is newInboxLiveEnv with an inbox grid of grid ticks.
func newInboxLiveEnvGrid(t *testing.T, policy *acl.Policy, grid int) *inboxLiveEnv {
	t.Helper()
	cs, err := filechannels.New(storage.NewMemFS(), "/channels", filelock.ProcessPrivate{})
	require.NoError(t, err)
	return newInboxLiveEnvOn(t, policy, nil, cs, grid)
}

// newInboxLiveEnvOn is newInboxLiveEnvGrid over the channel store cs.
func newInboxLiveEnvOn(
	tb testing.TB, policy *acl.Policy, seed func(app *App), cs channels.Store, grid int,
) *inboxLiveEnv {
	tb.Helper()
	return newInboxLiveEnvWith(tb, policy, seed, cs, grid, liveInboxRealtime)
}

// newInboxLiveEnvWith is newInboxLiveEnvOn with realtime.yaml rt.
func newInboxLiveEnvWith(
	tb testing.TB, policy *acl.Policy, seed func(app *App), cs channels.Store, grid int, rt string,
) *inboxLiveEnv {
	tb.Helper()
	meta := inboxTestMeta()
	person := meta.Entities["person"]
	person.Properties["email"] = metamodel.PropertyDef{Type: "string"}
	person.PropertyOrder = append(person.PropertyOrder, "email")
	meta.Entities["person"] = person
	app := newAppFromParts(&Config{}, meta, newFixture())
	for _, id := range []string{"alice", "bob", "carol", "dave"} {
		seedEntity(app, &entity.Entity{ID: id, Type: "person",
			Properties: map[string]any{"title": id, "email": id + "@example.com"}})
	}
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "one", "owner": "alice"}})
	seedEntity(app, &entity.Entity{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "two"}})
	seedEntity(app, &entity.Entity{ID: "TKT-9", Type: "ticket", Properties: map[string]any{"title": "nine", "owner": "alice"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-1", Type: "feature", Properties: map[string]any{"title": "f", "owner": "alice"}})
	for _, id := range []string{"TKT-1", "TKT-2"} {
		for _, p := range []string{"alice", "bob", "carol", "dave"} {
			seedRelation(app, entity.NewRelation(p, "officer-of", id))
		}
	}
	for _, p := range []string{"bob", "dave"} {
		seedRelation(app, entity.NewRelation(p, "officer-of", "TKT-9"))
	}
	seedRelation(app, entity.NewRelation("dave", "officer-of", "FEAT-1"))
	if seed != nil {
		seed(app)
	}
	d, err := acl.NewDeclarative(policy, aclstore.MustNewGraph(app.store), app.store,
		acl.WithPrincipalLookup(aclstore.MustNewPrincipalLookup(app.store)))
	require.NoError(tb, err)
	setTestACL(tb, app, d)
	clock := &liveInboxClock{t: time.Now()}
	local, err := realtime.NewLocal(clock.now)
	require.NoError(tb, err)
	tb.Cleanup(local.Stop)
	ch := newChannelServiceLocal(tb, app.Meta(), app.store, d, rt, cs, local)
	require.NoError(tb, SetChannels(app, ch.svc))
	require.NoError(tb, SetInboxGrid(app, grid))
	// Every principal takes the last safety slice, so the safety replay
	// runs on a known tick: the last grid tick of the first safety period.
	last := app.live.safetySlices() - 1
	app.live.safetySliceOf = func(string) uint64 { return last }
	env := &inboxLiveEnv{ch: ch, local: local, clock: clock}
	env.liveEnv = newLiveEnvFor(tb, app, func(app *App) { env.ticks = manualClock(app) })
	return env
}

// seedInboxAnswers gives alice a waiting session on TKT-1, an unread room
// on TKT-1 and a mention on TKT-2, and hidden rows of each kind on TKT-9
// and FEAT-1.
func (e *inboxLiveEnv) seedInboxAnswers(t *testing.T) {
	t.Helper()
	at := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	add := func(id, typ, msgType, author string, props map[string]any, m []channels.Mention) {
		t.Helper()
		_, err := e.ch.store.Append(context.Background(), channels.Key{Anchor: channels.Anchor{ID: id}, Type: typ},
			channels.NewMessage{Type: msgType, Author: channels.Author{User: author, Tool: principal.ToolDataEntry},
				At: at, Props: props, Mentions: m})
		require.NoError(t, err)
	}
	mention := []channels.Mention{{Target: "alice", Form: channels.FormLogin}}
	for _, id := range []string{"TKT-1", "TKT-9", "FEAT-1"} {
		add(id, "room", "chat", "bob", map[string]any{"body": "x"}, nil)
		_, _, err := e.ch.store.SetRead(context.Background(),
			channels.Key{Anchor: channels.Anchor{ID: id}, Type: "room"}, channels.Reader{User: "alice"}, 1, at)
		require.NoError(t, err)
		add(id, "room", "chat", "bob", map[string]any{"body": "y"}, nil)
	}
	for _, id := range []string{"TKT-2", "TKT-9", "FEAT-1"} {
		add(id, "room", "chat", "bob", map[string]any{"body": "`@alice`"}, mention)
	}
	for _, id := range []string{"TKT-1", "TKT-9", "FEAT-1"} {
		e.post(t, "dave", "/api/v1/"+map[bool]string{true: "features", false: "tickets"}[id == "FEAT-1"]+"/"+id+
			"/_channels/case", "ask", `{"body":"x"}`)
	}
}

// post posts a message of typ with props as user and requires 201.
func (e *inboxLiveEnv) post(t *testing.T, user, path, typ, props string) int64 {
	t.Helper()
	resp, b := e.do(t, user, http.MethodPost, path+"/messages", fmt.Sprintf(`{"type":%q,"props":%s}`, typ, props), nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "post to %s as %s: %s", path, user, b)
	return decodeAs[struct {
		Seq int64 `json:"seq"`
	}](t, b).Seq
}

// chat posts a chat message with body as user.
func (e *inboxLiveEnv) chat(t *testing.T, user, path, body string) int64 {
	t.Helper()
	return e.post(t, user, path, "chat", fmt.Sprintf(`{"body":%q}`, body))
}

// markRead moves user's marker on path to seq.
func (e *inboxLiveEnv) markRead(t *testing.T, user, path string, seq int64) {
	t.Helper()
	resp, b := e.do(t, user, http.MethodPut, path+"/_read", fmt.Sprintf(`{"seq":%d}`, seq), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode, "%s", b)
}

// inboxSubs opens a stream as user with an inbox and a summary
// subscription, and any extra.
func (e *inboxLiveEnv) inboxSubs(t *testing.T, user string, extra ...liveSubEntry) *sseClient {
	t.Helper()
	c, conn := e.open(t, user)
	e.subscribe(t, user, conn, append([]liveSubEntry{
		{"inbox", inboxPath}, {"summary", inboxSummaryPath},
	}, extra...)...)
	return c
}

// settle runs tick 0, so a test starts from a quiet hub.
func (e *inboxLiveEnv) settle(tb testing.TB) {
	tb.Helper()
	e.ticks.step(tb, 1)
}

// inboxReplays keeps the inbox and summary counts of got.
func inboxReplays(got map[string]int) map[string]int {
	out := map[string]int{}
	for k, n := range got {
		if strings.Contains(k, " "+inboxPath) {
			out[k] = n
		}
	}
	return out
}

// TestLive_InboxRoundMemo pins AC 5: in a full round, 20 principals with
// a summary and a default page subscription in two tabs each (80
// subscriptions) cost 20 Service.Inbox calls; a page with another limit
// adds one.
func TestLive_InboxRoundMemo(t *testing.T) {
	users := make([]string, 20)
	for i := range users {
		users[i] = fmt.Sprintf("p%02d", i)
	}
	env := newInboxLiveEnv(t, inboxLivePolicy(users...))
	calls := &countingInbox{inner: env.app.channels.inbox}
	env.app.channels.inbox = calls
	for _, u := range users {
		for range 2 {
			_, conn := env.open(t, u)
			env.subscribe(t, u, conn, liveSubEntry{"inbox", inboxPath}, liveSubEntry{"summary", inboxSummaryPath})
		}
	}
	env.settle(t)

	fullRound := func() int64 {
		t.Helper()
		env.ticks.through(t, env.ticks.nextGrid()) // nothing pending
		calls.n.Store(0)
		update(t, env.app, &entity.Entity{ID: "TKT-2", Type: "ticket",
			Properties: map[string]any{"title": fmt.Sprintf("two %d", env.ticks.next())}})
		env.ticks.through(t, env.ticks.nextGrid())
		return calls.n.Load()
	}
	require.Equal(t, int64(20), fullRound())

	_, conn := env.open(t, users[0])
	env.subscribe(t, users[0], conn, liveSubEntry{"small", inboxPath + "?limit=10"})
	require.Equal(t, int64(21), fullRound())
}

// TestLive_InboxFramesOnMention pins AC 9: a mention of alice in a room
// outside her window frames her inbox and summary only; carol's inbox
// and alice's kinds=waiting page are not replayed.
func TestLive_InboxFramesOnMention(t *testing.T) {
	env := newInboxLiveEnv(t, inboxLivePolicy())
	alice := env.inboxSubs(t, "alice", liveSubEntry{"waiting", inboxPath + "?kinds=waiting"})
	carol := env.inboxSubs(t, "carol")
	env.settle(t)
	replays := countPrincipalReplays(t, env.app.live)

	env.chat(t, "bob", inboxRoom2, "`@alice` look")
	env.ticks.through(t, env.ticks.nextGrid())
	requireChanged(t, alice, "inbox", "summary")
	liveQuiet(t, carol)
	got := inboxReplays(replays.take())
	require.Equal(t, map[string]int{"alice " + inboxPath: 1, "alice " + inboxSummaryPath: 1}, got)
}

// TestLive_InboxOwnPostLearnsChannel pins AC 10: alice's first post in a
// room puts it in her window, so bob's next post there frames her inbox.
func TestLive_InboxOwnPostLearnsChannel(t *testing.T) {
	env := newInboxLiveEnv(t, inboxLivePolicy())
	alice := env.inboxSubs(t, "alice")
	env.settle(t)

	env.chat(t, "alice", inboxRoom2, "first")
	env.ticks.through(t, env.ticks.nextGrid())
	liveQuiet(t, alice) // nothing unread yet

	env.chat(t, "bob", inboxRoom2, "reply")
	env.ticks.through(t, env.ticks.nextGrid())
	requireChanged(t, alice, "inbox", "summary")
}

// TestLive_InboxFramesOnOwnMarkRead pins AC 10: alice's first mark read on
// a room with nothing unread for her puts it in her window; bob's mark
// read does not replay her inbox.
func TestLive_InboxFramesOnOwnMarkRead(t *testing.T) {
	env := newInboxLiveEnv(t, inboxLivePolicy())
	alice := env.inboxSubs(t, "alice")
	env.settle(t)
	replays := countPrincipalReplays(t, env.app.live)

	seq := env.chat(t, "bob", inboxRoom2, "before her mark")
	env.ticks.through(t, env.ticks.nextGrid())
	require.Empty(t, inboxReplays(replays.take()), "outside her window")

	env.markRead(t, "alice", inboxRoom2, seq)
	env.ticks.through(t, env.ticks.nextGrid())
	liveQuiet(t, alice)
	require.Equal(t, 1, replays.take()["alice "+inboxPath], "her own mark replays her inbox")

	env.markRead(t, "bob", inboxRoom2, seq)
	env.ticks.through(t, env.ticks.nextGrid())
	require.Empty(t, inboxReplays(replays.take()), "bob's mark replays no inbox of alice")

	env.chat(t, "bob", inboxRoom2, "new")
	env.ticks.through(t, env.ticks.nextGrid())
	requireChanged(t, alice, "inbox", "summary")
}

// TestLive_InboxWindowTopics pins AC 11: a message in a room in alice's
// window frames her inbox; one in a room outside it does not replay it.
func TestLive_InboxWindowTopics(t *testing.T) {
	env := newInboxLiveEnv(t, inboxLivePolicy())
	seq := env.chat(t, "bob", inboxRoom1, "hello")
	env.markRead(t, "alice", inboxRoom1, seq)
	alice := env.inboxSubs(t, "alice")
	env.settle(t)
	replays := countPrincipalReplays(t, env.app.live)

	env.chat(t, "bob", inboxRoom2, "elsewhere")
	env.ticks.through(t, env.ticks.nextGrid())
	liveQuiet(t, alice)
	require.Empty(t, inboxReplays(replays.take()))

	env.chat(t, "bob", inboxRoom1, "in her window")
	env.ticks.through(t, env.ticks.nextGrid())
	requireChanged(t, alice, "inbox", "summary")
}

// TestLive_InboxFramesOnSessionMove pins AC 12: a post that moves a
// session into a state waiting on alice frames her inbox, and a timer
// that moves it out frames it again.
func TestLive_InboxFramesOnSessionMove(t *testing.T) {
	env := newInboxLiveEnv(t, inboxLivePolicy())
	alice := env.inboxSubs(t, "alice")
	env.settle(t)

	env.post(t, "bob", inboxCase1, "ask", `{"body":"please"}`)
	env.ticks.through(t, env.ticks.nextGrid())
	requireChanged(t, alice, "inbox", "summary")

	tm, err := realtime.NewTimers(realtime.TimerDeps{
		Store: env.ch.store, Config: testRealtimeConfig{p: env.ch.cfg, degraded: &env.ch.degraded},
		Marker: env.local, Clock: func() time.Time { return time.Now().Add(2 * time.Hour) },
	})
	require.NoError(t, err)
	rep, err := tm.RunRound(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, rep.Fired, "%+v", rep)
	env.ticks.through(t, env.ticks.nextGrid())
	requireChanged(t, alice, "inbox", "summary")
}

// TestLive_InboxIgnoresEphemeral pins AC 13: typing, a status signal and
// their expiry in a waiting-type channel replay no inbox, while the head
// of that channel still frames.
func TestLive_InboxIgnoresEphemeral(t *testing.T) {
	env := newInboxLiveEnv(t, inboxLivePolicy())
	alice := env.inboxSubs(t, "alice", liveSubEntry{"head", inboxCase1})
	env.settle(t)
	replays := countPrincipalReplays(t, env.app.live)

	steps := []struct {
		name string
		do   func(t *testing.T)
	}{
		{"typing", func(t *testing.T) {
			t.Helper()
			resp, b := env.do(t, "bob", http.MethodPost, inboxCase1+"/_typing", "", nil)
			require.Equal(t, http.StatusAccepted, resp.StatusCode, "%s", b)
		}},
		{"signal", func(t *testing.T) {
			t.Helper()
			resp, b := env.do(t, "bob", http.MethodPost, inboxCase1+"/messages", `{"type":"status","props":{}}`, nil)
			require.Equal(t, http.StatusAccepted, resp.StatusCode, "%s", b)
		}},
		{"expiry", func(t *testing.T) {
			t.Helper()
			// carol's typing and signal prune bob's expired ones.
			env.clock.advance(time.Minute)
			resp, b := env.do(t, "carol", http.MethodPost, inboxCase1+"/_typing", "", nil)
			require.Equal(t, http.StatusAccepted, resp.StatusCode, "%s", b)
			resp, b = env.do(t, "carol", http.MethodPost, inboxCase1+"/messages", `{"type":"status","props":{}}`, nil)
			require.Equal(t, http.StatusAccepted, resp.StatusCode, "%s", b)
		}},
	}
	for _, s := range steps {
		t.Run(s.name, func(t *testing.T) {
			t.Helper()
			s.do(t)
			env.ticks.through(t, env.ticks.nextGrid())
			requireChanged(t, alice, "head")
			liveQuiet(t, alice)
			require.Empty(t, inboxReplays(replays.take()))
		})
	}
}

// TestLive_InboxHiddenChangeNoFrame pins AC 14 on a row-hidden anchor
// (FEAT-1) and on a channel whose read rule denies alice (TKT-9): a plain
// message outside her window replays nothing; a mention of her and a
// session move replay her inbox but send no frame.
func TestLive_InboxHiddenChangeNoFrame(t *testing.T) {
	for _, anchor := range []string{"/api/v1/features/FEAT-1", "/api/v1/tickets/TKT-9"} {
		t.Run(anchor, func(t *testing.T) {
			t.Helper()
			env := newInboxLiveEnv(t, inboxLivePolicy())
			alice := env.inboxSubs(t, "alice")
			env.settle(t)
			replays := countPrincipalReplays(t, env.app.live)

			env.chat(t, "dave", anchor+"/_channels/room", "plain")
			env.ticks.through(t, env.ticks.nextGrid())
			liveQuiet(t, alice)
			require.Empty(t, inboxReplays(replays.take()), "a plain hidden message")

			env.chat(t, "dave", anchor+"/_channels/room", "`@alice` hidden")
			env.ticks.through(t, env.ticks.nextGrid())
			liveQuiet(t, alice)
			require.Equal(t, 1, replays.take()["alice "+inboxPath], "a hidden mention replays")

			env.post(t, "dave", anchor+"/_channels/case", "ask", `{"body":"x"}`)
			env.ticks.through(t, env.ticks.nextGrid())
			liveQuiet(t, alice)
			require.Equal(t, 1, replays.take()["alice "+inboxPath], "a hidden session move replays")
		})
	}
}

// TestLive_InboxGridTimingIndependent pins AC 15: a visible mention one
// tick after a hidden change frames on the same grid tick as without the
// hidden change.
func TestLive_InboxGridTimingIndependent(t *testing.T) {
	hidden := map[string]func(t *testing.T, env *inboxLiveEnv){
		"none": func(*testing.T, *inboxLiveEnv) {},
		"hidden mention": func(t *testing.T, env *inboxLiveEnv) {
			t.Helper()
			env.chat(t, "dave", "/api/v1/features/FEAT-1/_channels/room", "`@alice`")
		},
		"hidden waiting-type message": func(t *testing.T, env *inboxLiveEnv) {
			t.Helper()
			env.post(t, "dave", "/api/v1/tickets/TKT-9/_channels/case", "ask", `{"body":"x"}`)
		},
		"graph write to a hidden type": func(t *testing.T, env *inboxLiveEnv) {
			t.Helper()
			update(t, env.app, &entity.Entity{ID: "FEAT-1", Type: "feature",
				Properties: map[string]any{"title": "moved", "owner": "alice"}})
		},
	}
	for _, grid := range []int{LiveInboxGrid, LiveInboxGridScan} {
		for name, change := range hidden {
			t.Run(fmt.Sprintf("grid %d/%s", grid, name), func(t *testing.T) {
				t.Helper()
				env := newInboxLiveEnvGrid(t, inboxLivePolicy(), grid)
				alice := env.inboxSubs(t, "alice")
				env.ticks.through(t, uint64(grid)) // tick grid+1 runs next
				change(t, env)
				env.ticks.step(t, 1)
				env.chat(t, "bob", inboxRoom2, "`@alice` visible")
				env.ticks.step(t, grid-2) // up to the tick before the next grid tick
				liveQuiet(t, alice)
				env.ticks.step(t, 1) // tick 2*grid, the next grid tick
				requireChanged(t, alice, "inbox", "summary")
			})
		}
	}
}

// TestLive_InboxOnlyOnGrid pins AC 15: inbox replays run on grid ticks
// only, for topic and full rounds alike, and the PUT baseline does not
// wait for the grid.
func TestLive_InboxOnlyOnGrid(t *testing.T) {
	for _, grid := range []int{LiveInboxGrid, LiveInboxGridScan} {
		t.Run(fmt.Sprintf("grid %d", grid), func(t *testing.T) { testInboxOnlyOnGrid(t, grid) })
	}
}

func testInboxOnlyOnGrid(t *testing.T, grid int) {
	t.Helper()
	env := newInboxLiveEnvGrid(t, inboxLivePolicy(), grid)
	env.ticks.through(t, uint64(grid))
	replays := countPrincipalReplays(t, env.app.live)
	alice := env.inboxSubs(t, "alice")
	require.Equal(t, map[string]int{"alice " + inboxPath: 1, "alice " + inboxSummaryPath: 1},
		inboxReplays(replays.take()), "the baseline runs off the grid")

	for name, change := range map[string]func(){
		"topic round": func() { env.chat(t, "bob", inboxRoom2, "`@alice`") },
		"full round": func() {
			update(t, env.app, &entity.Entity{ID: "TKT-1", Type: "ticket",
				Properties: map[string]any{
					"title": "one " + strconv.FormatUint(env.ticks.next(), 10), "owner": "alice",
				}})
		},
	} {
		t.Run(name, func(t *testing.T) {
			at := env.ticks.nextGrid()
			if at == env.ticks.next() {
				env.ticks.step(t, 1)
				at = env.ticks.nextGrid()
			}
			liveDrain(alice)
			replays.take()
			change()
			for env.ticks.next() < at {
				env.ticks.step(t, 1)
				require.Empty(t, inboxReplays(replays.take()), "tick %d is off the grid", env.ticks.next()-1)
			}
			env.ticks.step(t, 1)
			require.NotEmpty(t, inboxReplays(replays.take()), "grid tick %d", at)
		})
	}
}

// TestLive_InboxSafetyReplay pins AC 16: with no change in the process,
// every inbox subscription with a wake set replays on its safety slice's
// grid tick, so a write made straight in the channel store frames by then.
// One without a wake set is not replayed. The env puts every principal in
// the last slice, whose first turn is the grid tick safety-grid.
func TestLive_InboxSafetyReplay(t *testing.T) {
	for _, tc := range []struct{ grid, safety int }{{LiveInboxGrid, 28}, {LiveInboxGridScan, 24}} {
		t.Run(fmt.Sprintf("grid %d", tc.grid), func(t *testing.T) { testInboxSafetyReplay(t, tc.grid, tc.safety) })
	}
}

func testInboxSafetyReplay(t *testing.T, grid, safety int) {
	t.Helper()
	env := newInboxLiveEnvGrid(t, inboxLivePolicy(), grid)
	require.Equal(t, uint64(safety), env.app.live.safetyPeriod())
	seq := env.chat(t, "bob", inboxRoom1, "hello")
	env.markRead(t, "alice", inboxRoom1, seq)
	env.app.channels.inbox = refusingInbox{inner: env.app.channels.inbox, user: "erin"}
	alice := env.inboxSubs(t, "alice")
	_, conn := env.open(t, "erin")
	env.subscribe(t, "erin", conn, liveSubEntry{"inbox", inboxPath})
	env.settle(t)
	replays := countPrincipalReplays(t, env.app.live)

	_, err := env.ch.store.Append(context.Background(),
		channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "room"},
		channels.NewMessage{Type: "chat", Author: channels.Author{User: "bob", Tool: principal.ToolDataEntry},
			At: time.Now(), Props: map[string]any{"body": "from another process"}})
	require.NoError(t, err)
	at := safety - grid // the last slice's first grid tick
	env.ticks.through(t, uint64(at-1))
	liveQuiet(t, alice)
	require.Empty(t, inboxReplays(replays.take()))

	env.ticks.step(t, 1) // tick at, alice's safety turn
	requireChanged(t, alice, "inbox", "summary")
	require.Equal(t, map[string]int{"alice " + inboxPath: 1, "alice " + inboxSummaryPath: 1},
		inboxReplays(replays.take()))
}

// TestLive_InboxWakeSetFromReplay pins AC 17: with realtime off the inbox
// replays to 404 and learns no wake set, so a channel change does not
// replay it; a full round does, at the next grid tick, so turning
// realtime on frames it. A replay that did not finish keeps the wake set.
func TestLive_InboxWakeSetFromReplay(t *testing.T) {
	env := newInboxLiveEnv(t, inboxLivePolicy())
	cfg := env.ch.cfg.Load()
	env.ch.cfg.Store(nil)
	alice := env.inboxSubs(t, "alice")
	env.settle(t)
	replays := countPrincipalReplays(t, env.app.live)
	for _, s := range env.app.live.subsOf("alice") {
		require.Nil(t, s.wake, "a 404 teaches no wake set")
	}

	env.local.Mark(channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "case"})
	env.ticks.through(t, env.ticks.nextGrid())
	require.Empty(t, inboxReplays(replays.take()))

	env.ch.cfg.Store(cfg)
	update(t, env.app, &entity.Entity{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "on"}})
	env.ticks.through(t, env.ticks.nextGrid())
	requireChanged(t, alice, "inbox", "summary")
	for _, s := range env.app.live.subsOf("alice") {
		require.NotNil(t, s.wake, "a 200 teaches the wake set")
	}

	t.Run("unfinished replay keeps it", func(t *testing.T) {
		w := &inboxWake{user: "alice"}
		s := &liveSub{inbox: true, wake: w}
		applyTopic(s, replayResult{})
		require.Same(t, w, s.wake)
		applyTopic(s, replayResult{ok: true, status: http.StatusNotFound})
		require.Nil(t, s.wake)
	})
}

// subsOf returns the subscriptions of user's streams.
func (h *liveHub) subsOf(user string) []*liveSub {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []*liveSub
	for _, c := range h.conns {
		if c.owner.User != user {
			continue
		}
		c.mu.Lock()
		for _, s := range c.subs {
			out = append(out, s)
		}
		c.mu.Unlock()
	}
	return out
}

// TestLive_InboxResync pins AC 18: a resync replays every inbox
// subscription with a wake set at the next grid tick, and none without.
func TestLive_InboxResync(t *testing.T) {
	fake := &fakeChanges{wake: make(chan struct{}, 1)}
	var ticks *liveStepper
	env := newChannelEnvWith(t, channelPolicy, func(app *App) {
		app.channels.changes = func() channelChanges { return fake }
		app.channels.inbox = refusingInbox{inner: app.channels.inbox, user: "bob"}
		ticks = manualClock(app)
	})
	c, conn := env.open(t, "alice")
	env.subscribe(t, "alice", conn, liveSubEntry{"inbox", inboxPath})
	_, refused := env.open(t, "bob")
	env.subscribe(t, "bob", refused, liveSubEntry{"inbox", inboxPath})
	ticks.step(t, 1)
	replays := countPrincipalReplays(t, env.app.live)

	fake.fire()
	grid := ticks.nextGrid()
	ticks.step(t, 1)
	require.Empty(t, replays.take(), "off the grid")
	ticks.through(t, grid)
	liveQuiet(t, c)
	require.Equal(t, map[string]int{"alice " + inboxPath: 1}, replays.take())
}

// TestRoundFilter_SelectsInbox pins the selection rule of an inbox
// subscription.
func TestRoundFilter_SelectsInbox(t *testing.T) {
	alice := channels.Reader{User: "alice"}
	roomKey := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "room"}
	wake := &inboxWake{
		user: "alice", reader: alice, mentionKinds: true,
		types: map[string]bool{"case": true},
		keys:  map[string]bool{channelTopic("TKT-1", "room", ""): true},
	}
	sub := &liveSub{inbox: true, wake: wake}
	waitingOnly := &liveSub{inbox: true, wake: &inboxWake{user: "alice", reader: alice, types: wake.types}}
	unlearned := &liveSub{inbox: true}
	filter := func(build func(f *roundFilter)) *roundFilter {
		f := newRoundFilter(nil, nil, false)
		build(f)
		return f
	}
	other := channels.Key{Anchor: channels.Anchor{ID: "TKT-2"}, Type: "room"}
	for _, tc := range []struct {
		name string
		f    *roundFilter
		sub  *liveSub
		want bool
	}{
		{"durable key in window", filter(func(f *roundFilter) { f.addInbox([]channels.Key{roomKey}, nil) }), sub, true},
		{"durable key elsewhere", filter(func(f *roundFilter) { f.addInbox([]channels.Key{other}, nil) }), sub, false},
		{"waiting type", filter(func(f *roundFilter) {
			f.addInbox([]channels.Key{{Anchor: channels.Anchor{ID: "TKT-7"}, Type: "case"}}, nil)
		}), sub, true},
		{"addressed", filter(func(f *roundFilter) { f.addInbox(nil, []string{"alice"}) }), sub, true},
		{"addressed, kinds without mentions", filter(func(f *roundFilter) { f.addInbox(nil, []string{"alice"}) }),
			waitingOnly, false},
		{"someone else addressed", filter(func(f *roundFilter) { f.addInbox(nil, []string{"carol"}) }), sub, false},
		{"own marker", filter(func(f *roundFilter) {
			f.addReads(map[channels.Reader][]channels.Key{alice: {other}})
		}), sub, true},
		{"another reader's marker", filter(func(f *roundFilter) {
			f.addReads(map[channels.Reader][]channels.Key{{User: "bob"}: {roomKey}})
		}), sub, false},
		{"non-durable key only", newRoundFilter(nil, []channels.Key{roomKey}, false), sub, false},
		{"resync", newRoundFilter(nil, nil, true), sub, true},
		{"resync without wake set", newRoundFilter(nil, nil, true), unlearned, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.f.selects(tc.sub))
		})
	}
}

// TestInboxRequestKey pins the memo key: equal requests share it.
func TestInboxRequestKey(t *testing.T) {
	all := []string{"waiting", "unread", "mention", "reply"}
	require.Equal(t, inboxRequestKey(realtime.InboxRequest{}), inboxRequestKey(realtime.InboxRequest{Limit: 50}))
	require.Equal(t, inboxRequestKey(realtime.InboxRequest{}), inboxRequestKey(realtime.InboxRequest{Kinds: all}))
	require.Equal(t, inboxRequestKey(realtime.InboxRequest{Kinds: []string{"mention", "waiting"}}),
		inboxRequestKey(realtime.InboxRequest{Kinds: []string{"waiting", "mention", "waiting"}}))
	require.NotEqual(t, inboxRequestKey(realtime.InboxRequest{}), inboxRequestKey(realtime.InboxRequest{Limit: 10}))
	require.NotEqual(t, inboxRequestKey(realtime.InboxRequest{}),
		inboxRequestKey(realtime.InboxRequest{Kinds: []string{"waiting"}}))
}

// TestSetInboxGrid pins the grid setter: it rejects a spacing outside one
// to liveSafetyMax ticks, and the safety spacing stays the largest
// multiple of the grid within that bound.
func TestSetInboxGrid(t *testing.T) {
	app := newAppFromParts(&Config{}, inboxTestMeta(), newFixture())
	require.Equal(t, uint64(LiveInboxGrid), app.live.inboxGrid, "the default is the database grid")
	for _, bad := range []int{0, -1, liveSafetyMax + 1} {
		require.Error(t, SetInboxGrid(app, bad), "grid %d", bad)
	}
	for grid, safety := range map[int]uint64{LiveInboxGrid: 28, LiveInboxGridScan: 24, 5: 25, 28: 28} {
		require.NoError(t, SetInboxGrid(app, grid))
		require.Equal(t, safety, app.live.safetyPeriod(), "grid %d", grid)
		require.Equal(t, safety/uint64(grid), app.live.safetySlices(), "grid %d", grid)
	}
}

// TestInboxClock_TickAt pins the wall-clock grid: a grid tick is the
// first tick of each grid period, counted from the first tick's time, so
// dropped ticks do not shift later grid ticks. A grid tick replays the
// safety slices of every period since the last grid tick.
func TestInboxClock_TickAt(t *testing.T) {
	const window = 500 * time.Millisecond
	base := time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)
	at := func(slot int) time.Time { return base.Add(time.Duration(slot) * window) }
	var c inboxClock
	got := func(slot int) inboxTick { return c.tickAt(at(slot), window, 4, 7) }

	require.Equal(t, inboxTick{grid: true, slices: 1}, got(0))
	require.Equal(t, inboxTick{}, got(1))
	require.Equal(t, inboxTick{}, got(3))
	require.Equal(t, inboxTick{grid: true, slices: 1 << 1}, got(4))
	// Slots 5 to 9 are dropped: slot 10 is the first tick of period 2.
	require.Equal(t, inboxTick{grid: true, slices: 1 << 2}, got(10))
	require.Equal(t, inboxTick{}, got(11), "a drop does not move the next grid tick off slot 12")
	require.Equal(t, inboxTick{grid: true, slices: 1 << 3}, got(12))
	// A round that ran over two grid periods: slot 21 is in period 5, and
	// the tick replays the slices of periods 4 and 5.
	require.Equal(t, inboxTick{grid: true, slices: 1<<4 | 1<<5}, got(21))
	// A jitter of under half a window keeps the slot.
	require.Equal(t, inboxTick{grid: true, slices: 1 << 6}, c.tickAt(at(24).Add(window/3), window, 4, 7))
	// More than a whole safety period skipped replays every slice.
	require.Equal(t, inboxTick{grid: true, slices: 1<<7 - 1}, got(60))
}

// TestLive_InboxGridSurvivesDroppedTicks pins cr1 on the hub: after the
// ticker drops ticks during a slow round, the next grid tick still falls
// on the wall-clock grid, not grid ticks after the last one that ran.
func TestLive_InboxGridSurvivesDroppedTicks(t *testing.T) {
	env := newInboxLiveEnv(t, inboxLivePolicy())
	alice := env.inboxSubs(t, "alice")
	env.settle(t)        // slot 0
	env.ticks.step(t, 2) // slots 1 and 2
	env.ticks.drop(3)    // slots 3 to 5: a slow round
	env.ticks.step(t, 1) // slot 6, the first tick of period 1
	env.ticks.step(t, 1) // slot 7
	replays := countPrincipalReplays(t, env.app.live)
	env.chat(t, "bob", inboxRoom2, "`@alice`")
	env.ticks.step(t, 1) // slot 8
	requireChanged(t, alice, "inbox", "summary")
	require.Equal(t, 1, replays.take()["alice "+inboxPath], "slot 8 is a grid tick although 6 ticks ran since slot 0")
}

// TestLive_InboxOverflowNoFallbackFrame pins sr1: an inbox subscription a
// round has no room (cap) or time (budget) for gets no fallback frame. The
// round here is selected only by a write alice cannot see, so a frame
// would tell her it happened. It replays from pending on a later grid
// tick and, unchanged, still sends nothing.
func TestLive_InboxOverflowNoFallbackFrame(t *testing.T) {
	for name, limit := range map[string]func(h *liveHub){
		"round cap":    func(h *liveHub) { h.roundCap = 1 },
		"round budget": func(h *liveHub) { h.roundBudget = -1 },
	} {
		t.Run(name, func(t *testing.T) {
			env := newInboxLiveEnv(t, inboxLivePolicy())
			alice := env.inboxSubs(t, "alice")
			env.settle(t)
			limit(env.app.live) // the worker reads it next on the tick below
			replays := countPrincipalReplays(t, env.app.live)
			env.post(t, "dave", "/api/v1/tickets/TKT-9/_channels/case", "ask", `{"body":"x"}`)
			env.ticks.through(t, env.ticks.nextGrid()) // the grid tick overflows
			liveQuiet(t, alice)
			require.NotEmpty(t, env.app.live.pendingOf("alice"), "the overflowed inbox waits in pending")
			env.ticks.step(t, LiveInboxGrid-1) // off-grid ticks keep it there
			liveQuiet(t, alice)
			require.NotEmpty(t, env.app.live.pendingOf("alice"))

			h := env.app.live
			h.roundCap, h.roundBudget = liveRoundCap, liveRoundBudget
			replays.take()
			env.ticks.through(t, env.ticks.nextGrid())
			liveQuiet(t, alice)
			require.NotEmpty(t, inboxReplays(replays.take()), "pending replays on the next grid tick")
			require.Empty(t, env.app.live.pendingOf("alice"))
		})
	}
}

// TestLive_InboxSafetyStaggered pins cr2: the safety replay spreads the
// principals over the grid ticks of a safety period, so each grid tick
// replays about one slice of them, and every pair replays once per period.
func TestLive_InboxSafetyStaggered(t *testing.T) {
	users := make([]string, 70)
	for i := range users {
		users[i] = fmt.Sprintf("u%02d", i)
	}
	env := newInboxLiveEnv(t, inboxLivePolicy(users...))
	env.app.live.safetySliceOf = nil // the real hash
	for _, u := range users {
		env.inboxSubs(t, u)
	}
	env.settle(t)
	h := env.app.live
	replays := countPrincipalReplays(t, h)
	total := map[string]int{}
	period := int(h.safetyPeriod())
	perTick := 2 * len(users) / int(h.safetySlices()) * 2 // twice the even share
	for range period {
		env.ticks.step(t, 1)
		got := inboxReplays(replays.take())
		n := 0
		for k, c := range got {
			total[k] += c
			n += c
		}
		require.LessOrEqual(t, n, perTick, "tick %d replays one slice, not every pair", env.ticks.next()-1)
	}
	require.Len(t, total, 2*len(users), "every pair replays within one safety period")
	for k, c := range total {
		require.Equal(t, 1, c, k)
	}
}

// pendingOf returns the pending subscriptions of user's streams.
func (h *liveHub) pendingOf(user string) []*liveSub {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []*liveSub
	for _, c := range h.conns {
		if c.owner.User != user {
			continue
		}
		c.mu.Lock()
		for _, s := range c.subs {
			if _, ok := h.pending[s]; ok {
				out = append(out, s)
			}
		}
		c.mu.Unlock()
	}
	return out
}

// TestLiveInbox_NoUnreadPostSendsNoFrame pins TKT-O6RC8Z AC 7: a post of a
// type with `unread: false` into a room in alice's window with nothing
// unread replays her inbox but sends no frame.
func TestLiveInbox_NoUnreadPostSendsNoFrame(t *testing.T) {
	env := newInboxLiveEnv(t, inboxLivePolicy())
	seq := env.chat(t, "bob", inboxRoom1, "hello")
	env.markRead(t, "alice", inboxRoom1, seq)
	alice := env.inboxSubs(t, "alice")
	env.settle(t)
	replays := countPrincipalReplays(t, env.app.live)

	env.post(t, "bob", inboxRoom1, "progress", `{"body":"step"}`)
	env.ticks.through(t, env.ticks.nextGrid())
	liveQuiet(t, alice)
	require.Equal(t, 1, replays.take()["alice "+inboxPath], "the post is in her window, so it replays")
}

// TestLiveInbox_NoUnreadPostMovesLastSeq pins TKT-O6RC8Z AC 7: a post of a
// type with `unread: false` into a room already listed under alice's
// unread channels frames her inbox, because the row's last_seq moves; the
// count stays 1 and the summary sends no frame.
func TestLiveInbox_NoUnreadPostMovesLastSeq(t *testing.T) {
	env := newInboxLiveEnv(t, inboxLivePolicy())
	seq := env.chat(t, "bob", inboxRoom1, "hello")
	env.markRead(t, "alice", inboxRoom1, seq)
	env.chat(t, "bob", inboxRoom1, "unread")
	alice := env.inboxSubs(t, "alice")
	env.settle(t)

	env.post(t, "bob", inboxRoom1, "progress", `{"body":"step"}`)
	env.ticks.through(t, env.ticks.nextGrid())
	requireChanged(t, alice, "inbox")
	liveQuiet(t, alice)
	resp, b := env.do(t, "alice", http.MethodGet, inboxPath, "", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode, "%s", b)
	inbox := decodeAs[v1.Inbox](t, b)
	require.Equal(t, 1, *inbox.UnreadCount)
	require.Len(t, inbox.UnreadChannels, 1)
	require.Equal(t, 1, inbox.UnreadChannels[0].Unread, "the count does not change")
	require.Equal(t, seq+2, inbox.UnreadChannels[0].LastSeq, "last_seq moves to the progress post")
}

// TestLiveInbox_NoUnreadPostSummaryNoFrame pins TKT-O6RC8Z AC 7: the
// summary subscription, the sidebar badge, sends no frame for a post of a
// type with `unread: false`, whether the room has unread messages or not.
func TestLiveInbox_NoUnreadPostSummaryNoFrame(t *testing.T) {
	env := newInboxLiveEnv(t, inboxLivePolicy())
	seq := env.chat(t, "bob", inboxRoom1, "hello")
	env.markRead(t, "alice", inboxRoom1, seq)
	c, conn := env.open(t, "alice")
	env.subscribe(t, "alice", conn, liveSubEntry{"summary", inboxSummaryPath})
	env.settle(t)

	env.post(t, "bob", inboxRoom1, "progress", `{"body":"caught up"}`)
	env.ticks.through(t, env.ticks.nextGrid())
	liveQuiet(t, c)

	env.chat(t, "bob", inboxRoom1, "unread")
	env.ticks.through(t, env.ticks.nextGrid())
	requireChanged(t, c, "summary")
	env.post(t, "bob", inboxRoom1, "progress", `{"body":"after unread"}`)
	env.ticks.through(t, env.ticks.nextGrid())
	liveQuiet(t, c)
}
