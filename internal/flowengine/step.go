package flowengine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

// settle runs run until it waits. While the run carries a pending mark it
// writes the current state's outputs, finishes the run on a final state,
// and follows the first `done` branch whose guard holds; when no branch
// applies it clears the mark. flowcfg refuses a cycle of done branches, so
// the loop ends; the bound is a backstop.
//
// It returns the run as last stored, or a zero run when another engine
// took it over or the run ended. On an error the returned run is the one
// to back off: its mark is kept, so a later round settles it again.
func (e *Engine) settle(ctx context.Context, f *flowcfg.Flow, run flowrun.Run) (flowrun.Run, error) {
	def := f.Definition()
	for range flow.MaxStates + 1 {
		if run.Pending == nil {
			return run, nil
		}
		cur := flow.Read(def, run.Instance, run.CreatedAt)
		if cur.Stranded {
			return run, fmt.Errorf("%w: state %q", ErrStranded, cur.State)
		}
		st, _ := f.State(cur.State)
		var err error
		if run, err = e.writeOutputs(ctx, f, run, cur, st); err != nil || run.ID == "" {
			return run, err
		}
		if st.Final {
			if ferr := e.finish(ctx, f, run, st.Status); ferr != nil {
				return run, ferr
			}
			return flowrun.Run{}, nil
		}
		to, ok, err := e.doneTarget(ctx, f, run, cur)
		if err != nil {
			return run, err
		}
		if !ok {
			return e.rest(ctx, f, run, cur)
		}
		moved, err := e.move(ctx, f, run, cur, flowrun.Step{State: to, Via: flowrun.ViaDone}, e.now())
		if err != nil {
			if isLost(err) {
				return flowrun.Run{}, nil
			}
			return run, err
		}
		run = moved
	}
	return run, fmt.Errorf("flowengine: run %s did not settle", run.ID)
}

// writeOutputs writes the outputs the run's pending mark leaves for the
// current state, recording progress after each. It returns the run as
// stored after the last write, or a zero run when another engine took the
// run over.
func (e *Engine) writeOutputs(ctx context.Context, f *flowcfg.Flow, run flowrun.Run, cur flow.Current,
	st flowcfg.State) (flowrun.Run, error) {
	p := run.Pending
	outs := st.Outputs()
	if p.State != cur.State || p.Generation != cur.Generation {
		// A mark that names a state the run left: never write the current
		// state's outputs from it, but settle the state.
		return e.mark(ctx, run, cur, len(outs))
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
		if run, err = e.writeOutput(ctx, f, run, cur, outs[i], i, vars); err != nil || run.ID == "" {
			return run, err
		}
	}
	return run, nil
}

// writeOutput writes output o, number i of the current state, and records
// it in the pending mark.
func (e *Engine) writeOutput(ctx context.Context, f *flowcfg.Flow, run flowrun.Run, cur flow.Current,
	o flowcfg.Output, i int, vars flowcfg.Vars) (flowrun.Run, error) {
	props := evalProps(o, vars)
	wrap := func(err error) error {
		return fmt.Errorf("flowengine: run %s state %s output %d: %w", run.ID, cur.State, i, err)
	}
	if o.Kind != flowcfg.OutputCreate {
		// A set is idempotent: written first, marked after, so a crash
		// between the two writes it again.
		if err := e.d.Outputs.Set(ctx, f.RunAs(), run.Subject, props); err != nil {
			return run, wrap(err)
		}
		return e.mark(ctx, run, cur, i+1)
	}
	// Claim first: a crash after the claim skips the create rather than
	// writing it twice.
	claimed, err := e.mark(ctx, run, cur, i+1)
	if err != nil || claimed.ID == "" {
		return claimed, err
	}
	created, err := e.d.Outputs.Create(ctx, f.RunAs(), o.Type, props, o.Relate, run.Subject)
	if err != nil {
		// The create reported its failure, so nothing was written: give
		// the claim back, and a later round writes it again.
		if back, merr := e.mark(ctx, claimed, cur, i); merr == nil && back.ID != "" {
			claimed = back
		}
		return claimed, wrap(err)
	}
	if lerr := e.d.Headers.LinkProduced(ctx, run.ID, created); lerr != nil {
		slog.WarnContext(ctx, "flowengine: a produced entity is not linked to its run",
			"run", run.ID, "entity", created.String(), "error", lerr)
	}
	return claimed, nil
}

// mark stores the pending mark for the current state with done outputs
// written. Progress resets the failure count. A lost compare-and-set
// returns a zero run.
func (e *Engine) mark(ctx context.Context, run flowrun.Run, cur flow.Current, done int) (flowrun.Run, error) {
	p := &flowrun.Pending{State: cur.State, Generation: cur.Generation, Done: done}
	next, err := e.save(ctx, run, run.Instance, p, nil, e.now())
	if isLost(err) {
		return flowrun.Run{}, nil
	}
	return next, err
}

