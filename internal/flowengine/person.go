package flowengine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
)

// Errors of the person-facing calls: [Engine.Claim], [Engine.Change] and
// [Engine.Retry].
var (
	// ErrStale: the run is no longer at the generation the caller saw.
	ErrStale = errors.New("flowengine: the run moved on; reload it")
	// ErrClaimed: someone else is completing the step.
	ErrClaimed = errors.New("flowengine: someone else is completing this step")
	// ErrSettling: the run is still writing the outputs of its state, so
	// its step cannot be done yet.
	ErrSettling = errors.New("flowengine: the run is still settling; try again shortly")
	// ErrNotPending: a retry of a run that has nothing to retry.
	ErrNotPending = errors.New("flowengine: the run has no failed step to retry")
	// ErrGuard: a guard failed to evaluate. The text is fixed, since the
	// evaluation error can quote subject values.
	ErrGuard = errors.New("flowengine: a guard could not be evaluated")
)

// ClaimTTL is how long a step claim holds. A claim older than this is void,
// so a caller who claimed and then failed does not block the step.
const ClaimTTL = 2 * time.Minute

// maxClaimTries bounds the compare-and-set retries of [Engine.Claim].
const maxClaimTries = 4

// clip bounds a principal name for the run state.
func clip(by string) string {
	if len(by) > flowrun.MaxClaimBy {
		return by[:flowrun.MaxClaimBy]
	}
	return by
}

// Claim marks the step of run id at generation gen as being completed by
// by (RR-LGRBAP4). It returns [ErrStale] when the run moved on,
// [ErrSettling] while it writes outputs and [ErrClaimed] when another
// principal holds a live claim. A claim by the same principal is renewed.
// The caller has authorized the step; the engine does not.
func (e *Engine) Claim(ctx context.Context, id flow.RunID, gen int64, by string) error {
	if by == "" {
		return errors.New("flowengine: a claim needs a principal")
	}
	by = clip(by)
	for range maxClaimTries {
		run, err := e.Run(ctx, id)
		if err != nil {
			return err
		}
		tok, cerr := claimable(run, gen, by, e.now())
		if cerr != nil {
			return cerr
		}
		st := cloneState(run)
		c := &flowrun.Claim{Generation: gen, By: by, At: e.now()}
		if tok == "" {
			st.Claim = c
		} else {
			st.Branches[branchAt(run, tok)].Claim = c
		}
		_, err = e.put(ctx, run, st, e.now())
		if errors.Is(err, flowrun.ErrConflict) {
			continue // re-read: someone else may have claimed it
		}
		if errors.Is(err, flowrun.ErrNotFound) {
			return ErrRunNotFound
		}
		return err
	}
	return ErrClaimed
}

// claimable checks that by may claim or change run's step at gen, and
// returns the token at gen, at now: the run, or the branch whose step it is.
func claimable(run flowrun.Run, gen int64, by string, now time.Time) (string, error) {
	if !forked(run) && run.Pending != nil {
		return "", ErrSettling
	}
	tok, ok := tokenAt(run, gen)
	if !ok {
		return "", ErrStale
	}
	_, p, c, _ := slot(run, tok)
	if p != nil {
		return "", ErrSettling
	}
	if c != nil && c.Generation == gen && c.By != by && now.Sub(c.At) < ClaimTTL {
		return "", ErrClaimed
	}
	return tok, nil
}

// Changed is what [Engine.Change] did.
type Changed struct {
	// Moved reports whether a change branch applied.
	Moved bool
	// State is the state of the step's branch after the change, or of the
	// run when it has no branches; empty when that place is gone.
	State string
	// Finished reports that the run ended.
	Finished bool
}

// Change fires the `change` event on run id at generation gen, caused by
// by: the first change branch of the state at gen, the run's or one of
// its branches', whose roles by holds
// (holds reports one role; a branch without roles admits anyone) and whose
// guard holds on the raw subject moves the run, which then settles. No
// branch applying is not an error. The claim by holds on the step, if any,
// is released either way.
//
// A write to another branch of a forked run between the read and the move
// does not make the step stale: Change reads the run again and retries
// while the step's branch is still at gen. A failure of another branch to
// settle is logged, not returned.
//
// The caller has authorized the event; the engine does not. The writes
// carry the [audit.FlowStep] By of ctx, if any.
func (e *Engine) Change(ctx context.Context, id flow.RunID, gen int64, by string,
	holds func(role string) bool) (Changed, error) {
	by = clip(by)
	for range changeAttempts {
		out, err := e.change(ctx, id, gen, by, holds)
		if !errors.Is(err, errLostMove) {
			return out, err
		}
	}
	return Changed{}, ErrStale
}

