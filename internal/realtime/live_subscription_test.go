package realtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
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
		l.markAddressed([]string{"alice"}, keyOf("TKT-1"), markKindMoved)
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
				l.markAddressed([]string{u}, keyOf("TKT-1"), markKindMessage)
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
			require.Nil(t, hub.TakeMoved(), "an overflow clears moved")
		})
	}

	t.Run("TakeMoved empties the set", func(t *testing.T) {
		t.Parallel()
		l, hub := newLive(t)
		l.mark(keyOf("TKT-2"), keyOf("TKT-1"))
		l.markMessage(keyOf("TKT-3"))
		require.Equal(t, []channels.Key{keyOf("TKT-1"), keyOf("TKT-2")}, hub.TakeMoved())
		require.Nil(t, hub.TakeMoved())
		require.Equal(t, []channels.Key{keyOf("TKT-1"), keyOf("TKT-2"), keyOf("TKT-3")}, hub.TakeDurable())
	})

	t.Run("a plain subscription takes no moved keys", func(t *testing.T) {
		t.Parallel()
		l, _ := newLive(t)
		plain := l.Subscribe()
		l.mark(keyOf("TKT-1"))
		require.Nil(t, plain.TakeMoved())
		require.Equal(t, []channels.Key{keyOf("TKT-1")}, takeKeys(t, plain))
	})

	t.Run("overflow clears moved", func(t *testing.T) {
		t.Parallel()
		l, hub := newLive(t)
		hub.max = 2
		l.mark(keyOf("TKT-1"))
		l.markMessage(keyOf("TKT-2"), keyOf("TKT-3"))
		_, resync := hub.Take()
		require.True(t, resync)
		require.Nil(t, hub.TakeMoved(), "a moved key from before the overflow is gone")
	})
}

// TestTakeLive_OneCriticalSection pins that TakeLive takes every set at
// once: a key moved by a mark is in Moved of the same call whose Keys hold
// it. With one Take call per set, a mark between Take and TakeMoved split
// them across two calls, and a consumer that starts a round on Keys
// dropped the moved key. Marks race the takes here, so the race detector
// and the per-call check would catch a split.
func TestTakeLive_OneCriticalSection(t *testing.T) {
	t.Parallel()
	newLive := func(t *testing.T) (*Local, *Subscription) {
		t.Helper()
		l, err := NewLocal(time.Now)
		require.NoError(t, err)
		t.Cleanup(l.Stop)
		hub := l.SubscribeLive()
		t.Cleanup(hub.Close)
		return l, hub
	}

	t.Run("one mark", func(t *testing.T) {
		t.Parallel()
		l, hub := newLive(t)
		l.markAddressed([]string{"alice"}, keyOf("TKT-1"), markKindMoved)
		l.markMessage(keyOf("TKT-2"))
		c := hub.TakeLive()
		require.Equal(t, LiveChanges{
			Keys:      []channels.Key{keyOf("TKT-1"), keyOf("TKT-2")},
			Durable:   []channels.Key{keyOf("TKT-1"), keyOf("TKT-2")},
			Addressed: []string{"alice"},
			Moved:     []channels.Key{keyOf("TKT-1")},
		}, c)
		require.True(t, hub.TakeLive().Empty(), "TakeLive empties every set")
	})

	t.Run("marks racing the takes", func(t *testing.T) {
		t.Parallel()
		l, hub := newLive(t)
		// Sixteen keys stay well under the set's limit, so no take resyncs.
		const n, distinct = 5000, 16
		done := make(chan struct{})
		go func() {
			defer close(done)
			for i := range n {
				l.mark(keyOf(fmt.Sprintf("TKT-%d", i%distinct)))
			}
		}()
		seen := 0
		check := func(c LiveChanges) {
			require.False(t, c.Resync)
			keys := map[channels.Key]bool{}
			for _, k := range c.Keys {
				keys[k] = true
			}
			for _, k := range c.Moved {
				require.True(t, keys[k], "moved key %v taken without its changed key", k)
			}
			require.Len(t, c.Moved, len(c.Keys), "every key here is moved")
			seen += len(c.Moved)
		}
		for running := true; running; {
			select {
			case <-done:
				running = false
			default:
			}
			check(hub.TakeLive())
		}
		check(hub.TakeLive())
		require.GreaterOrEqual(t, seen, distinct, "every key was taken at least once")
	})
}

