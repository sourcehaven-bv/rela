package flowengine

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

// guards evaluates the `when:` conditions of a flow over the raw subject:
// branch guards, `done` branches and join conditions.
type guards struct {
	meta     *metamodel.Metamodel
	subjects Subjects
	now      func() time.Time
}

// doneBranch returns the first `done` branch of cur whose guard holds. A
// guard that fails to evaluate stops the token where it is, as in
// [flow.Decide]; its mark stays, so a later round tries again.
func (g guards) doneBranch(ctx context.Context, f *flowcfg.Flow, run flowrun.Run,
	cur flow.Current) (b flowcfg.Branch, ok bool, err error) {
	branches := f.Branches(cur.State, flowcfg.EventDone)
	if len(branches) == 0 {
		return b, false, nil
	}
	var subj *entity.Entity
	holds := func(b flowcfg.Branch) (bool, error) {
		if b.When == nil {
			return true, nil
		}
		if subj == nil {
			var err error
			if subj, err = g.subjects.Subject(ctx, run.Subject); err != nil {
				return false, err
			}
		}
		return g.holds(ctx, b.When, subj)
	}
	d := flow.Decide(cur.State, branches, flow.Rules[flowcfg.Branch]{
		Admits: func(flowcfg.Branch) bool { return true },
		Holds:  holds,
		Target: func(b flowcfg.Branch) string { return b.To },
	})
	switch d.Outcome {
	case flow.Moved:
		return d.Branch, true, nil
	case flow.GuardFailed:
		return b, false, fmt.Errorf("flowengine: run %s: done guard %q: %w", run.ID, d.Branch.When.Source(), d.Err)
	default:
		return b, false, nil
	}
}

// holds evaluates w over subj, bound as `entity`.
func (g guards) holds(ctx context.Context, w *flowcfg.Guard, subj *entity.Entity) (bool, error) {
	def, ok := g.meta.GetEntityDef(subj.Type)
	if !ok {
		return false, fmt.Errorf("%w: type %q", ErrSubjectNotFound, subj.Type)
	}
	b := predicate.NewBindings()
	err := b.SetVar("entity", predicatefns.EntityRecord(g.meta, def, subj.ID, subj.Type, subj.Properties))
	if err == nil {
		err = predicatefns.Bind(b, g.now())
	}
	var v predicate.Value
	if err == nil {
		v, err = w.Program().Eval(ctx, b)
	}
	if err != nil {
		return false, err
	}
	got, isBool := v.(predicate.Bool)
	return isBool && got.Bool(), nil
}

// joinHolds reports whether the join state joinName lets the run continue
// with arrived of its branches at the join.
func (g guards) joinHolds(ctx context.Context, f *flowcfg.Flow, run flowrun.Run, joinName string,
	arrived int) bool {
	if arrived == len(run.Branches) {
		return true
	}
	js, _ := f.State(joinName)
	j, _ := js.Join()
	if j.Any {
		return true
	}
	if j.When == nil {
		return false
	}
	ok, err := g.joinGuard(ctx, f, run, j.When, arrived)
	if err != nil {
		// A join that cannot be evaluated waits; the last arrival
		// continues the run regardless.
		slog.WarnContext(ctx, "flowengine: a join condition failed", "run", run.ID, "state", joinName,
			"error", err)
		return false
	}
	return ok
}

// joinGuard evaluates a join's `when:` over the raw subject, with arrived
// and branches bound.
func (g guards) joinGuard(ctx context.Context, f *flowcfg.Flow, run flowrun.Run, w *flowcfg.Guard,
	arrived int) (bool, error) {
	subj, err := g.subjects.Subject(ctx, run.Subject)
	if err != nil {
		return false, err
	}
	def, ok := g.meta.GetEntityDef(subj.Type)
	if !ok || subj.Type != f.Subject() {
		return false, fmt.Errorf("%w: type %q", ErrSubjectNotFound, subj.Type)
	}
	b := predicate.NewBindings()
	err = b.SetVar("entity", predicatefns.EntityRecord(g.meta, def, subj.ID, subj.Type, subj.Properties))
	if err == nil {
		err = b.SetVar(flowcfg.VarArrived, predicate.NewInt(int64(arrived)))
	}
	if err == nil {
		err = b.SetVar(flowcfg.VarBranches, predicate.NewInt(int64(len(run.Branches))))
	}
	if err == nil {
		err = predicatefns.Bind(b, g.now())
	}
	var v predicate.Value
	if err == nil {
		v, err = w.Program().Eval(ctx, b)
	}
	if err != nil {
		return false, err
	}
	got, isBool := v.(predicate.Bool)
	return isBool && got.Bool(), nil
}
