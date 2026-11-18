package flowengine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
)

// OnCreate starts the runs of the flows that start on the create of their
// subject (`on: create`), and settles them once the create has committed.
// It is a type of its own so that [Engine] stays within its method cap.
type OnCreate struct{ e *Engine }

// NewOnCreate returns the create starts of e.
//
// Nil: e rejected; the caller passes the engine it built.
func NewOnCreate(e *Engine) *OnCreate { return &OnCreate{e: e} }

// StartCreated starts a run of every flow that starts on the create of
// subj (`on: create`) and whose `when:`, if any, subj satisfies. A flow
// with several such starts gets one run.
//
// It runs inside the create's store transaction, so the runs commit or
// roll back with subj: runs is the run store on that transaction, and ctx
// carries the transaction, which the header writes join. Each run is stored
// with its initial state pending. [OnCreate.Settle] settles it once the
// transaction commits; if that never happens, a round of [Engine.Due] does.
// Because the start is part of the create, a retried or repeated delivery
// of anything cannot start a second run: the create itself happens once.
//
// Each run is started by the flow's run_as: it is the By of the start. A
// `when:` that fails to evaluate, and a subject that already carries a run
// of a per_subject: one flow, skip that flow with a log line rather than
// fail the create. Any other error fails the create.
func (o *OnCreate) StartCreated(ctx context.Context, runs flowrun.Store, subj *entity.Entity) ([]flow.RunID, error) {
	e := o.e
	var ids []flow.RunID
	for _, f := range e.d.Flows.Flows() {
		if f.Subject() != subj.Type {
			continue
		}
		starts := createStarts(f)
		if len(starts) == 0 || !o.anyHolds(ctx, f, starts, subj) {
			continue
		}
		fctx := audit.WithFlowStep(ctx, audit.FlowStep{By: f.RunAs()})
		run, err := begin(fctx, e, runs, f, subj)
		if errors.Is(err, ErrSubjectBusy) {
			slog.WarnContext(ctx, "flowengine: no run started on create: the subject already carries one",
				"flow", f.Name(), "subject", subj.ID)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("flowengine: start flow %s on the create of %s: %w", f.Name(), subj.ID, err)
		}
		ids = append(ids, run.ID)
	}
	return ids, nil
}

// createStarts returns the `on: create` starts of f.
func createStarts(f *flowcfg.Flow) []flowcfg.Start {
	var out []flowcfg.Start
	for _, s := range f.Starts() {
		if s.On == flowcfg.StartCreate {
			out = append(out, s)
		}
	}
	return out
}

// anyHolds reports whether subj satisfies one of starts. A start without
// `when:` always holds; one whose `when:` fails to evaluate does not.
func (o *OnCreate) anyHolds(ctx context.Context, f *flowcfg.Flow, starts []flowcfg.Start, subj *entity.Entity) bool {
	for _, s := range starts {
		if s.When == nil {
			return true
		}
		ok, err := o.e.g.holds(ctx, s.When, subj)
		if err != nil {
			// The error can quote a property value of the subject, so it
			// stays at debug level.
			slog.WarnContext(ctx, "flowengine: a create start condition failed", "flow", f.Name(),
				"subject", subj.ID, "when", s.When.Source())
			slog.DebugContext(ctx, "flowengine: create start condition error", "flow", f.Name(),
				"subject", subj.ID, "error", err)
			continue
		}
		if ok {
			return true
		}
	}
	return false
}

// Settle settles the run id now, as a due round would: it writes the
// outputs of the state it waits to settle and follows its `done` branches.
// A run that is gone or has no pending mark is left alone. A run that
// fails to settle is backed off, so a later round tries again.
func (o *OnCreate) Settle(ctx context.Context, id flow.RunID) error {
	e := o.e
	run, err := e.d.Runs.Get(ctx, id)
	if errors.Is(err, flowrun.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if run.Pending == nil || forked(run) {
		return nil
	}
	f, err := e.flowOf(ctx, run)
	if err != nil {
		return err
	}
	left, err := e.settle(ctx, f, run, "")
	if err != nil {
		e.backoff(ctx, left)
	}
	return err
}
