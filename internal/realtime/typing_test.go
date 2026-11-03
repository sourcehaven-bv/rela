package realtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

var (
	chat      = RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "chat"}
	plainChat = RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "plain"}
)

func chatKey() channels.Key {
	return channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "chat"}
}

func chatAt(id string) RawAddress {
	return RawAddress{EntityType: "ticket", Anchor: id, Type: "chat"}
}

// typist may read chat and post its text type; reader may only read;
// lead may also close and archive.
func typist(name string) *fakeAccess { return user(name, "chat.read", "chat.post") }
func reader(name string) *fakeAccess { return user(name, "chat.read") }
func chatLead() *fakeAccess          { return user("lee", "chat.read", "chat.post", "chat.lead") }

func newTypingHarness(t *testing.T, opts ...harnessOpt) *harness {
	t.Helper()
	h := newHarness(t, append([]harnessOpt{withYAML(typingRealtime)}, opts...)...)
	for _, id := range []string{"TKT-3", "TKT-4"} {
		h.anchors.views[id] = AnchorView{ID: id, Type: "ticket"}
	}
	return h
}

// typingUsers returns the users the head at addr lists as typing, for a.
func typingUsers(t *testing.T, h *harness, a *fakeAccess, addr RawAddress) []string {
	t.Helper()
	head, err := h.svc.Head(as(a), addr)
	require.NoError(t, err)
	require.NotNil(t, head.TypingNow)
	out := []string{}
	for _, e := range head.TypingNow {
		out = append(out, e.Author.User)
	}
	return out
}

// TestTyping_KeptNotStored pins AC 6: typing keeps an entry another reader
// sees and writes nothing to the channel store.
func TestTyping_KeptNotStored(t *testing.T) {
	t.Parallel()
	h := newTypingHarness(t)
	sub := h.svc.Subscribe()
	defer sub.Close()

	res, err := h.svc.Typing(as(typist("ann")), chat)
	require.NoError(t, err)
	require.Equal(t, realtimelimits.TypingTTL, res.TTL)

	head, err := h.svc.Head(as(reader("bob")), chat)
	require.NoError(t, err)
	require.Len(t, head.TypingNow, 1)
	require.Equal(t, "ann", head.TypingNow[0].Author.User)
	require.Equal(t, h.clock.Now(), head.TypingNow[0].Since)
	require.Empty(t, head.Ephemeral, "typing is not an operator signal")

	_, err = h.store.Head(t.Context(), chatKey())
	require.ErrorIs(t, err, channels.ErrNotFound, "typing creates no channel")
	require.Zero(t, h.store.appends.Load())
	require.Zero(t, h.store.records.Load())
	require.Equal(t, []channels.Key{chatKey()}, takeKeys(t, sub))
	require.Empty(t, takeKeys(t, sub))
}

// TestTyping_SelfExcluded pins AC 7: a reader sees the typist; the typist's
// own identity, under any tool, does not.
func TestTyping_SelfExcluded(t *testing.T) {
	t.Parallel()
	h := newTypingHarness(t)
	_, err := h.svc.Typing(as(typist("ann")), chat)
	require.NoError(t, err)
	require.Equal(t, []string{"ann"}, typingUsers(t, h, reader("bob"), chat))
	require.Empty(t, typingUsers(t, h, typist("ann"), chat))
	agent := typist("ann")
	agent.p.Tool = principal.ToolMCP
	require.Empty(t, typingUsers(t, h, agent, chat))
}

// TestTyping_RefreshSilent pins AC 8: a refresh moves the deadline and
// nothing else: no wake, no change to since or the head.
func TestTyping_RefreshSilent(t *testing.T) {
	t.Parallel()
	h := newTypingHarness(t)
	sub := h.svc.Subscribe()
	defer sub.Close()
	ann, bob := typist("ann"), reader("bob")
	start := h.clock.Now()

	_, err := h.svc.Typing(as(ann), chat)
	require.NoError(t, err)
	require.Equal(t, []channels.Key{chatKey()}, takeKeys(t, sub))
	first, err := h.svc.Head(as(bob), chat)
	require.NoError(t, err)
	want, err := json.Marshal(first)
	require.NoError(t, err)

	for range 2 {
		h.clock.Add(time.Second)
		_, err = h.svc.Typing(as(ann), chat)
		require.NoError(t, err)
		require.Empty(t, takeKeys(t, sub), "a refresh wakes nobody")
		head, herr := h.svc.Head(as(bob), chat)
		require.NoError(t, herr)
		require.Equal(t, start, head.TypingNow[0].Since)
		got, merr := json.Marshal(head)
		require.NoError(t, merr)
		require.Equal(t, string(want), string(got))
	}

	h.clock.Add(realtimelimits.TypingTTL - 100*time.Millisecond)
	require.Equal(t, []string{"ann"}, typingUsers(t, h, bob, chat), "the deadline moved with the refresh")
	h.clock.Add(100 * time.Millisecond)
	require.Empty(t, typingUsers(t, h, bob, chat))
}

