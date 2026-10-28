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
// signals, the typing entries, the rate-limit buckets of writes and of
// read marks, the change sets of live subscribers and
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
	// readLimiter limits read marks, apart from the writes of limiter, so
	// marks never use up a caller's post budget or its buckets.
	readLimiter *limiter

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
		clock:       clock,
		ephemeral:   newEphemeralStore(lim),
		typing:      defaultTypingStore(),
		limiter:     newLimiter(caps),
		readLimiter: newLimiterAt(caps, realtimelimits.ReadIdentityRate),
		subs:        map[*Subscription]struct{}{},
		recorded:    map[string]channels.TypeRecord{},
	}, nil
}

// Subscription is one live consumer's view of channel changes. Each
// subscriber has its own change set: Take empties only its own.
//
// The set holds at most [realtimelimits.MaxSubscriberChanges] keys. A
// subscriber that falls further behind gets a resync flag from Take instead
// of the keys, and rechecks every channel it follows.
type Subscription struct {
	local *Local
	// allReaders, allAddressed and allDurable are set before the
	// subscription is registered and never change, so they need no lock
	// (see Local.subscribe).
	//
	// allReaders is set by [Local.SubscribeLive]: the subscriber takes
	// every reader's marks, tagged by reader, through TakeReads.
	allReaders bool
	// allAddressed is set by [Local.SubscribeAddressed] and
	// [Local.SubscribeLive]: the subscriber also takes the principal keys a
	// post or edit addressed, through TakeAddressed.
	allAddressed bool
	// allDurable is set by [Local.SubscribeLive]: the subscriber also
	// takes the channels a stored write changed, through TakeDurable.
	allDurable bool
	wake       chan struct{}
	mu         sync.Mutex
	changed    map[channels.Key]struct{}
	// reads holds the marks of an allReaders subscriber, by reader. It
	// shares max with changed: past it, the subscriber resyncs.
	reads  map[channels.Reader]map[channels.Key]struct{}
	nReads int
	// addressed holds the principal keys an allAddressed subscriber has
	// not taken yet. It shares max with changed: past it, the subscriber
	// resyncs.
	addressed map[string]struct{}
	// durable holds the channels of an allDurable subscriber that a
	// stored write changed: a message, an edit, a removal, a session or
	// lifecycle move, an action. Typing and ephemeral signals, and their
	// expiry, are not in it. Every key in it is in changed too, so it
	// shares max with changed.
	durable map[channels.Key]struct{}
	resync  bool
	max     int
}

// Subscribe registers a consumer. Close it when done.
func (l *Local) Subscribe() *Subscription {
	return l.subscribe(func(*Subscription) {})
}

// subscribe registers a consumer after setup has set its options. The
// options are set before the subscription is published under subMu and
// never change afterwards, so the markers read them under subMu alone.
func (l *Local) subscribe(setup func(*Subscription)) *Subscription {
	s := &Subscription{
		local: l, wake: make(chan struct{}, 1), changed: map[channels.Key]struct{}{},
		max: realtimelimits.MaxSubscriberChanges,
	}
	setup(s)
	l.subMu.Lock()
	l.subs[s] = struct{}{}
	l.subMu.Unlock()
	return s
}

// SubscribeLive registers a consumer that serves many readers, such as
// the live hub: it takes channel changes as [Local.Subscribe] does, every
// reader's moved markers, tagged by reader, from TakeReads, the principal
// keys a post or edit addressed from TakeAddressed, and the channels a
// stored write changed from TakeDurable. An overflow of any of them
// resyncs. Close it when done.
func (l *Local) SubscribeLive() *Subscription {
	return l.subscribe(func(s *Subscription) {
		s.allReaders = true
		s.reads = map[channels.Reader]map[channels.Key]struct{}{}
		s.allAddressed = true
		s.addressed = map[string]struct{}{}
		s.allDurable = true
		s.durable = map[channels.Key]struct{}{}
	})
}

// TakeDurable returns the channels a stored write changed since the
// previous TakeDurable, sorted, and empties the set. It returns nil for a
// subscription not made by [Local.SubscribeLive], and nil when the set
// overflowed: Take then reports resync.
func (s *Subscription) TakeDurable() []channels.Key {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.durable) == 0 {
		return nil
	}
	out := make([]channels.Key, 0, len(s.durable))
	for k := range s.durable {
		out = append(out, k)
	}
	clear(s.durable)
	channels.SortKeys(out)
	return out
}

// SubscribeAddressed registers a consumer that wakes a principal's inbox,
// such as the MCP watch: it takes channel changes as [Local.Subscribe]
// does, and the principal keys a post or edit in this process addressed
// (mentioned, or replied to) from TakeAddressed. A plain subscription
// never sees those keys and never resyncs from them. Close it when done.
func (l *Local) SubscribeAddressed() *Subscription {
	return l.subscribe(func(s *Subscription) {
		s.allAddressed = true
		s.addressed = map[string]struct{}{}
	})
}

// TakeAddressed returns the principal keys addressed since the previous
// TakeAddressed, sorted, and empties the set. It returns nil for a
// subscription made by neither [Local.SubscribeAddressed] nor
// [Local.SubscribeLive], and nil when the
// set overflowed: Take then reports resync.
func (s *Subscription) TakeAddressed() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.addressed) == 0 {
		return nil
	}
	out := make([]string, 0, len(s.addressed))
	for k := range s.addressed {
		out = append(out, k)
	}
	clear(s.addressed)
	slices.Sort(out)
	return out
}

