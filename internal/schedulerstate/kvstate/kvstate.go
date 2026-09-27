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
//
// # for_each subjects live outside the document
//
// A fan-out run has up to 10,000 subjects, and each one is claimed and settled
// separately. Were the subjects part of the document, every settle would
// rewrite all of them, so a run would cost writes quadratic in its size, all
// behind the mutex the scheduler's tick also takes. Each subject is instead
// one small key, written when it settles, plus one index key per run listing
// its subjects. The document keeps only the run's counters.
package kvstate

import (
	"context"
	"encoding/hex"
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

// childKeyPrefix holds the per-run subject keys: "<prefix><run>.json" is the
// run's subject index and "<prefix><run>-<subject>" one subject's state. Both
// ids are hex-encoded, so any id makes a valid key and no id can reach
// outside the prefix.
const childKeyPrefix = "scheduler-run-children/"

// leaseStep is the smallest lease extension StartChild writes. Children start
// in bursts, and writing the document for each would put a whole-document
// write back on every subject.
const leaseStep = time.Minute

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

	// dropped lists deleted runs whose subject keys are still to be removed.
	dropped []string
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
}

// subjectState is one for_each subject's progress within a run.
type subjectState string

const (
	subjectPending   subjectState = "pending"
	subjectSucceeded subjectState = "succeeded"
	subjectFailed    subjectState = "failed"
)

func indexKey(run string) string { return childKeyPrefix + hex.EncodeToString([]byte(run)) + ".json" }