// TestTyping_Expires pins AC 9: the expiry timer drops the entry after the
// TTL and marks the channel once.
func TestTyping_Expires(t *testing.T) {
	t.Parallel()
	h := newTypingHarness(t)
	_, err := h.svc.Typing(as(typist("ann")), chat)
	require.NoError(t, err)
	sub := h.svc.Subscribe()
	defer sub.Close()

	h.clock.Add(realtimelimits.TypingTTL)
	h.local.fire()
	require.Equal(t, []channels.Key{chatKey()}, takeKeys(t, sub))
	require.Empty(t, takeKeys(t, sub))
	require.Empty(t, typingUsers(t, h, reader("bob"), chat))
}

// TestTyping_TimerWakes pins that the expiry timer is armed for a typing
// deadline and wakes a subscriber without a Take.
func TestTyping_TimerWakes(t *testing.T) {
	t.Parallel()
	local, err := NewLocal(time.Now)
	require.NoError(t, err)
	defer local.Stop()
	sub := local.Subscribe()
	defer sub.Close()
	now := time.Now().Add(-realtimelimits.TypingTTL + 20*time.Millisecond)
	started, _, err := local.typing.touch(chatKey(), "x", channels.Author{User: "x"}, now)
	require.NoError(t, err)
	require.True(t, started)
	local.scheduleExpiry()
	select {
	case <-sub.Wake():
	case <-time.After(5 * time.Second):
		t.Fatal("the expiry timer did not wake the subscriber")
	}
	require.Equal(t, []channels.Key{chatKey()}, takeKeys(t, sub))
}

// TestTyping_ClearedOnPost pins AC 10: a log post ends the poster's entry
// and marks the channel once.
func TestTyping_ClearedOnPost(t *testing.T) {
	t.Parallel()
	h := newTypingHarness(t)
	ann := typist("ann")
	_, err := h.svc.Typing(as(ann), chat)
	require.NoError(t, err)
	sub := h.svc.Subscribe()
	defer sub.Close()
	_, err = h.svc.Post(as(ann), chat, text("hi"))
	require.NoError(t, err)
	require.Empty(t, typingUsers(t, h, reader("bob"), chat))
	require.Equal(t, []channels.Key{chatKey()}, takeKeys(t, sub))
	require.Empty(t, takeKeys(t, sub))
}

// TestTyping_NotClearedBySignal pins AC 10: an operator signal is not the
// message typing announces, so it leaves the entry in place.
func TestTyping_NotClearedBySignal(t *testing.T) {
	t.Parallel()
	h := newTypingHarness(t)
	ann := typist("ann")
	_, err := h.svc.Typing(as(ann), chat)
	require.NoError(t, err)
	_, err = h.svc.Post(as(ann), chat, PostInput{Type: "status"})
	require.NoError(t, err)
	require.Equal(t, []string{"ann"}, typingUsers(t, h, reader("bob"), chat))
}

// TestTyping_SeparateFromRing pins AC 11: typing and operator signals hold
// separate slots, so neither refuses nor evicts the other.
func TestTyping_SeparateFromRing(t *testing.T) {
	t.Parallel()
	lim := defaultEphemeralLimits()
	lim.ringShare, lim.channelsPerIdentity = 2, 2
	h := newTypingHarness(t, withEphemeral(lim))
	ann := typist("ann")
	status := PostInput{Type: "status"}
	for range 2 {
		_, err := h.svc.Post(as(ann), chat, status)
		require.NoError(t, err)
	}
	_, err := h.svc.Post(as(ann), chatAt("TKT-2"), status)
	require.NoError(t, err)
	_, err = h.svc.Post(as(ann), chatAt("TKT-3"), status)
	requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)

	_, err = h.svc.Typing(as(ann), chatAt("TKT-3"))
	require.NoError(t, err, "typing starts beyond the signal ring caps")
	require.Len(t, h.local.liveSignals(chatKey()), 2)
	require.Len(t, h.local.liveSignals(channels.Key{Anchor: channels.Anchor{ID: "TKT-2"}, Type: "chat"}), 1)

	h.clock.Add(time.Second)
	_, err = h.svc.Post(as(ann), chat, status)
	require.NoError(t, err, "at its share, a post replaces its own oldest signal")
	require.Equal(t, []string{"ann"}, typingUsers(t, h, reader("bob"), chatAt("TKT-3")))
}

