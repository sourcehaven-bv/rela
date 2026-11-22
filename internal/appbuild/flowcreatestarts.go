package appbuild

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/flowengine"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// createStartEngine is what the create hook drives: [flowengine.OnCreate].
type createStartEngine interface {
	StartCreated(ctx context.Context, runs flowrun.Store, subj *entity.Entity, depth int) ([]flow.RunID, error)
	Settle(ctx context.Context, id flow.RunID) error
}

// settleTimeout bounds the settling of the runs a create started, after
// the create has committed. A run it cuts off keeps its pending mark, and
// the flow worker settles it.
const settleTimeout = 30 * time.Second

// maxCreateDepth bounds a chain of create starts: a run whose outputs
// create an entity that starts another run, and so on. A flow whose create
// output makes an entity of its own subject type would otherwise start
// runs without end. Each run started on create stores its depth on its
// header ([metamodel.FlowRunPropDepth]): one more than the run whose
// output made the create, read through the create's [audit.FlowStep].
// The bound holds however the runs settle, at once or by the flow worker.
// A create that would start a run at the bound starts none, with a log
// line.
const maxCreateDepth = 8

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

// createStartEngineBox lets the engine and its identity binding sit in an
// atomic.Pointer.
type createStartEngineBox struct {
	e createStartEngine
	// policy is nil when no acl.yaml is configured.
	policy *acl.Declarative
	as     asIdentity
}

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

// setEngine connects the engine once it is built, with the policy and the
// binder the engine writes its identities with.
//
// Nil: h is accepted (no create starts); policy is accepted (no acl.yaml).
func (h *flowCreateStarts) setEngine(eng *flowengine.Engine, policy *acl.Declarative, as asIdentity) {
	if h == nil || eng == nil {
		return
	}
	h.eng.Store(&createStartEngineBox{e: flowengine.NewOnCreate(eng), policy: policy, as: as})
}

// errFlowEngineNotReady is a watched create before the engine was set.
var errFlowEngineNotReady = errors.New("flows: the flow engine is not ready; a flow starts on this create")

// Watches implements [entitymanager.CreateHook].
func (h *flowCreateStarts) Watches(typ string) bool { return h.types[typ] }

// PrepareCreate implements [entitymanager.CreateHook]. It binds
// system:flow, who writes the run headers, before the create's
// transaction opens, so that the membership walk does not read the store
// on another connection while the transaction holds its own (see
// [flowHeaders.engine]). The Request is good for this create only: ctx
// carries it no further than the create's own transaction.
func (h *flowCreateStarts) PrepareCreate(ctx context.Context) (context.Context, error) {
	box := h.eng.Load()
	if box == nil {
		return nil, errFlowEngineNotReady
	}
	if box.policy == nil {
		return ctx, nil
	}
	bound, err := box.as(ctx, principal.UserFlow)
	if err != nil {
		return nil, err
	}
	req, err := box.policy.RequestFrom(bound)
	if err != nil {
		return nil, err
	}
	return context.WithValue(ctx, preboundFlowKey{}, req), nil
}

// flowPrincipal is system:flow, who writes run headers.
var flowPrincipal = principal.Principal{User: principal.UserFlow, Tool: principal.ToolFlow}

// preboundFlowKey carries the Request of system:flow that
// [flowCreateStarts.PrepareCreate] bound for one create.
type preboundFlowKey struct{}

// preboundFlow returns the Request PrepareCreate bound on ctx, or nil.
func preboundFlow(ctx context.Context) *acl.Request {
	req, _ := ctx.Value(preboundFlowKey{}).(*acl.Request)
	return req
}

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
	depth, err := createDepth(ctx, view)
	if err != nil {
		return nil, err
	}
	if depth >= maxCreateDepth {
		slog.WarnContext(ctx, "flows: no run started on create: too many create starts in a chain",
			"entity", e.ID, "depth", depth)
		return nil, nil //nolint:nilnil // no run started, so there is nothing to settle and no error
	}
	runs := h.runs
	if h.join != nil {
		if runs, err = h.join(view); err != nil {
			return nil, err
		}
	}
	ids, err := box.e.StartCreated(ctx, runs, e, depth)
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	// The settle runs on its own ctx as the flow principal, not on the
	// creator's (RR-G0JC9QS).
	return func(context.Context) { settleStarted(box.e, ids) }, nil //nolint:contextcheck // see above
}

// createDepth returns the depth of a run started by the create on ctx: 0
// for a create no flow made, and one more than the producing run's depth
// for a create a run's output made. The producing run is read on view.
func createDepth(ctx context.Context, view store.Store) (int, error) {
	step, ok := audit.FlowStepFrom(ctx)
	if !ok || step.Run == "" {
		return 0, nil
	}
	ref, err := entity.ParseRef(step.Run)
	if err != nil {
		return 0, err
	}
	hdr, err := view.GetEntity(ctx, ref)
	if errors.Is(err, store.ErrNotFound) {
		// The producing run is gone. Count it as the deepest run, so a
		// chain can never restart its count by losing a link.
		return maxCreateDepth, nil
	}
	if err != nil {
		return 0, fmt.Errorf("flows: read the run %s that made the create: %w", step.Run, err)
	}
	return depthOf(hdr.Properties[metamodel.FlowRunPropDepth]) + 1, nil
}

// depthOf reads a stored depth. An absent value is 0. The stores hand
// back the integer they were given, an int64, or a float64 from JSON.
func depthOf(v any) int {
	switch d := v.(type) {
	case int:
		return d
	case int64:
		return int(d)
	case float64:
		return int(d)
	default:
		return 0
	}
}

// settleStarted settles ids after the create committed. It runs on a ctx
// of its own, not the creator's: it does not depend on the creator staying
// connected, and it carries no value of the creator's request (their
// Request, read world or read time). A failure only leaves the run to the
// flow worker, so it is logged rather than returned.
func settleStarted(eng createStartEngine, ids []flow.RunID) {
	ctx, cancel := context.WithTimeout(context.Background(), settleTimeout)
	defer cancel()
	ctx = principal.With(ctx, flowPrincipal)
	for _, id := range ids {
		if err := eng.Settle(ctx, id); err != nil {
			slog.WarnContext(ctx, "flows: a run started on create did not settle; the flow worker retries it",
				"run", id)
			slog.DebugContext(ctx, "flows: settle error", "run", id, "error", err)
		}
	}
}