// TestPostMoved pins the rule for an appended message: it is moved when
// its append moved the session or created the implicit channel.
func TestPostMoved(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		m    channels.Message
		want bool
	}{
		{"plain message", channels.Message{}, false},
		{"moved the session", channels.Message{MovedSession: true}, true},
		{"created the channel", channels.Message{CreatedChannel: true}, true},
		{"both", channels.Message{MovedSession: true, CreatedChannel: true}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, postMoved(tc.m))
			want := markKindMessage
			if tc.want {
				want = markKindMoved
			}
			require.Equal(t, want, postKind(tc.m))
		})
	}
}

// retryWriter holds, per harness, the writer of the "outputs progress
// write" row of TestLocal_MovedKeys, which its new makes and its op uses.
var retryWriter sync.Map

// typingHarness is newTypingHarness with no options.
func typingHarness(t *testing.T) *harness { t.Helper(); return newTypingHarness(t) }

// TestLocal_MovedKeys pins TKT-W5ZSYJ AC 1 in the service: which writes
// reach TakeMoved. Every moved key is also durable and changed; a
// message-only write is durable only; typing and ephemeral signals are
// neither.
func TestLocal_MovedKeys(t *testing.T) {
	t.Parallel()
	type kind int
	const (
		neither kind = iota
		message
		moved
	)
	alice := user("alice")
	for _, tc := range []struct {
		name string
		new  func(t *testing.T) *harness
		// setup runs before the subscription takes its first sets.
		setup func(t *testing.T, h *harness)
		op    func(t *testing.T, h *harness)
		key   func(t *testing.T, h *harness) channels.Key
		want  kind
	}{
		{
			name: "post that moves", new: newSessionHarness,
			setup: func(t *testing.T, h *harness) { t.Helper(); h.postTo(t, alice, incident, "text", props("body", "hi")) },
			op:    func(t *testing.T, h *harness) { t.Helper(); h.postTo(t, alice, incident, "contained", nil) },
			want:  moved,
		},
		{
			name: "first post creates the channel", new: newSessionHarness,
			op:   func(t *testing.T, h *harness) { t.Helper(); h.postTo(t, alice, incident, "text", props("body", "hi")) },
			want: moved,
		},
		{
			name: "post that does not move", new: newSessionHarness,
			setup: func(t *testing.T, h *harness) { t.Helper(); h.postTo(t, alice, incident, "text", props("body", "hi")) },
			op: func(t *testing.T, h *harness) {
				t.Helper()
				h.postTo(t, alice, incident, "text", props("body", "again"))
			},
			want: message,
		},
		{
			name: "edit", new: newSessionHarness,
			setup: func(t *testing.T, h *harness) { t.Helper(); h.postTo(t, alice, incident, "text", props("body", "hi")) },
			op: func(t *testing.T, h *harness) {
				t.Helper()
				_, err := h.svc.Edit(as(alice), incident, 1, props("body", "changed"))
				require.NoError(t, err)
			},
			want: message,
		},
		{
			name: "remove", new: newSessionHarness,
			setup: func(t *testing.T, h *harness) { t.Helper(); h.postTo(t, alice, incident, "text", props("body", "hi")) },
			op: func(t *testing.T, h *harness) {
				t.Helper()
				_, err := h.svc.Delete(as(alice), incident, 1, "")
				require.NoError(t, err)
			},
			want: message,
		},
		{
			name: "action", new: func(t *testing.T) *harness { t.Helper(); return newActionHarness(t, &fakePromoter{}) },
			setup: func(t *testing.T, h *harness) { t.Helper(); h.post(t, user("ann"), proposal("x")) },
			op: func(t *testing.T, h *harness) {
				t.Helper()
				_, err := h.svc.RunAction(as(user("ann")), room, 1, "ack")
				require.NoError(t, err)
			},
			key:  func(*testing.T, *harness) channels.Key { return roomKey() },
			want: message,
		},
		{
			name: "failed promote", new: func(t *testing.T) *harness {
				t.Helper()
				return newActionHarness(t, &fakePromoter{err: errors.New("disk on fire")})
			},
			setup: func(t *testing.T, h *harness) { t.Helper(); h.post(t, user("ann"), proposal("x")) },
			op: func(t *testing.T, h *harness) {
				t.Helper()
				_, err := h.svc.RunAction(as(decider()), room, 1, "accept")
				require.Error(t, err)
			},
			key:  func(*testing.T, *harness) channels.Key { return roomKey() },
			want: message,
		},
		{
			name: "trigger post that moves", new: newSessionHarness,
			setup: func(t *testing.T, h *harness) { t.Helper(); h.postTo(t, alice, incident, "text", props("body", "hi")) },
			op: func(t *testing.T, h *harness) {
				t.Helper()
				h.triggerPost(t, incidentKey(), "escalated", nil)
			},
			want: moved,
		},
		{
			name: "trigger post that does not move", new: newSessionHarness,
			setup: func(t *testing.T, h *harness) { t.Helper(); h.postTo(t, alice, incident, "text", props("body", "hi")) },
			op: func(t *testing.T, h *harness) {
				t.Helper()
				h.triggerPost(t, incidentKey(), "text", props("body", "from a trigger"))
			},
			want: message,
		},
		{
			name: "outputs progress write", new: func(t *testing.T) *harness {
				t.Helper()
				// The inline write failed, so the mark at seq 2 is pending;
				// the runner's retry writes only the outputs mark.
				h, w := pendingCreate(t)
				h.clock.Add(realtimelimits.OutputClaimTTL + time.Second)
				retryWriter.Store(h, w)
				return h
			},
			op: func(t *testing.T, h *harness) {
				t.Helper()
				w, ok := retryWriter.Load(h)
				require.True(t, ok)
				require.Equal(t, 1, h.runOutputs(t, w.(*fakeOutputs)).Ran)
			},
			key:  func(*testing.T, *harness) channels.Key { return outKey },
			want: message,
		},
		{
			name: "create", new: newSessionHarness,
			op: func(t *testing.T, h *harness) {
				t.Helper()
				_, err := h.svc.Create(as(alice), "ticket", "TKT-1", "cases", CreateInput{})
				require.NoError(t, err)
			},
			key: func(t *testing.T, h *harness) channels.Key {
				t.Helper()
				cs, err := h.store.List(context.Background(), channels.Anchor{ID: "TKT-1"}, "cases", "")
				require.NoError(t, err)
				require.Len(t, cs, 1)
				return cs[0].Key
			},
			want: moved,
		},
		{
			name: "close", new: newSessionHarness,
			setup: func(t *testing.T, h *harness) { t.Helper(); h.postTo(t, alice, incident, "text", props("body", "hi")) },
			op: func(t *testing.T, h *harness) {
				t.Helper()
				_, err := h.svc.Lifecycle(as(alice), incident, realtimecfg.LifecycleClose)
				require.NoError(t, err)
			},
			want: moved,
		},
		{
			name: "delete", new: newLifecycleHarness,
			setup: func(t *testing.T, h *harness) {
				t.Helper()
				h.moveRoom(t, channels.StateClosed)
				h.clock.Add(7 * 24 * time.Hour)
			},
			op: func(t *testing.T, h *harness) {
				t.Helper()
				_, err := h.svc.Lifecycle(as(lead()), room, realtimecfg.LifecycleDelete)
				require.NoError(t, err)
			},
			key:  func(*testing.T, *harness) channels.Key { return roomKey() },
			want: moved,
		},
		{
			name: "Local.Mark", new: newSessionHarness,
			op:   func(_ *testing.T, h *harness) { h.local.Mark(incidentKey()) },
			want: moved,
		},
		{
			name: "typing", new: typingHarness,
			op: func(t *testing.T, h *harness) {
				t.Helper()
				_, err := h.svc.Typing(as(typist("ann")), chat)
				require.NoError(t, err)
			},
			key:  func(*testing.T, *harness) channels.Key { return chatKey() },
			want: neither,
		},
		{
			name: "ephemeral signal", new: typingHarness,
			op: func(t *testing.T, h *harness) {
				t.Helper()
				_, err := h.svc.Post(as(typist("ann")), chat, PostInput{Type: "status", Props: map[string]any{}})
				require.NoError(t, err)
			},
			key:  func(*testing.T, *harness) channels.Key { return chatKey() },
			want: neither,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := tc.new(t)
			if tc.setup != nil {
				tc.setup(t, h)
			}
			hub := h.svc.SubscribeLive()
			t.Cleanup(hub.Close)
			tc.op(t, h)
			k := incidentKey()
			if tc.key != nil {
				k = tc.key(t, h)
			}
			want := []channels.Key{k}
			require.Equal(t, want, takeKeys(t, hub), "every write changes the channel")
			switch tc.want {
			case moved:
				require.Equal(t, want, hub.TakeMoved())
				require.Equal(t, want, hub.TakeDurable(), "a moved key is durable too")
			case message:
				require.Nil(t, hub.TakeMoved())
				require.Equal(t, want, hub.TakeDurable())
			case neither:
				require.Nil(t, hub.TakeMoved())
				require.Nil(t, hub.TakeDurable())
			}
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
