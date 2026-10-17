package realtime

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

func keyOf(id string) channels.Key {
	return channels.Key{Anchor: channels.Anchor{ID: id}, Type: "room"}
}

func sig(id, identity string, at time.Time, ttl time.Duration) signal {
	return signal{id: id, identity: identity, at: at, expires: at.Add(ttl), size: 10}
}

// TestEphemeralStore_OwnOverwriteIsNetZero pins that at the per-identity
// cap a poster's next signal in a ring where they already hold one
// replaces it: the count and the bytes stay the same.
func TestEphemeralStore_OwnOverwriteIsNetZero(t *testing.T) {
	t.Parallel()
	lim := defaultEphemeralLimits()
	lim.perIdentity, lim.totalBytes = 2, 20
	e := newEphemeralStore(lim)
	now := time.Unix(0, 0)
	for i := range 2 {
		_, err := e.add(keyOf("A"), sig(strconv.Itoa(i), "x", now, time.Minute), now)
		require.NoError(t, err)
	}
	_, err := e.add(keyOf("A"), sig("2", "x", now, time.Minute), now)
	require.NoError(t, err, "the overwrite is checked against the caps as net zero")
	require.Equal(t, 2, e.perIdentity["x"])
	require.Equal(t, 20, e.bytes)
	live := e.live(keyOf("A"), now)
	require.Equal(t, []string{"1", "2"}, []string{live[0].id, live[1].id}, "the oldest own signal went")
	_, err = e.add(keyOf("B"), sig("3", "x", now, time.Minute), now)
	requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
}

// TestEphemeralStore_RingShare pins that one identity holds at most its
// share of a ring: past it, it overwrites its own oldest signal, so the
// others keep their room.
func TestEphemeralStore_RingShare(t *testing.T) {
	t.Parallel()
	lim := defaultEphemeralLimits()
	lim.ring, lim.ringShare = 4, 2
	e := newEphemeralStore(lim)
	now := time.Unix(0, 0)
	for i := range 10 {
		_, err := e.add(keyOf("A"), sig(strconv.Itoa(i), "x", now, time.Minute), now)
		require.NoError(t, err)
	}
	require.Len(t, e.live(keyOf("A"), now), 2)
	for i := range 2 {
		_, err := e.add(keyOf("A"), sig(fmt.Sprint("y", i), "y", now, time.Minute), now)
		require.NoError(t, err, "another identity still has room")
	}
	require.Len(t, e.live(keyOf("A"), now), 4)
}

// TestEphemeralStore_FullRingRetryAfter pins that a full ring answers
// Retry-After from the soonest expiry in it, not from its oldest insert.
func TestEphemeralStore_FullRingRetryAfter(t *testing.T) {
	t.Parallel()
	lim := defaultEphemeralLimits()
	lim.ring, lim.ringShare = 2, 2
	e := newEphemeralStore(lim)
	now := time.Unix(0, 0)
	_, err := e.add(keyOf("A"), sig("old", "x", now, 30*time.Second), now)
	require.NoError(t, err)
	_, err = e.add(keyOf("A"), sig("new", "y", now, 3*time.Second), now)
	require.NoError(t, err)
	_, err = e.add(keyOf("A"), sig("z", "z", now, time.Minute), now)
	d := requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
	require.Equal(t, 3*time.Second, d.RetryAfter)
}

// TestEphemeralStore_IdentityCapRetryAfter pins that the per-identity caps
// answer Retry-After from the caller's own signals: another poster's
// sooner expiry frees nothing for them.
func TestEphemeralStore_IdentityCapRetryAfter(t *testing.T) {
	t.Parallel()
	now := time.Unix(0, 0)
	t.Run("count", func(t *testing.T) {
		t.Parallel()
		lim := defaultEphemeralLimits()
		lim.perIdentity = 2
		e := newEphemeralStore(lim)
		_, err := e.add(keyOf("A"), sig("other", "y", now, 2*time.Second), now)
		require.NoError(t, err)
		_, err = e.add(keyOf("A"), sig("x1", "x", now, 40*time.Second), now)
		require.NoError(t, err)
		_, err = e.add(keyOf("B"), sig("x2", "x", now, 20*time.Second), now)
		require.NoError(t, err)
		_, err = e.add(keyOf("C"), sig("x3", "x", now, time.Minute), now)
		d := requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
		require.Equal(t, 20*time.Second, d.RetryAfter)
	})
	t.Run("channels", func(t *testing.T) {
		t.Parallel()
		lim := defaultEphemeralLimits()
		lim.channelsPerIdentity = 2
		e := newEphemeralStore(lim)
		_, err := e.add(keyOf("A"), sig("other", "y", now, 2*time.Second), now)
		require.NoError(t, err)
		_, err = e.add(keyOf("A"), sig("a1", "x", now, 10*time.Second), now)
		require.NoError(t, err)
		_, err = e.add(keyOf("A"), sig("a2", "x", now, 50*time.Second), now)
		require.NoError(t, err)
		_, err = e.add(keyOf("B"), sig("b1", "x", now, 30*time.Second), now)
		require.NoError(t, err)
		_, err = e.add(keyOf("C"), sig("c1", "x", now, time.Minute), now)
		d := requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
		require.Equal(t, 30*time.Second, d.RetryAfter,
			"ring B is the first the caller leaves; ring A still holds a2")
	})
}

