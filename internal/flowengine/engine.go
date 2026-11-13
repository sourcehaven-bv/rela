// Package flowengine runs the flows declared in flows.yaml (FEAT-J0O10QI):
// it starts runs, writes the outputs of the states a run enters, follows
// `done` branches, fires due timers, finishes runs on a final state and
// cancels them.
//
// The engine itself writes nothing to the graph. It drives four narrow
// collaborators the composition root supplies:
//
//   - [Headers] writes the run's `_flow-run` header as system:flow, with
//     values from the configuration only (RR-K4KY2I8).
//   - [Outputs] writes a state's outputs as the flow's run_as identity, so
//     the ACL, validation and audit apply as to any write.
//   - [Subjects] reads the subject: raw for guards, which flowcfg holds to
//     properties no visible: grant restricts, and through the run_as read
//     gate for output templates (RR-JIX6EZR).
//   - [flowrun.Store] keeps the execution state; the engine is its only
//     user besides the composition root.
//
// Every move into a state is stored with a pending mark (RR-U30RNUZ),
// which stays until the run waits: outputs written, `done` branches
// followed, or the run ended. A run with a mark is due, so a round of
// [Engine.Due] settles a run a crash or a failure left unsettled. A create output is
// claimed in the store before it is written, so it is written at most once;
// a set output is written and then marked, so it is written at least once.
package flowengine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Errors the engine returns. The adapters' own errors are wrapped.
var (
	// ErrUnknownFlow: flows.yaml declares no flow of that name.
	ErrUnknownFlow = errors.New("flowengine: unknown flow")
	// ErrSubjectNotFound: the subject does not exist or is not of the
	// flow's subject type.
	ErrSubjectNotFound = errors.New("flowengine: subject not found")
	// ErrSubjectBusy: per_subject is one and the subject carries an
	// unfinished run of the flow.
	ErrSubjectBusy = errors.New("flowengine: the subject already carries a run of the flow")
	// ErrRunNotFound: no unfinished run has that id.
	ErrRunNotFound = errors.New("flowengine: run not found")
	// ErrStranded: the run's definition no longer compiles, or its state is
	// one the definition does not declare. The run stays as it is.
	ErrStranded = errors.New("flowengine: run is stranded")
)

// Headers writes run headers as system:flow. Every value it is handed comes
// from flows.yaml or the clock, never from the subject.
type Headers interface {
	// CreateHeader creates the `_flow-run` entity with props and its
	// `_carries` edge to subject, and returns its id. It fails when the
	// subject is gone by the time the edge is written.
	CreateHeader(ctx context.Context, subject entity.Ref, props map[string]any) (flow.RunID, error)
	// PatchHeader writes props on the header.
	PatchHeader(ctx context.Context, id flow.RunID, props map[string]any) error
	// DeleteHeader deletes the header and its relations.
	DeleteHeader(ctx context.Context, id flow.RunID) error
	// LinkProduced writes the `_produced` edge from the header to created.
	LinkProduced(ctx context.Context, id flow.RunID, created entity.Ref) error
}

// Outputs writes a state's outputs as runAs.
type Outputs interface {
	// Set patches props on the subject. run is the run whose output it is.
	Set(ctx context.Context, runAs string, run flow.RunID, state string, subject entity.Ref,
		props map[string]any) error
	// Create creates an entity of typ with props and, when relate is set,
	// an edge of that type from it to subject. It returns the new entity.
	Create(ctx context.Context, runAs, typ string, props map[string]any, relate string,
		subject entity.Ref) (entity.Ref, error)
}

// Subjects reads a run's subject.
type Subjects interface {
	// Subject reads the subject as the engine: the raw row. A missing
	// subject is an error wrapping [ErrSubjectNotFound].
	Subject(ctx context.Context, ref entity.Ref) (*entity.Entity, error)
	// SubjectAs returns the subject's properties as runAs reads them: empty
	// when runAs cannot read the row, and without the values a visible:
	// grant hides from runAs.
	SubjectAs(ctx context.Context, runAs string, ref entity.Ref) (map[string]any, error)
}

// Compiler compiles a run's stored flow declaration against the current
// schema and acl.yaml (see [flowcfg.ParseFlow]).
type Compiler func(name string, source []byte, version string) (*flowcfg.Flow, error)

