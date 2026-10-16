package realtime

import (
	"context"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// Local is the process-local state of the channel service: the ephemeral
// signals, the rate-limit buckets, the change sets of live subscribers and
// the channel types already recorded in the store. Services rebuilt over
// one store share it, so a schema reload neither resets anyone's rate limit
// nor drops live signals or subscribers.
//
// It is also an alias subscriber: a deleted or renamed entity takes its
// signals and rate buckets with it.
type Local struct {
	clock     Clock
	ephemeral *ephemeralStore
	limiter   *limiter

	subMu sync.Mutex
	subs  map[*Subscription]struct{}

	timerMu sync.Mutex
	timer   *time.Timer
	timerAt time.Time

	recMu    sync.Mutex
	recorded map[string]channels.TypeRecord
}

// NewLocal returns the process-local state with the limits of
// [realtimelimits], timed by clock.
//
// Nil: clock is rejected.
func NewLocal(clock Clock) (*Local, error) {
	return newLocal(clock, defaultEphemeralLimits(), realtimelimits.MaxRateBuckets)
}

func newLocal(clock Clock, lim ephemeralLimits, maxBuckets int) (*Local, error) {
	if clock == nil {
		return nil, errNil("clock")
	}
	return &Local{
		clock:     clock,
		ephemeral: newEphemeralStore(lim),
		limiter:   newLimiter(maxBuckets),
		subs:      map[*Subscription]struct{}{},
		recorded:  map[string]channels.TypeRecord{},
	}, nil
}

// Subscription is one live consumer's view of channel changes. Each
// subscriber has its own change set: Take empties only its own.
type Subscription struct {
	local   *Local
	wake    chan struct{}
	mu      sync.Mutex
	changed map[channels.Key]struct{}
}

// Subscribe registers a consumer. Close it when done.
func (l *Local) Subscribe() *Subscription {
	s := &Subscription{local: l, wake: make(chan struct{}, 1), changed: map[channels.Key]struct{}{}}
	l.subMu.Lock()
	l.subs[s] = struct{}{}
	l.subMu.Unlock()
	return s
}

// Wake receives a value after a channel changed since the last Take. A
// receive may be spurious; Take then returns nothing.
func (s *Subscription) Wake() <-chan struct{} { return s.wake }

// Take returns the keys that changed since the previous Take, sorted, and
// empties the set. It first drops expired signals, so their expiry shows up
// here even before the expiry timer fires.
func (s *Subscription) Take() []channels.Key {
	s.local.expire()
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]channels.Key, 0, len(s.changed))
	for k := range s.changed {
		out = append(out, k)
	}
	clear(s.changed)
	channels.SortKeys(out)
	return out
}

// Close unregisters the subscription. Take keeps working and returns
// nothing new.
func (s *Subscription) Close() {
	s.local.subMu.Lock()
	delete(s.local.subs, s)
	s.local.subMu.Unlock()
}

// mark adds keys to every subscriber's change set and wakes each one.
func (l *Local) mark(keys ...channels.Key) {
	if len(keys) == 0 {
		return
	}
	l.subMu.Lock()
	subs := make([]*Subscription, 0, len(l.subs))
	for s := range l.subs {
		subs = append(subs, s)
	}
	l.subMu.Unlock()
	for _, s := range subs {
		s.mu.Lock()
		for _, k := range keys {
			s.changed[k] = struct{}{}
		}
		s.mu.Unlock()
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
}

// expire drops expired signals and marks their channels changed.
func (l *Local) expire() {
	l.mark(l.ephemeral.sweep(l.clock())...)
}

// scheduleExpiry arms the timer for the earliest signal expiry, so a
// subscriber hears about an expiry without polling.
func (l *Local) scheduleExpiry() {
	next, ok := l.ephemeral.nextExpiry()
	if !ok {
		return
	}
	l.timerMu.Lock()
	defer l.timerMu.Unlock()
	if l.timer != nil && !l.timerAt.After(next) {
		return
	}
	if l.timer != nil {
		l.timer.Stop()
	}
	l.timerAt = next
	l.timer = time.AfterFunc(max(next.Sub(l.clock()), 0), l.fire)
}

// fire is the expiry timer's callback.
func (l *Local) fire() {
	l.timerMu.Lock()
	l.timer, l.timerAt = nil, time.Time{}
	l.timerMu.Unlock()
	l.expire()
	l.scheduleExpiry()
}

// Stop stops the expiry timer. Signals still expire on the next Take.
func (l *Local) Stop() {
	l.timerMu.Lock()
	defer l.timerMu.Unlock()
	if l.timer != nil {
		l.timer.Stop()
		l.timer, l.timerAt = nil, time.Time{}
	}
}

// dropAnchor forgets the signals and rate buckets of the channels of id, or
// only those anchored to face when face is non-empty.
func (l *Local) dropAnchor(id string, face entity.Face) {
	l.limiter.dropAnchor(id, face)
	l.mark(l.ephemeral.dropAnchor(id, face)...)
}

// EntityRenamed drops what was held for the channels of oldID. The stored
// channels move to newID; signals and rate buckets start afresh there.
func (l *Local) EntityRenamed(_ context.Context, oldID, _ string) error {
	l.dropAnchor(oldID, "")
	return nil
}

// EntityDeleted drops what was held for the channels of entityID.
func (l *Local) EntityDeleted(_ context.Context, entityID string) error {
	l.dropAnchor(entityID, "")
	return nil
}

// EntityFaceDeleted drops what was held for the channels anchored to the
// deleted face. Channels of the whole entity keep theirs.
func (l *Local) EntityFaceDeleted(_ context.Context, entityID string, face entity.Face) error {
	if face == "" {
		return nil
	}
	l.dropAnchor(entityID, face)
	return nil
}

// needsRecord reports whether rec must be written for typ: the store has
// not been told about it in this process, or was told something else.
func (l *Local) needsRecord(typ string, rec channels.TypeRecord) bool {
	l.recMu.Lock()
	defer l.recMu.Unlock()
	got, ok := l.recorded[typ]
	return !ok || got != rec
}

// noteRecorded remembers that the store holds rec for typ.
func (l *Local) noteRecorded(typ string, rec channels.TypeRecord) {
	l.recMu.Lock()
	defer l.recMu.Unlock()
	l.recorded[typ] = rec
}

// liveSignals returns the live signals of key.
func (l *Local) liveSignals(key channels.Key) []signal {
	return l.ephemeral.live(key, l.clock())
}
