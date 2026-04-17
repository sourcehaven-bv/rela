package fsstore

import "github.com/Sourcehaven-BV/rela/internal/store"

// Subscribe registers a new event subscriber with the given buffer size.
// Events are delivered on a best-effort basis: if the subscriber's channel
// is full, events are dropped silently.
func (s *FSStore) Subscribe(bufSize int) (<-chan store.Event, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan store.Event, bufSize)
	id := s.nextSubID
	s.nextSubID++
	s.subscribers[id] = ch

	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if _, ok := s.subscribers[id]; ok {
			delete(s.subscribers, id)
			close(ch)
		}
	}
	return ch, cancel
}

// emit sends an event to all subscribers. Non-blocking: drops if full.
// Must be called under mu.Lock.
func (s *FSStore) emit(ev store.Event) {
	for _, ch := range s.subscribers {
		select {
		case ch <- ev:
		default:
		}
	}
}

// Close shuts down the store, persists the index, and closes all subscriber channels.
func (s *FSStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Persist index and prop cache for fast startup next time.
	_ = s.savePersistedIndex()

	for id, ch := range s.subscribers {
		close(ch)
		delete(s.subscribers, id)
	}
	return s.searchIndex.Close()
}