// TestEphemeralStore_ChannelsPerIdentity pins that one identity holds
// signals in at most a fixed number of channels, and that a channel it is
// already in still takes its signals.
func TestEphemeralStore_ChannelsPerIdentity(t *testing.T) {
	t.Parallel()
	lim := defaultEphemeralLimits()
	lim.channelsPerIdentity = 2
	e := newEphemeralStore(lim)
	now := time.Unix(0, 0)
	for _, id := range []string{"A", "B"} {
		_, err := e.add(keyOf(id), sig(id, "x", now, time.Minute), now)
		require.NoError(t, err)
	}
	_, err := e.add(keyOf("C"), sig("C", "x", now, time.Minute), now)
	requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
	_, err = e.add(keyOf("A"), sig("A2", "x", now, time.Minute), now)
	require.NoError(t, err)
	_, err = e.add(keyOf("C"), sig("C", "y", now, time.Minute), now)
	require.NoError(t, err, "another identity is not affected")
	e.dropAnchor("A", "")
	_, err = e.add(keyOf("C"), sig("C2", "x", now, time.Minute), now)
	require.NoError(t, err, "a dropped channel frees the identity's slot")
}

// TestLocal_StopPreventsRearm pins that after Stop a new signal does not
// arm the expiry timer again.
func TestLocal_StopPreventsRearm(t *testing.T) {
	t.Parallel()
	local, err := NewLocal(time.Now)
	require.NoError(t, err)
	local.Stop()
	now := time.Now()
	_, err = local.ephemeral.add(roomKey(), sig("a", "x", now, time.Hour), now)
	require.NoError(t, err)
	local.scheduleExpiry()
	local.timerMu.Lock()
	defer local.timerMu.Unlock()
	require.Nil(t, local.timer)
}

// TestLocal_SubscriberResync pins that a subscriber whose change set
// overflows gets a resync flag once and then plain keys again.
func TestLocal_SubscriberResync(t *testing.T) {
	t.Parallel()
	local, err := NewLocal(time.Now)
	require.NoError(t, err)
	defer local.Stop()
	sub := local.Subscribe()
	defer sub.Close()
	sub.max = 3
	for i := range 5 {
		local.mark(keyOf(fmt.Sprintf("T-%d", i)))
	}
	keys, resync := sub.Take()
	require.True(t, resync)
	require.Empty(t, keys)
	require.Empty(t, sub.changed, "the set is not kept while resyncing")
	local.mark(keyOf("T-9"))
	require.Equal(t, []channels.Key{keyOf("T-9")}, takeKeys(t, sub))
	require.Equal(t, realtimelimits.MaxSubscriberChanges, local.Subscribe().max)
}

// TestLocal_Concurrent runs posts, head reads, Takes, the expiry timer,
// renames and deletes in parallel, for the race detector.
func TestLocal_Concurrent(t *testing.T) {
	t.Parallel()
	local, err := NewLocal(time.Now)
	require.NoError(t, err)
	defer local.Stop()
	sub := local.Subscribe()
	defer sub.Close()
	rate := realtimelimits.Rate{Count: 1000, Per: time.Second}
	var wg sync.WaitGroup
	for w := range 8 {
		wg.Go(func() {
			id := fmt.Sprintf("u%d", w)
			for i := range 200 {
				now := time.Now()
				key := keyOf(fmt.Sprintf("T-%d", i%5))
				if local.limiter.take(now, id, key, opEdit, rate) == nil {
					expired, _ := local.ephemeral.add(key, sig(strconv.Itoa(i), id, now, time.Millisecond), now)
					local.mark(append(expired, key)...)
					local.scheduleExpiry()
				}
				_ = local.liveSignals(key)
			}
		})
	}
	wg.Go(func() {
		for range 200 {
			sub.Take()
		}
	})
	wg.Go(func() {
		for i := range 100 {
			_ = local.EntityRenamed(t.Context(), fmt.Sprintf("T-%d", i%5), fmt.Sprintf("T-%d", (i+1)%5))
			_ = local.EntityDeleted(t.Context(), fmt.Sprintf("T-%d", i%5))
		}
	})
	wg.Wait()
}