// Deps are the engine's collaborators.
type Deps struct {
	// Flows is the compiled flows.yaml. Nil: rejected.
	Flows *flowcfg.Compiled
	// Meta is the schema the guards bind the subject against. Nil: rejected.
	Meta *metamodel.Metamodel
	// Runs keeps the execution state. Nil: rejected.
	Runs flowrun.Store
	// Headers writes run headers. Nil: rejected.
	Headers Headers
	// Outputs writes outputs. Nil: rejected.
	Outputs Outputs
	// Subjects reads subjects. Nil: rejected.
	Subjects Subjects
	// Compile compiles an older version of a flow. Nil: rejected.
	Compile Compiler
	// Jobs queues the attempts of job steps. Nil: accepted, for a
	// process that runs no jobs; an attempt then waits for one that does.
	Jobs JobQueue
	// JobRunner runs the targets of job steps. Nil: accepted; then
	// [AttemptHandler.HandleAttempt] fails.
	JobRunner JobRunner
	// Now is the clock. Nil: accepted, means time.Now.
	Now func() time.Time
}

// Engine runs flows. It holds no mutable state; every call reads the run
// store, so several engines over one store may run at once. A conflicting
// write loses its compare-and-set and leaves the run to the winner.
type Engine struct {
	d Deps
	g guards
}

// New returns an engine over d.
func New(d Deps) (*Engine, error) {
	if d.Flows == nil || d.Meta == nil || d.Runs == nil || d.Headers == nil || d.Outputs == nil ||
		d.Subjects == nil || d.Compile == nil {

		return nil, errors.New("flowengine: flows, metamodel, run store, headers, outputs, subjects and compiler " +
			"are required")
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Engine{d: d, g: guards{meta: d.Meta, subjects: d.Subjects, now: func() time.Time {
		return flow.NormTime(d.Now())
	}}}, nil
}

// Flow returns the flow name declares in the current flows.yaml.
func (e *Engine) Flow(name string) (*flowcfg.Flow, bool) { return e.d.Flows.Flow(name) }

func (e *Engine) now() time.Time { return flow.NormTime(e.d.Now()) }

// Start starts a run of flow name on subject and runs it until it waits:
// it writes the initial state's outputs and follows `done` branches. The
// caller has authorized the start; the engine does not. It returns the run
// id, which is also the header's id. A run created but not settled, because
// an output failed say, is returned with the error: its pending mark makes
// a later [Engine.Due] round try again.
//
// The writes carry the [audit.FlowStep] By of ctx, if any: the principal
// who asked for the start.
func (e *Engine) Start(ctx context.Context, name string, subject entity.Ref) (flow.RunID, error) {
	f, ok := e.d.Flows.Flow(name)
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownFlow, name)
	}
	subj, err := e.d.Subjects.Subject(ctx, subject)
	if err != nil {
		return "", err
	}
	if subj.Type != f.Subject() {
		return "", fmt.Errorf("%w: %s is a %s, flow %s carries a %s", ErrSubjectNotFound, subj.ID, subj.Type,
			f.Name(), f.Subject())
	}
	singleton := f.PerSubject() != flowcfg.PerSubjectMany
	if singleton {
		// A cheap refusal before anything is written. The store's create
		// is the authority; this only spares a header write and delete.
		open, lerr := e.d.Runs.List(ctx, flowrun.Query{Flow: f.Name(), SubjectID: subj.ID, Limit: flowrun.MaxPage})
		if lerr != nil {
			return "", lerr
		}
		for _, r := range open {
			if r.Singleton {
				return "", ErrSubjectBusy
			}
		}
	}
	by := byOf(ctx)
	ctx = audit.WithFlowStep(ctx, audit.FlowStep{Flow: f.Name(), Step: f.Initial(), By: by})
	now := e.now()
	initial, _ := f.State(f.Initial())
	props := map[string]any{
		metamodel.FlowRunPropFlow:    f.Name(),
		metamodel.FlowRunPropVersion: f.Version(),
		metamodel.FlowRunPropStatus:  string(initial.Status),
		metamodel.FlowRunPropState:   initial.Name,
		metamodel.FlowRunPropStarted: now,
	}
	if !f.Deadline().IsZero() {
		props[metamodel.FlowRunPropDeadline] = f.Deadline().AddTo(now)
	}
	id, err := e.d.Headers.CreateHeader(ctx, subj.Ref(), props)
	if err != nil {
		return "", fmt.Errorf("flowengine: write run header: %w", err)
	}
	ctx = audit.WithFlowStep(ctx, audit.FlowStep{Flow: f.Name(), Run: string(id), Step: f.Initial(), By: by})
	// Every new run is unsettled in its initial state, so a crash before
	// it settles leaves it due.
	nr := flowrun.NewRun{
		ID: id, Flow: f.Name(), Version: f.Version(), Definition: f.Source(),
		Subject: subj.Ref(), Singleton: singleton, At: now,
		Pending: &flowrun.Pending{State: initial.Name},
		Log:     []flowrun.Step{{At: now, State: initial.Name, Via: flowrun.ViaStart, By: clip(by)}},
	}
	nr.Instance.NextDueAt = now
	run, err := e.d.Runs.Create(ctx, nr)
	if err != nil {
		e.dropHeader(ctx, id)
		if errors.Is(err, flowrun.ErrSubjectBusy) {
			return "", ErrSubjectBusy
		}
		return "", fmt.Errorf("flowengine: store run: %w", err)
	}
	// A delete of the subject that raced the start must not leave a run on
	// a missing subject (RR-DTSDBII).
	if _, err := e.d.Subjects.Subject(ctx, subj.Ref()); errors.Is(err, ErrSubjectNotFound) {
		if ferr := e.finish(ctx, f, run, flow.StatusCancelled); ferr != nil {
			slog.WarnContext(ctx, "flowengine: a run on a deleted subject was not cancelled", "run", id,
				"error", ferr)
		}
		return "", err
	}
	if left, err := e.settle(ctx, f, run, ""); err != nil {
		e.backoff(ctx, left)
		return id, err
	}
	return id, nil
}

