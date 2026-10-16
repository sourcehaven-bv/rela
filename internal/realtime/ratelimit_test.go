package realtime

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// TestRateLimit_Buckets pins AC9: each message type of a channel has its
// own bucket per identity, a refusal names when to retry, and the bucket
// refills over time.
func TestRateLimit_Buckets(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	bob := user("bob")
	for range realtimelimits.DefaultEphemeralRate.Count {
		_, err := h.svc.Post(as(bob), room, typing())
		require.NoError(t, err)
	}
	_, err := h.svc.Post(as(bob), room, typing())
	d := requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
	require.GreaterOrEqual(t, d.RetryAfter, time.Second, "Retry-After is at least one second")

	_, err = h.svc.Post(as(bob), room, PostInput{Type: "cursor"})
	require.NoError(t, err, "another message type has its own bucket")
	_, err = h.svc.Post(as(user("carol")), room, typing())
	require.NoError(t, err, "another identity has its own bucket")

	h.clock.Add(time.Second)
	_, err = h.svc.Post(as(bob), room, typing())
	require.NoError(t, err, "the bucket refills")
}

// TestRateLimit_DeniedDoesNotCharge pins that a request refused before the
// limiter (no post role) creates and charges no bucket.
func TestRateLimit_DeniedDoesNotCharge(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	for range 5 {
		_, err := h.svc.Post(as(user("bob")), room, PostInput{Type: "perm-text", Props: map[string]any{"body": "x"}})
		requireDenial(t, err, http.StatusForbidden, TypePostDenied)
	}
	require.Zero(t, h.local.limiter.size())
}

// TestRateLimit_Aggregate pins that every write of an identity also draws
// on one aggregate bucket across channels and operations.
func TestRateLimit_Aggregate(t *testing.T) {
	t.Parallel()
	l := newLimiter(1 << 20)
	now := time.Unix(0, 0)
	generous := realtimelimits.Rate{Count: 1000, Per: time.Minute}
	for i := range realtimelimits.IdentityRate.Count {
		k := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: fmt.Sprintf("t%d", i%7)}
		require.NoError(t, l.take(now, "bob", k, opEdit, generous), "take %d", i)
	}
	err := l.take(now, "bob", channels.Key{Anchor: channels.Anchor{ID: "TKT-2"}, Type: "room"}, opCreate, generous)
	requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
	require.NoError(t, l.take(now, "carol", roomKey(), opEdit, generous))
}

// TestRateLimit_CapRefusesNeverEvicts pins that a full bucket table refuses
// a request that needs a new bucket, keeps every existing bucket, and frees
// slots once buckets turn stale.
func TestRateLimit_CapRefusesNeverEvicts(t *testing.T) {
	t.Parallel()
	l := newLimiter(4)
	now := time.Unix(0, 0)
	rate := realtimelimits.Rate{Count: 1, Per: time.Minute}
	require.NoError(t, l.take(now, "bob", roomKey(), opEdit, rate))
	require.NoError(t, l.take(now, "carol", roomKey(), opEdit, rate))
	require.Equal(t, 4, l.size())

	err := l.take(now, "dave", roomKey(), opEdit, rate)
	d := requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
	require.Equal(t, 2*time.Minute, d.RetryAfter)
	require.Equal(t, 4, l.size(), "no bucket was evicted")
	err = l.take(now, "bob", roomKey(), opEdit, rate)
	requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)

	later := now.Add(2*time.Minute + time.Second)
	require.NoError(t, l.take(later, "dave", roomKey(), opEdit, rate))
	require.Equal(t, 2, l.size(), "the stale buckets were swept")
}

// TestRateLimit_HarnessCap pins the cap through the service: with room
// for one caller's buckets, a second caller is refused.
func TestRateLimit_HarnessCap(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withBuckets(2))
	_, err := h.svc.Post(as(user("bob")), room, text("x"))
	require.NoError(t, err)
	_, err = h.svc.Post(as(user("carol")), room, text("x"))
	requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
	require.EqualValues(t, 1, h.store.appends.Load(), "the refused post did not append")
}

// TestRateLimit_Sweep pins the periodic sweep of stale buckets.
func TestRateLimit_Sweep(t *testing.T) {
	t.Parallel()
	l := newLimiter(1 << 20)
	now := time.Unix(0, 0)
	rate := realtimelimits.Rate{Count: 10, Per: time.Second}
	for i := range sweepEvery - 1 {
		k := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "room", Name: fmt.Sprintf("n%d", i)}
		require.NoError(t, l.take(now, fmt.Sprintf("u%d", i), k, opEdit, rate))
	}
	before := l.size()
	require.NoError(t, l.take(now.Add(time.Hour), "z", roomKey(), opEdit, rate))
	require.Less(t, l.size(), before)
}
