package flowengine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// settle runs run until it waits. While a token of the run carries a
// pending mark it writes the token's state's outputs, finishes the run on
// a final state, and follows the first `done` branch whose guard holds;
// when no branch applies it clears the mark. A forked run with no mark
// left tests its join. flowcfg refuses a cycle of done branches, so the
// loop ends; the bound is a backstop.
//
// It settles the branch first first, when it has a mark. A branch that
// fails is backed off alone and the others settle; the run then returns
// the failures as [*BranchError]s, with a zero run, since nothing is left
// to back off.
//
// It returns the run as last stored, or a zero run when another engine
// took it over or the run ended. On another error the returned run is the
// one to back off: its mark is kept, so a later round settles it again.
func (e *Engine) settle(ctx context.Context, f *flowcfg.Flow, run flowrun.Run, first string) (flowrun.Run, error) {
	failed := map[string]bool{}
	var errs []error
	for range (flow.MaxStates+1)*(flowrun.MaxBranches+1) + 1 {
		tok, ok := pendingToken(run, failed, first)
		if !ok {
			if len(errs) > 0 {
				return flowrun.Run{}, errors.Join(errs...)
			}
			joined, fired, err := e.join(ctx, f, run)
			if !fired {
				return joined, err
			}
			run = joined
			continue
		}
		next, err := e.settleStep(ctx, f, run, tok)
		if errors.Is(err, errParked) {
			if tok == "" {
				return next, nil
			}
			failed[tok], run = true, next
			continue
		}
		if err == nil && next.ID != "" {
			run = next
			continue
		}
		if tok == "" || next.ID == "" {
			return next, errors.Join(append(errs, err)...)
		}
		// A branch failed: it is backed off alone, and the others go on.
		slog.WarnContext(ctx, "flowengine: a branch did not settle", "run", run.ID, "branch", tok, "error", err)
		errs = append(errs, &BranchError{Branch: tok, Err: err})
		failed[tok] = true
		run = next
		now := e.now()
		if st, ok := branchBackedOff(next, tok, now); ok {
			if run, err = e.put(ctx, next, st, now); err != nil {
				return flowrun.Run{}, errors.Join(append(errs, notLost(err))...)
			}
		}
	}
	return run, fmt.Errorf("flowengine: run %s did not settle", run.ID)
}

// BranchError is the failure to settle one branch of a forked run, which
// was backed off on its own.
type BranchError struct {
	Branch string
	Err    error
}

func (b *BranchError) Error() string { return fmt.Sprintf("branch %s: %v", b.Branch, b.Err) }

func (b *BranchError) Unwrap() error { return b.Err }

// othersOnly reports whether err holds only failures of branches other
// than tok: a step of tok that settled cleanly.
func othersOnly(err error, tok string) bool {
	if tok == "" {
		return false
	}
	var list []error
	if j, ok := err.(interface{ Unwrap() []error }); ok {
		list = j.Unwrap()
	} else {
		list = []error{err}
	}
	for _, x := range list {
		var b *BranchError
		if !errors.As(x, &b) || b.Branch == tok {
			return false
		}
	}
	return true
}

// settleStep takes one step of settling token tok: it writes the outputs
// left, then ends the run, follows a done branch or clears the mark.
func (e *Engine) settleStep(ctx context.Context, f *flowcfg.Flow, run flowrun.Run, tok string) (flowrun.Run, error) {
	inst, p, _, ok := slot(run, tok)
	if !ok || p == nil {
		// Not a token with a mark: never read a zero instance as the
		// initial state (RR-1D9R0UF). The run changed under the caller.
		return flowrun.Run{}, nil
	}
	cur := flow.Read(f.Definition(), inst, run.CreatedAt)
	if cur.Stranded {
		return run, fmt.Errorf("%w: state %q", ErrStranded, cur.State)
	}
	st, _ := f.State(cur.State)
	var err error
	if run, err = e.writeOutputs(ctx, f, run, tok, cur, st); err != nil || run.ID == "" {
		return run, err
	}
	if job, hasJob := st.Job(); hasJob {
		if run, err = (jobSteps{e}).jobStep(ctx, f, run, tok, cur, job); err != nil || run.ID == "" {
			return run, err
		}
	}
	if st.Final {
		if ferr := e.finish(ctx, f, run, st.Status); ferr != nil {
			return run, ferr
		}
		return flowrun.Run{}, nil
	}
	b, ok, err := e.g.doneBranch(ctx, f, run, cur)
	if err != nil {
		return run, err
	}
	if !ok {
		return e.rest(ctx, f, run, tok, cur)
	}
	moved, err := e.take(ctx, f, run, tok, cur, b.Targets(), flowrun.Step{Via: flowrun.ViaDone}, e.now())
	if err != nil {
		if isLost(err) {
			return flowrun.Run{}, nil
		}
		return run, err
	}
	return moved, nil
}

