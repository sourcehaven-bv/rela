// Package kvflowrun is the [flowrun.Store] backend for the single-process
// tiers (fs, memory, desktop): every run in one document in a [state.KV],
// the definitions in a second one.
//
// # Two documents, read-modify-write under a lock
//
// The runs live in one JSON value under [RunsKey] and the definition
// snapshots in another under [DefinitionsKey], so a prune finds every
// snapshot without listing keys, which a state.KV cannot do. Every method
// takes the store's one-slot semaphore and then its [Locker], re-reads
// what it needs, applies its change and writes it back through the KV's
// own write path (an atomic replace on disk). That makes each method
// atomic, as kvlease's are. A create that stores a new definition writes
// the definitions first, so the runs document never names a snapshot that
// is not stored; a failure between the two writes leaves an unused
// snapshot, which the next prune drops.
//
// # Several processes, one machine
//
// The desktop app, `rela mcp`, CLI calls and `rela scheduler` can open one
// project at once. The semaphore only excludes callers of one instance, so
// over an on-disk KV the Locker is a filelock.Locker on a lock file next
// to the documents. Several machines are out of scope; the postgres build
// uses pgflowrun.
//
// # Cost
//
// Every write rewrites the runs document, about half a kilobyte per run,
// so the store holds at most [MaxRuns] runs ([flowrun.ErrFull] beyond).
// Retention keeps only live runs, so that is live runs, not history. This
// is the good-enough tier: a deployment with more live runs uses a
// database backend.
//
// # A damaged document is an error
//
// Unlike leases, a run cannot be rebuilt from anything else: its state is
// where a business process stands. A document that does not decode is an
// error naming the key, never reset, so the operator restores it from a
// backup or removes it on purpose.
package kvflowrun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

const (
	// RunsKey is where the runs document lives inside the KV.
	RunsKey = "flow-runs.json"
	// DefinitionsKey is where the definitions document lives.
	DefinitionsKey = "flow-definitions.json"
	// MaxRuns caps the runs the store holds.
	MaxRuns = 2000
)

// Locker excludes every other writer of the documents for the span of one
// read-modify-write: other processes over the same KV included. Lock
// blocks until the lock is held or ctx ends, and returns the release func.
// A KV no other process can open takes filelock.ProcessPrivate.
type Locker interface {
	Lock(ctx context.Context) (unlock func(), err error)
}

// Store is a [flowrun.Store] over a [state.KV].
type Store struct {
	kv     state.KV
	locker Locker
	// sem is a one-slot semaphore over this instance's callers.
	sem chan struct{}
}

var _ flowrun.Store = (*Store)(nil)

type runsDoc struct {
	Runs map[flowrun.RunID]storedRun `json:"runs"`
}

