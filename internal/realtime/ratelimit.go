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
	opLifecycle = "lifecycle"
	opPostPfx   = "post:"
)

// sweepEvery is how many takes pass between two sweeps of full buckets.
const sweepEvery = 256

// channelOp identifies one per-channel bucket of an identity: a channel and
// an operation.
type channelOp struct {
	key channels.Key
	op  string
}

// bucket is a classic token bucket: Count tokens, refilled at Count per Per.
// A bucket that does not exist counts as full.
type bucket struct {
	tokens float64
	last   time.Time
	rate   realtimelimits.Rate
}

// tokensAt is how many tokens b holds at now under rate. A rate lowered by
// a reload caps the tokens at once. Nil: b is accepted and is a full bucket.
func (b *bucket) tokensAt(now time.Time, rate realtimelimits.Rate) float64 {
	if b == nil {
		return float64(rate.Count)
	}
	t := b.tokens
	if elapsed := now.Sub(b.last); elapsed > 0 {
		t += elapsed.Seconds() * rate.PerSecond()
	}
	return min(t, float64(rate.Count))
}

// waitFor is how long until a bucket holding tokens has one under rate.
func waitFor(tokens float64, rate realtimelimits.Rate) time.Duration {
	if tokens >= 1 {
		return 0
	}
	ps := rate.PerSecond()
	if ps <= 0 { // coverage-ignore: invariant: every rate is positive
		return time.Minute
	}
	return time.Duration((1 - tokens) / ps * float64(time.Second))
}

// fullAt is when b holds Count tokens again. From then on it is no
// different from a missing bucket, so dropping it loses nothing.
func (b *bucket) fullAt() time.Time {
	ps := b.rate.PerSecond()
	if ps <= 0 { // coverage-ignore: invariant: every rate is positive
		return b.last
	}
	missing := float64(b.rate.Count) - b.tokens
	return b.last.Add(time.Duration(missing / ps * float64(time.Second)))
}

// full reports whether b is full at now.
func (b *bucket) full(now time.Time) bool { return !now.Before(b.fullAt()) }

// identityBuckets are the buckets of one identity: its aggregate bucket
// across everything, and one per channel and operation.
type identityBuckets struct {
	aggregate *bucket
	perOp     map[channelOp]*bucket
}

func (ib *identityBuckets) empty() bool { return ib.aggregate == nil && len(ib.perOp) == 0 }

// limitCaps bound the buckets of one process.
type limitCaps struct {
	// buckets caps the per-channel buckets across identities.
	buckets int
	// perIdentity caps the per-channel buckets of one identity.
	perIdentity int
	// identities caps the identities held, and with them the aggregate
	// buckets.
	identities int
}

func defaultLimitCaps() limitCaps {
	return limitCaps{
		buckets:     realtimelimits.MaxRateBuckets,
		perIdentity: realtimelimits.MaxRateBucketsPerIdentity,
		identities:  realtimelimits.MaxRateIdentities,
	}
}

// limiter keeps the token buckets of one process. Every write charges two
// buckets at once: one per identity, channel and operation, and one per
// identity across everything (realtimelimits.IdentityRate).
//
// A missing bucket counts as full, so a take computes the outcome before it
// inserts anything and inserts only when the take succeeds: a refused take
// leaves the table as it was. A bucket is dropped once it is full again.
//
// The table is bounded three ways, and each bound refuses rather than
// evicts, so one caller cannot reset another's limits by flooding it: an
// identity holds at most limitCaps.perIdentity per-channel buckets, which
// refuses only that identity; the process holds at most limitCaps.buckets
// per-channel buckets; and at most limitCaps.identities identities. The
// aggregate buckets live in the identity space, apart from the per-channel
// ones, so a full per-channel table never keeps a new identity out.
//
// The buckets live in process memory. Each process of a multi-process
// deployment limits on its own, so the effective limit is that many times
// higher there.
type limiter struct {
	mu    sync.Mutex
	ids   map[string]*identityBuckets
	perOp int // per-channel buckets across ids
	calls int
	caps  limitCaps
}

func newLimiter(caps limitCaps) *limiter {
	return &limiter{ids: map[string]*identityBuckets{}, caps: caps}
}

// bucketIdentity is the identity rate limits key on: the user and the
// organization. The tool is left out, so a user's browser and agent tokens
// share one budget.
func bucketIdentity(p principal.Principal) string {
	return p.User + "\x00" + p.OrgID()
}

