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
	channels    int
	perIdentity int
	totalBytes  int
}

func defaultEphemeralLimits() ephemeralLimits {
	return ephemeralLimits{
		ring:        realtimelimits.EphemeralRingSize,
		channels:    realtimelimits.MaxEphemeralChannels,
		perIdentity: realtimelimits.MaxEphemeralPerIdentity,
		totalBytes:  realtimelimits.MaxEphemeralTotalBytes,
	}
}

// ephemeralStore holds the live signals of every channel in this process:
// one ring of at most [realtimelimits.EphemeralRingSize] entries per
// channel. Signals are never stored and never audited.
//
// Every bound refuses rather than evicts. A ring that is full of live
// signals takes a new one only by overwriting the poster's own oldest
// signal; another user's signal is never pushed out. The per-identity cap,
// the byte budget and the channel cap refuse with a 429 the same way.
//
// The store is process-local: a head served by another process of a
// multi-process deployment does not see these signals.
type ephemeralStore struct {
	mu          sync.Mutex
	lim         ephemeralLimits
	rings       map[channels.Key][]signal
	perIdentity map[string]int
	bytes       int
}

func newEphemeralStore(lim ephemeralLimits) *ephemeralStore {
	return &ephemeralStore{lim: lim, rings: map[channels.Key][]signal{}, perIdentity: map[string]int{}}
}

// add puts s into the ring of key at now. It first drops expired signals
// and returns the keys whose rings that changed, for the caller to mark.
func (e *ephemeralStore) add(key channels.Key, s signal, now time.Time) (expired []channels.Key, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	expired = e.sweepLocked(now)
	if e.bytes+s.size > e.lim.totalBytes || e.perIdentity[s.identity] >= e.lim.perIdentity {
		return expired, rateLimited(e.soonestLocked(now))
	}
	ring, ok := e.rings[key]
	if !ok && len(e.rings) >= e.lim.channels {
		return expired, rateLimited(e.soonestLocked(now))
	}
	if len(ring) >= e.lim.ring {
		own := slices.IndexFunc(ring, func(o signal) bool { return o.identity == s.identity })
		if own < 0 {
			return expired, rateLimited(ring[0].expires.Sub(now))
		}
		e.forgetLocked(ring[own])
		ring = slices.Delete(ring, own, own+1)
	}
	e.rings[key] = append(ring, s)
	e.perIdentity[s.identity]++
	e.bytes += s.size
	return expired, nil
}

// live returns the signals of key that have not expired at now, oldest
// first.
func (e *ephemeralStore) live(key channels.Key, now time.Time) []signal {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []signal
	for _, s := range e.rings[key] {
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
	for key, ring := range e.rings {
		kept := ring[:0]
		for _, s := range ring {
			if now.Before(s.expires) {
				kept = append(kept, s)
			} else {
				e.forgetLocked(s)
			}
		}
		if len(kept) == len(ring) {
			continue
		}
		changed = append(changed, key)
		if len(kept) == 0 {
			delete(e.rings, key)
		} else {
			e.rings[key] = kept
		}
	}
	channels.SortKeys(changed)
	return changed
}

// forgetLocked releases the per-identity count and bytes of s.
func (e *ephemeralStore) forgetLocked(s signal) {
	e.bytes -= s.size
	e.perIdentity[s.identity]--
	if e.perIdentity[s.identity] <= 0 {
		delete(e.perIdentity, s.identity)
	}
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
	for _, ring := range e.rings {
		for _, s := range ring {
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
	for key, ring := range e.rings {
		if key.ID != id || (face != "" && key.Face != face) {
			continue
		}
		for _, s := range ring {
			e.forgetLocked(s)
		}
		delete(e.rings, key)
		dropped = append(dropped, key)
	}
	channels.SortKeys(dropped)
	return dropped
}
