// Package changefeed numbers store change events and answers one question for
// a live-update stream: which entity types changed since a given position.
//
// A [Bus] reads the store's change events and keeps, per entity type, the
// sequence number of the last change, plus the sequence number of the last
// relation change. It keeps no event history, so its memory and the cost of a
// resume are proportional to the number of types, not to the number of
// writes. A stream resumes from an opaque token (see [Subscription.Token]);
// a token this Bus did not mint is reported through [Subscription.Resync].
//
// The Bus is an ungated base source, like the store it reads. It holds entity
// type names and no entity ids, and every consumer must gate those types per
// principal before anything reaches a client. Because the state is per type,
// nothing a consumer observes depends on how many writes happened, only on
// which types changed (TKT-3A1200).
package changefeed

import (
	"errors"
	"slices"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Source is the store change feed the Bus reads. store.Watcher satisfies it.
type Source interface {
	Subscribe(bufSize int) (events <-chan store.Event, cancel func())
}

// sourceBufSize is the buffer of the Bus's store subscription. The store drops
// events when it is full; the pump does constant work per event, so a drop
// needs a burst of this size to arrive faster than a map write each.
const sourceBufSize = 1024

// ErrClosed is returned by [Bus.Start] after [Bus.Close].
var ErrClosed = errors.New("changefeed: bus closed")

// Bus tracks the last change per entity type. Create it with [New], feed it
// with [Bus.Start], stop it with [Bus.Close].
type Bus struct {
	seal *sealer

	mu      sync.Mutex
	head    uint64            // sequence number of the last counted change
	typeSeq map[string]uint64 // entity type -> sequence number of its last change
	relSeq  uint64            // sequence number of the last relation change
	relType map[string]uint64 // relation type -> sequence number of its last change
	subs    map[*Subscription]struct{}
	started bool
	closed  bool
	stop    chan struct{} // closed by Close; ends the pump
	done    chan struct{} // closed when the pump returns
	cancel  func()        // cancels the store subscription
}

// New returns a Bus with a fresh token key. It fails only when the system
// random source does.
func New() (*Bus, error) {
	s, err := newSealer()
	if err != nil {
		return nil, err
	}
	return &Bus{
		seal:    s,
		typeSeq: make(map[string]uint64),
		relType: make(map[string]uint64),
		subs:    make(map[*Subscription]struct{}),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}, nil
}

// Start subscribes to src and counts its events in a goroutine until
// [Bus.Close] or until src closes its channel. A Bus that is never started
// still serves subscriptions; they just see no changes.
//
// Nil: rejected — a Bus with no source would look started and never change.
func (b *Bus) Start(src Source) error {
	if src == nil {
		return errors.New("changefeed: nil source")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	switch {
	case b.closed:
		return ErrClosed
	case b.started:
		return errors.New("changefeed: bus already started")
	}
	events, cancel := src.Subscribe(sourceBufSize)
	b.started = true
	b.cancel = cancel
	go b.pump(events)
	return nil
}

// Close stops counting and waits for the pump to return. It is idempotent and
// safe before [Bus.Start]. Subscriptions stay usable: a stream that is still
// open keeps running and simply sees no further changes.
func (b *Bus) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	started, cancel := b.started, b.cancel
	close(b.stop)
	b.mu.Unlock()

	if !started {
		return
	}
	cancel()
	<-b.done
}

func (b *Bus) pump(events <-chan store.Event) {
	defer close(b.done)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return
			}
			b.publish(ev)
		case <-b.stop:
			return
		}
	}
}

// publish counts one event and wakes every subscriber. Numbering, the state
// update and the wake happen under one lock, so a Subscribe or Take never
// sees a sequence number whose state is not yet recorded.
func (b *Bus) publish(ev store.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch ev.Op {
	case store.EventEntityCreated, store.EventEntityUpdated, store.EventEntityDeleted:
		b.head++
		b.typeSeq[ev.EntityType] = b.head
	case store.EventRelationCreated, store.EventRelationUpdated, store.EventRelationDeleted:
		b.head++
		b.relSeq = b.head
		if ev.RelationType != "" {
			b.relType[ev.RelationType] = b.head
		}
	default:
		return
	}
	for s := range b.subs {
		s.poke()
	}
}

// Subscribe returns a subscription positioned at token. An empty token starts
// at the current position. A token this Bus minted resumes where it points,
// and the subscription wakes at once when anything changed since. Any other
// token starts at the current position with [Subscription.Resync] set.
func (b *Bus) Subscribe(token string) *Subscription {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := &Subscription{bus: b, wake: make(chan struct{}, 1), cursor: b.head}
	if token != "" {
		if seq, ok := b.seal.open(token); ok && seq <= b.head {
			s.cursor = seq
			if seq < b.head {
				s.poke()
			}
		} else {
			s.resync = true
		}
	}
	s.start = s.cursor
	b.subs[s] = struct{}{}
	return s
}

// Batch is what changed between two positions.
type Batch struct {
	Types    []string // entity types with a change, sorted
	Relation bool     // a relation changed; it may alter what a principal can read
	// RelationTypes are the relation types with a change, sorted. A consumer
	// uses them to mark the types at both ends stale, since an edge is part
	// of how both ends are shown.
	RelationTypes []string
	Seq      uint64   // the position this batch brings the subscription to
}

// Subscription follows a Bus from a position. Its methods are safe for
// concurrent use, but a stream normally owns one subscription.
type Subscription struct {
	bus    *Bus
	wake   chan struct{}
	start  uint64
	resync bool
	cursor uint64 // guarded by bus.mu
}

func (s *Subscription) poke() {
	select {
	case s.wake <- struct{}{}:
	default: // already pending; Take collects everything
	}
}

// Wake receives a value when something changed since the last [Subscription.Take].
// The Bus never closes it.
func (s *Subscription) Wake() <-chan struct{} { return s.wake }

// Take returns the types that changed since the last Take (or since the
// starting position) and moves the subscription to the current position.
// Several writes to one type appear once.
func (s *Subscription) Take() Batch {
	b := s.bus
	b.mu.Lock()
	defer b.mu.Unlock()
	var out Batch
	for typ, seq := range b.typeSeq {
		if seq > s.cursor {
			out.Types = append(out.Types, typ)
		}
	}
	slices.Sort(out.Types)
	out.Relation = b.relSeq > s.cursor
	for typ, seq := range b.relType {
		if seq > s.cursor {
			out.RelationTypes = append(out.RelationTypes, typ)
		}
	}
	slices.Sort(out.RelationTypes)
	s.cursor = b.head
	out.Seq = s.cursor
	return out
}

// Start is the position the subscription began at: the resumed position, or
// the current one for an empty or unusable token.
func (s *Subscription) Start() uint64 { return s.start }

// Resync reports that a token was presented but could not be honored: it was
// malformed, minted by another process, or points past the current position.
// The consumer cannot know what it missed and must tell its client to refetch.
func (s *Subscription) Resync() bool { return s.resync }

// Token returns an opaque resume token for seq. Two calls for one seq return
// different strings. Token accepts any seq: the caller must pass a position
// whose changes it has already delivered, or a client resuming from the
// token skips them.
func (s *Subscription) Token(seq uint64) string { return s.bus.seal.seal(seq) }

// Close detaches the subscription from the Bus.
func (s *Subscription) Close() {
	s.bus.mu.Lock()
	delete(s.bus.subs, s)
	s.bus.mu.Unlock()
}