// TestTyping_Caps pins AC 12: the caps refuse a new entry and never evict;
// a refresh of a held entry passes them.
func TestTyping_Caps(t *testing.T) {
	t.Parallel()
	t.Run("per channel", func(t *testing.T) {
		t.Parallel()
		h := newTypingHarness(t)
		for i := range realtimelimits.MaxTypingPerChannel {
			_, err := h.svc.Typing(as(typist("u"+string(rune('a'+i/26))+string(rune('a'+i%26)))), chat)
			require.NoError(t, err)
		}
		charged := h.local.limiter.size()
		_, err := h.svc.Typing(as(typist("late")), chat)
		requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
		require.Equal(t, charged, h.local.limiter.size(), "a cap refusal charges no token")
		require.Len(t, typingUsers(t, h, reader("bob"), chat), realtimelimits.MaxTypingPerChannel)
	})
	t.Run("channels", func(t *testing.T) {
		t.Parallel()
		h := newTypingHarness(t)
		h.local.typing = newTypingStore(realtimelimits.MaxTypingPerChannel, 2)
		ann := typist("ann")
		for _, id := range []string{"TKT-1", "TKT-2"} {
			_, err := h.svc.Typing(as(ann), chatAt(id))
			require.NoError(t, err)
		}
		_, err := h.svc.Typing(as(ann), chatAt("TKT-3"))
		d := requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
		require.Equal(t, realtimelimits.TypingTTL, d.RetryAfter)
		h.clock.Add(time.Second)
		_, err = h.svc.Typing(as(ann), chat)
		require.NoError(t, err, "a refresh in a held channel passes the cap")
		require.Len(t, h.local.typing.live(chatKey(), h.clock.Now()), 1, "one entry per identity")
	})
}

// TestTyping_Refused pins AC 13: every refusal, its status and type, and
// that a refusal before the role check charges and keeps nothing.
func TestTyping_Refused(t *testing.T) {
	t.Parallel()
	nobody := &fakeAccess{perms: map[string]bool{"chat.read": true, "chat.post": true}, ceiling: true}
	cases := []struct {
		name   string
		setup  func(*testing.T, *harness)
		caller *fakeAccess
		addr   RawAddress
		status int
		typ    string
		// clean is set when nothing may be charged or kept.
		clean bool
	}{
		{"typing off", nil, typist("ann"), plainChat, http.StatusForbidden, TypeTypingNotOffered, true},
		{"no read role", nil, user("ann", "chat.post"), chat, http.StatusForbidden, TypeReadDenied, true},
		{"no log post role", nil, reader("ann"), chat, http.StatusForbidden, TypePostDenied, true},
		{"no principal", nil, nobody, chat, http.StatusForbidden, TypeNoPrincipal, true},
		{"closed", closeChat, typist("ann"), chat, http.StatusConflict, TypeChannelNotOpen, false},
		{"archived", archiveChat, typist("ann"), chat, http.StatusConflict, TypeChannelNotOpen, false},
		{"rate", func(t *testing.T, h *harness) {
			t.Helper()
			_, err := h.svc.Typing(as(typist("ann")), chat)
			require.NoError(t, err)
		}, typist("ann"), chat, http.StatusTooManyRequests, TypeRateLimited, false},
		{"unknown name", nil, typist("ann"),
			RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "rooms", Name: "nosuch"},
			http.StatusNotFound, TypeChannelNotFound, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newTypingHarness(t)
			if tc.setup != nil {
				tc.setup(t, h)
			}
			_, err := h.svc.Typing(as(tc.caller), tc.addr)
			d := requireDenial(t, err, tc.status, tc.typ)
			switch tc.typ {
			case TypePostDenied:
				require.Equal(t, "channel_types.chat.typing", d.Rule)
				require.Contains(t, d.Title, "lead")
				require.Contains(t, d.Title, "poster")
			case TypeRateLimited:
				require.Positive(t, d.RetryAfter)
			}
			if tc.clean {
				require.Zero(t, h.local.limiter.size(), "no token charged")
				_, ok := h.local.typing.nextDeadline()
				require.False(t, ok, "no entry kept")
				require.Zero(t, h.store.records.Load(), "no channel created")
			}
		})
	}
}

