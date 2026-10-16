package realtime

import (
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// Rate-limit operation names. A post is keyed by its message type, so each
// type has its own bucket; the other operations share one per channel.
const (
	opCreate    = "create"
	opEdit      = "edit"
	opRemove    = "remove"
	opPostPfx   = "post:"
	opAggregate = "*"
)

// sweepEvery is how many takes pass between two sweeps of stale buckets.
const sweepEvery = 256

// bucketKey identifies one token bucket: an identity, a channel and an
// operation. The aggregate bucket of an identity has the zero key and
// [opAggregate].
type bucketKey struct {
	identity string
	key      channels.Key
	op       string
}

// bucket is a classic token bucket: Count tokens, refilled at Count per Per.
type bucket struct {
	tokens float64
	last   time.Time
	rate   realtimelimits.Rate
}

// refill brings b up to date at now under rate. A rate lowered by a reload
// caps the tokens at once.
func (b *bucket) refill(now time.Time, rate realtimelimits.Rate) {
	b.rate = rate
	if elapsed := now.Sub(b.last); elapsed > 0 {
		b.tokens += elapsed.Seconds() * rate.PerSecond()
		b.last = now
	}
	b.tokens = min(b.tokens, float64(rate.Count))
}

// wait is how long until b holds one token.
func (b *bucket) wait() time.Duration {
	if b.tokens >= 1 {
		return 0
	}
	ps := b.rate.PerSecond()
	if ps <= 0 { // coverage-ignore: invariant: every rate is positive
		return time.Minute
	}
	return time.Duration((1 - b.tokens) / ps * float64(time.Second))
}

// stale reports whether b has been full for longer than its period at now.
// Refilling from empty takes one period, so after two periods a bucket is
// as good as new and dropping it loses nothing.
func (b *bucket) stale(now time.Time) bool {
	return now.Sub(b.last) > 2*b.rate.Per
}

// limiter keeps the token buckets of one process. Every write charges two
// buckets at once: one per identity, channel and operation, and one per
// identity across everything (realtimelimits.IdentityRate). The buckets are
// bounded: at the cap a request that needs a new bucket is refused, and no
// bucket is evicted to make room, so one caller cannot reset another's
// limits by flooding the table.
//
// The buckets live in process memory. Each process of a multi-process
// deployment limits on its own, so the effective limit is that many times
// higher there.
type limiter struct {
	mu      sync.Mutex
	buckets map[bucketKey]*bucket
	calls   int
	max     int
}

func newLimiter(maxBuckets int) *limiter {
	return &limiter{buckets: map[bucketKey]*bucket{}, max: maxBuckets}
}

// bucketIdentity is the identity rate limits key on: the user and the
// organization. The tool is left out, so a user's browser and agent tokens
// share one budget.
func bucketIdentity(p principal.Principal) string {
	return p.User + "\x00" + p.OrgID()
}

// take charges one token from the bucket of (identity, key, op) at rate and
// from the identity's aggregate bucket. Either empty refuses with a 429
// [DenialError] naming when to retry, and charges neither.
func (l *limiter) take(now time.Time, identity string, key channels.Key, op string, rate realtimelimits.Rate) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls++
	if l.calls%sweepEvery == 0 {
		l.sweepLocked(now)
	}
	keys := [2]bucketKey{{identity: identity, key: key, op: op}, {identity: identity, op: opAggregate}}
	rates := [2]realtimelimits.Rate{rate, realtimelimits.IdentityRate}
	missing := 0
	for _, k := range keys {
		if _, ok := l.buckets[k]; !ok {
			missing++
		}
	}
	if missing > 0 && len(l.buckets)+missing > l.max {
		l.sweepLocked(now)
		if len(l.buckets)+missing > l.max {
			return rateLimited(l.nextFreeLocked(now))
		}
	}
	var got [2]*bucket
	var wait time.Duration
	for i, k := range keys {
		b, ok := l.buckets[k]
		if !ok {
			b = &bucket{tokens: float64(rates[i].Count), last: now, rate: rates[i]}
			l.buckets[k] = b
		}
		b.refill(now, rates[i])
		got[i] = b
		wait = max(wait, b.wait())
	}
	if wait > 0 {
		return rateLimited(wait)
	}
	for _, b := range got {
		b.tokens--
	}
	return nil
}

// sweepLocked drops every stale bucket.
func (l *limiter) sweepLocked(now time.Time) {
	for k, b := range l.buckets {
		if b.stale(now) {
			delete(l.buckets, k)
		}
	}
}

// nextFreeLocked is how long until some bucket turns stale and frees a slot.
func (l *limiter) nextFreeLocked(now time.Time) time.Duration {
	var soonest time.Duration
	first := true
	for _, b := range l.buckets {
		d := b.last.Add(2 * b.rate.Per).Sub(now)
		if first || d < soonest {
			soonest, first = d, false
		}
	}
	return soonest
}

// dropAnchor drops the buckets of every channel of entity id, or only those
// anchored to face when face is non-empty. The aggregate buckets stay.
func (l *limiter) dropAnchor(id string, face entity.Face) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k := range l.buckets {
		if k.key.ID == id && (face == "" || k.key.Face == face) {
			delete(l.buckets, k)
		}
	}
}

// size is the number of buckets held.
func (l *limiter) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
