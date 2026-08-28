// Package memuserstate is the in-memory [userstate.Store] backend.
//
// Used by the memorybackend build and by tests. State is per-process and
// lost on restart, which is acceptable for this data: a lost snooze costs a
// user one repeated suggestion.
package memuserstate

import (
	"context"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/userstate"
)

// Store is an in-memory [userstate.Store]. Safe for concurrent use.
type Store struct {
	mu     sync.RWMutex
	closed bool

	snoozes map[userstate.Key]time.Time
	shown   map[userstate.Key]time.Time
	// muted is keyed by user, then source, so MutedSources is a map walk
	// rather than a scan over every user's entries.
	muted map[string]map[string]bool
}

// New returns an empty in-memory store.
func New() *Store {
	return &Store{
		snoozes: make(map[userstate.Key]time.Time),
		shown:   make(map[userstate.Key]time.Time),
		muted:   make(map[string]map[string]bool),
	}
}

// compile-time check
var _ userstate.Store = (*Store)(nil)

func (s *Store) SnoozedUntil(
	_ context.Context, key userstate.Key, now time.Time,
) (time.Time, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return time.Time{}, false, userstate.ErrClosed
	}
	until, ok := s.snoozes[key]
	// Expiry is judged at READ time so correctness never depends on Prune
	// having run. `now` is exclusive-of-equal: a snooze "until T" is over at
	// T, matching how a deadline reads to a human.
	if !ok || !now.Before(until) {
		return time.Time{}, false, nil
	}
	return until, true, nil
}

func (s *Store) SetSnooze(_ context.Context, key userstate.Key, until time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return userstate.ErrClosed
	}
	// Replace, never stack: a later "not for a week" must win over an
	// earlier "not for an hour".
	s.snoozes[key] = until
	return nil
}

func (s *Store) Muted(_ context.Context, user, source string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return false, userstate.ErrClosed
	}
	return s.muted[user][source], nil
}

func (s *Store) SetMuted(_ context.Context, user, source string, muted bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return userstate.ErrClosed
	}
	if !muted {
		delete(s.muted[user], source)
		if len(s.muted[user]) == 0 {
			delete(s.muted, user)
		}
		return nil
	}
	if s.muted[user] == nil {
		s.muted[user] = make(map[string]bool)
	}
	s.muted[user][source] = true
	return nil
}

func (s *Store) MutedSources(_ context.Context, user string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, userstate.ErrClosed
	}
	return slices.Sorted(maps.Keys(s.muted[user])), nil
}

func (s *Store) LastShown(_ context.Context, key userstate.Key) (time.Time, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return time.Time{}, false, userstate.ErrClosed
	}
	at, ok := s.shown[key]
	return at, ok, nil
}

func (s *Store) MarkShown(_ context.Context, key userstate.Key, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return userstate.ErrClosed
	}
	s.shown[key] = at
	return nil
}

func (s *Store) Prune(_ context.Context, now time.Time, keepShown time.Duration) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, userstate.ErrClosed
	}
	removed := 0
	for k, until := range s.snoozes {
		if !now.Before(until) {
			delete(s.snoozes, k)
			removed++
		}
	}
	cutoff := now.Add(-keepShown)
	for k, at := range s.shown {
		if at.Before(cutoff) {
			delete(s.shown, k)
			removed++
		}
	}
	return removed, nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.snoozes = nil
	s.shown = nil
	s.muted = nil
	return nil
}