func closeChat(t *testing.T, h *harness) {
	t.Helper()
	_, err := h.svc.Post(as(chatLead()), chat, text("first"))
	require.NoError(t, err)
	_, err = h.svc.Lifecycle(as(chatLead()), chat, "close")
	require.NoError(t, err)
}

func archiveChat(t *testing.T, h *harness) {
	t.Helper()
	closeChat(t, h)
	_, err := h.svc.Lifecycle(as(chatLead()), chat, "archive")
	require.NoError(t, err)
}

// TestTyping_OwnBucket pins AC 14: typing has its own rate bucket, and
// limits.signal_rate applies to operator signals only.
func TestTyping_OwnBucket(t *testing.T) {
	t.Parallel()
	h := newTypingHarness(t, withYAML(strings.Replace(typingRealtime,
		"version: 1\n", "version: 1\nlimits: { signal_rate: 1/m }\n", 1)))
	ann := typist("ann")
	for i := range 3 {
		if i > 0 {
			h.clock.Add(time.Second)
		}
		_, err := h.svc.Typing(as(ann), chat)
		require.NoError(t, err)
	}
	_, err := h.svc.Post(as(ann), chat, PostInput{Type: "status"})
	require.NoError(t, err, "the signal bucket is untouched by typing")
	_, err = h.svc.Post(as(ann), chat, PostInput{Type: "status"})
	requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
}

// TestTyping_HiddenWhenClosedOrOff pins AC 15: a closed channel, or a type
// a reload turned typing off for, lists no entry though one is held.
func TestTyping_HiddenWhenClosedOrOff(t *testing.T) {
	t.Parallel()
	t.Run("closed", func(t *testing.T) {
		t.Parallel()
		h := newTypingHarness(t)
		_, err := h.svc.Post(as(chatLead()), chat, text("first"))
		require.NoError(t, err)
		_, err = h.svc.Typing(as(typist("ann")), chat)
		require.NoError(t, err)
		require.Equal(t, []string{"ann"}, typingUsers(t, h, reader("bob"), chat))
		_, err = h.svc.Lifecycle(as(chatLead()), chat, "close")
		require.NoError(t, err)
		require.Empty(t, typingUsers(t, h, reader("bob"), chat))
		require.Len(t, h.local.typing.live(chatKey(), h.clock.Now()), 1)
	})
	t.Run("reload off", func(t *testing.T) {
		t.Parallel()
		h := newTypingHarness(t)
		_, err := h.svc.Typing(as(typist("ann")), chat)
		require.NoError(t, err)
		h.cfg.set(Snapshot{Config: testCompiled(t, strings.Replace(typingRealtime, "    typing: true\n", "", 1))})
		require.Empty(t, typingUsers(t, h, reader("bob"), chat))
		require.Len(t, h.local.typing.live(chatKey(), h.clock.Now()), 1)
	})
}

// TestTyping_AffordanceMatches pins AC 16: _actions.typing matches what
// Typing answers for every case the head can express.
func TestTyping_AffordanceMatches(t *testing.T) {
	t.Parallel()
	nobody := &fakeAccess{perms: map[string]bool{"chat.read": true, "chat.post": true}, ceiling: true}
	cases := []struct {
		name   string
		setup  func(*testing.T, *harness)
		caller *fakeAccess
		addr   RawAddress
		want   bool
	}{
		{"typing off", nil, typist("ann"), plainChat, false},
		{"no log post role", nil, reader("ann"), chat, false},
		{"no principal", nil, nobody, chat, false},
		{"closed", closeChat, typist("ann"), chat, false},
		{"archived", archiveChat, typist("ann"), chat, false},
		{"allowed", nil, typist("ann"), chat, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newTypingHarness(t)
			if tc.setup != nil {
				tc.setup(t, h)
			}
			head, err := h.svc.Head(as(tc.caller), tc.addr)
			require.NoError(t, err)
			require.Equal(t, tc.want, head.Actions.Typing)
			_, err = h.svc.Typing(as(tc.caller), tc.addr)
			require.Equal(t, tc.want, err == nil, "Typing answered %v", err)
		})
	}
}

