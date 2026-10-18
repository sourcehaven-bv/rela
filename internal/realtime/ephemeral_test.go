package realtime

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

func typing() PostInput { return PostInput{Type: "typing"} }

// TestEphemeral_NotStored pins AC8: an ephemeral post returns an id and an
// expiry but no message, writes nothing to the store, and shows in the
// head until it expires.
func TestEphemeral_NotStored(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	bob := user("bob")
	res, err := h.svc.Post(as(bob), room, typing())
	require.NoError(t, err)
	require.True(t, res.Ephemeral)
	require.NotEmpty(t, res.ID)
	require.Nil(t, res.Message)
	require.Equal(t, h.clock.Now().Add(5*time.Second), res.ExpiresAt)
	require.Zero(t, h.store.appends.Load())
	require.Zero(t, h.store.records.Load(), "a signal records no channel type")
	_, err = h.store.Head(as(bob), roomKey())
	require.ErrorIs(t, err, channels.ErrNotFound)

	head, err := h.svc.Head(as(bob), room)
	require.NoError(t, err)
	require.Len(t, head.Ephemeral, 1)
	require.Equal(t, res.ID, head.Ephemeral[0].ID)
	require.Equal(t, "bob", head.Ephemeral[0].Author.User)

	h.clock.Add(5 * time.Second)
	head, err = h.svc.Head(as(bob), room)
	require.NoError(t, err)
	require.Empty(t, head.Ephemeral)

	_, err = h.svc.Post(as(bob), room, PostInput{Type: "typing", ReplyTo: 1})
	requireDenial(t, err, http.StatusBadRequest, TypeInvalid)
}

// TestEphemeral_TTLClamp pins that a signal lives for its type's ttl, and
// for at most realtimelimits.MaxEphemeralTTL when the type sets none.
func TestEphemeral_TTLClamp(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	res, err := h.svc.Post(as(user("bob")), room, PostInput{Type: "cursor"})
	require.NoError(t, err)
	require.Equal(t, realtimelimits.MaxEphemeralTTL, res.ExpiresAt.Sub(h.clock.Now()))
	res, err = h.svc.Post(as(user("bob")), room, typing())
	require.NoError(t, err)
	require.Equal(t, 5*time.Second, res.ExpiresAt.Sub(h.clock.Now()))
}

// TestEphemeral_RingWrap64 pins the ring bound: a full ring takes a new
// signal only by overwriting the poster's own oldest one; another user's
// signal is never pushed out, and a stranger gets 429.
func TestEphemeral_RingWrap64(t *testing.T) {
	t.Parallel()
	lim := defaultEphemeralLimits()
	lim.perIdentity, lim.ringShare = 1000, realtimelimits.EphemeralRingSize
	h := newHarness(t, withEphemeral(lim))
	first := ""
	for i := range realtimelimits.EphemeralRingSize {
		who := user(fmt.Sprintf("u%d", i%2))
		res, err := h.svc.Post(as(who), room, PostInput{Type: "cursor"})
		require.NoError(t, err, "signal %d", i)
		if i == 0 {
			first = res.ID
		}
		h.clock.Add(100 * time.Millisecond)
		h.local.limiter = newLimiter(defaultLimitCaps()) // isolate the ring from the rate limit
	}
	head, err := h.svc.Head(as(user("u0")), room)
	require.NoError(t, err)
	require.Len(t, head.Ephemeral, realtimelimits.EphemeralRingSize)

	_, err = h.svc.Post(as(user("stranger")), room, PostInput{Type: "cursor"})
	requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)

	_, err = h.svc.Post(as(user("u0")), room, PostInput{Type: "cursor"})
	require.NoError(t, err)
	head, err = h.svc.Head(as(user("u0")), room)
	require.NoError(t, err)
	require.Len(t, head.Ephemeral, realtimelimits.EphemeralRingSize)
	for _, e := range head.Ephemeral {
		require.NotEqual(t, first, e.ID, "u0's oldest signal is the one overwritten")
	}
}