// byOf returns the By of the flow step ctx carries.
func byOf(ctx context.Context) string {
	s, _ := audit.FlowStepFrom(ctx)
	return s.By
}

// dropHeader deletes a header whose run was never stored.
func (e *Engine) dropHeader(ctx context.Context, id flow.RunID) {
	if err := e.d.Headers.DeleteHeader(ctx, id); err != nil {
		slog.WarnContext(ctx, "flowengine: a run header without a run was left behind", "run", id, "error", err)
	}
}

// maxCancelTries bounds the compare-and-set retries of [Engine.Cancel].
const maxCancelTries = 8

// Cancel ends an unfinished run as cancelled: it drops the run's state and
// then records the end on the header. The caller has authorized it. It
// returns [ErrRunNotFound] when the run is gone, also when it ended while
// Cancel ran, so a cancel never claims a run another engine finished.
//
// The writes carry the [audit.FlowStep] By of ctx, if any.
func (e *Engine) Cancel(ctx context.Context, id flow.RunID) error {
	for range maxCancelTries {
		run, err := e.d.Runs.Get(ctx, id)
		if errors.Is(err, flowrun.ErrNotFound) {
			return ErrRunNotFound
		}
		if err != nil {
			return err
		}
		err = e.d.Runs.Delete(ctx, id, run.Rev)
		switch {
		case errors.Is(err, flowrun.ErrConflict):
			continue // another engine moved it: cancel what it is now
		case errors.Is(err, flowrun.ErrNotFound):
			return ErrRunNotFound
		case err != nil:
			return err
		}
		props := endProps(flow.StatusCancelled, e.now())
		// A forked run's header keeps the join it was headed for.
		if f, ferr := e.flowOf(ctx, run); ferr == nil && !forked(run) {
			props[metamodel.FlowRunPropState] = flow.Read(f.Definition(), run.Instance, run.CreatedAt).State
		}
		wctx := audit.WithFlowStep(ctx, audit.FlowStep{Flow: run.Flow, Run: string(id), By: byOf(ctx)})
		if err := e.d.Headers.PatchHeader(wctx, id, props); err != nil {
			return fmt.Errorf("flowengine: run %s is cancelled, but its header is not: %w", id, err)
		}
		return nil
	}
	return fmt.Errorf("flowengine: run %s kept changing; cancel it again", id)
}