// changeAttempts bounds the times [Engine.Change] reads the run again
// after losing a compare-and-set to another write on it.
const changeAttempts = 5

// errLostMove is change losing the compare-and-set of its move.
var errLostMove = errors.New("flowengine: the move lost to another write")

// change is one attempt of [Engine.Change].
func (e *Engine) change(ctx context.Context, id flow.RunID, gen int64, by string,
	holds func(role string) bool) (Changed, error) {
	run, err := e.Run(ctx, id)
	if err != nil {
		return Changed{}, err
	}
	tok, cerr := claimable(run, gen, by, e.now())
	if cerr != nil {
		return Changed{}, cerr
	}
	f, err := e.flowOf(ctx, run)
	if err != nil {
		return Changed{}, err
	}
	inst := instOf(run, tok)
	cur := flow.Read(f.Definition(), inst, run.CreatedAt)
	if cur.Stranded {
		return Changed{}, fmt.Errorf("%w: state %q", ErrStranded, cur.State)
	}
	ctx = audit.WithFlowStep(ctx, audit.FlowStep{Flow: f.Name(), Run: string(id), Step: cur.State, By: by})
	var subj *entity.Entity
	d := flow.Decide(cur.State, f.Branches(cur.State, flowcfg.EventChange), flow.Rules[flowcfg.Branch]{
		Admits: func(b flowcfg.Branch) bool { return len(b.Roles) == 0 || slices.ContainsFunc(b.Roles, holds) },
		Holds: func(b flowcfg.Branch) (bool, error) {
			if b.When == nil {
				return true, nil
			}
			if subj == nil {
				var serr error
				if subj, serr = e.d.Subjects.Subject(ctx, run.Subject); serr != nil {
					return false, serr
				}
			}
			return e.g.holds(ctx, b.When, subj)
		},
		Target: func(b flowcfg.Branch) string { return b.To },
	})
	switch d.Outcome {
	case flow.Moved:
	case flow.GuardFailed:
		slog.WarnContext(ctx, "flowengine: a change guard failed", "run", id, "state", cur.State, "error", d.Err)
		e.release(ctx, run, tok)
		return Changed{State: cur.State}, ErrGuard
	default:
		e.release(ctx, run, tok)
		return Changed{State: cur.State}, nil
	}
	moved, err := e.take(ctx, f, run, tok, cur, d.Branch.Targets(), flowrun.Step{Via: flowrun.ViaChange, By: by},
		e.now())
	if err != nil {
		if isLost(err) {
			return Changed{}, errLostMove
		}
		return Changed{}, err
	}
	out := Changed{Moved: true, State: d.Move.To}
	left, err := e.settle(ctx, f, moved, tok)
	if err != nil && othersOnly(err, tok) {
		// The step settled; another branch did not, and was backed off.
		left, err = flowrun.Run{}, nil
	}
	if err != nil {
		e.backoff(ctx, left)
		return out, err
	}
	if left.ID == "" {
		// Ended, or another engine took it over: read what is stored. A
		// run that cannot be read any more has ended.
		now, gerr := e.Run(ctx, id)
		if out.Finished = gerr != nil; out.Finished {
			return out, nil
		}
		left = now
	}
	out.State = placeOf(left, tok)
	return out, nil
}

// placeOf returns the state of token tok in run: the branch's while it
// is live, else the run's own. A forked run has none of its own.
func placeOf(run flowrun.Run, tok string) string {
	if i := branchAt(run, tok); tok != "" && i >= 0 {
		return run.Branches[i].Instance.State
	}
	if forked(run) {
		return ""
	}
	return run.Instance.State
}

// ReleaseClaim drops the claim by holds on the step of run id at
// generation gen, best effort: a write that failed after the claim must
// not keep other holders out for [ClaimTTL].
func (e *Engine) ReleaseClaim(ctx context.Context, id flow.RunID, gen int64, by string) {
	run, err := e.Run(ctx, id)
	if err != nil {
		return
	}
	tok, ok := tokenAt(run, gen)
	if _, _, c, _ := slot(run, tok); !ok || c == nil || c.By != clip(by) || c.Generation != gen {
		return
	}
	e.release(ctx, run, tok)
}