func subjectKey(run, subject string) string {
	return childKeyPrefix + hex.EncodeToString([]byte(run)) + "-" + hex.EncodeToString([]byte(subject))
}

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
	err := s.view(ctx, func(doc *document) error {
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
		return nil
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
		if st.LastRun.IsZero() && st.NextRetry.IsZero() {
			// Nothing to carry over, and no time to date the record by.
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

// StartChild implements [schedulerstate.Store].
func (s *Store) StartChild(ctx context.Context, id, subject string, leaseUntil time.Time) (bool, error) {
	claimed := false
	err := s.mutate(ctx, func(doc *document) (bool, error) {
		rec, ok := doc.Runs[id]
		if !ok {
			return false, schedulerstate.ErrNoRun
		}
		if !rec.Status.Active() {
			return false, nil
		}
		st, err := s.subject(ctx, id, subject)
		if err != nil || st != subjectPending {
			return false, err
		}
		claimed = true
		if !leaseUntil.After(rec.LeaseUntil.Add(leaseStep)) {
			return false, nil
		}
		rec.LeaseUntil = leaseUntil
		return true, nil
	})
	return claimed, err
}

// ExpectChildren implements [schedulerstate.Store].
func (s *Store) ExpectChildren(ctx context.Context, id string, subjects []string) error {
	return s.mutate(ctx, func(doc *document) (bool, error) {
		rec, ok := doc.Runs[id]
		if !ok {
			return false, schedulerstate.ErrNoRun
		}
		index, err := s.index(ctx, id)
		if err != nil {
			return false, err
		}
		known := make(map[string]bool, len(index))
		for _, subject := range index {
			known[subject] = true
		}
		added := 0
		for _, subject := range subjects {
			if known[subject] {
				continue
			}
			known[subject] = true
			index = append(index, subject)
			if putErr := s.putSubject(ctx, id, subject, subjectPending); putErr != nil {
				return false, putErr
			}
			added++
		}
		if added == 0 {
			return false, nil
		}
		data, err := json.Marshal(index)
		if err != nil {
			return false, fmt.Errorf("kvstate: marshal subjects of %q: %w", id, err)
		}
		if err := s.kv.Put(ctx, indexKey(id), data); err != nil {
			return false, fmt.Errorf("kvstate: write subjects of %q: %w", id, err)
		}
		rec.Children += added
		return true, nil
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
		current, err := s.subject(ctx, id, subject)
		if err != nil || current != subjectPending {
			return false, err
		}
		final := subjectSucceeded
		if out.Error != "" {
			final = subjectFailed
		}
		if err := s.putSubject(ctx, id, subject, final); err != nil {
			return false, err
		}
		settled = true
		rec.ChildrenSettled++
		if out.Error != "" {
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
	err := s.view(ctx, func(doc *document) error {
		for _, rec := range doc.Runs {
			if rec.Task != task || rec.Occurrence != occurrence || rec.Children == 0 {
				continue
			}
			index, err := s.index(ctx, rec.ID)
			if err != nil {
				return err
			}
			for _, subject := range index {
				st, err := s.subject(ctx, rec.ID, subject)
				if err != nil {
					return err
				}
				if st == subjectSucceeded {
					out = append(out, subject)
				}
			}
		}
		return nil
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
			if rec.Status != schedulerstate.RunAbandoned || out.Error != "" {
				return false, nil
			}
			ts, changed := schedulerstate.ApplyOutcome(doc.Tasks[rec.Task].state(), rec.CreatedAt, out, policy)
			if changed {
				doc.Tasks[rec.Task] = recordOf(ts, out.At)
				fin.LateSuccess = true
			}
			return changed, nil
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
		for name, rec := range doc.Tasks {
			if rec.Touched.Before(before) && doc.activeRun(name) == nil {
				removed = append(removed, name)
				delete(doc.Tasks, name)
				changed = true
			}
		}
		for id, rec := range doc.Runs {
			if rec.Status.Active() {
				continue
			}
			_, taskKept := doc.Tasks[rec.Task]
			if rec.FinishedAt.Before(before) || !taskKept {
				doc.dropRun(id)
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
		doc.dropRun(rec.ID)
	}
}

// dropRun deletes a run from the document and queues its subject keys for
// deletion once the document is written.
func (doc *document) dropRun(id string) {
	if doc.Runs[id].Children > 0 {
		doc.dropped = append(doc.dropped, id)
	}
	delete(doc.Runs, id)
}

func latest(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

// view runs fn over a freshly-read document without writing.
func (s *Store) view(ctx context.Context, fn func(*document) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return schedulerstate.ErrClosed
	}
	doc, err := s.read(ctx)
	if err != nil {
		return err
	}
	return fn(doc)
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
	if err := s.write(ctx, doc); err != nil {
		return err
	}
	s.deleteChildren(ctx, doc.dropped)
	return nil
}

// deleteChildren removes the subject keys of runs already deleted from the
// document. Best effort: a key left behind is never read again, because
// every read of a subject goes through a run the document still holds.
func (s *Store) deleteChildren(ctx context.Context, runs []string) {
	for _, id := range runs {
		index, err := s.index(ctx, id)
		if err != nil {
			continue
		}
		for _, subject := range index {
			_ = s.kv.Delete(ctx, subjectKey(id, subject))
		}
		_ = s.kv.Delete(ctx, indexKey(id))
	}
}

// index returns the subjects recorded for run, or none.
func (s *Store) index(ctx context.Context, run string) ([]string, error) {
	data, err := s.kv.Get(ctx, indexKey(run))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("kvstate: read subjects of %q: %w", run, err)
	}
	var index []string
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("kvstate: decode subjects of %q: %w", run, err)
	}
	return index, nil
}

// subject returns one subject's state within run. A subject the run does not
// expect is an error: it would corrupt the run's counters.
func (s *Store) subject(ctx context.Context, run, subject string) (subjectState, error) {
	data, err := s.kv.Get(ctx, subjectKey(run, subject))
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("kvstate: run %q does not expect subject %q", run, subject)
		}
		return "", fmt.Errorf("kvstate: read subject %q of %q: %w", subject, run, err)
	}
	return subjectState(data), nil
}

func (s *Store) putSubject(ctx context.Context, run, subject string, st subjectState) error {
	if err := s.kv.Put(ctx, subjectKey(run, subject), []byte(st)); err != nil {
		return fmt.Errorf("kvstate: write subject %q of %q: %w", subject, run, err)
	}
	return nil
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
