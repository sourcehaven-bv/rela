// Package memcomments is an in-memory [comments.Store] for tests and the
// memory build. Nothing survives process exit.
package memcomments

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/entity"
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

	stored := s.byward[target.Key()]
	out := make([]comments.Comment, len(stored))
	copy(out, stored)
	comments.SortComments(out)
	return out, nil
}

// Add appends a comment to the target's thread.
func (s *Store) Add(_ context.Context, target comments.Target, c comments.Comment) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.byward[target.Key()] = append(s.byward[target.Key()], c)
	return nil
}

// Update replaces the mutable fields of one comment.
func (s *Store) Update(_ context.Context, target comments.Target, id, body string, resolved bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored := s.byward[target.Key()]
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

	stored := s.byward[target.Key()]
	for i := range stored {
		if stored[i].ID != id {
			continue
		}
		s.byward[target.Key()] = append(stored[:i:i], stored[i+1:]...)
		if len(s.byward[target.Key()]) == 0 {
			delete(s.byward, target.Key())
		}
		return nil
	}
	return comments.ErrNotFound
}

// DeleteTarget drops a target's whole thread.
func (s *Store) DeleteTarget(_ context.Context, target comments.Target) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.byward, target.Key())
	return nil
}

// DeleteAllFaces removes every face's thread for an entity id.
func (s *Store) DeleteAllFaces(_ context.Context, entityID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, key := range faceKeysFor(s.byward, entityID) {
		delete(s.byward, key)
	}
	return nil
}

// Rename re-keys a target's thread, appending to any thread already filed
// under newID rather than replacing it — ID reuse means the destination is not
// guaranteed empty, and silently discarding the occupant's comments would be
// worse than merging.
func (s *Store) Rename(_ context.Context, oldID, newID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if oldID == newID {
		return nil
	}
	// Move EVERY face's thread, not just the bare id: an entity with a draft
	// and a published face keeps a thread per face, and re-keying only the
	// default one would strand the rest at an id that no longer exists.
	for _, key := range faceKeysFor(s.byward, oldID) {
		stored := s.byward[key]
		delete(s.byward, key)
		dest := reKey(key, oldID, newID)
		s.byward[dest] = append(s.byward[dest], stored...)
		comments.SortComments(s.byward[dest])
	}
	return nil
}

// faceKeysFor returns every thread key belonging to id — the bare id plus any
// "id@face" — so a rename or delete covers all of an entity's faces.
func faceKeysFor[V any](m map[string]V, id string) []string {
	var out []string
	for k := range m {
		if k == id || strings.HasPrefix(k, id+entity.StateRefSeparator) {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// reKey swaps the id half of a thread key, preserving the face.
func reKey(key, oldID, newID string) string {
	if key == oldID {
		return newID
	}
	return newID + strings.TrimPrefix(key, oldID)
}
