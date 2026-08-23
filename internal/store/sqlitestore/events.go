package sqlitestore

import (
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Subscribe implements store.Watcher. Events are sent without holding any
// write lock and dropped when a subscriber's buffer is full, per the contract
// in store.go:969-975.
func (s *Store) Subscribe(bufSize int) (events <-chan store.Event, cancel func()) {
	root := s.root()
	root.subMu.Lock()
	defer root.subMu.Unlock()

	if root.subs == nil {
		root.subs = map[int]chan store.Event{}
	}
	id := root.next
	root.next++
	ch := make(chan store.Event, bufSize)
	root.subs[id] = ch

	return ch, func() {
		root.subMu.Lock()
		defer root.subMu.Unlock()
		if c, ok := root.subs[id]; ok {
			delete(root.subs, id)
			close(c)
		}
	}
}

// root returns the store owning the subscriber registry — a view delegates to
// its parent so events raised inside a Tx reach subscribers registered
// outside it.
func (s *Store) root() *Store {
	if s.parent != nil {
		return s.parent
	}
	return s
}

// emit buffers an event inside a transaction and publishes it immediately
// otherwise.
//
// Buffering is unconditional rather than configurable: this backend takes the
// strong Tx contract, which requires that no event escape for a transaction
// that later rolls back.
//
// It must never be called while holding a lock that a subscriber could
// contend on — the contract forbids emitting under a store lock.
func (s *Store) emit(ev store.Event) {
	if s.txPending != nil {
		s.txPending.mu.Lock()
		s.txPending.events = append(s.txPending.events, ev)
		s.txPending.mu.Unlock()
		return
	}
	s.publish(ev)
}

// publish does the non-blocking fan-out. A full buffer drops the event.
func (s *Store) publish(ev store.Event) {
	root := s.root()
	root.subMu.Lock()
	chans := make([]chan store.Event, 0, len(root.subs))
	for _, ch := range root.subs {
		chans = append(chans, ch)
	}
	root.subMu.Unlock()

	// Send outside the lock so a slow subscriber cannot stall a writer.
	for _, ch := range chans {
		select {
		case ch <- ev:
		default: // drop, per contract
		}
	}
}
