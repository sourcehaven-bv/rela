package dataentry

import (
	"context"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// Live channel delivery (TKT-ZU7P13). The channel head and the channel
// list are the live shapes; a frame tells the client to fetch
// messages?after_rev= itself.

// replayCounter wraps the hub's replay handler and counts the replays of
// each URL.
type replayCounter struct {
	inner http.Handler
	mu    sync.Mutex
	n     map[string]int
}

// countReplaysByURL installs a replayCounter on h, after NewRouter installed the
// replay handler.
func countReplaysByURL(t *testing.T, h *liveHub) *replayCounter {
	t.Helper()
	inner := h.replay.Load()
	if inner == nil {
		t.Fatal("no replay handler installed")
	}
	c := &replayCounter{inner: *inner, n: map[string]int{}}
	var hh http.Handler = c
	h.replay.Store(&hh)
	return c
}

func (c *replayCounter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	c.n[r.URL.RequestURI()]++
	c.mu.Unlock()
	c.inner.ServeHTTP(w, r)
}

// take returns the counts since the last take and clears them.
func (c *replayCounter) take() map[string]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := c.n
	c.n = map[string]int{}
	return out
}

// AC5: a message frames the head of its channel and the channel list of its
// anchor, and nothing else: not another channel, not the entity, and not a
// subscription to a channel the subscriber cannot read. The round replays
// only that head and that list: not another channel of the same anchor,
// and not a subscription without a topic.
func TestLive_ChannelFrameIsolation(t *testing.T) {
	env := newChannelEnv(t)
	thread, err := env.ch.store.CreateMany(context.Background(), channels.Anchor{ID: "TKT-1"}, channels.NewChannel{
		Type: "threads", Title: "side", By: channels.Author{User: "alice"}, At: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	listPath := "/api/v1/tickets/TKT-1/_channels"
	threadPath := listPath + "/threads/" + thread.Key.Name
	c, conn := env.open(t, "alice")
	env.subscribe(t, "alice", conn,
		liveSubEntry{"head", roomPath},
		liveSubEntry{"list", listPath},
		liveSubEntry{"thread", threadPath},
		liveSubEntry{"other", "/api/v1/tickets/TKT-2/_channels/room"},
		liveSubEntry{"entity", "/api/v1/tickets/TKT-1"},
		liveSubEntry{"hidden", "/api/v1/features/FEAT-1/_channels/room"},
		liveSubEntry{"secret", "/api/v1/tickets/TKT-1/_channels/secret"},
	)
	if got := env.app.live.subTopic(conn, "thread"); got == "" {
		t.Fatal("the second channel on the anchor has no topic, so the test would not see it replayed")
	}
	liveQuiet(t, c)
	replays := countReplaysByURL(t, env.app.live)

	env.postText(t, "bob", roomPath, "hello")
	requireChanged(t, c, "head", "list")
	liveQuiet(t, c)
	if got, want := replays.take(), map[string]int{roomPath: 1, listPath: 1}; !maps.Equal(got, want) {
		t.Fatalf("a post to the room replayed %v, want only its head and its anchor's list %v", got, want)
	}

	// An ephemeral message shows in the head only; the list holds no
	// ephemeral state.
	if resp, b := env.do(t, "bob", http.MethodPost, roomPath+"/messages", `{"type":"typing"}`, nil); resp.StatusCode != http.StatusAccepted {
		t.Fatalf("typing = %d %s", resp.StatusCode, b)
	}
	requireChanged(t, c, "head")
	liveQuiet(t, c)

	// A message on a channel alice cannot read sends her no frame, though
	// she subscribed to it.
	env.postText(t, "dave", "/api/v1/features/FEAT-1/_channels/room", "secret")
	liveQuiet(t, c)

	env.postText(t, "bob", "/api/v1/tickets/TKT-2/_channels/room", "elsewhere")
	requireChanged(t, c, "other")
	liveQuiet(t, c)
}

// AC5: the topic comes from the replay, never from the client, and only
// from a 200.
func TestLive_ChannelTopicFromReplay(t *testing.T) {
	env := newChannelEnv(t)
	_, conn := env.open(t, "alice")
	env.subscribe(t, "alice", conn,
		liveSubEntry{"head", roomPath},
		liveSubEntry{"list", "/api/v1/tickets/TKT-1/_channels"},
		liveSubEntry{"hidden", "/api/v1/features/FEAT-1/_channels/room"},
		liveSubEntry{"secret", "/api/v1/tickets/TKT-1/_channels/secret"},
	)
	want := map[string]string{
		"head":   channelTopic("TKT-1", "room", ""),
		"list":   anchorTopic("TKT-1"),
		"hidden": "",
		"secret": "",
	}
	for key, topic := range want {
		if got := env.app.live.subTopic(conn, key); got != topic {
			t.Errorf("%s topic = %q, want %q", key, got, topic)
		}
	}
}

// subTopic returns the topic of the subscription key of the stream conn.
func (h *liveHub) subTopic(conn, key string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	c := h.conns[conn]
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.subs[key].topic
}

// AC5: a reader who loses read access gets one frame (the head now answers
// 404) and none for later messages on the channel.
func TestLive_RevokedReaderStops(t *testing.T) {
	env := newChannelEnv(t)
	alice, aliceConn := env.open(t, "alice")
	bob, bobConn := env.open(t, "bob")
	env.subscribe(t, "alice", aliceConn, liveSubEntry{"head", roomPath})
	env.subscribe(t, "bob", bobConn, liveSubEntry{"head", roomPath})
	liveDrain(alice)
	liveDrain(bob)

	k := entity.RelationKey{From: "bob", Type: "member-of", To: "editors"}
	if err := env.app.store.DeleteRelation(context.Background(), k); err != nil {
		t.Fatal(err)
	}
	requireChanged(t, bob, "head")
	liveQuiet(t, bob)
	liveQuiet(t, alice)
	if code, _ := env.get(t, "bob", roomPath); code != http.StatusNotFound {
		t.Fatalf("bob's head after revocation = %d, want 404", code)
	}
	if got := env.app.live.subTopic(bobConn, "head"); got != "" {
		t.Fatalf("bob's topic after revocation = %q, want none", got)
	}

	env.postText(t, "alice", roomPath, "after")
	requireChanged(t, alice, "head")
	liveQuiet(t, bob)
}

// fakeChanges is a channelChanges a test drives: a wake makes the next
// Take report resync.
type fakeChanges struct {
	wake   chan struct{}
	resync atomic.Bool
}

func (f *fakeChanges) Wake() <-chan struct{} { return f.wake }
func (f *fakeChanges) Take() ([]channels.Key, bool) {
	return nil, f.resync.Swap(false)
}
func (f *fakeChanges) Close() {}

// resync makes the worker's next tick a channel resync round.
func (f *fakeChanges) fire() {
	f.resync.Store(true)
	f.wake <- struct{}{}
}

// A resync replays every channel subscription that has a topic and none
// without one. A subscription the round does not collect is not replayed,
// and is not told to refetch when the round overflows either: the overflow
// fallback only takes collected pairs.
func TestLive_ResyncSkipsTopiclessSubscriptions(t *testing.T) {
	fake := &fakeChanges{wake: make(chan struct{}, 1)}
	env := newChannelEnvWith(t, channelPolicy, func(app *App) {
		app.channels.changes = func() channelChanges { return fake }
	})
	hiddenPath := "/api/v1/features/FEAT-1/_channels/room"
	secretPath := "/api/v1/tickets/TKT-1/_channels/secret"
	c, conn := env.open(t, "alice")
	env.subscribe(t, "alice", conn,
		liveSubEntry{"head", roomPath},
		liveSubEntry{"hidden", hiddenPath},
		liveSubEntry{"secret", secretPath},
	)
	liveQuiet(t, c)
	replays := countReplaysByURL(t, env.app.live)

	fake.fire()
	env.waitTicks(t, 2)
	liveQuiet(t, c)
	if got, want := replays.take(), map[string]int{roomPath: 1}; !maps.Equal(got, want) {
		t.Fatalf("a resync replayed %v, want only the subscription with a topic %v", got, want)
	}

	pairs := env.app.live.collect(true, newRoundFilter(nil, nil, true))
	for _, p := range pairs {
		if p.url == hiddenPath || p.url == secretPath {
			t.Fatalf("a resync round collects %s, which has no topic; an overflow would frame it", p.url)
		}
	}
	if len(pairs) != 1 || pairs[0].url != roomPath {
		t.Fatalf("a resync round collects %d pairs, want only %s", len(pairs), roomPath)
	}
}

// A realtime.yaml reload that removes read access runs a full round: the
// revoked subscriber gets its one frame at the reload, and none when the
// channel next gets a message, so that message's time is not revealed.
func TestLive_RealtimeReloadRevokes(t *testing.T) {
	// dave holds chat.read, so he still reads the room after the reload.
	policy := func(t *testing.T, app *App) *acl.Declarative {
		t.Helper()
		return mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{
				"editor": {Read: []string{"ticket"}, Update: []string{"ticket"}},
				"admin": {
					Read: []string{"ticket", "feature"}, Update: []string{"ticket", "feature"},
					Permissions: []string{"chat.read"},
				},
			},
			Assignments: map[string]string{"editors": "editor", "dave": "admin"},
		}, app.store)
	}
	env := newChannelEnvWith(t, policy, nil)
	alice, aliceConn := env.open(t, "alice")
	dave, daveConn := env.open(t, "dave")
	env.subscribe(t, "alice", aliceConn, liveSubEntry{"head", roomPath})
	env.subscribe(t, "dave", daveConn, liveSubEntry{"head", roomPath})
	liveDrain(alice)
	liveDrain(dave)

	restricted := strings.Replace(channelTestRealtime,
		"    attach_to: [ticket, feature]\n    read: { from: attached }",
		"    attach_to: [ticket, feature]\n    read: { permission: chat.read }", 1)
	if restricted == channelTestRealtime {
		t.Fatal("the room's read rule was not replaced")
	}
	next := env.ch.parse(t, restricted)
	if err := SetRealtime(env.app, reloaderFunc(func() { env.ch.cfg.Store(next) })); err != nil {
		t.Fatal(err)
	}
	if err := reloadRealtime(env.app); err != nil {
		t.Fatal(err)
	}
	requireChanged(t, alice, "head")
	liveQuiet(t, dave)
	code, b := env.get(t, "alice", roomPath)
	if code != http.StatusForbidden || problemType(t, b) != realtime.TypeReadDenied {
		t.Fatalf("alice's head after the reload = %d %s, want 403 read_denied", code, b)
	}
	if got := env.app.live.subTopic(aliceConn, "head"); got != "" {
		t.Fatalf("alice's topic after the reload = %q, want none", got)
	}

	env.postText(t, "dave", roomPath, "after")
	requireChanged(t, dave, "head")
	liveQuiet(t, alice)
}