// writeOutputs writes the outputs the pending mark of token tok leaves
// for its state, recording progress after each. It returns the run as
// stored after the last write, or a zero run when another engine took the
// run over.
func (e *Engine) writeOutputs(ctx context.Context, f *flowcfg.Flow, run flowrun.Run, tok string, cur flow.Current,
	st flowcfg.State) (flowrun.Run, error) {
	p := pendingOf(run, tok)
	outs := st.Outputs()
	if p.State != cur.State || p.Generation != cur.Generation {
		// A mark that names a state the run left: never write the current
		// state's outputs from it, but settle the state.
		return e.mark(ctx, run, tok, cur, len(outs), 0)
	}
	if p.Done >= len(outs) {
		return run, nil
	}
	step := audit.FlowStep{Flow: f.Name(), Run: string(run.ID), Step: cur.State}
	if prev, ok := audit.FlowStepFrom(ctx); ok {
		step.By = prev.By
	}
	ctx = audit.WithFlowStep(ctx, step)
	vars := flowcfg.Vars{Subject: run.Subject, Run: run.ID, Now: e.now()}
	if needsSubject(outs[p.Done:]) {
		props, err := e.d.Subjects.SubjectAs(ctx, f.RunAs(), run.Subject)
		if err != nil {
			return run, fmt.Errorf("flowengine: read subject as %s: %w", f.RunAs(), err)
		}
		vars.Props = props
	}
	for i := p.Done; i < len(outs); i++ {
		var err error
		if run, err = e.writeOutput(ctx, f, run, tok, cur, outs[i], i, vars); err != nil || run.ID == "" {
			return run, err
		}
	}
	return run, nil
}

// writeOutput writes output o, number i of token tok's state, and records
// it in the pending mark.
func (e *Engine) writeOutput(ctx context.Context, f *flowcfg.Flow, run flowrun.Run, tok string, cur flow.Current,
	o flowcfg.Output, i int, vars flowcfg.Vars) (flowrun.Run, error) {
	props := evalProps(o, vars)
	wrap := func(err error) error {
		return fmt.Errorf("flowengine: run %s state %s output %d: %w", run.ID, cur.State, i, err)
	}
	if o.Kind != flowcfg.OutputCreate {
		// A set is idempotent: written first, marked after, so a crash
		// between the two writes it again.
		if err := e.d.Outputs.Set(ctx, f.RunAs(), run.ID, cur.State, run.Subject, props); err != nil {
			return run, wrap(err)
		}
		return e.mark(ctx, run, tok, cur, i+1, 0)
	}
	// Claim first: a crash after the claim skips the create rather than
	// writing it twice.
	before := pendingOf(run, tok)
	claimed, err := e.mark(ctx, run, tok, cur, i+1, 0)
	if err != nil || claimed.ID == "" {
		return claimed, err
	}
	// The caller's deadline or shutdown does not cut the create: a cut
	// create may have landed, so it could only be skipped.
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), outputTimeout)
	defer cancel()
	created, err := e.d.Outputs.Create(cctx, f.RunAs(), o.Type, props, o.Relate, run.Subject)
	if err != nil {
		// A create that ran out of time may have written all the same, so
		// the claim stays and the output is skipped rather than written
		// twice.
		if cctx.Err() != nil {
			slog.ErrorContext(ctx, "flowengine: a create output timed out and is skipped",
				"run", run.ID, "state", cur.State, "output", i, "error", err)
			return claimed, wrap(err)
		}
		// The create reported its failure, so nothing was written: give
		// the claim back, and a later round writes it again. That is no
		// progress, so the failures carry over.
		if back, merr := e.mark(cctx, claimed, tok, cur, i, before.Failures); merr == nil && back.ID != "" {
			claimed = back
		}
		return claimed, wrap(err)
	}
	if lerr := e.d.Headers.LinkProduced(cctx, run.ID, created); lerr != nil {
		slog.WarnContext(ctx, "flowengine: a produced entity is not linked to its run",
			"run", run.ID, "entity", created.String(), "error", lerr)
	}
	return claimed, nil
}