// TakeReads returns the channels on which each reader's marker moved since
// the previous TakeReads, and empties the set. It returns nil for a
// subscription not made by [Local.SubscribeLive], and nil when the set
// overflowed: Take then reports resync.
func (s *Subscription) TakeReads() map[channels.Reader][]channels.Key {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.reads) == 0 {
		return nil
	}
	out := make(map[channels.Reader][]channels.Key, len(s.reads))
	for r, set := range s.reads {
		keys := make([]channels.Key, 0, len(set))
		for k := range set {
			keys = append(keys, k)
		}
		channels.SortKeys(keys)
		out[r] = keys
	}
	clear(s.reads)
	s.nReads = 0
	return out
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
		clear(s.reads)
		clear(s.addressed)
		clear(s.durable)
		s.nReads = 0
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

// mark adds keys to every subscriber's change set, and to the durable set
// of each [Local.SubscribeLive] subscriber, and wakes each one. It is for
// changes a stored write made.
func (l *Local) mark(keys ...channels.Key) {
	l.markWhere(func(*Subscription) bool { return true }, keys, true)
}

// markEphemeral adds keys to every subscriber's change set only: a typing
// entry or an ephemeral signal started, ended or expired. A channel head
// shows them; an inbox does not, so the live hub need not replay one.
func (l *Local) markEphemeral(keys ...channels.Key) {
	l.markWhere(func(*Subscription) bool { return true }, keys, false)
}

// markAddressed marks key changed for every subscriber, as mark does, and
// adds targets, the principal keys a post or edit addressed, to every
// [Local.SubscribeAddressed] subscriber.
func (l *Local) markAddressed(targets []string, key channels.Key) {
	l.mark(key)
	if len(targets) == 0 {
		return
	}
	l.subMu.Lock()
	subs := make([]*Subscription, 0, len(l.subs))
	for s := range l.subs {
		if s.allAddressed {
			subs = append(subs, s)
		}
	}
	l.subMu.Unlock()
	for _, s := range subs {
		s.mu.Lock()
		for _, t := range targets {
			if s.resync {
				break
			}
			s.addressed[t] = struct{}{}
			if len(s.addressed) > s.max {
				s.resync = true
				clear(s.changed)
				clear(s.addressed)
				clear(s.durable)
			}
		}
		s.mu.Unlock()
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
}

// markFor adds keys to the reads of every [Local.SubscribeLive]
// subscriber, tagged with reader, and to no change set: a moved read
// marker changes nothing another principal sees, and the live hub scopes
// the replay to reader's own connections.
func (l *Local) markFor(reader channels.Reader, keys ...channels.Key) {
	if reader.User == "" || len(keys) == 0 {
		return
	}
	l.subMu.Lock()
	subs := make([]*Subscription, 0, len(l.subs))
	for s := range l.subs {
		if s.allReaders {
			subs = append(subs, s)
		}
	}
	l.subMu.Unlock()
	for _, s := range subs {
		s.addReads(reader, keys)
	}
}

// addReads records reader's moved markers on keys in s and wakes it.
func (s *Subscription) addReads(reader channels.Reader, keys []channels.Key) {
	s.mu.Lock()
	for _, k := range keys {
		if s.resync {
			break
		}
		set := s.reads[reader]
		if set == nil {
			set = map[channels.Key]struct{}{}
			s.reads[reader] = set
		}
		if _, ok := set[k]; !ok {
			set[k] = struct{}{}
			s.nReads++
		}
		if s.nReads > s.max {
			s.resync = true
			clear(s.changed)
			clear(s.reads)
			clear(s.durable)
			s.nReads = 0
		}
	}
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// markWhere adds keys to the change set of each subscriber that pick
// admits, and wakes it. durable also adds them to the durable set of each
// such subscriber that takes one.
func (l *Local) markWhere(pick func(*Subscription) bool, keys []channels.Key, durable bool) {
	if len(keys) == 0 {
		return
	}
	l.subMu.Lock()
	subs := make([]*Subscription, 0, len(l.subs))
	for s := range l.subs {
		if pick(s) {
			subs = append(subs, s)
		}
	}
	l.subMu.Unlock()
	for _, s := range subs {
		s.mu.Lock()
		for _, k := range keys {
			if s.resync {
				break
			}
			s.changed[k] = struct{}{}
			if durable && s.allDurable {
				s.durable[k] = struct{}{}
			}
			if len(s.changed) > s.max {
				s.resync = true
				clear(s.changed)
				clear(s.durable)
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
	l.markEphemeral(l.ephemeral.sweep(now)...)
	l.markEphemeral(l.typing.sweep(now)...)
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
	l.readLimiter.dropAnchor(id, face)
	l.markEphemeral(l.ephemeral.dropAnchor(id, face)...)
	l.markEphemeral(l.typing.dropAnchor(id, face)...)
}

// EntityRenamed moves the rate buckets of the channels of oldID to newID,
// where the stored channels move, so a rename does not refill anyone's
// per-channel tokens. The live signals and typing entries of oldID are
// dropped.
func (l *Local) EntityRenamed(_ context.Context, oldID, newID string) error {
	l.limiter.rekey(oldID, newID)
	l.readLimiter.rekey(oldID, newID)
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
