package realtime

import (
	"slices"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// signal is one ephemeral message held in memory. Its props are the
// validated JSON round trip of what was posted; ref properties hold the
// bare id and are resolved for each reader when the head is read.
type signal struct {
	id       string
	typ      string
	author   channels.Author
	identity string
	at       time.Time
	expires  time.Time
	props    map[string]any
	size     int
}

// ephemeralLimits bound the signals of one process.
type ephemeralLimits struct {
	ring        int
	ringShare   int
	channels    int
	perIdentity int
	// channelsPerIdentity caps the rings one identity holds signals in.
	channelsPerIdentity int
	totalBytes          int
}

func defaultEphemeralLimits() ephemeralLimits {
	return ephemeralLimits{
		ring:                realtimelimits.EphemeralRingSize,
		ringShare:           realtimelimits.EphemeralRingShare,
		channels:            realtimelimits.MaxEphemeralChannels,
		perIdentity:         realtimelimits.MaxEphemeralPerIdentity,
		channelsPerIdentity: realtimelimits.MaxEphemeralChannelsPerIdentity,
		totalBytes:          realtimelimits.MaxEphemeralTotalBytes,
	}
}

// ring is the live signals of one channel, oldest first, with a count per
// identity.
type ring struct {
	sigs []signal
	by   map[string]int
}

// ephemeralStore holds the live signals of every channel in this process:
// one ring of at most [realtimelimits.EphemeralRingSize] entries per
// channel. Signals are never stored and never audited.
//
// Every bound refuses rather than evicts another user's signal. A poster
// who already holds signals in the ring overwrites their own oldest one
// there when they are at their share of the ring, the ring is full, or
// they are at their cap across channels; that overwrite is net zero, so
// the caps count it as such. Otherwise the per-identity caps, the byte
// budget, the ring size and the channel cap refuse with a 429.
//
// The store is process-local: a head served by another process of a
// multi-process deployment does not see these signals.
type ephemeralStore struct {
	mu    sync.Mutex
	lim   ephemeralLimits
	rings map[channels.Key]*ring
	// perIdentity counts each identity's signals, and perIdentityRings the
	// rings it holds signals in.
	perIdentity      map[string]int
	perIdentityRings map[string]int
	bytes            int
}

func newEphemeralStore(lim ephemeralLimits) *ephemeralStore {
	return &ephemeralStore{
		lim: lim, rings: map[channels.Key]*ring{},
		perIdentity: map[string]int{}, perIdentityRings: map[string]int{},
	}
}

// add puts s into the ring of key at now. It first drops expired signals
// and returns the keys whose rings that changed, for the caller to mark.
func (e *ephemeralStore) add(key channels.Key, s signal, now time.Time) (expired []channels.Key, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	expired = e.sweepLocked(now)
	r := e.rings[key]
	own, ringLen := 0, 0
	if r != nil {
		own, ringLen = r.by[s.identity], len(r.sigs)
	}
	count := e.perIdentity[s.identity]
	replace := -1
	if own > 0 && (own >= e.lim.ringShare || ringLen >= e.lim.ring || count >= e.lim.perIdentity) {
		replace = slices.IndexFunc(r.sigs, func(o signal) bool { return o.identity == s.identity })
	}
	replaced, oldBytes := 0, 0
	if replace >= 0 {
		replaced, oldBytes = 1, r.sigs[replace].size
	}
	switch {
	case count-replaced >= e.lim.perIdentity, e.bytes-oldBytes+s.size > e.lim.totalBytes:
		return expired, rateLimited(e.soonestLocked(now))
	case r != nil && ringLen-replaced >= e.lim.ring:
		return expired, rateLimited(r.soonest(now))
	case own == 0 && e.perIdentityRings[s.identity] >= e.lim.channelsPerIdentity:
		return expired, rateLimited(e.soonestLocked(now))
	case r == nil && len(e.rings) >= e.lim.channels:
		return expired, rateLimited(e.soonestLocked(now))
	}
	if r == nil {
		r = &ring{by: map[string]int{}}
		e.rings[key] = r
	}
	if replace >= 0 {
		e.removeLocked(r, replace)
	}
	r.sigs = append(r.sigs, s)
	r.by[s.identity]++
	if r.by[s.identity] == 1 {
		e.perIdentityRings[s.identity]++
	}
	e.perIdentity[s.identity]++
	e.bytes += s.size
	return expired, nil
}

// removeLocked drops the signal at i from r and releases its counts and
// bytes. It leaves an empty ring in place; the caller deletes it.
func (e *ephemeralStore) removeLocked(r *ring, i int) {
	s := r.sigs[i]
	r.sigs = slices.Delete(r.sigs, i, i+1)
	e.forgetLocked(r, s)
}

// forgetLocked releases the counts and bytes of s, which r held.
func (e *ephemeralStore) forgetLocked(r *ring, s signal) {
	e.bytes -= s.size
	r.by[s.identity]--
	if r.by[s.identity] <= 0 {
		delete(r.by, s.identity)
		decrement(e.perIdentityRings, s.identity)
	}
	decrement(e.perIdentity, s.identity)
}

func decrement(m map[string]int, k string) {
	m[k]--
	if m[k] <= 0 {
		delete(m, k)
	}
}

// soonest is how long until the first signal of r expires.
func (r *ring) soonest(now time.Time) time.Duration {
	var next time.Time
	for _, s := range r.sigs {
		if next.IsZero() || s.expires.Before(next) {
			next = s.expires
		}
	}
	return next.Sub(now)
}

// live returns the signals of key that have not expired at now, oldest
// first.
func (e *ephemeralStore) live(key channels.Key, now time.Time) []signal {
	e.mu.Lock()
	defer e.mu.Unlock()
	r := e.rings[key]
	if r == nil {
		return nil
	}
	var out []signal
	for _, s := range r.sigs {
		if now.Before(s.expires) {
			out = append(out, s)
		}
	}
	return out
}

// sweep drops expired signals and returns the keys whose rings changed.
func (e *ephemeralStore) sweep(now time.Time) []channels.Key {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sweepLocked(now)
}

func (e *ephemeralStore) sweepLocked(now time.Time) []channels.Key {
	var changed []channels.Key
	for key, r := range e.rings {
		kept := r.sigs[:0]
		for _, s := range r.sigs {
			if now.Before(s.expires) {
				kept = append(kept, s)
			} else {
				e.forgetLocked(r, s)
			}
		}
		if len(kept) == len(r.sigs) {
			continue
		}
		clear(r.sigs[len(kept):])
		r.sigs = kept
		changed = append(changed, key)
		if len(kept) == 0 {
			delete(e.rings, key)
		}
	}
	channels.SortKeys(changed)
	return changed
}

// soonestLocked is how long until the next signal expires, freeing room.
func (e *ephemeralStore) soonestLocked(now time.Time) time.Duration {
	next, ok := e.nextExpiryLocked()
	if !ok {
		return time.Second
	}
	return next.Sub(now)
}

// nextExpiry returns the earliest expiry of a held signal.
func (e *ephemeralStore) nextExpiry() (time.Time, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.nextExpiryLocked()
}

func (e *ephemeralStore) nextExpiryLocked() (time.Time, bool) {
	var next time.Time
	for _, r := range e.rings {
		for _, s := range r.sigs {
			if next.IsZero() || s.expires.Before(next) {
				next = s.expires
			}
		}
	}
	return next, !next.IsZero()
}

// dropAnchor drops the rings of every channel of entity id, or only those
// anchored to face when face is non-empty, and returns their keys.
func (e *ephemeralStore) dropAnchor(id string, face entity.Face) []channels.Key {
	e.mu.Lock()
	defer e.mu.Unlock()
	var dropped []channels.Key
	for key, r := range e.rings {
		if key.ID != id || (face != "" && key.Face != face) {
			continue
		}
		for _, s := range r.sigs {
			e.forgetLocked(r, s)
		}
		delete(e.rings, key)
		dropped = append(dropped, key)
	}
	channels.SortKeys(dropped)
	return dropped
}
