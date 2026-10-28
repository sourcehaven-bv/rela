package realtime

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// The live hub's subscription (TKT-9YRRM5): one subscription serves every
// reader, so it takes channel changes, durable keys, reads tagged by
// reader and addressed targets.

// TestSubscribeLive pins AC 19: SubscribeLive delivers channel changes,
// durable keys, reads tagged by reader and addressed targets, and an
// overflow of any of them resyncs.
func TestSubscribeLive(t *testing.T) {
	t.Parallel()
	newLive := func(t *testing.T) (*Local, *Subscription) {
		t.Helper()
		l, err := NewLocal(time.Now)
		require.NoError(t, err)
		t.Cleanup(l.Stop)
		return l, l.SubscribeLive()
	}
	alice := channels.Reader{User: "alice"}

	t.Run("delivers", func(t *testing.T) {
		t.Parallel()
		l, hub := newLive(t)
		plain := l.Subscribe()
		l.markAddressed([]string{"alice"}, keyOf("TKT-1"))
		l.markFor(alice, keyOf("TKT-2"))
		l.markEphemeral(keyOf("TKT-3"))
		require.Equal(t, []channels.Key{keyOf("TKT-1")}, hub.TakeDurable())
		require.Equal(t, []string{"alice"}, hub.TakeAddressed())
		require.Equal(t, map[channels.Reader][]channels.Key{alice: {keyOf("TKT-2")}}, hub.TakeReads())
		require.Equal(t, []channels.Key{keyOf("TKT-1"), keyOf("TKT-3")}, takeKeys(t, hub))
		require.Nil(t, hub.TakeDurable(), "TakeDurable empties the set")
		require.Nil(t, plain.TakeDurable(), "a plain subscription takes no durable keys")
		require.Nil(t, plain.TakeAddressed(), "a plain subscription takes no addressed targets")
	})

	for _, tc := range []struct {
		name     string
		overflow func(l *Local)
	}{
		{"durable", func(l *Local) { l.mark(keyOf("TKT-1"), keyOf("TKT-2"), keyOf("TKT-3")) }},
		{"reads", func(l *Local) { l.markFor(alice, keyOf("TKT-1"), keyOf("TKT-2"), keyOf("TKT-3")) }},
		{"addressed", func(l *Local) {
			for _, u := range []string{"a", "b", "c"} {
				l.markAddressed([]string{u}, keyOf("TKT-1"))
			}
		}},
	} {
		t.Run("overflow "+tc.name, func(t *testing.T) {
			t.Parallel()
			l, hub := newLive(t)
			hub.max = 2
			tc.overflow(l)
			keys, resync := hub.Take()
			require.True(t, resync)
			require.Empty(t, keys)
			require.Nil(t, hub.TakeDurable())
			require.Nil(t, hub.TakeReads())
			require.Nil(t, hub.TakeAddressed())
		})
	}
}

// TestLocal_DurableKeys pins AC 13 in Local: typing start and expiry, an
// ephemeral signal and its expiry, and the drop of an anchor's live state
// change the channel (a head shows them) but are not durable; a stored
// post is.
func TestLocal_DurableKeys(t *testing.T) {
	t.Parallel()
	h := newTypingHarness(t)
	hub := h.svc.SubscribeLive()
	t.Cleanup(hub.Close)
	ann := typist("ann")

	_, err := h.svc.Typing(as(ann), chat)
	require.NoError(t, err)
	require.Equal(t, []channels.Key{chatKey()}, takeKeys(t, hub))
	require.Nil(t, hub.TakeDurable(), "typing start is not durable")

	h.clock.Add(time.Second)
	_, err = h.svc.Post(as(ann), chat, PostInput{Type: "status", Props: map[string]any{}})
	require.NoError(t, err)
	require.Equal(t, []channels.Key{chatKey()}, takeKeys(t, hub))
	require.Nil(t, hub.TakeDurable(), "an ephemeral signal is not durable")

	h.clock.Add(time.Hour)
	h.local.fire()
	require.Equal(t, []channels.Key{chatKey()}, takeKeys(t, hub))
	require.Nil(t, hub.TakeDurable(), "typing and signal expiry are not durable")

	_, err = h.svc.Typing(as(ann), chat)
	require.NoError(t, err)
	takeKeys(t, hub)
	h.local.dropAnchor("TKT-1", "")
	require.Equal(t, []channels.Key{chatKey()}, takeKeys(t, hub))
	require.Nil(t, hub.TakeDurable(), "dropping an anchor's live state is not durable")

	h.clock.Add(time.Minute)
	_, err = h.svc.Post(as(ann), chat, PostInput{Type: "text", Props: map[string]any{"body": "x"}})
	require.NoError(t, err)
	require.Equal(t, []channels.Key{chatKey()}, hub.TakeDurable(), "a stored post is durable")
}

// TestOwnMarkerWrites pins AC 10 in the service: a post and a channel
// create write the caller's marker, so they reach the live subscription
// tagged with the caller's reader, even on a channel the caller held no
// marker on.
func TestOwnMarkerWrites(t *testing.T) {
	t.Parallel()
	t.Run("post", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		hub := h.svc.SubscribeLive()
		t.Cleanup(hub.Close)
		alice := user("alice")
		h.post(t, alice, text("first"))
		require.Equal(t, map[channels.Reader][]channels.Key{readerOf(alice): {roomKey()}}, hub.TakeReads())
	})
	t.Run("create", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		hub := h.svc.SubscribeLive()
		t.Cleanup(hub.Close)
		alice := user("alice")
		ch, err := h.svc.Create(as(alice), "ticket", "TKT-1", "threads", CreateInput{Title: "side"})
		require.NoError(t, err)
		reads := hub.TakeReads()
		require.Len(t, reads[readerOf(alice)], 1)
		require.Equal(t, ch.Name, reads[readerOf(alice)][0].Name)
	})
}

// TestInboxView_Summary pins AC 1's projection: the counts of the first
// page, nil counts as 0.
func TestInboxView_Summary(t *testing.T) {
	t.Parallel()
	two, one := 2, 1
	for _, tc := range []struct {
		name string
		v    InboxView
		want InboxSummary
	}{
		{"empty", InboxView{}, InboxSummary{}},
		{"all", InboxView{Count: 3, Truncated: true, UnreadCount: &two, MentionCount: &one, Degraded: true},
			InboxSummary{Waiting: 3, WaitingTruncated: true, UnreadChannels: 2, Mentions: 1, Degraded: true}},
		{"no unread or mention part", InboxView{Count: 4}, InboxSummary{Waiting: 4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tc.v.Summary())
		})
	}
}