// rest clears the pending mark of a run that waits in its state, and makes
// its due time its next timer.
func (e *Engine) rest(ctx context.Context, f *flowcfg.Flow, run flowrun.Run, cur flow.Current) (flowrun.Run, error) {
	inst := run.Instance
	inst.NextDueAt = flow.NextDue(f.Definition(), cur)
	next, err := e.save(ctx, run, inst, nil, nil, e.now())
	if isLost(err) {
		return flowrun.Run{}, nil
	}
	if err != nil {
		return run, err
	}
	return next, nil
}

// save stores the run's next state: inst with the pending mark p. It is the
// one way the engine updates a run (RR-SHLMJUL): it carries the timeline,
// adding step when set, and the step claim while the claim's generation is
// the instance's, so a move clears it.
func (e *Engine) save(ctx context.Context, run flowrun.Run, inst flow.Instance, p *flowrun.Pending,
	step *flowrun.Step, at time.Time) (flowrun.Run, error) {
	st := flowrun.State{Instance: inst, Pending: p, Log: run.Log}
	if step != nil {
		st.Log = flowrun.AppendLog(run.Log, *step)
	}
	if c := run.Claim; c != nil && c.Generation == inst.Generation {
		st.Claim = c
	}
	return e.d.Runs.Update(ctx, run.ID, run.Rev, st, at)
}

// move stores the run's move into the state step names, made at now, with
// a pending mark that makes the run due at once, and records the new state
// on the header. step is the timeline entry; move fills in its time and the
// state left.
func (e *Engine) move(ctx context.Context, f *flowcfg.Flow, run flowrun.Run, cur flow.Current,
	step flowrun.Step, now time.Time) (flowrun.Run, error) {
	def := f.Definition()
	to := step.State
	step.At, step.From = now, cur.State
	inst := cur.Enter(flow.Move{From: cur.State, To: to}, now, 0)
	next := flow.Read(def, inst, run.CreatedAt)
	sched := flow.StepSchedule(def, cur, next)
	inst.NextDueAt, inst.Restarts = now, sched.Restarts
	if sched.Reminders != nil {
		inst.Reminders = sched.Reminders
	}
	p := &flowrun.Pending{State: to, Generation: inst.Generation}
	moved, err := e.save(ctx, run, inst, p, &step, now)
	if err != nil {
		return flowrun.Run{}, err
	}
	if st, _ := f.State(to); !st.Final {
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

// doneTarget returns the state the first `done` branch of cur whose guard
// holds moves to. A guard that fails to evaluate stops the run where it
// is, as in [flow.Decide]; its mark stays, so a later round tries again.
func (e *Engine) doneTarget(ctx context.Context, f *flowcfg.Flow, run flowrun.Run,
	cur flow.Current) (to string, ok bool, err error) {
	branches := f.Branches(cur.State, flowcfg.EventDone)
	if len(branches) == 0 {
		return "", false, nil
	}
	var subj *entity.Entity
	holds := func(b flowcfg.Branch) (bool, error) {
		if b.When == nil {
			return true, nil
		}
		if subj == nil {
			var err error
			if subj, err = e.d.Subjects.Subject(ctx, run.Subject); err != nil {
				return false, err
			}
		}
		return e.guardHolds(ctx, b.When, subj)
	}
	d := flow.Decide(cur.State, branches, flow.Rules[flowcfg.Branch]{
		Admits: func(flowcfg.Branch) bool { return true },
		Holds:  holds,
		Target: func(b flowcfg.Branch) string { return b.To },
	})
	switch d.Outcome {
	case flow.Moved:
		return d.Move.To, true, nil
	case flow.GuardFailed:
		return "", false, fmt.Errorf("flowengine: run %s: done guard %q: %w", run.ID, d.Branch.When.Source(), d.Err)
	default:
		return "", false, nil
	}
}

// guardHolds evaluates g over subj, bound as `entity`.
func (e *Engine) guardHolds(ctx context.Context, g *flowcfg.Guard, subj *entity.Entity) (bool, error) {
	def, ok := e.d.Meta.GetEntityDef(subj.Type)
	if !ok {
		return false, fmt.Errorf("%w: type %q", ErrSubjectNotFound, subj.Type)
	}
	b := predicate.NewBindings()
	err := b.SetVar("entity", predicatefns.EntityRecord(e.d.Meta, def, subj.ID, subj.Type, subj.Properties))
	if err == nil {
		err = predicatefns.Bind(b, e.now())
	}
	var v predicate.Value
	if err == nil {
		v, err = g.Program().Eval(ctx, b)
	}
	if err != nil {
		return false, err
	}
	got, isBool := v.(predicate.Bool)
	return isBool && got.Bool(), nil
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