// release drops the claim on token tok's step, best effort: a claim left
// behind runs out after [ClaimTTL].
func (e *Engine) release(ctx context.Context, run flowrun.Run, tok string) {
	_, _, c, ok := slot(run, tok)
	if !ok || c == nil {
		return
	}
	st := cloneState(run)
	if tok == "" {
		st.Claim = nil
	} else {
		st.Branches[branchAt(run, tok)].Claim = nil
	}
	if _, err := e.put(ctx, run, st, e.now()); err != nil && !isLost(err) {
		slog.WarnContext(ctx, "flowengine: a step claim was not released", "run", run.ID, "error", err)
	}
}

// Retry settles run id now: it writes the outputs a failure left and
// follows `done` branches, as a [Engine.Due] round would. It returns
// [ErrNotPending] for a run that waits in its state. A failure backs the
// run off again and is returned.
func (e *Engine) Retry(ctx context.Context, id flow.RunID) error {
	run, err := e.Run(ctx, id)
	if err != nil {
		return err
	}
	if !Pending(run) {
		return ErrNotPending
	}
	f, err := e.flowOf(ctx, run)
	if err != nil {
		return err
	}
	if left, err := e.settle(ctx, f, run, ""); err != nil {
		e.backoff(ctx, left)
		return err
	}
	return nil
}

// Explanation says why a run is where it is: what it waits for, what can
// move it and when.
type Explanation struct {
	State      string
	Label      string
	Status     flow.Status
	Generation int64
	EnteredAt  time.Time
	// WaitingFor are the roles the state waits on.
	WaitingFor []string
	// Writes are the subject properties the step form edits.
	Writes []string
	// Timers are the state's timers with their due times.
	Timers []TimerDue
	// Branches are the state's change branches with their guard verdicts.
	Branches []BranchCheck
	// Pending is set while the run settles: outputs left to write.
	Pending *PendingStep
	// Claim is the live claim on the step, if any.
	Claim *flowrun.Claim
	// Stranded is set when the run's state or definition is gone.
	Stranded bool
	// Parallel explains each branch of a forked run, in fork order. The
	// run itself then explains as the join it waits at, with generation
	// 0: its steps are the branches'.
	Parallel []Explanation
	// Branch names the branch a Parallel entry explains.
	Branch string
	// Arrived reports a branch that waits at the join for the others.
	Arrived bool
}

// TimerDue is one timer of the run's state.
type TimerDue struct {
	Name  string
	Label string
	To    string
	// Due is when it fires; zero while paused.
	Due time.Time
}

// BranchCheck is one change branch of the run's state.
type BranchCheck struct {
	To string
	// Fork are the states a fork branch starts, in place of To.
	Fork  []string
	Label string
	Roles []string
	// Guard is the guard's source, or empty for none.
	Guard string
	// Holds is whether the guard holds on the subject now; true without a
	// guard.
	Holds bool
	// Failed is set when the guard could not be evaluated. The error is
	// not reported, since it can quote subject values (RR-RKBBA7H).
	Failed bool
}

// PendingStep is a run that has not settled in its state.
type PendingStep struct {
	Failures    int
	NextAttempt time.Time
}

// Explain reads why each run waits, in the order given. It evaluates the
// change guards on the raw subject, which flowcfg holds to properties no
// visible: grant restricts. Within one call each flow version is compiled
// once and each subject read once (RR-SKOL4WA), so the cost does not grow
// with the runs of one subject. A run whose definition no longer compiles
// explains as stranded.
func (e *Engine) Explain(ctx context.Context, runs ...flowrun.Run) ([]Explanation, error) {
	ex := explainer{e: e, flows: FlowCache{e: e}, subjects: map[entity.Ref]*entity.Entity{}}
	out := make([]Explanation, len(runs))
	for i, run := range runs {
		x, err := ex.explain(ctx, run)
		if err != nil {
			return nil, err
		}
		out[i] = x
	}
	return out, nil
}

// FlowCache compiles each flow version once for one operation
// (RR-SKOL4WA). The zero value is not usable; get one from
// [Engine.FlowCache].
type FlowCache struct {
	e    *Engine
	byFV map[string]flowOrErr
}

type flowOrErr struct {
	f   *flowcfg.Flow
	err error
}

// FlowCache returns an empty per-operation cache of compiled flows.
func (e *Engine) FlowCache() *FlowCache { return &FlowCache{e: e} }

