// Package memmigstate keeps migration state in memory.
//
// It backs the memory build tier and gives tests a [datamigration.StateStore]
// with no filesystem or database. State is lost with the process, which is
// correct for a tier whose entity store is equally ephemeral.
package memmigstate

import (
	"context"
	"errors"
	"slices"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/datamigration"
)

// Store is an in-memory [datamigration.StateStore]. The zero value is ready
// to use, and it is safe for concurrent use.
type Store struct {
	mu    sync.Mutex
	state *datamigration.State
}

// New returns an empty store.
func New() *Store { return &Store{} }

// Load returns a deep copy of the stored state, or (nil, nil) when nothing has
// been saved.
//
// The copy matters: handing out the stored pointer would let a caller mutate
// this store's state by editing what it read, which no other backend allows.
// A test that passed against this backend and failed against postgres would be
// the result.
func (s *Store) Load(_ context.Context) (*datamigration.State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == nil {
		return nil, nil //nolint:nilnil // (nil, nil) = un-bootstrapped, the documented StateStore contract
	}
	return cloneState(s.state), nil
}

// Save replaces the stored state with a deep copy of st.
func (s *Store) Save(_ context.Context, st *datamigration.State) error {
	if st == nil {
		return errors.New("memmigstate: Save requires a state")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = cloneState(st)
	return nil
}

func cloneState(s *datamigration.State) *datamigration.State {
	out := *s
	out.Applied = slices.Clone(s.Applied)
	out.Projection = slices.Clone(s.Projection)
	return &out
}

// Interface check.
var _ datamigration.StateStore = (*Store)(nil)