// mark stores the pending mark of token tok, at cur, with done outputs
// written and failures failed attempts; progress passes 0. A lost
// compare-and-set returns a zero run.
func (e *Engine) mark(ctx context.Context, run flowrun.Run, tok string, cur flow.Current,
	done, failures int) (flowrun.Run, error) {
	p := &flowrun.Pending{State: cur.State, Generation: cur.Generation, Done: done, Failures: failures}
	if old := pendingOf(run, tok); old != nil && old.State == cur.State && old.Generation == cur.Generation {
		p.Job = old.Job // never drop an attempt
	}
	inst := instOf(run, tok)
	next, err := e.save(ctx, run, tok, inst, p, nil, e.now())
	if isLost(err) {
		return flowrun.Run{}, nil
	}
	return next, err
}

// rest clears the pending mark of token tok, which waits in its state,
// and makes its due time its next timer.
func (e *Engine) rest(ctx context.Context, f *flowcfg.Flow, run flowrun.Run, tok string,
	cur flow.Current) (flowrun.Run, error) {
	inst := instOf(run, tok)
	inst.NextDueAt = flow.NextDue(f.Definition(), cur)
	next, err := e.save(ctx, run, tok, inst, nil, nil, e.now())
	if isLost(err) {
		return flowrun.Run{}, nil
	}
	if err != nil {
		return run, err
	}
	return next, nil
}

// save stores the next state of token tok: inst with the pending mark p.
// It is the one way the engine updates a token (RR-SHLMJUL): it carries
// the timeline, adding step when set, and the token's step claim while
// the claim's generation is the instance's, so a move clears it. A branch
// the run no longer carries is a lost compare-and-set.
func (e *Engine) save(ctx context.Context, run flowrun.Run, tok string, inst flow.Instance, p *flowrun.Pending,
	step *flowrun.Step, at time.Time) (flowrun.Run, error) {
	st := cloneState(run)
	if step != nil {
		st.Log = flowrun.AppendLog(run.Log, *step)
	}
	_, _, c, ok := slot(run, tok)
	if !ok {
		return flowrun.Run{}, flowrun.ErrConflict
	}
	if c != nil && c.Generation != inst.Generation {
		c = nil
	}
	if tok == "" {
		st.Instance, st.Pending, st.Claim = inst, p, c
	} else {
		i := branchAt(run, tok)
		st.Branches[i].Instance, st.Branches[i].Pending, st.Branches[i].Claim = inst, p, c
	}
	return e.put(ctx, run, st, at)
}

// take moves token tok from cur along a branch to targets: one target is
// a move, several a fork. A branch that moves into its join or a final
// state arrives there; join ends the run in a final state once no
// branch has outputs left to write.
func (e *Engine) take(ctx context.Context, f *flowcfg.Flow, run flowrun.Run, tok string, cur flow.Current,
	targets []string, step flowrun.Step, now time.Time) (flowrun.Run, error) {
	if len(targets) > 1 {
		if tok != "" {
			return flowrun.Run{}, fmt.Errorf("%w: branch %s forks again", ErrStranded, tok)
		}
		return e.fork(ctx, f, run, cur, targets, step, now)
	}
	step.State = targets[0]
	if tok == "" {
		return e.move(ctx, f, run, tok, cur, step, now)
	}
	// Every join closes a fork, which flowcfg checks.
	ts, _ := f.State(step.State)
	if _, joins := ts.Join(); joins || ts.Final {
		st, ok := arrived(run, tok, cur, step, now)
		if !ok {
			return flowrun.Run{}, flowrun.ErrConflict
		}
		return e.put(ctx, run, st, now)
	}
	return e.move(ctx, f, run, tok, cur, step, now)
}