// CloseHeader records a cancel on the header of a run whose state is gone:
// a crash between the header write and the run's create, or between a
// cancel's two writes. The caller has checked that no run is stored.
func (e *Engine) CloseHeader(ctx context.Context, id flow.RunID) error {
	if _, err := e.d.Runs.Get(ctx, id); !errors.Is(err, flowrun.ErrNotFound) {
		if err != nil {
			return err
		}
		return fmt.Errorf("flowengine: run %s is stored; cancel it instead", id)
	}
	ctx = audit.WithFlowStep(ctx, audit.FlowStep{Run: string(id), By: byOf(ctx)})
	return e.d.Headers.PatchHeader(ctx, id, endProps(flow.StatusCancelled, e.now()))
}

// Run returns the stored state of an unfinished run.
func (e *Engine) Run(ctx context.Context, id flow.RunID) (flowrun.Run, error) {
	run, err := e.d.Runs.Get(ctx, id)
	if errors.Is(err, flowrun.ErrNotFound) {
		return flowrun.Run{}, ErrRunNotFound
	}
	return run, err
}

// SubjectRuns returns the unfinished runs on the subject with id subjectID,
// on every face, at most [flowrun.MaxPage] of them.
func (e *Engine) SubjectRuns(ctx context.Context, subjectID string) ([]flowrun.Run, error) {
	return e.d.Runs.List(ctx, flowrun.Query{SubjectID: subjectID, Limit: flowrun.MaxPage})
}

// FlowOf returns the flow run follows, at the run's own version.
func (e *Engine) FlowOf(ctx context.Context, run flowrun.Run) (*flowcfg.Flow, error) {
	return e.flowOf(ctx, run)
}

// MaxDuePerRound caps the runs one [Engine.Due] round advances.
const MaxDuePerRound = 64

// Backoff bounds for a run that fails to advance.
const (
	minBackoff = 30 * time.Second
	maxBackoff = time.Hour
)

// Bounds of the writes the engine does not let its caller's context cut:
// a create output, whose cut leaves it unknown whether it landed, and the
// backoff of a run that failed, which would otherwise stay at the front of
// the due order. A test shortens them.
var (
	outputTimeout  = 30 * time.Second
	backoffTimeout = 10 * time.Second
)

// Due runs one round of [Engine.DueRound] from the start, and returns how
// many runs it advanced.
func (e *Engine) Due(ctx context.Context) (int, error) {
	rep, err := e.DueRound(ctx, flowrun.DueCursor{})
	return rep.Advanced, err
}

// DueReport is what one round of [Engine.DueRound] did.
type DueReport struct {
	// Advanced counts the runs the round advanced.
	Advanced int
	// Failed counts the runs it could not advance and backed off.
	Failed int
	// More reports a full round: more runs may be due after Next.
	More bool
	// Next resumes the listing after the last run of the round.
	Next flowrun.DueCursor
}

// DueRound advances the runs due at or before the engine's clock, after
// the cursor, at most [MaxDuePerRound] of them: it settles runs with a
// pending mark, fires due timers and tests a join waiting on its `when:`.
// A run it cannot advance is logged and backed off, so a stranded or
// failing run does not hold back the others. A caller that runs several
// rounds passes the last round's Next to go on after the last run.
//
// When ctx ends mid-round, the round stops with ctx's error and Next at
// the run it cut off. That run keeps its pending mark, but a create output
// it was writing is finished first, bounded by outputTimeout: a cut create
// may have landed, so it would be skipped rather than written again.
func (e *Engine) DueRound(ctx context.Context, after flowrun.DueCursor) (DueReport, error) {
	now := e.now()
	runs, err := e.d.Runs.ListDue(ctx, now, after, MaxDuePerRound)
	if err != nil {
		return DueReport{Next: after}, err
	}
	rep := DueReport{Next: after, More: len(runs) == MaxDuePerRound}
	for _, run := range runs {
		if ctx.Err() != nil {
			return rep, ctx.Err()
		}
		rep.Next = flowrun.DueCursor{At: run.Instance.NextDueAt, ID: run.ID}
		if left, err := e.advance(ctx, run, now); err != nil {
			slog.WarnContext(ctx, "flowengine: a due run did not advance", "run", run.ID, "flow", run.Flow,
				"error", err)
			// Backed off even when ctx ended: a run cut off by a deadline
			// would else stay first in the due order.
			bctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), backoffTimeout)
			e.backoff(bctx, left)
			cancel()
			rep.Failed++
			continue
		}
		rep.Advanced++
	}
	return rep, nil
}