// Of returns the flow run follows, as [Engine.FlowOf] does.
func (c *FlowCache) Of(ctx context.Context, run flowrun.Run) (*flowcfg.Flow, error) {
	key := run.Flow + "@" + run.Version
	if v, ok := c.byFV[key]; ok {
		return v.f, v.err
	}
	f, err := c.e.flowOf(ctx, run)
	if c.byFV == nil {
		c.byFV = map[string]flowOrErr{}
	}
	c.byFV[key] = flowOrErr{f: f, err: err}
	return f, err
}

// explainer carries the caches of one [Engine.Explain] call.
type explainer struct {
	e        *Engine
	flows    FlowCache
	subjects map[entity.Ref]*entity.Entity
}

func (ex *explainer) subject(ctx context.Context, ref entity.Ref) (*entity.Entity, error) {
	if s, ok := ex.subjects[ref]; ok {
		return s, nil
	}
	s, err := ex.e.d.Subjects.Subject(ctx, ref)
	if err != nil {
		return nil, err
	}
	ex.subjects[ref] = s
	return s, nil
}

func (ex *explainer) explain(ctx context.Context, run flowrun.Run) (Explanation, error) {
	f, err := ex.flows.Of(ctx, run)
	if err != nil {
		if errors.Is(err, ErrStranded) {
			return Explanation{State: run.Instance.State, Label: run.Instance.State,
				Generation: run.Instance.Generation, Stranded: true}, nil
		}
		return Explanation{}, err
	}
	if !forked(run) {
		return ex.place(ctx, f, run, run.Instance, run.Pending, run.Claim)
	}
	// A forked run waits at its join for its branches: the run explains as
	// the join, with no step of its own, and each branch as a place.
	fk, _, ok := f.Region(run.Branches[0].Name)
	if !ok {
		return Explanation{State: run.Instance.State, Label: run.Instance.State, Stranded: true}, nil
	}
	js, _ := f.State(fk.Join)
	x := Explanation{State: js.Name, Label: js.Label, Status: flow.StatusWorking, EnteredAt: run.Instance.EnteredAt}
	if fk.Waits {
		x.Status = flow.StatusInputRequired
	}
	for _, b := range run.Branches {
		bx, err := ex.place(ctx, f, run, b.Instance, b.Pending, b.Claim)
		if err != nil {
			return Explanation{}, err
		}
		bx.Branch, bx.Arrived = b.Name, b.Arrived
		if b.Arrived {
			// It waits for the others, not for anyone in the join.
			bx.WaitingFor, bx.Writes, bx.Timers, bx.Branches = nil, nil, nil, nil
		}
		if bx.Pending != nil {
			x.Pending = bx.Pending
		}
		x.Parallel = append(x.Parallel, bx)
	}
	return x, nil
}

// place explains one place of run: the run itself, or a branch, at inst
// with its pending mark p and claim c.
func (ex *explainer) place(ctx context.Context, f *flowcfg.Flow, run flowrun.Run, inst flow.Instance,
	p *flowrun.Pending, c *flowrun.Claim) (Explanation, error) {
	e := ex.e
	cur := flow.Read(f.Definition(), inst, run.CreatedAt)
	x := Explanation{State: cur.State, Label: cur.State, Generation: cur.Generation, EnteredAt: cur.EnteredAt}
	if cur.Stranded {
		x.Stranded = true
		return x, nil
	}
	st, _ := f.State(cur.State)
	x.Label, x.Status, x.WaitingFor, x.Writes = st.Label, st.Status, st.WaitingFor(), st.Writes()
	def := f.Definition()
	for _, t := range st.Timers() {
		td := TimerDue{Name: t.Name, Label: t.Label, To: t.To}
		if !def.Paused(t.Name) {
			td.Due = flow.TimerDue(t.Timer, cur)
		}
		x.Timers = append(x.Timers, td)
	}
	for _, b := range st.Branches(flowcfg.EventChange) {
		bc := BranchCheck{To: b.To, Fork: b.Fork, Label: b.Label, Roles: b.Roles, Holds: true}
		if b.When != nil {
			bc.Guard = b.When.Source()
			subj, err := ex.subject(ctx, run.Subject)
			if err != nil {
				return Explanation{}, err
			}
			ok, gerr := e.g.holds(ctx, b.When, subj)
			bc.Holds, bc.Failed = ok && gerr == nil, gerr != nil
		}
		x.Branches = append(x.Branches, bc)
	}
	if p != nil {
		x.Pending = &PendingStep{Failures: p.Failures, NextAttempt: inst.NextDueAt}
	}
	if c != nil && c.Generation == cur.Generation && e.now().Sub(c.At) < ClaimTTL {
		cp := *c
		x.Claim = &cp
	}
	return x, nil
}