// move stores token tok's move into the state step names, made at now,
// with a pending mark that makes the run due at once, and records the new
// state of the run itself on the header; a branch's state is not the
// header's. step is the timeline entry; move fills in its time, the state
// left and the branch.
func (e *Engine) move(ctx context.Context, f *flowcfg.Flow, run flowrun.Run, tok string, cur flow.Current,
	step flowrun.Step, now time.Time) (flowrun.Run, error) {
	to := step.State
	step.At, step.From, step.Branch = now, cur.State, tok
	inst := cur.Enter(flow.Move{From: cur.State, To: to}, now, 0)
	inst.Generation = maxGen(run) + 1
	schedule(f.Definition(), cur, &inst, run.CreatedAt, now)
	p := &flowrun.Pending{State: to, Generation: inst.Generation}
	moved, err := e.save(ctx, run, tok, inst, p, &step, now)
	if err != nil {
		return flowrun.Run{}, err
	}
	if st, _ := f.State(to); tok == "" && !st.Final {
		e.patchHeader(ctx, run.ID, map[string]any{
			metamodel.FlowRunPropState:  to,
			metamodel.FlowRunPropStatus: string(st.Status),
		})
	}
	return moved, nil
}

// finish records status on the header and drops the run's state. The
// header comes first: a crash after it leaves a finished header over a run
// whose mark makes the next round finish it again, never a running header
// over no run.
func (e *Engine) finish(ctx context.Context, f *flowcfg.Flow, run flowrun.Run, status flow.Status) error {
	props := endProps(status, e.now())
	props[metamodel.FlowRunPropState] = flow.Read(f.Definition(), run.Instance, run.CreatedAt).State
	if err := e.d.Headers.PatchHeader(ctx, run.ID, props); err != nil {
		return fmt.Errorf("flowengine: record end of run %s: %w", run.ID, err)
	}
	if err := e.d.Runs.Delete(ctx, run.ID, run.Rev); err != nil && !isLost(err) {
		return err
	}
	return nil
}

func endProps(status flow.Status, at time.Time) map[string]any {
	return map[string]any{
		metamodel.FlowRunPropStatus:    string(status),
		metamodel.FlowRunPropCompleted: at,
	}
}

func (e *Engine) patchHeader(ctx context.Context, id flow.RunID, props map[string]any) {
	if err := e.d.Headers.PatchHeader(ctx, id, props); err != nil {
		slog.WarnContext(ctx, "flowengine: run header lags its run", "run", id, "error", err)
	}
}

// needsSubject reports whether a template of outs reads a subject property.
func needsSubject(outs []flowcfg.Output) bool {
	for _, o := range outs {
		for _, t := range o.Properties() {
			for _, ref := range t.Refs() {
				if ref != flowcfg.RefSubjectID && strings.HasPrefix(ref, "subject.") {
					return true
				}
			}
		}
	}
	return false
}

// evalProps evaluates o's templates. A template that names a subject
// property without a value leaves the property alone.
func evalProps(o flowcfg.Output, vars flowcfg.Vars) map[string]any {
	out := map[string]any{}
	for name, t := range o.Properties() {
		if v, ok := t.Eval(vars); ok {
			out[name] = v
		}
	}
	return out
}

// isLost reports a lost compare-and-set: another engine advanced or ended
// the run.
func isLost(err error) bool {
	return errors.Is(err, flowrun.ErrConflict) || errors.Is(err, flowrun.ErrNotFound)
}