// TestTyping_DropOnDeleteAndRename pins that a deleted or renamed entity
// takes its typing entries with it and wakes readers.
func TestTyping_DropOnDeleteAndRename(t *testing.T) {
	t.Parallel()
	drops := map[string]func(*Local) error{
		"delete": func(l *Local) error { return l.EntityDeleted(t.Context(), "TKT-1") },
		"rename": func(l *Local) error { return l.EntityRenamed(t.Context(), "TKT-1", "TKT-7") },
	}
	for name, drop := range drops {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newTypingHarness(t)
			_, err := h.svc.Typing(as(typist("ann")), chat)
			require.NoError(t, err)
			sub := h.svc.Subscribe()
			defer sub.Close()
			require.NoError(t, drop(h.local))
			require.Empty(t, h.local.typing.live(chatKey(), h.clock.Now()))
			require.Equal(t, []channels.Key{chatKey()}, takeKeys(t, sub))
		})
	}
}

// TestRead_Typing pins that Read with signals lists other typists in
// TypingNow, and only operator signals in Signals.
func TestRead_Typing(t *testing.T) {
	t.Parallel()
	h := newTypingHarness(t)
	ann := typist("ann")
	_, err := h.svc.Typing(as(ann), chat)
	require.NoError(t, err)
	_, err = h.svc.Post(as(ann), chat, PostInput{Type: "status"})
	require.NoError(t, err)

	v, err := h.svc.Read(as(reader("bob")), chat, ReadRequest{Signals: true, Actions: true})
	require.NoError(t, err)
	require.Len(t, v.TypingNow, 1)
	require.Equal(t, "ann", v.TypingNow[0].Author.User)
	require.Len(t, v.Signals, 1)
	require.Equal(t, "status", v.Signals[0].Type)
	require.False(t, v.Actions.Typing, "a reader without a log post role may not type")

	v, err = h.svc.Read(as(ann), chat, ReadRequest{Signals: true, Actions: true})
	require.NoError(t, err)
	require.Empty(t, v.TypingNow)
	require.True(t, v.Actions.Typing)

	v, err = h.svc.Read(as(reader("bob")), chat, ReadRequest{})
	require.NoError(t, err)
	require.Empty(t, v.TypingNow, "typing comes with signals")
}

// TestTypingStore_StaysInItsChannel pins that the request path never walks
// the whole store: touch, admits and live look only at the channel they
// name, and sweep pops only what is due.
func TestTypingStore_StaysInItsChannel(t *testing.T) {
	t.Parallel()
	ts := defaultTypingStore()
	now := time.Unix(1_000_000, 0)
	key := func(i int) channels.Key {
		return channels.Key{Anchor: channels.Anchor{ID: fmt.Sprintf("TKT-%d", i)}, Type: "chat"}
	}
	for i := range 200 {
		for _, id := range []string{"a", "b", "c"} {
			_, _, err := ts.touch(key(i), id, channels.Author{User: id}, now)
			require.NoError(t, err)
		}
	}
	later := now.Add(time.Second)
	for _, call := range []struct {
		name string
		run  func()
	}{
		{"refresh", func() { _, _, _ = ts.touch(key(7), "a", channels.Author{User: "a"}, later) }},
		{"start", func() { _, _, _ = ts.touch(key(7), "d", channels.Author{User: "d"}, later) }},
		{"admits", func() { _ = ts.admits(key(7), "e", later) }},
		{"live", func() { _ = ts.live(key(7), later) }},
		{"sweep with nothing due", func() { _ = ts.sweep(later) }},
	} {
		before := ts.visits
		call.run()
		require.LessOrEqual(t, ts.visits-before, 2*realtimelimits.MaxTypingPerChannel,
			"%s visited %d entries", call.name, ts.visits-before)
	}
	got, ok := ts.nextDeadline()
	require.True(t, ok)
	require.Equal(t, now.Add(realtimelimits.TypingTTL), got)
	require.Len(t, ts.sweep(now.Add(realtimelimits.TypingTTL)), 200)
	require.Len(t, ts.live(key(7), now.Add(realtimelimits.TypingTTL)), 2, "the refreshed and the new entry stay")
	require.Len(t, ts.byKey, 1)
}
