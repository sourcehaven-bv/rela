package realtime

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// Local is the process-local state of the channel service: the ephemeral
// signals, the typing entries, the rate-limit buckets, the change sets of live subscribers and
// the channel types already recorded in the store. Services rebuilt over
// one store share it, so a schema reload neither resets anyone's rate limit
// nor drops live signals or subscribers.
//
// It is also an alias subscriber: a deleted or renamed entity takes its
// signals, typing entries and rate buckets with it.
type Local struct {
	clock     Clock
	ephemeral *ephemeralStore
	typing    *typingStore
	limiter   *limiter

	subMu sync.Mutex
	subs  map[*Subscription]struct{}

	timerMu sync.Mutex
	timer   *time.Timer
	timerAt time.Time
	// stopped is set by Stop; the expiry timer is never armed again.
	stopped bool

	// recorded caches the type records this process has written, so a
	// write costs no store call once its type is recorded. The store
	// decides conflicts; the cache only skips repeats.
	recMu    sync.Mutex
	recorded map[string]channels.TypeRecord

	// stored caches the refs the store's type records hold, by channel
	// type, so rendering reads the store once per config generation rather
	// than once per message. storedFor is the config it was read under;
	// nil means not read. See Local.storedRefs.
	storedMu  sync.Mutex
	storedFor *realtimecfg.Compiled
	stored    map[string][]string

	// outputLocks serialize the outputs runs of a channel in this process,
	// so the supersede read and the writes of one run see every run before
	// it.
	outputLocks outputLocks
}

// typeRecordReader is the part of a channel store Local.storedRefs reads.
type typeRecordReader interface {
	TypeRecords(ctx context.Context) (map[string]channels.TypeRecord, error)
}

// NewLocal returns the process-local state with the limits of
// [realtimelimits], timed by clock.
//
// Nil: clock is rejected.
func NewLocal(clock Clock) (*Local, error) {
	return newLocal(clock, defaultEphemeralLimits(), defaultLimitCaps())
}

func newLocal(clock Clock, lim ephemeralLimits, caps limitCaps) (*Local, error) {
	if clock == nil {
		return nil, errNil("clock")
	}
	return &Local{
		clock:     clock,
		ephemeral: newEphemeralStore(lim),
		typing:    defaultTypingStore(),
		limiter:   newLimiter(caps),
		subs:      map[*Subscription]struct{}{},
		recorded:  map[string]channels.TypeRecord{},
	}, nil
}

// Subscription is one live consumer's view of channel changes. Each
// subscriber has its own change set: Take empties only its own.
//
// The set holds at most [realtimelimits.MaxSubscriberChanges] keys. A
// subscriber that falls further behind gets a resync flag from Take instead
// of the keys, and rechecks every channel it follows.
type Subscription struct {
	local   *Local
	wake    chan struct{}
	mu      sync.Mutex
	changed map[channels.Key]struct{}
	resync  bool
	max     int
}

// Subscribe registers a consumer. Close it when done.
func (l *Local) Subscribe() *Subscription {
	s := &Subscription{
		local: l, wake: make(chan struct{}, 1), changed: map[channels.Key]struct{}{},
		max: realtimelimits.MaxSubscriberChanges,
	}
	l.subMu.Lock()
	l.subs[s] = struct{}{}
	l.subMu.Unlock()
	return s
}

// Wake receives a value after a channel changed since the last Take. A
// receive may be spurious; Take then returns nothing.
func (s *Subscription) Wake() <-chan struct{} { return s.wake }

// Take returns the keys that changed since the previous Take, sorted, and
// empties the set. When more channels changed than the set holds, it
// returns no keys and resync set: every channel may have changed. It first
// drops expired signals, so their expiry shows up here even before the
// expiry timer fires.
func (s *Subscription) Take() (keys []channels.Key, resync bool) {
	s.local.expire()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.resync {
		s.resync = false
		clear(s.changed)
		return []channels.Key{}, true
	}
	out := make([]channels.Key, 0, len(s.changed))
	for k := range s.changed {
		out = append(out, k)
	}
	clear(s.changed)
	channels.SortKeys(out)
	return out, false
}

// Close unregisters the subscription. Take keeps working and returns
// nothing new.
func (s *Subscription) Close() {
	s.local.subMu.Lock()
	delete(s.local.subs, s)
	s.local.subMu.Unlock()
}