// TestEphemeral_Caps pins the size cap (413), the per-identity cap and the
// byte budget (429), and that the channel cap refuses a new channel.
func TestEphemeral_Caps(t *testing.T) {
	t.Parallel()
	t.Run("size", func(t *testing.T) {
		t.Parallel()
		big := strings.Repeat("x", realtimelimits.MaxEphemeralBytes)
		mt := &realtimecfg.MessageType{Name: "big", Properties: map[string]realtimecfg.Property{
			"s": {Name: "s", Type: realtimecfg.PropertyString},
		}}
		_, err := validateProps(mt, map[string]any{"s": big}, realtimelimits.MaxEphemeralBytes)
		requireDenial(t, err, http.StatusRequestEntityTooLarge, TypeTooLarge)
		_, err = validateProps(mt, map[string]any{"s": "small"}, realtimelimits.MaxEphemeralBytes)
		require.NoError(t, err)
	})
	t.Run("per identity", func(t *testing.T) {
		t.Parallel()
		lim := defaultEphemeralLimits()
		lim.perIdentity = 2
		h := newHarness(t, withEphemeral(lim))
		for range 2 {
			_, err := h.svc.Post(as(user("bob")), room, PostInput{Type: "cursor"})
			require.NoError(t, err)
			h.clock.Add(time.Second)
		}
		other := RawAddress{EntityType: "ticket", Anchor: "TKT-2", Type: "room"}
		_, err := h.svc.Post(as(user("bob")), other, PostInput{Type: "cursor"})
		d := requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
		require.Positive(t, d.RetryAfter)
		_, err = h.svc.Post(as(user("bob")), room, PostInput{Type: "cursor"})
		require.NoError(t, err, "at the cap, a signal replaces the poster's own oldest one in the ring")
		head, err := h.svc.Head(as(user("bob")), room)
		require.NoError(t, err)
		require.Len(t, head.Ephemeral, 2)
		_, err = h.svc.Post(as(user("carol")), room, PostInput{Type: "cursor"})
		require.NoError(t, err, "another identity is not affected")
	})
	t.Run("byte budget", func(t *testing.T) {
		t.Parallel()
		lim := defaultEphemeralLimits()
		lim.totalBytes = 20 // one {"at":"TKT-2"} signal is 14 bytes
		h := newHarness(t, withEphemeral(lim))
		at := map[string]any{"at": "TKT-2"}
		_, err := h.svc.Post(as(user("bob")), room, PostInput{Type: "cursor", Props: at})
		require.NoError(t, err)
		_, err = h.svc.Post(as(user("carol")), room, PostInput{Type: "cursor", Props: at})
		requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
	})
	t.Run("channels", func(t *testing.T) {
		t.Parallel()
		lim := defaultEphemeralLimits()
		lim.channels = 1
		h := newHarness(t, withEphemeral(lim))
		_, err := h.svc.Post(as(user("bob")), room, typing())
		require.NoError(t, err)
		other := RawAddress{EntityType: "ticket", Anchor: "TKT-2", Type: "room"}
		_, err = h.svc.Post(as(user("bob")), other, typing())
		requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
	})
}

// TestEphemeral_ExpiryMarksChanged pins that an expiring signal marks its
// channel changed for every subscriber, on Take and from the timer.
func TestEphemeral_ExpiryMarksChanged(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	sub := h.svc.Subscribe()
	defer sub.Close()
	_, err := h.svc.Post(as(user("bob")), room, typing())
	require.NoError(t, err)
	<-sub.Wake()
	require.Equal(t, []channels.Key{roomKey()}, takeKeys(t, sub))
	require.Empty(t, takeKeys(t, sub))

	h.clock.Add(6 * time.Second)
	require.Equal(t, []channels.Key{roomKey()}, takeKeys(t, sub), "expiry is a change")
	require.Empty(t, takeKeys(t, sub))
}

// TestEphemeral_TimerWakes pins that the expiry timer wakes a subscriber
// without a Take.
func TestEphemeral_TimerWakes(t *testing.T) {
	t.Parallel()
	local, err := NewLocal(time.Now)
	require.NoError(t, err)
	defer local.Stop()
	sub := local.Subscribe()
	defer sub.Close()
	key := roomKey()
	now := time.Now()
	_, err = local.ephemeral.add(key, signal{id: "a", identity: "x", at: now, expires: now.Add(20 * time.Millisecond)}, now)
	require.NoError(t, err)
	local.scheduleExpiry()
	select {
	case <-sub.Wake():
	case <-time.After(5 * time.Second):
		t.Fatal("the expiry timer did not wake the subscriber")
	}
	require.Equal(t, []channels.Key{key}, takeKeys(t, sub))
}

// TestEphemeral_DropOnDeleteAndRename pins that a deleted or renamed
// entity takes its signals with it, a deleted entity its rate buckets too,
// and a deleted face only its own. A rename moves the rate buckets.
func TestEphemeral_DropOnDeleteAndRename(t *testing.T) {
	t.Parallel()
	draft := RawAddress{EntityType: "page", Anchor: "PAGE-1@draft", Type: "facechat"}
	tests := []struct {
		name    string
		drop    func(*Local) error
		room    bool // the TKT-1 room signal survives
		buckets int  // buckets left: the aggregate, and a moved per-channel one
	}{
		{"delete", func(l *Local) error { return l.EntityDeleted(t.Context(), "TKT-1") }, false, 1},
		{"rename", func(l *Local) error { return l.EntityRenamed(t.Context(), "TKT-1", "TKT-7") }, false, 2},
		{"other face", func(l *Local) error { return l.EntityFaceDeleted(t.Context(), "TKT-1", "draft") }, true, 2},
		{"family face no-op", func(l *Local) error { return l.EntityFaceDeleted(t.Context(), "TKT-1", "") }, true, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			bob := user("bob")
			_, err := h.svc.Post(as(bob), room, typing())
			require.NoError(t, err)
			require.Positive(t, h.local.limiter.size())
			sub := h.svc.Subscribe()
			defer sub.Close()
			require.NoError(t, tc.drop(h.local))
			head, err := h.svc.Head(as(bob), room)
			require.NoError(t, err)
			require.Equal(t, tc.buckets, h.local.limiter.size())
			if tc.room {
				require.Len(t, head.Ephemeral, 1)
				return
			}
			require.Empty(t, head.Ephemeral)
			require.Equal(t, []channels.Key{roomKey()}, takeKeys(t, sub))
		})
	}
	t.Run("face delete drops the face", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		_, err := h.svc.Post(as(user("bob")), draft, PostInput{Type: "text", Props: map[string]any{"body": "x"}})
		require.NoError(t, err)
		require.NoError(t, h.local.EntityFaceDeleted(t.Context(), "PAGE-1", "draft"))
		for _, ib := range h.local.limiter.ids {
			for co := range ib.perOp {
				require.NotEqual(t, "PAGE-1", co.key.ID)
			}
		}
	})
}
