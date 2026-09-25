// Package kvstate is the [schedulerstate.Store] backend for the filesystem and
// desktop tiers, persisting through a [state.KV].
//
// # One document, and why that is still right here
//
// All of a project's run-state lives in a single JSON value: every task's
// record and its recent runs. That looks like the very
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
	"maps"
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
// must not silently read a document written in the new shape. The scheduler
// imports the legacy document once through [Store.Seed] and then deletes it.
const StateKey = "scheduler-run-state.json"

// keepEndedRuns is how many ended runs are kept per task.
//
// The document is rewritten whole on every write, so its size is a cost paid
// per run: a one-minute task would otherwise add 1,440 runs a day until Prune.
// The latest few are all an operator needs to see why a task is failing.
const keepEndedRuns = 10

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

// compile-time check
var _ schedulerstate.Store = (*Store)(nil)

// document is the stored shape.
type document struct {
	Tasks map[string]taskRecord `json:"tasks"`
	Runs  map[string]*runRecord `json:"runs,omitempty"`
}

type taskRecord struct {
	LastRun   time.Time `json:"last_run,omitzero"`
	Failures  int       `json:"failures,omitempty"`
	NextRetry time.Time `json:"next_retry,omitzero"`
	Version   int64     `json:"version,omitempty"`
	// Touched is when the record last changed, for Prune.
	Touched time.Time `json:"touched,omitzero"`
}

type runRecord struct {
	schedulerstate.Run
	// Subjects maps each for_each subject to where it is.
	Subjects map[string]subjectState `json:"subjects,omitempty"`
}

// subjectState is one for_each subject's progress within a run.
type subjectState string

const (
	subjectPending   subjectState = "pending"
	subjectSucceeded subjectState = "succeeded"
	subjectFailed    subjectState = "failed"
)

func (r taskRecord) state() schedulerstate.TaskState {
	return schedulerstate.TaskState{
		LastRun: r.LastRun, Failures: r.Failures, NextRetry: r.NextRetry, Version: r.Version,
	}
}

func recordOf(ts schedulerstate.TaskState, touched time.Time) taskRecord {
	return taskRecord{
		LastRun: ts.LastRun, Failures: ts.Failures, NextRetry: ts.NextRetry,
		Version: ts.Version, Touched: touched,
	}
}