// Mark implements [ChangeMarker]: the background sweep reports the channels
// it changed, so this process's subscribers see them.
func (l *Local) Mark(keys ...channels.Key) { l.mark(keys...) }

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
			if s.resync {
				break
			}
			s.changed[k] = struct{}{}
			if len(s.changed) > s.max {
				s.resync = true
				clear(s.changed)
			}
		}
		s.mu.Unlock()
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
}

// expire drops expired signals and typing entries and marks their
// channels changed.
func (l *Local) expire() {
	now := l.clock()
	l.mark(l.ephemeral.sweep(now)...)
	l.mark(l.typing.sweep(now)...)
}

// scheduleExpiry arms the timer for the earliest signal expiry or typing
// deadline, so a subscriber hears about an expiry without polling.
func (l *Local) scheduleExpiry() {
	next, ok := l.ephemeral.nextExpiry()
	if deadline, held := l.typing.nextDeadline(); held && (!ok || deadline.Before(next)) {
		next, ok = deadline, true
	}
	if !ok {
		return
	}
	l.timerMu.Lock()
	defer l.timerMu.Unlock()
	if l.stopped || (l.timer != nil && !l.timerAt.After(next)) {
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

// Stop stops the expiry timer for good: a later post does not arm it
// again. Signals still expire on the next Take.
func (l *Local) Stop() {
	l.timerMu.Lock()
	defer l.timerMu.Unlock()
	l.stopped = true
	if l.timer != nil {
		l.timer.Stop()
		l.timer, l.timerAt = nil, time.Time{}
	}
}

// dropAnchor forgets the signals, typing entries and rate buckets of the
// channels of id, or only those anchored to face when face is non-empty.
func (l *Local) dropAnchor(id string, face entity.Face) {
	l.limiter.dropAnchor(id, face)
	l.mark(l.ephemeral.dropAnchor(id, face)...)
	l.mark(l.typing.dropAnchor(id, face)...)
}

// EntityRenamed moves the rate buckets of the channels of oldID to newID,
// where the stored channels move, so a rename does not refill anyone's
// per-channel tokens. The live signals and typing entries of oldID are
// dropped.
func (l *Local) EntityRenamed(_ context.Context, oldID, newID string) error {
	l.limiter.rekey(oldID, newID)
	l.mark(l.ephemeral.dropAnchor(oldID, "")...)
	l.mark(l.typing.dropAnchor(oldID, "")...)
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
	return !ok || !got.Equal(rec)
}

// noteRecorded remembers that the store holds rec for typ. The store adds
// refs and never drops them, so rec's refs join the cached stored refs.
func (l *Local) noteRecorded(typ string, rec channels.TypeRecord) {
	l.recMu.Lock()
	l.recorded[typ] = rec
	l.recMu.Unlock()
	l.storedMu.Lock()
	defer l.storedMu.Unlock()
	if l.storedFor != nil && len(rec.Refs) > 0 {
		refs := slices.Concat(l.stored[typ], rec.Refs)
		slices.Sort(refs)
		l.stored[typ] = slices.Compact(refs)
	}
}

// storedRefs returns the refs the store has recorded for channel type typ,
// as "message-type.property". A stored id renders as a ref even when cfg
// declares its property as another type, so a config change cannot turn a
// hidden entity's id into plain text.
//
// The records are read from st once per cfg: a reload publishes a new
// config, which reads them again. A refused record write
// ([channels.ErrTypeShapeConflict]) also drops the cache, since another
// process may have added refs.
func (l *Local) storedRefs(
	ctx context.Context, st typeRecordReader, cfg *realtimecfg.Compiled, typ string,
) ([]string, error) {
	l.storedMu.Lock()
	defer l.storedMu.Unlock()
	if l.storedFor != cfg || l.storedFor == nil {
		recs, err := st.TypeRecords(ctx)
		if err != nil {
			return nil, fmt.Errorf("realtime: read the recorded channel types: %w", err)
		}
		l.stored = make(map[string][]string, len(recs))
		for t, rec := range recs {
			l.stored[t] = rec.Refs
		}
		l.storedFor = cfg
	}
	return l.stored[typ], nil
}

// forgetStoredRefs drops the cache of Local.storedRefs.
func (l *Local) forgetStoredRefs() {
	l.storedMu.Lock()
	defer l.storedMu.Unlock()
	l.storedFor, l.stored = nil, nil
}

// liveSignals returns the live signals of key.
func (l *Local) liveSignals(key channels.Key) []signal {
	return l.ephemeral.live(key, l.clock())
}