// reloaderFunc is a realtimeReloader whose Reload runs f and succeeds.
type reloaderFunc func()

func (f reloaderFunc) Reload(context.Context) error {
	f()
	return nil
}

// A topic round replays a presence subscription on a changed face and a
// channel subscription on a changed topic, or every channel subscription
// on a resync, and nothing else.
func TestRoundFilter_Selects(t *testing.T) {
	key := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "room"}
	byKey := newRoundFilter(nil, []channels.Key{key}, false)
	byFace := newRoundFilter([]string{"TKT-1"}, nil, false)
	resync := newRoundFilter(nil, nil, true)
	head := &liveSub{channel: true, topic: channelTopic("TKT-1", "room", "")}
	list := &liveSub{channel: true, topic: anchorTopic("TKT-1")}
	other := &liveSub{channel: true, topic: channelTopic("TKT-2", "room", "")}
	untopiced := &liveSub{channel: true}
	present := &liveSub{presenceID: "TKT-1", watching: "TKT-1"}
	left := &liveSub{presenceID: "TKT-1", left: "TKT-1"}
	plain := &liveSub{}
	for _, tc := range []struct {
		name string
		f    *roundFilter
		sub  *liveSub
		want bool
	}{
		{"head on its key", byKey, head, true},
		{"list on its anchor", byKey, list, true},
		{"other channel", byKey, other, false},
		{"channel without topic", byKey, untopiced, false},
		{"presence on a channel change", byKey, present, false},
		{"plain on a channel change", byKey, plain, false},
		{"watching face", byFace, present, true},
		{"left face", byFace, left, true},
		{"channel on a face change", byFace, head, false},
		{"resync: channel without topic", resync, untopiced, false},
		{"resync: head", resync, head, true},
		{"resync: other channel", resync, other, true},
		{"resync: plain", resync, plain, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.f.selects(tc.sub); got != tc.want {
				t.Fatalf("selects = %v, want %v", got, tc.want)
			}
		})
	}
}

// A faced channel key's head topic is the head path's anchor form, and its
// list topic is the family.
func TestChannelKeyTopics(t *testing.T) {
	k := channels.Key{Anchor: channels.Anchor{ID: "TKT-1", Face: "draft"}, Type: "threads", Name: "0a1b2c3d4e5f6"}
	got := channelKeyTopics(k)
	want := []string{channelTopic("TKT-1@draft", "threads", "0a1b2c3d4e5f6"), anchorTopic("TKT-1")}
	if !slices.Equal(got, want) {
		t.Fatalf("topics = %v, want %v", got, want)
	}
	if got[0] != topicChannel+k.String() {
		t.Fatalf("head topic %q does not follow channels.Key.String %q", got[0], k.String())
	}
}
