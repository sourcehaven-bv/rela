// Package memcomments is an in-memory [comments.Store] for tests and the
// memory build. Nothing survives process exit.
package memcomments

import (
	"context"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/comments"
)

// Store keeps every target's thread in a map.
//
// A single mutex guards the whole map rather than one per target: the map
// itself is mutated by Rename and DeleteTarget, and a per-target lock would
// not cover that. Contention is irrelevant at the scale this backend serves.
type Store struct {
	mu     sync.Mutex
	byward map[string][]comments.Comment
}

// New returns an empty store.
func New() *Store {
	return &Store{byward: map[string][]comments.Comment{}}
}

var _ comments.Store = (*Store)(nil)

// List returns the target's comments in the contract order, as a copy.
//
// The copy is load-bearing: handing out the backing slice would let a caller
// mutate stored comments in place, which no persistent backend permits — so
// tests passing against this one would fail against those.
func (s *Store) List(_ context.Context, target comments.Target) ([]comments.Comment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored := s.byward[target.ID]
	out := make([]comments.Comment, len(stored))
	copy(out, stored)
	comments.SortComments(out)
	return out, nil
}

// Add appends a comment to the target's thread.
func (s *Store) Add(_ context.Context, target comments.Target, c comments.Comment) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.byward[target.ID] = append(s.byward[target.ID], c)
	return nil
}

// Update replaces the mutable fields of one comment.
func (s *Store) Update(_ context.Context, target comments.Target, id, body string, resolved bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored := s.byward[target.ID]
	for i := range stored {
		if stored[i].ID != id {
			continue
		}
		stored[i].Body = body
		stored[i].Resolved = resolved
		return nil
	}
	return comments.ErrNotFound
}

// Delete removes one comment.
func (s *Store) Delete(_ context.Context, target comments.Target, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored := s.byward[target.ID]
	for i := range stored {
		if stored[i].ID != id {
			continue
		}
		s.byward[target.ID] = append(stored[:i:i], stored[i+1:]...)
		if len(s.byward[target.ID]) == 0 {
			delete(s.byward, target.ID)
		}
		return nil
	}
	return comments.ErrNotFound
}

// DeleteTarget drops a target's whole thread.
func (s *Store) DeleteTarget(_ context.Context, target comments.Target) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.byward, target.ID)
	return nil
}

// Rename re-keys a target's thread, appending to any thread already filed
// under newID rather than replacing it — ID reuse means the destination is not
// guaranteed empty, and silently discarding the occupant's comments would be
// worse than merging.
func (s *Store) Rename(_ context.Context, oldID, newID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored, ok := s.byward[oldID]
	if !ok {
		return nil
	}
	delete(s.byward, oldID)
	s.byward[newID] = append(s.byward[newID], stored...)
	return nil
}