// backoff makes a run that failed to advance due again later: after
// minBackoff, doubling with each failure in a row up to maxBackoff. A run
// without a mark waits minBackoff. A forked run backs off each branch (see
// branchesBackedOff). A zero run is left alone.
func (e *Engine) backoff(ctx context.Context, run flowrun.Run) {
	if run.ID == "" {
		return
	}
	if forked(run) {
		now := e.now()
		if _, err := e.put(ctx, run, branchesBackedOff(run, now), now); err != nil && !isLost(err) {
			slog.WarnContext(ctx, "flowengine: a failed run was not backed off", "run", run.ID, "error", err)
		}
		return
	}
	delay := minBackoff
	var p *flowrun.Pending
	if run.Pending != nil {
		cp := *run.Pending
		delay = backoffDelay(cp.Failures)
		cp.Failures = min(cp.Failures+1, flowrun.MaxPendingFailures)
		p = &cp
	}
	inst := run.Instance
	inst.NextDueAt = e.now().Add(delay)
	if _, err := e.save(ctx, run, "", inst, p, nil, e.now()); err != nil && !isLost(err) {
		slog.WarnContext(ctx, "flowengine: a failed run was not backed off", "run", run.ID, "error", err)
	}
}

// advance runs one due run: it settles a run with a pending mark, else
// fires its due timer. It returns the run as last stored, for backoff.
func (e *Engine) advance(ctx context.Context, run flowrun.Run, now time.Time) (flowrun.Run, error) {
	f, err := e.flowOf(ctx, run)
	if err != nil {
		return run, err
	}
	if forked(run) {
		return e.advanceBranches(ctx, f, run, now)
	}
	if run.Pending != nil {
		return e.settle(ctx, f, run, "")
	}
	def := f.Definition()
	cur := flow.Read(def, run.Instance, run.CreatedAt)
	if cur.Stranded {
		return run, fmt.Errorf("%w: state %q", ErrStranded, cur.State)
	}
	t, due, ok := flow.NextTimer(def, cur)
	if !ok || due.After(now) {
		// The stored due time was stale: store the right one.
		inst := run.Instance
		inst.NextDueAt = flow.NextDue(def, cur)
		if !inst.NextDueAt.After(now) {
			inst.NextDueAt = time.Time{} // never a busy loop on a due time nothing fires
		}
		_, uerr := e.save(ctx, run, "", inst, nil, nil, now)
		if uerr != nil && !isLost(uerr) {
			return run, uerr
		}
		return flowrun.Run{}, nil
	}
	moved, err := e.take(ctx, f, run, "", cur, []string{t.To}, flowrun.Step{Via: flowrun.ViaTimer, Timer: t.Name}, now)
	if err != nil {
		if isLost(err) {
			return flowrun.Run{}, nil
		}
		return run, err
	}
	return e.settle(ctx, f, moved, "")
}

// flowOf returns the flow a run follows: the current declaration when its
// version matches, else the run's stored snapshot compiled against the
// current schema (RR-7E3BWVM).
func (e *Engine) flowOf(ctx context.Context, run flowrun.Run) (*flowcfg.Flow, error) {
	if f, ok := e.d.Flows.Flow(run.Flow); ok && f.Version() == run.Version {
		return f, nil
	}
	src, err := e.d.Runs.Definition(ctx, run.Flow, run.Version)
	if err != nil {
		return nil, fmt.Errorf("%w: definition %s@%s: %w", ErrStranded, run.Flow, run.Version, err)
	}
	f, err := e.d.Compile(run.Flow, src, run.Version)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrStranded, err)
	}
	return f, nil
}

// Flows returns the flows of the current flows.yaml, in file order.
func (e *Engine) Flows() []*flowcfg.Flow { return e.d.Flows.Flows() }
