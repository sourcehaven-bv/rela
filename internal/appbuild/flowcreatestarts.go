package appbuild

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/flowengine"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// createStartEngine is what the create hook drives: [flowengine.OnCreate].
type createStartEngine interface {
	StartCreated(ctx context.Context, runs flowrun.Store, subj *entity.Entity) ([]flow.RunID, error)
	Settle(ctx context.Context, id flow.RunID) error
}

// settleTimeout bounds the settling of the runs a create started, after
// the create has committed. A run it cuts off keeps its pending mark, and
// the flow worker settles it.
const settleTimeout = 30 * time.Second

// maxCreateStartHops bounds a chain of create starts: a run whose outputs
// create an entity that starts another run, and so on. A flow whose create
// output makes an entity of its own subject type would otherwise start
// runs without end. Past the bound a create starts no run, with a log line.
const maxCreateStartHops = 8

// createStartHopsKey carries, on the ctx of a settle, how many create
// starts led to it.
type createStartHopsKey struct{}

func createStartHops(ctx context.Context) int {
	n, _ := ctx.Value(createStartHopsKey{}).(int)
	return n
}

// flowCreateStarts is the manager's [entitymanager.CreateHook] for the
// flows that start on create (`on: create`). Inside the create's store
// transaction it starts their runs on the new entity, through the run store
// joined to that transaction where the backend has one (postgres, sqlite).
// The header, the run and the entity then commit or roll back together, so
// a create that fails starts no run, and a create happens once, so no
// retry or repeated event starts a second run. After the commit it settles
// the runs: it writes the initial state's outputs and follows its `done`
// branches, as a manual start does.
//
// On the fs and memory tiers nothing rolls back, so a create that fails
// after its row was written keeps its run, as it keeps its row.
//
// The engine needs the entity manager, which needs this hook, so the
// engine is set once it is built (see finishAssembly). A create before
// that fails rather than skip its runs.
type flowCreateStarts struct {
	types map[string]bool
	runs  flowrun.Store
	// join is nil when the run store cannot join a store transaction.
	join joinFlowRuns
	eng  atomic.Pointer[createStartEngineBox]
}

// createStartEngineBox lets an interface value sit in an atomic.Pointer.
type createStartEngineBox struct{ e createStartEngine }

// newFlowCreateStarts returns the hook for the create starts of flows, over
// runs. The result is a nil pointer when no flow starts on create.
//
// Nil: flows is accepted (no flows) and gives no hook; runs is rejected by
// the caller, which builds it with the flows.
func newFlowCreateStarts(flows *flowcfg.Compiled, st store.Store, runs flowrun.Store) *flowCreateStarts {
	types := map[string]bool{}
	for _, f := range flows.Flows() {
		for _, s := range f.Starts() {
			if s.On == flowcfg.StartCreate {
				types[f.Subject()] = true
			}
		}
	}
	if len(types) == 0 || runs == nil {
		return nil
	}
	return &flowCreateStarts{types: types, runs: runs, join: joinFlowRunsFor(st, runs)}
}

// hook returns h as the manager's hook. The result is a nil interface,
// not a typed nil, so the manager's nil check holds.
//
// Nil: h is accepted (no create starts) and gives no hook.
func (h *flowCreateStarts) hook() entitymanager.CreateHook {
	if h == nil {
		return nil
	}
	return h
}

// setEngine connects the engine once it is built.
//
// Nil: h is accepted (no create starts).
func (h *flowCreateStarts) setEngine(eng *flowengine.Engine) {
	if h == nil || eng == nil {
		return
	}
	h.eng.Store(&createStartEngineBox{e: flowengine.NewOnCreate(eng)})
}

// errFlowEngineNotReady is a watched create before the engine was set.
var errFlowEngineNotReady = errors.New("flows: the flow engine is not ready; a flow starts on this create")

// Watches implements [entitymanager.CreateHook].
func (h *flowCreateStarts) Watches(typ string) bool { return h.types[typ] }

// EntityCreated implements [entitymanager.CreateHook]. The runs are
// started as the flow's run_as, which the engine records as the By of
// each start; the header writes are system:flow's, as every header's.
func (h *flowCreateStarts) EntityCreated(
	ctx context.Context, view store.Store, e *entity.Entity,
) (func(context.Context), error) {
	box := h.eng.Load()
	if box == nil {
		return nil, errFlowEngineNotReady
	}
	hops := createStartHops(ctx)
	if hops >= maxCreateStartHops {
		slog.WarnContext(ctx, "flows: no run started on create: too many create starts in a chain",
			"entity", e.ID, "hops", hops)
		return nil, nil
	}
	runs := h.runs
	if h.join != nil {
		var err error
		if runs, err = h.join(view); err != nil {
			return nil, err
		}
	}
	ids, err := box.e.StartCreated(ctx, runs, e)
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	return func(ctx context.Context) {
		h.settle(context.WithValue(ctx, createStartHopsKey{}, hops+1), box.e, ids)
	}, nil
}

// settle settles ids after the create committed. It does not depend on
// the creator staying connected, and a failure only leaves the run to the
// flow worker, so it is logged rather than returned.
func (h *flowCreateStarts) settle(ctx context.Context, eng createStartEngine, ids []flow.RunID) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), settleTimeout)
	defer cancel()
	ctx = principal.With(ctx, principal.Principal{User: principal.UserFlow, Tool: principal.ToolFlow})
	for _, id := range ids {
		if err := eng.Settle(ctx, id); err != nil {
			slog.WarnContext(ctx, "flows: a run started on create did not settle; the flow worker retries it",
				"run", id, "error", err)
		}
	}
}
