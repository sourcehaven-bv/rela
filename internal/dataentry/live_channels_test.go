package dataentry

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// Live channel delivery (TKT-ZU7P13). The channel head and the channel
// list are the live shapes; a frame tells the client to fetch
// messages?after_rev= itself.

// AC5: a message frames the head of its channel and the channel list of its
// anchor, and nothing else: not another channel, not the entity, and not a
// subscription to a channel the subscriber cannot read.
func TestLive_ChannelFrameIsolation(t *testing.T) {
	env := newChannelEnv(t)
	c, conn := env.open(t, "alice")
	env.subscribe(t, "alice", conn,
		liveSubEntry{"head", roomPath},
		liveSubEntry{"list", "/api/v1/tickets/TKT-1/_channels"},
		liveSubEntry{"other", "/api/v1/tickets/TKT-2/_channels/room"},
		liveSubEntry{"entity", "/api/v1/tickets/TKT-1"},
		liveSubEntry{"hidden", "/api/v1/features/FEAT-1/_channels/room"},
		liveSubEntry{"secret", "/api/v1/tickets/TKT-1/_channels/secret"},
	)
	liveQuiet(t, c)

	env.postText(t, "bob", roomPath, "hello")
	requireChanged(t, c, "head", "list")
	liveQuiet(t, c)

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
		{"resync: channel without topic", resync, untopiced, true},
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
