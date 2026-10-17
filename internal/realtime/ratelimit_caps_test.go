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

// TestRateLimit_RefusedTakeInsertsNothing pins that a take refused for an
// empty bucket inserts no bucket, so an identity with a drained aggregate
// cannot grow the table by asking for new channels.
func TestRateLimit_RefusedTakeInsertsNothing(t *testing.T) {
	t.Parallel()
	l := newLimiter(defaultLimitCaps())
	now := time.Unix(0, 0)
	generous := realtimelimits.Rate{Count: 1000, Per: time.Minute}
	for range realtimelimits.IdentityRate.Count {
		require.NoError(t, l.take(now, "bob", roomKey(), opEdit, generous))
	}
	before := l.size()
	for i := range 50 {
		k := channels.Key{Anchor: channels.Anchor{ID: fmt.Sprintf("TKT-%d", i+10)}, Type: "room"}
		err := l.take(now, "bob", k, opEdit, generous)
		requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
	}
	require.Equal(t, before, l.size(), "refused takes left the table as it was")
}

// TestRateLimit_FullBucketIsDropped pins that a bucket is stale as soon as
// it is full again, and that a missing bucket counts as full.
func TestRateLimit_FullBucketIsDropped(t *testing.T) {
	t.Parallel()
	l := newLimiter(defaultLimitCaps())
	now := time.Unix(0, 0)
	rate := realtimelimits.Rate{Count: 10, Per: time.Second}
	require.NoError(t, l.take(now, "bob", roomKey(), opEdit, rate))
	b := l.ids["bob"].perOp[channelOp{key: roomKey(), op: opEdit}]
	require.Equal(t, now.Add(100*time.Millisecond), b.fullAt(), "one token refills in a tenth of a second")
	require.False(t, b.full(now))
	require.True(t, b.full(now.Add(100*time.Millisecond)))
	var missing *bucket
	require.InDelta(t, float64(rate.Count), missing.tokensAt(now, rate), 0)

	l.mu.Lock()
	l.sweepLocked(now.Add(100 * time.Millisecond))
	l.mu.Unlock()
	require.Equal(t, 1, l.size(), "the per-channel bucket is full and dropped; the aggregate is not yet full")
	l.mu.Lock()
	l.sweepLocked(now.Add(time.Second))
	l.mu.Unlock()
	require.Zero(t, l.size())
}

// TestRateLimit_PerIdentityCap pins that one identity cannot fill the
// table: past its own cap it alone is refused. Many identities together can
// fill the per-channel table, and then a request that needs a new
// per-channel bucket is refused for everyone. The identity space is
// bounded on its own.
func TestRateLimit_PerIdentityCap(t *testing.T) {
	t.Parallel()
	rate := realtimelimits.Rate{Count: 5, Per: time.Minute}
	keyN := func(i int) channels.Key {
		return channels.Key{Anchor: channels.Anchor{ID: fmt.Sprintf("TKT-%d", i+1)}, Type: "room"}
	}
	t.Run("one identity", func(t *testing.T) {
		t.Parallel()
		l := newLimiter(limitCaps{buckets: 8, perIdentity: 3, identities: 100})
		now := time.Unix(0, 0)
		for i := range 3 {
			require.NoError(t, l.take(now, "bob", keyN(i), opEdit, rate))
		}
		before := l.size()
		err := l.take(now, "bob", keyN(3), opEdit, rate)
		d := requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
		require.Equal(t, 12*time.Second, d.RetryAfter, "bob's first bucket is full in one token's time")
		require.Equal(t, before, l.size())
		require.NoError(t, l.take(now, "bob", keyN(0), opEdit, rate), "an existing bucket still serves")
		require.NoError(t, l.take(now, "carol", keyN(3), opEdit, rate), "another identity is not refused")
	})
	t.Run("many identities", func(t *testing.T) {
		t.Parallel()
		l := newLimiter(limitCaps{buckets: 4, perIdentity: 3, identities: 100})
		now := time.Unix(0, 0)
		for i := range 4 {
			require.NoError(t, l.take(now, fmt.Sprintf("u%d", i/2), keyN(i), opEdit, rate))
		}
		err := l.take(now, "newcomer", keyN(9), opEdit, rate)
		requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
		require.Equal(t, 6, l.size(), "four per-channel buckets and two aggregates; the refusal added none")
	})
	t.Run("identity space", func(t *testing.T) {
		t.Parallel()
		l := newLimiter(limitCaps{buckets: 100, perIdentity: 3, identities: 2})
		now := time.Unix(0, 0)
		require.NoError(t, l.take(now, "bob", keyN(0), opEdit, rate))
		require.NoError(t, l.take(now, "carol", keyN(0), opEdit, rate))
		err := l.take(now, "dave", keyN(0), opEdit, rate)
		d := requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
		require.Equal(t, time.Second, d.RetryAfter, "an aggregate bucket is full again within a second")
		require.NoError(t, l.take(now.Add(12*time.Second), "dave", keyN(0), opEdit, rate))
	})
}

// TestRateLimit_Rekey pins that a rename moves the per-channel buckets to
// the new id with their spent tokens, and that a clash keeps the emptier
// bucket.
func TestRateLimit_Rekey(t *testing.T) {
	t.Parallel()
	l := newLimiter(defaultLimitCaps())
	now := time.Unix(0, 0)
	rate := realtimelimits.Rate{Count: 2, Per: time.Minute}
	oldKey := roomKey()
	newKey := channels.Key{Anchor: channels.Anchor{ID: "TKT-7"}, Type: "room"}
	for range 2 {
		require.NoError(t, l.take(now, "bob", oldKey, opEdit, rate))
	}
	require.NoError(t, l.take(now, "carol", newKey, opEdit, rate))
	require.NoError(t, l.take(now, "carol", oldKey, opEdit, rate))
	require.NoError(t, l.take(now, "carol", oldKey, opEdit, rate))
	l.rekey("TKT-1", "TKT-7")
	requireDenial(t, l.take(now, "bob", newKey, opEdit, rate), http.StatusTooManyRequests, TypeRateLimited)
	requireDenial(t, l.take(now, "carol", newKey, opEdit, rate), http.StatusTooManyRequests, TypeRateLimited)
	require.NoError(t, l.take(now, "bob", oldKey, opEdit, rate), "nothing is left under the old id")
	require.Equal(t, 5, l.size(), "bob: two per-channel and the aggregate; carol: one and the aggregate")
}

// TestRateLimit_RenameKeepsTokens pins through the service that a rename
// does not refill a caller's per-channel tokens.
func TestRateLimit_RenameKeepsTokens(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.anchors.mu.Lock()
	h.anchors.views["TKT-7"] = AnchorView{ID: "TKT-7", Type: "ticket"}
	h.anchors.mu.Unlock()
	bob := user("bob")
	for range realtimelimits.DefaultEphemeralRate.Count {
		_, err := h.svc.Post(as(bob), room, typing())
		require.NoError(t, err)
	}
	require.NoError(t, h.local.EntityRenamed(t.Context(), "TKT-1", "TKT-7"))
	_, err := h.svc.Post(as(bob), RawAddress{EntityType: "ticket", Anchor: "TKT-7", Type: "room"}, typing())
	requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
}