// Load implements [schedulerstate.Store].
func (s *Store) Load(ctx context.Context, tasks []string) (map[string]schedulerstate.TaskState, error) {
	out := make(map[string]schedulerstate.TaskState, len(tasks))
	err := s.view(ctx, func(doc *document) {
		for _, name := range tasks {
			rec, ok := doc.Tasks[name]
			active := doc.activeRun(name)
			if !ok && active == nil {
				continue
			}
			ts := rec.state()
			if active != nil {
				run := active.Run
				ts.Active = &run
			}
			out[name] = ts
		}
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Seed implements [schedulerstate.Store].
func (s *Store) Seed(ctx context.Context, task string, st schedulerstate.TaskState) error {
	if task == "" {
		return schedulerstate.ErrNoTask
	}
	return s.mutate(ctx, func(doc *document) (bool, error) {
		if _, ok := doc.Tasks[task]; ok {
			return false, nil
		}
		st.Version = 0
		doc.Tasks[task] = recordOf(st, latest(st.LastRun, st.NextRetry))
		return true, nil
	})
}

// CreateRun implements [schedulerstate.Store].
func (s *Store) CreateRun(ctx context.Context, run schedulerstate.Run, expectVersion int64) error {
	if run.Task == "" {
		return schedulerstate.ErrNoTask
	}
	if run.ID == "" {
		return schedulerstate.ErrNoRun
	}
	return s.mutate(ctx, func(doc *document) (bool, error) {
		if doc.activeRun(run.Task) != nil {
			return false, schedulerstate.ErrRunActive
		}
		if doc.Tasks[run.Task].Version != expectVersion {
			return false, schedulerstate.ErrStale
		}
		if _, exists := doc.Runs[run.ID]; exists {
			return false, fmt.Errorf("kvstate: run %q already exists", run.ID)
		}
		run.Status = schedulerstate.RunQueued
		doc.Runs[run.ID] = &runRecord{Run: run}
		return true, nil
	})
}

// StartRun implements [schedulerstate.Store].
func (s *Store) StartRun(
	ctx context.Context, id, node string, now, leaseUntil time.Time,
) (schedulerstate.Run, error) {
	var out schedulerstate.Run
	err := s.mutate(ctx, func(doc *document) (bool, error) {
		rec, ok := doc.Runs[id]
		if !ok {
			return false, schedulerstate.ErrNoRun
		}
		if rec.Status != schedulerstate.RunQueued {
			out = rec.Run
			return false, schedulerstate.ErrNotQueued
		}
		rec.Status = schedulerstate.RunRunning
		rec.StartedAt = now
		rec.Node = node
		rec.LeaseUntil = leaseUntil
		out = rec.Run
		return true, nil
	})
	return out, err
}

// ExtendLease implements [schedulerstate.Store].
func (s *Store) ExtendLease(ctx context.Context, id string, leaseUntil time.Time) error {
	return s.mutate(ctx, func(doc *document) (bool, error) {
		rec, ok := doc.Runs[id]
		if !ok {
			return false, schedulerstate.ErrNoRun
		}
		if !rec.Status.Active() || !leaseUntil.After(rec.LeaseUntil) {
			return false, nil
		}
		rec.LeaseUntil = leaseUntil
		return true, nil
	})
}

// ExpectChildren implements [schedulerstate.Store].
func (s *Store) ExpectChildren(ctx context.Context, id string, subjects []string) error {
	return s.mutate(ctx, func(doc *document) (bool, error) {
		rec, ok := doc.Runs[id]
		if !ok {
			return false, schedulerstate.ErrNoRun
		}
		if rec.Subjects == nil {
			rec.Subjects = make(map[string]subjectState, len(subjects))
		}
		added := false
		for _, subject := range subjects {
			if _, known := rec.Subjects[subject]; known {
				continue
			}
			rec.Subjects[subject] = subjectPending
			rec.Children++
			added = true
		}
		return added, nil
	})
}

// SettleChild implements [schedulerstate.Store].
func (s *Store) SettleChild(
	ctx context.Context, id, subject string, out schedulerstate.Outcome, policy schedulerstate.RetryPolicy,
) (bool, *schedulerstate.Finished, error) {
	var (
		settled bool
		done    *schedulerstate.Finished
	)
	err := s.mutate(ctx, func(doc *document) (bool, error) {
		rec, ok := doc.Runs[id]
		if !ok {
			return false, schedulerstate.ErrNoRun
		}
		current, known := rec.Subjects[subject]
		if !known {
			return false, fmt.Errorf("kvstate: run %q does not expect subject %q", id, subject)
		}
		if current != subjectPending {
			return false, nil
		}
		settled = true
		rec.Subjects[subject] = subjectSucceeded
		rec.ChildrenSettled++
		if out.Error != "" {
			rec.Subjects[subject] = subjectFailed
			rec.ChildrenFailed++
			if rec.Error == "" {
				rec.Error = out.Error
			}
		}
		if rec.ChildrenSettled == rec.Children && rec.Status.Active() {
			fin := doc.finish(rec, schedulerstate.ChildOutcome(rec.Run, rec.Error, out.At), rec.Status, policy)
			done = &fin
		}
		return true, nil
	})
	if err != nil {
		return false, nil, err
	}
	return settled, done, nil
}

// SucceededSubjects implements [schedulerstate.Store].
func (s *Store) SucceededSubjects(ctx context.Context, task, occurrence string) ([]string, error) {
	var out []string
	err := s.view(ctx, func(doc *document) {
		for _, rec := range doc.Runs {
			if rec.Task != task || rec.Occurrence != occurrence {
				continue
			}
			for subject, st := range rec.Subjects {
				if st == subjectSucceeded {
					out = append(out, subject)
				}
			}
		}
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

// FinishRun implements [schedulerstate.Store].
func (s *Store) FinishRun(
	ctx context.Context, id string, out schedulerstate.Outcome, policy schedulerstate.RetryPolicy,
) (schedulerstate.Finished, error) {
	var fin schedulerstate.Finished
	err := s.mutate(ctx, func(doc *document) (bool, error) {
		rec, ok := doc.Runs[id]
		if !ok {
			return false, schedulerstate.ErrNoRun
		}
		if !rec.Status.Active() {
			fin = schedulerstate.Finished{Run: rec.Run}
			return false, nil
		}
		fin = doc.finish(rec, out, rec.Status, policy)
		return true, nil
	})
	return fin, err
}

// Reap implements [schedulerstate.Store].
func (s *Store) Reap(
	ctx context.Context, now time.Time, policy schedulerstate.RetryPolicy,
) ([]schedulerstate.Finished, error) {
	var out []schedulerstate.Finished
	err := s.mutate(ctx, func(doc *document) (bool, error) {
		for _, id := range slices.Sorted(maps.Keys(doc.Runs)) {
			rec := doc.Runs[id]
			if !rec.Status.Active() || !rec.LeaseUntil.Before(now) {
				continue
			}
			msg := fmt.Sprintf("lease expired while %s", rec.Status)
			out = append(out, doc.finish(rec, schedulerstate.Outcome{Error: msg, At: now},
				schedulerstate.RunAbandoned, policy))
		}
		return len(out) > 0, nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Prune implements [schedulerstate.Store].
func (s *Store) Prune(ctx context.Context, before time.Time) ([]string, error) {
	var removed []string
	err := s.mutate(ctx, func(doc *document) (bool, error) {
		changed := false
		for id, rec := range doc.Runs {
			if !rec.Status.Active() && rec.FinishedAt.Before(before) {
				delete(doc.Runs, id)
				changed = true
			}
		}
		for name, rec := range doc.Tasks {
			if rec.Touched.Before(before) && doc.activeRun(name) == nil {
				removed = append(removed, name)
				delete(doc.Tasks, name)
				changed = true
			}
		}
		return changed, nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(removed)
	return removed, nil
}

// Close implements [schedulerstate.Store]. Idempotent.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// finish ends rec with status (the run's own outcome status when it is still
// active, or RunAbandoned) and applies the outcome to its task.
func (doc *document) finish(
	rec *runRecord, out schedulerstate.Outcome, status schedulerstate.RunStatus, policy schedulerstate.RetryPolicy,
) schedulerstate.Finished {
	switch {
	case status == schedulerstate.RunAbandoned:
	case out.Error == "":
		status = schedulerstate.RunSucceeded
	default:
		status = schedulerstate.RunFailed
	}
	rec.Status = status
	rec.FinishedAt = out.At
	rec.Error = out.Error

	ts, changed := schedulerstate.ApplyOutcome(doc.Tasks[rec.Task].state(), rec.CreatedAt, out, policy)
	if changed {
		doc.Tasks[rec.Task] = recordOf(ts, out.At)
	}
	doc.trimEnded(rec.Task)

	fin := schedulerstate.Finished{Run: rec.Run, Applied: true}
	if out.Error != "" {
		fin.Failures = ts.Failures
		fin.NextRetry = ts.NextRetry
	}
	return fin
}

// activeRun returns the task's queued or running run, if any.
func (doc *document) activeRun(task string) *runRecord {
	for _, rec := range doc.Runs {
		if rec.Task == task && rec.Status.Active() {
			return rec
		}
	}
	return nil
}

// trimEnded keeps only the newest keepEndedRuns ended runs of task, plus every
// run of the newest run's for_each occurrence: those record which subjects
// were already delivered, and dropping one would let a retry re-send.
func (doc *document) trimEnded(task string) {
	var ended []*runRecord
	for _, rec := range doc.Runs {
		if rec.Task == task && !rec.Status.Active() {
			ended = append(ended, rec)
		}
	}
	if len(ended) <= keepEndedRuns {
		return
	}
	slices.SortFunc(ended, func(a, b *runRecord) int { return b.FinishedAt.Compare(a.FinishedAt) })
	current := ended[0].Occurrence
	for _, rec := range ended[keepEndedRuns:] {
		if current != "" && rec.Occurrence == current {
			continue
		}
		delete(doc.Runs, rec.ID)
	}
}

func latest(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

// view runs fn over a freshly-read document without writing.
func (s *Store) view(ctx context.Context, fn func(*document)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return schedulerstate.ErrClosed
	}
	doc, err := s.read(ctx)
	if err != nil {
		return err
	}
	fn(doc)
	return nil
}

// mutate applies fn to a freshly-read document and writes the result when fn
// reports a change and no error.
//
// Re-reading rather than holding an in-memory snapshot is the whole difference
// from the layout this replaced: a write can no longer carry a stale copy of
// another task's state.
func (s *Store) mutate(ctx context.Context, fn func(*document) (bool, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return schedulerstate.ErrClosed
	}

	doc, err := s.read(ctx)
	if err != nil {
		return err
	}
	changed, err := fn(doc)
	if err != nil || !changed {
		return err
	}
	return s.write(ctx, doc)
}

// read returns the stored document, or an empty one when the key is absent.
func (s *Store) read(ctx context.Context) (*document, error) {
	data, err := s.kv.Get(ctx, StateKey)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyDocument(), nil
		}
		return nil, fmt.Errorf("kvstate: read state: %w", err)
	}
	var doc document
	//nolint:nilerr // a corrupt document is recovered as empty, not propagated: see below.
	if err := json.Unmarshal(data, &doc); err != nil {
		// A corrupt document is treated as empty rather than fatal: the
		// scheduler must still start, and the cost is re-running tasks
		// once. This matches the behavior the legacy parser had.
		return emptyDocument(), nil
	}
	if doc.Tasks == nil {
		doc.Tasks = map[string]taskRecord{}
	}
	if doc.Runs == nil {
		doc.Runs = map[string]*runRecord{}
	}
	return &doc, nil
}

func emptyDocument() *document {
	return &document{Tasks: map[string]taskRecord{}, Runs: map[string]*runRecord{}}
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
