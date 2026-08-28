// Package kvstate is the [schedulerstate.Store] backend for the filesystem and
// desktop tiers, persisting through a [state.KV].
//
// # One document, and why that is still right here
//
// All of a project's run-state lives in a single JSON value — the same shape
// the scheduler used before this package existed. That looks like the very
// thing schedulerstate was created to escape, so the distinction matters:
//
// The problem with the old design was not the document. It was that the
// document was read ONCE at startup and then rewritten from an in-memory
// snapshot, so a second process overwrote the first's tasks wholesale. This
// backend re-reads before every write and applies the change under a mutex, so
// a write about one task cannot carry a stale copy of another.
//
// # Single-process only
//
// Two PROCESSES sharing one project directory can still interleave a read and
// a write and lose an update. That is not fixed here and does not need to be:
// the filesystem tier is a single-user desktop or CLI deployment, single-writer
// by nature. The postgres backend exists for the multi-process deployment,
// where each record is written independently.
//
// Stated plainly rather than left to be discovered, exactly as
// kvuserstate does for the same trade.
package kvstate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/schedulerstate"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

// StateKey is where the document lives inside the KV.
//
// Deliberately NOT the legacy "scheduler-state.json": that key holds the old
// three-parallel-maps layout, and an old binary rolled back onto a project
// must not silently read a document written in the new shape. See Import.
const StateKey = "scheduler-run-state.json"

// LegacyStateKey is the document the scheduler wrote before this package.
const LegacyStateKey = "scheduler-state.json"

// Store is the KV-backed [schedulerstate.Store].
type Store struct {
	kv state.KV

	// mu serializes read-modify-write within this process. It is NOT a
	// substitute for cross-process safety — see the package doc.
	mu     sync.Mutex
	closed bool
}

// New returns a Store over kv.
//
// Nil: rejected — a nil KV would make every task look permanently due, which
// surfaces as duplicated work rather than as a wiring error.
func New(kv state.KV) (*Store, error) {
	if kv == nil {
		return nil, errors.New("kvstate: state store must not be nil")
	}
	return &Store{kv: kv}, nil
}

// document is the on-disk shape: one record per task, keyed by name.
//
// A map of structs rather than the legacy three parallel maps — the old layout
// existed for backward compatibility with a file that predated retry state,
// which Import now handles instead.
type document struct {
	Tasks map[string]record `json:"tasks"`
}

type record struct {
	LastRun   time.Time `json:"last_run,omitzero"`
	Failures  int       `json:"failures,omitempty"`
	NextRetry time.Time `json:"next_retry,omitzero"`
}

func (r record) runState() schedulerstate.RunState {
	return schedulerstate.RunState{LastRun: r.LastRun, Failures: r.Failures, NextRetry: r.NextRetry}
}

// Load implements [schedulerstate.Store].
func (s *Store) Load(ctx context.Context, tasks []string) (map[string]schedulerstate.RunState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, schedulerstate.ErrClosed
	}

	doc, err := s.read(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]schedulerstate.RunState, len(tasks))
	for _, name := range tasks {
		if rec, ok := doc.Tasks[name]; ok {
			out[name] = rec.runState()
		}
	}
	return out, nil
}

// RecordSuccess implements [schedulerstate.Store].
func (s *Store) RecordSuccess(ctx context.Context, task string, start time.Time) error {
	return s.mutate(ctx, task, func(doc *document) {
		if rec, ok := doc.Tasks[task]; ok && !rec.LastRun.Before(start) {
			// A newer success is already recorded; a stale writer must not
			// regress it.
			return
		}
		// Clearing the ladder is the ONLY reset, and it happens with the
		// stamp rather than after it: a success that left NextRetry set
		// would keep the task ladder-driven forever.
		doc.Tasks[task] = record{LastRun: start}
	})
}

// RecordFailure implements [schedulerstate.Store].
func (s *Store) RecordFailure(ctx context.Context, task string, start time.Time) (int, error) {
	var failures int
	err := s.mutate(ctx, task, func(doc *document) {
		rec := doc.Tasks[task]
		if !rec.LastRun.IsZero() && rec.LastRun.After(start) {
			// This attempt began before a success that is already recorded.
			// Report the stored count without resurrecting a ladder.
			failures = rec.Failures
			return
		}
		rec.Failures++
		failures = rec.Failures
		doc.Tasks[task] = rec
	})
	if err != nil {
		return 0, err
	}
	return failures, nil
}

// SetNextRetry implements [schedulerstate.Store].
func (s *Store) SetNextRetry(ctx context.Context, task string, start, retryAt time.Time) error {
	return s.mutate(ctx, task, func(doc *document) {
		rec := doc.Tasks[task]
		if !rec.LastRun.IsZero() && rec.LastRun.After(start) {
			return
		}
		rec.NextRetry = retryAt
		doc.Tasks[task] = rec
	})
}

// Prune implements [schedulerstate.Store].
func (s *Store) Prune(ctx context.Context, before time.Time) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, schedulerstate.ErrClosed
	}

	doc, err := s.read(ctx)
	if err != nil {
		return nil, err
	}
	var removed []string
	for name, rec := range doc.Tasks {
		if touched(rec).Before(before) {
			removed = append(removed, name)
			delete(doc.Tasks, name)
		}
	}
	if len(removed) == 0 {
		return nil, nil
	}
	slices.Sort(removed)
	if err := s.write(ctx, doc); err != nil {
		return nil, err
	}
	return removed, nil
}

// touched is a record's last activity: the later of its run and its pending
// retry, so a task mid-ladder is not pruned merely because it has not
// succeeded recently.
func touched(r record) time.Time {
	if r.NextRetry.After(r.LastRun) {
		return r.NextRetry
	}
	return r.LastRun
}

// Close implements [schedulerstate.Store]. Idempotent.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// mutate applies fn to a freshly-read document and writes the result.
//
// Re-reading rather than holding an in-memory snapshot is the whole difference
// from the layout this replaced: a write can no longer carry a stale copy of
// another task's state.
func (s *Store) mutate(ctx context.Context, task string, fn func(*document)) error {
	if task == "" {
		return schedulerstate.ErrNoTask
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return schedulerstate.ErrClosed
	}

	doc, err := s.read(ctx)
	if err != nil {
		return err
	}
	before := doc.Tasks[task]
	fn(doc)
	if doc.Tasks[task] == before {
		// A guarded no-op: skip the write rather than rewriting an
		// identical document.
		return nil
	}
	return s.write(ctx, doc)
}

// read returns the stored document, or an empty one when the key is absent.
func (s *Store) read(ctx context.Context) (*document, error) {
	data, err := s.kv.Get(ctx, StateKey)
	if err != nil {
		if os.IsNotExist(err) {
			return &document{Tasks: map[string]record{}}, nil
		}
		return nil, fmt.Errorf("kvstate: read state: %w", err)
	}
	var doc document
	//nolint:nilerr // a corrupt document is recovered as empty, not propagated: see below.
	if err := json.Unmarshal(data, &doc); err != nil {
		// A corrupt document is treated as empty rather than fatal: the
		// scheduler must still start, and the cost is re-running tasks
		// once. This matches the behavior the legacy parser had.
		return &document{Tasks: map[string]record{}}, nil
	}
	if doc.Tasks == nil {
		doc.Tasks = map[string]record{}
	}
	return &doc, nil
}

func (s *Store) write(ctx context.Context, doc *document) error {
	data, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("kvstate: marshal state: %w", err)
	}
	if err := s.kv.Put(ctx, StateKey, data); err != nil {
		return fmt.Errorf("kvstate: write state: %w", err)
	}
	return nil
}