type storedRun struct {
	Flow        string          `json:"flow"`
	Version     string          `json:"version"`
	SubjectID   string          `json:"subject_id"`
	SubjectFace entity.Face     `json:"subject_face,omitempty"`
	Singleton   bool            `json:"singleton,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	Rev         int64           `json:"rev"`
	State       json.RawMessage `json:"state"`
}

type defsDoc struct {
	// Definitions is keyed by defKey.
	Definitions map[string]storedDef `json:"definitions"`
}

type storedDef struct {
	Flow    string    `json:"flow"`
	Version string    `json:"version"`
	Digest  string    `json:"digest"`
	SavedAt time.Time `json:"saved_at"`
	Bytes   []byte    `json:"bytes"`
}

func defKey(flowName, version string) string { return flowName + "/" + version }

// New returns a store backed by kv, with locker excluding other processes.
//
// Nil: both rejected. A nil KV would defer the failure to the first
// write; a nil locker would hide the choice between filelock.Locker and
// filelock.ProcessPrivate.
func New(kv state.KV, locker Locker) (*Store, error) {
	if kv == nil {
		return nil, errors.New("kvflowrun: state KV must be non-nil")
	}
	if locker == nil {
		return nil, errors.New("kvflowrun: locker must be non-nil")
	}
	return &Store{kv: kv, locker: locker, sem: make(chan struct{}, 1)}, nil
}

// lock takes the semaphore and then the locker, and returns the release of
// both. It stops waiting when ctx ends. Reads lock too: another process
// may replace a document meanwhile.
func (s *Store) lock(ctx context.Context) (func(), error) {
	select {
	case s.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, fmt.Errorf("kvflowrun: lock: %w", ctx.Err())
	}
	unlock, err := s.locker.Lock(ctx)
	if err != nil {
		<-s.sem
		return nil, fmt.Errorf("kvflowrun: lock: %w", err)
	}
	return func() {
		unlock()
		<-s.sem
	}, nil
}

// get reads and decodes the value at key into v. A missing value leaves v
// as it is.
func (s *Store) get(ctx context.Context, key string, v any) error {
	data, err := s.kv.Get(ctx, key)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("kvflowrun: read %s: %w", key, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		// The decode error may quote the document, so only the key goes out.
		return fmt.Errorf("kvflowrun: %s is damaged; restore it from a backup or remove it", key)
	}
	return nil
}

func (s *Store) put(ctx context.Context, key string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("kvflowrun: encode %s: %w", key, err)
	}
	if err := s.kv.Put(ctx, key, data); err != nil {
		return fmt.Errorf("kvflowrun: write %s: %w", key, err)
	}
	return nil
}

func (s *Store) loadRuns(ctx context.Context) (*runsDoc, error) {
	d := &runsDoc{}
	if err := s.get(ctx, RunsKey, d); err != nil {
		return nil, err
	}
	if d.Runs == nil {
		d.Runs = map[flowrun.RunID]storedRun{}
	}
	return d, nil
}

func (s *Store) loadDefs(ctx context.Context) (*defsDoc, error) {
	d := &defsDoc{}
	if err := s.get(ctx, DefinitionsKey, d); err != nil {
		return nil, err
	}
	if d.Definitions == nil {
		d.Definitions = map[string]storedDef{}
	}
	return d, nil
}

// withRuns runs fn over the runs document under the lock, and writes it
// back when fn reports a change.
func (s *Store) withRuns(ctx context.Context, fn func(d *runsDoc) (changed bool, err error)) error {
	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	d, err := s.loadRuns(ctx)
	if err != nil {
		return err
	}
	changed, err := fn(d)
	if err != nil || !changed {
		return err
	}
	return s.put(ctx, RunsKey, d)
}

func (r storedRun) run(id flowrun.RunID) (flowrun.Run, error) {
	st, err := flowrun.DecodeState(r.State)
	if err != nil {
		return flowrun.Run{}, fmt.Errorf("kvflowrun: run %s: %w", id, err)
	}
	return flowrun.Run{
		ID: id, Flow: r.Flow, Version: r.Version,
		Subject:   entity.Ref{ID: r.SubjectID, Face: r.SubjectFace},
		Singleton: r.Singleton, CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(),
		Rev: r.Rev, Instance: st.Instance, Pending: st.Pending, Log: st.Log, Claim: st.Claim,
	}, nil
}

// subjectCheck refuses a run of flowName with singleton on subjectID when
// the runs in d, other than those skip names, already hold a singleton
// run of the flow there or as many runs as the cap allows. extra counts
// runs about to join the subject.
func subjectCheck(d *runsDoc, flowName, subjectID string, singleton bool, extra int,
	skip func(flowrun.RunID) bool) error {
	n := extra
	for id, r := range d.Runs {
		if skip(id) || r.Flow != flowName || r.SubjectID != subjectID {
			continue
		}
		if singleton && r.Singleton {
			return fmt.Errorf("kvflowrun: %s on %s: %w", flowName, subjectID, flowrun.ErrSubjectBusy)
		}
		n++
	}
	if n > flowrun.MaxRunsPerSubject {
		return fmt.Errorf("kvflowrun: %s on %s: %w", flowName, subjectID, flowrun.ErrSubjectFull)
	}
	return nil
}

func noSkip(flowrun.RunID) bool { return false }

// Create implements [flowrun.Store.Create].
func (s *Store) Create(ctx context.Context, nr flowrun.NewRun) (flowrun.Run, error) {
	if err := flowrun.CheckNewRun(nr); err != nil {
		return flowrun.Run{}, err
	}
	run := flowrun.FromNew(nr)
	stateDoc, err := flowrun.EncodeState(run.State())
	if err != nil {
		return flowrun.Run{}, fmt.Errorf("kvflowrun: encode state: %w", err)
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return flowrun.Run{}, err
	}
	defer unlock()
	d, err := s.loadRuns(ctx)
	if err != nil {
		return flowrun.Run{}, err
	}
	if _, ok := d.Runs[nr.ID]; ok {
		return flowrun.Run{}, fmt.Errorf("kvflowrun: run %s: %w", nr.ID, flowrun.ErrExists)
	}
	if len(d.Runs) >= MaxRuns {
		return flowrun.Run{}, fmt.Errorf("kvflowrun: %d runs: %w", MaxRuns, flowrun.ErrFull)
	}
	if err := subjectCheck(d, nr.Flow, nr.Subject.ID, nr.Singleton, 1, noSkip); err != nil {
		return flowrun.Run{}, err
	}
	if err := s.storeDefinition(ctx, nr, run.CreatedAt); err != nil {
		return flowrun.Run{}, err
	}
	d.Runs[nr.ID] = storedRun{
		Flow: nr.Flow, Version: nr.Version, SubjectID: nr.Subject.ID, SubjectFace: nr.Subject.Face,
		Singleton: nr.Singleton, CreatedAt: run.CreatedAt, UpdatedAt: run.UpdatedAt, Rev: 1, State: stateDoc,
	}
	if err := s.put(ctx, RunsKey, d); err != nil {
		return flowrun.Run{}, err
	}
	return run, nil
}

// storeDefinition stores nr's definition when its version has none, or
// checks it against the stored one. Caller holds the lock.
func (s *Store) storeDefinition(ctx context.Context, nr flowrun.NewRun, at time.Time) error {
	defs, err := s.loadDefs(ctx)
	if err != nil {
		return err
	}
	key, digest := defKey(nr.Flow, nr.Version), flowrun.Digest(nr.Definition)
	if held, ok := defs.Definitions[key]; ok {
		if held.Digest != digest {
			return fmt.Errorf("kvflowrun: %s: %w", key, flowrun.ErrDefinitionConflict)
		}
		return nil
	}
	defs.Definitions[key] = storedDef{
		Flow: nr.Flow, Version: nr.Version, Digest: digest, SavedAt: at, Bytes: slices.Clone(nr.Definition),
	}
	return s.put(ctx, DefinitionsKey, defs)
}

// Get implements [flowrun.Store.Get].
func (s *Store) Get(ctx context.Context, id flowrun.RunID) (flowrun.Run, error) {
	if err := flowrun.CheckID(id); err != nil {
		return flowrun.Run{}, err
	}
	var out flowrun.Run
	err := s.withRuns(ctx, func(d *runsDoc) (bool, error) {
		r, ok := d.Runs[id]
		if !ok {
			return false, fmt.Errorf("kvflowrun: run %s: %w", id, flowrun.ErrNotFound)
		}
		var err error
		out, err = r.run(id)
		return false, err
	})
	return out, err
}

// Update implements [flowrun.Store.Update].
func (s *Store) Update(ctx context.Context, id flowrun.RunID, expectRev int64, st flowrun.State,
	at time.Time) (flowrun.Run, error) {
	if err := flowrun.CheckUpdate(id, expectRev, st, at); err != nil {
		return flowrun.Run{}, err
	}
	stateDoc, err := flowrun.EncodeState(st)
	if err != nil {
		return flowrun.Run{}, fmt.Errorf("kvflowrun: encode state: %w", err)
	}
	var out flowrun.Run
	err = s.withRuns(ctx, func(d *runsDoc) (bool, error) {
		r, ok := d.Runs[id]
		if !ok {
			return false, fmt.Errorf("kvflowrun: run %s: %w", id, flowrun.ErrNotFound)
		}
		if r.Rev != expectRev {
			return false, fmt.Errorf("kvflowrun: run %s: %w", id, flowrun.ErrConflict)
		}
		r.Rev++
		r.UpdatedAt = flow.NormTime(at)
		r.State = stateDoc
		d.Runs[id] = r
		run, runErr := r.run(id)
		out = run
		return true, runErr
	})
	return out, err
}

// Delete implements [flowrun.Store.Delete].
func (s *Store) Delete(ctx context.Context, id flowrun.RunID, expectRev int64) error {
	if err := flowrun.CheckDelete(id, expectRev); err != nil {
		return err
	}
	return s.withRuns(ctx, func(d *runsDoc) (bool, error) {
		r, ok := d.Runs[id]
		if !ok {
			return false, fmt.Errorf("kvflowrun: run %s: %w", id, flowrun.ErrNotFound)
		}
		if r.Rev != expectRev {
			return false, fmt.Errorf("kvflowrun: run %s: %w", id, flowrun.ErrConflict)
		}
		delete(d.Runs, id)
		return true, nil
	})
}

// ListDue implements [flowrun.Store.ListDue].
func (s *Store) ListDue(ctx context.Context, until time.Time, after flowrun.DueCursor,
	limit int) ([]flowrun.Run, error) {
	if err := flowrun.CheckDue(until, after, limit); err != nil {
		return nil, err
	}
	until = flow.NormTime(until)
	return s.collect(ctx, func(r flowrun.Run) bool {
		due := r.Instance.NextDueAt
		return !due.IsZero() && !due.After(until) && after.Before(due, r.ID)
	}, flowrun.SortDue, limit)
}

// List implements [flowrun.Store.List].
func (s *Store) List(ctx context.Context, q flowrun.Query) ([]flowrun.Run, error) {
	if err := flowrun.CheckQuery(q); err != nil {
		return nil, err
	}
	subjects := make(map[string]bool, len(q.SubjectIDs))
	for _, id := range q.SubjectIDs {
		subjects[id] = true
	}
	return s.collect(ctx, func(r flowrun.Run) bool {
		return (q.Flow == "" || r.Flow == q.Flow) &&
			(q.SubjectID == "" || r.Subject.ID == q.SubjectID) &&
			(len(subjects) == 0 || subjects[r.Subject.ID]) &&
			r.ID > q.After
	}, sortByID, q.Limit)
}

func sortByID(runs []flowrun.Run) {
	slices.SortFunc(runs, func(a, b flowrun.Run) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		}
		return 0
	})
}

// collect returns the runs keep selects, sorted, at most n. A run whose
// state does not decode is an error: listing around it would hide a run
// the timers loop must move.
func (s *Store) collect(ctx context.Context, keep func(flowrun.Run) bool, sortRuns func([]flowrun.Run),
	n int) ([]flowrun.Run, error) {
	out := []flowrun.Run{}
	err := s.withRuns(ctx, func(d *runsDoc) (bool, error) {
		for _, id := range slices.Sorted(maps.Keys(d.Runs)) {
			r, err := d.Runs[id].run(id)
			if err != nil {
				return false, err
			}
			if keep(r) {
				out = append(out, r)
			}
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	sortRuns(out)
	if len(out) > n {
		out = out[:n]
	}
	return out, nil
}

// RekeySubject implements [flowrun.Store.RekeySubject].
func (s *Store) RekeySubject(ctx context.Context, oldID, newID string) (int, error) {
	if err := flowrun.CheckRekey(oldID, newID); err != nil {
		return 0, err
	}
	moved := 0
	err := s.withRuns(ctx, func(d *runsDoc) (bool, error) {
		if oldID == newID {
			return false, nil
		}
		var ids []flowrun.RunID
		joining := map[string]int{}
		singles := map[string]bool{}
		for id, r := range d.Runs {
			if r.SubjectID != oldID {
				continue
			}
			ids = append(ids, id)
			joining[r.Flow]++
			if r.Singleton {
				singles[r.Flow] = true
			}
		}
		moving := func(id flowrun.RunID) bool { return d.Runs[id].SubjectID == oldID }
		for _, flowName := range slices.Sorted(maps.Keys(joining)) {
			if err := subjectCheck(d, flowName, newID, singles[flowName], joining[flowName], moving); err != nil {
				return false, err
			}
		}
		for _, id := range ids {
			r := d.Runs[id]
			r.SubjectID = newID
			d.Runs[id] = r
		}
		moved = len(ids)
		return moved > 0, nil
	})
	return moved, err
}

// Definition implements [flowrun.Store.Definition].
func (s *Store) Definition(ctx context.Context, flowName, version string) ([]byte, error) {
	if err := flowrun.CheckDefinitionKey(flowName, version); err != nil {
		return nil, err
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	defs, err := s.loadDefs(ctx)
	if err != nil {
		return nil, err
	}
	held, ok := defs.Definitions[defKey(flowName, version)]
	if !ok {
		return nil, fmt.Errorf("kvflowrun: definition %s: %w", defKey(flowName, version), flowrun.ErrNotFound)
	}
	return held.Bytes, nil
}

// PruneDefinitions implements [flowrun.Store.PruneDefinitions].
func (s *Store) PruneDefinitions(ctx context.Context, before time.Time) (int, error) {
	if err := flowrun.CheckPrune(before); err != nil {
		return 0, err
	}
	before = flow.NormTime(before)
	unlock, err := s.lock(ctx)
	if err != nil {
		return 0, err
	}
	defer unlock()
	runs, err := s.loadRuns(ctx)
	if err != nil {
		return 0, err
	}
	defs, err := s.loadDefs(ctx)
	if err != nil {
		return 0, err
	}
	used := map[string]bool{}
	for _, r := range runs.Runs {
		used[defKey(r.Flow, r.Version)] = true
	}
	dropped := 0
	for key, def := range defs.Definitions {
		if !used[key] && def.SavedAt.Before(before) {
			delete(defs.Definitions, key)
			dropped++
		}
	}
	if dropped == 0 {
		return 0, nil
	}
	return dropped, s.put(ctx, DefinitionsKey, defs)
}