// take charges one token from the bucket of (identity, key, op) at rate and
// from the identity's aggregate bucket. Either empty, or no room for a
// bucket it needs, refuses with a 429 [DenialError] naming when to retry;
// a refusal charges and inserts nothing.
func (l *limiter) take(now time.Time, identity string, key channels.Key, op string, rate realtimelimits.Rate) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls++
	if l.calls%sweepEvery == 0 {
		l.sweepLocked(now)
	}
	co := channelOp{key: key, op: op}
	ib := l.ids[identity]
	var agg, per *bucket
	if ib != nil {
		agg, per = ib.aggregate, ib.perOp[co]
	}
	aggTokens := agg.tokensAt(now, realtimelimits.IdentityRate)
	perTokens := per.tokensAt(now, rate)
	if wait := max(waitFor(aggTokens, realtimelimits.IdentityRate), waitFor(perTokens, rate)); wait > 0 {
		return rateLimited(wait)
	}
	if err := l.roomLocked(now, identity, agg == nil, per == nil); err != nil {
		return err
	}
	if ib = l.ids[identity]; ib == nil {
		ib = &identityBuckets{perOp: map[channelOp]*bucket{}}
		l.ids[identity] = ib
	}
	ib.aggregate = &bucket{tokens: aggTokens - 1, last: now, rate: realtimelimits.IdentityRate}
	if per == nil {
		l.perOp++
	}
	ib.perOp[co] = &bucket{tokens: perTokens - 1, last: now, rate: rate}
	return nil
}

// roomLocked checks that the buckets a take must insert fit: a new
// identity, and a new per-channel bucket. It sweeps full buckets once
// before it refuses.
func (l *limiter) roomLocked(now time.Time, identity string, newAgg, newPer bool) error {
	fits := func() error {
		cur := l.ids[identity]
		if cur == nil && len(l.ids) >= l.caps.identities {
			return rateLimited(l.soonestLocked(now, func(ib *identityBuckets) *bucket { return ib.aggregate }))
		}
		if !newPer {
			return nil
		}
		if cur != nil && len(cur.perOp) >= l.caps.perIdentity {
			return rateLimited(soonest(now, cur.perOp))
		}
		if l.perOp >= l.caps.buckets {
			return rateLimited(l.soonestPerOpLocked(now))
		}
		return nil
	}
	if !newAgg && !newPer {
		return nil
	}
	if fits() == nil {
		return nil
	}
	l.sweepLocked(now)
	return fits()
}

// sweepLocked drops every bucket that is full, and every identity left
// with none.
func (l *limiter) sweepLocked(now time.Time) {
	for id, ib := range l.ids {
		if ib.aggregate != nil && ib.aggregate.full(now) {
			ib.aggregate = nil
		}
		for co, b := range ib.perOp {
			if b.full(now) {
				delete(ib.perOp, co)
				l.perOp--
			}
		}
		if ib.empty() {
			delete(l.ids, id)
		}
	}
}

// soonest is how long until the first of bs is full and can be dropped.
func soonest[K comparable](now time.Time, bs map[K]*bucket) time.Duration {
	var at time.Time
	for _, b := range bs {
		if f := b.fullAt(); at.IsZero() || f.Before(at) {
			at = f
		}
	}
	return at.Sub(now)
}

// soonestLocked is how long until the first bucket pick returns for some
// identity is full.
func (l *limiter) soonestLocked(now time.Time, pick func(*identityBuckets) *bucket) time.Duration {
	bs := map[string]*bucket{}
	for id, ib := range l.ids {
		if b := pick(ib); b != nil {
			bs[id] = b
		}
	}
	return soonest(now, bs)
}

// soonestPerOpLocked is how long until the first per-channel bucket of any
// identity is full.
func (l *limiter) soonestPerOpLocked(now time.Time) time.Duration {
	var d time.Duration
	first := true
	for _, ib := range l.ids {
		if len(ib.perOp) == 0 {
			continue
		}
		if s := soonest(now, ib.perOp); first || s < d {
			d, first = s, false
		}
	}
	return d
}

// dropAnchor drops the buckets of every channel of entity id, or only those
// anchored to face when face is non-empty. The aggregate buckets stay.
func (l *limiter) dropAnchor(id string, face entity.Face) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for ident, ib := range l.ids {
		for co := range ib.perOp {
			if co.key.ID == id && (face == "" || co.key.Face == face) {
				delete(ib.perOp, co)
				l.perOp--
			}
		}
		if ib.empty() {
			delete(l.ids, ident)
		}
	}
}

// rekey moves the buckets of every channel of entity oldID to newID, so a
// rename keeps each caller's spent tokens. Where newID already has a bucket
// for the same channel and operation, the one with fewer tokens stays.
func (l *limiter) rekey(oldID, newID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, ib := range l.ids {
		for co, b := range ib.perOp {
			if co.key.ID != oldID {
				continue
			}
			delete(ib.perOp, co)
			moved := co
			moved.key.ID = newID
			if cur, ok := ib.perOp[moved]; ok {
				l.perOp--
				if cur.tokens <= b.tokens {
					continue
				}
			}
			ib.perOp[moved] = b
		}
	}
}

// size is the number of buckets held, per-channel and aggregate.
func (l *limiter) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := l.perOp
	for _, ib := range l.ids {
		if ib.aggregate != nil {
			n++
		}
	}
	return n
}
