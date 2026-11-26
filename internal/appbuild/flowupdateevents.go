package appbuild

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/flowengine"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// writeEventEngine is what the update hook drives: [flowengine.OnChange].
type writeEventEngine interface {
	Changed(ctx context.Context, w flowengine.Write) (flowengine.Changes, error)
	Settle(ctx context.Context, id flow.RunID) error
	MayStart(typ string, changed map[string]bool) bool
}

// flowUpdateEvents is the manager's [entitymanager.UpdateHook] for flows.
// Inside an update's store transaction it starts the runs of the flows
// whose `on: predicate` start the write makes the subject come to match,
// and fires the `change` event on the subject's open runs, through the
// run store joined to that transaction where the backend has one. The
// moves and starts commit or roll back with the write, and a write
// happens once, so no retry or repeated delivery moves a run twice. After
// the commit it settles the runs it started or moved.
//
// The author of the event is the write's principal. Its roles on the
// subject are read before the transaction opens; without that preparation
// (an update inside a transaction its caller opened) no role is held. A
// run's own writes (outputs, jobs) fire no event; a person's step write
// fires none on the run whose step it completes, which takes the event
// itself.
//
// The engine needs the entity manager, which needs this hook, so the
// engine is set once it is built (see finishAssembly).
type flowUpdateEvents struct {
	types      map[string]bool
	predicates map[string]bool
	runs       flowrun.Store
	// join is nil when the run store cannot join a store transaction.
	join joinFlowRuns
	eng  atomic.Pointer[updateEventBox]
}

// updateEventBox lets the engine and its identity binding sit in an
// atomic.Pointer.
type updateEventBox struct {
	e writeEventEngine
	// policy is nil when no acl.yaml is configured.
	policy *acl.Declarative
	as     asIdentity
}

// newFlowUpdateEvents returns the hook for the write events of flows over
// runs. It watches every flow's subject type, since a run follows the
// version of its flow it started on. The result is a nil pointer when no
// flow is declared.
//
// Nil: flows is accepted (no flows) and gives no hook; runs is rejected by
// the caller, which builds it with the flows.
func newFlowUpdateEvents(flows *flowcfg.Compiled, st store.Store, runs flowrun.Store) *flowUpdateEvents {
	types, predicates := map[string]bool{}, map[string]bool{}
	for _, f := range flows.Flows() {
		types[f.Subject()] = true
		for _, s := range f.Starts() {
			if s.On == flowcfg.StartPredicate {
				predicates[f.Subject()] = true
			}
		}
	}
	if len(types) == 0 || runs == nil {
		return nil
	}
	return &flowUpdateEvents{types: types, predicates: predicates, runs: runs, join: joinFlowRunsFor(st, runs)}
}

// hook returns h as the manager's hook. The result is a nil interface,
// not a typed nil, so the manager's nil check holds.
//
// Nil: h is accepted (no flows) and gives no hook.
func (h *flowUpdateEvents) hook() entitymanager.UpdateHook {
	if h == nil {
		return nil
	}
	return h
}

// setEngine connects the engine once it is built, with the policy and the
// binder the engine writes its identities with.
//
// Nil: h is accepted (no flows); policy is accepted (no acl.yaml).
func (h *flowUpdateEvents) setEngine(eng *flowengine.Engine, policy *acl.Declarative, as asIdentity) {
	if h == nil || eng == nil {
		return
	}
	h.eng.Store(&updateEventBox{e: flowengine.NewOnChange(eng), policy: policy, as: as})
}

// Watches implements [entitymanager.UpdateHook].
func (h *flowUpdateEvents) Watches(typ string) bool { return h.types[typ] }

// preparedUpdate is what [flowUpdateEvents.PrepareUpdate] read for one
// update, before its transaction.
type preparedUpdate struct {
	subject entity.Ref
	// idle: the update can start no run and its subject has no open run
	// it may move, so it fires nothing.
	idle bool
	// all: every role counts as held (no acl.yaml).
	all   bool
	roles map[string]bool
}

type preparedUpdateKey struct{}

// PrepareUpdate implements [entitymanager.UpdateHook]. Before the
// transaction opens it reads what the event needs from the store: whether
// the subject has open runs, the author's roles on the subject, and the
// Request of system:flow, who writes the run headers. An update that can
// start no run and move none needs none of it. A failure to read the
// author's roles holds no role; a failure to list the runs or to bind
// system:flow fails the update, since its event could not be stored with it.
func (h *flowUpdateEvents) PrepareUpdate(ctx context.Context, old, e *entity.Entity) (context.Context, error) {
	box := h.eng.Load()
	if box == nil {
		return nil, errFlowEngineNotReady
	}
	if old == nil {
		old = &entity.Entity{}
	}
	p := &preparedUpdate{subject: e.Ref(), all: box.policy == nil, roles: map[string]bool{}}
	starts := box.e.MayStart(e.Type, flowengine.ChangedProps(old, e))
	open := false
	if !machineWrite(ctx) {
		runs, err := h.runs.List(ctx, flowrun.Query{SubjectID: e.ID, Limit: 1})
		if err != nil {
			return nil, err
		}
		open = len(runs) > 0
	}
	p.idle = !starts && !open
	if p.idle || box.policy == nil {
		return context.WithValue(ctx, preparedUpdateKey{}, p), nil
	}
	if req, err := box.policy.RequestFrom(ctx); open && err == nil {
		for _, a := range req.ForEntity(ctx, "", e.ID) {
			p.roles[a.Role] = true
		}
	}
	bound, err := box.as(ctx, principal.UserFlow)
	if err != nil {
		return nil, fmt.Errorf("flows: bind system:flow for a write event: %w", err)
	}
	flowReq, err := box.policy.RequestFrom(bound)
	if err != nil {
		return nil, fmt.Errorf("flows: bind system:flow for a write event: %w", err)
	}
	ctx = context.WithValue(ctx, preboundFlowKey{}, flowReq)
	return context.WithValue(ctx, preparedUpdateKey{}, p), nil
}

// machineWrite reports whether ctx carries a write a run made itself: an
// output or a job, not the step write of a person.
func machineWrite(ctx context.Context) bool {
	step, ok := audit.FlowStepFrom(ctx)
	return ok && step.Run != "" && stepWriteOf(ctx) != flow.RunID(step.Run)
}

// prepared returns what PrepareUpdate read for subject on ctx, or nil.
func prepared(ctx context.Context, subject entity.Ref) *preparedUpdate {
	p, _ := ctx.Value(preparedUpdateKey{}).(*preparedUpdate)
	if p == nil || p.subject != subject {
		return nil
	}
	return p
}

// holder returns the role check for the author of an update. Without
// preparation it holds every role only when there is no acl.yaml.
func (p *preparedUpdate) holder(policy *acl.Declarative) func(string) bool {
	switch {
	case p == nil:
		all := policy == nil
		return func(string) bool { return all }
	case p.all:
		return func(string) bool { return true }
	default:
		return func(r string) bool { return p.roles[r] }
	}
}

// EntityUpdated implements [entitymanager.UpdateHook].
func (h *flowUpdateEvents) EntityUpdated(
	ctx context.Context, view store.Store, old, e *entity.Entity,
) (func(context.Context), error) {
	box := h.eng.Load()
	if box == nil {
		return nil, errFlowEngineNotReady
	}
	p := prepared(ctx, e.Ref())
	if p != nil && p.idle {
		return nil, nil //nolint:nilnil // nothing to fire, so nothing to settle and no error
	}
	depth, err := createDepth(ctx, view)
	if err != nil {
		return nil, err
	}
	runs := h.runs
	if h.join != nil {
		if runs, err = h.join(view); err != nil {
			return nil, err
		}
	}
	w := flowengine.Write{
		Runs: runs, Old: old, New: e, By: principal.From(ctx).User, Holds: p.holder(box.policy),
		Depth: depth, Starts: depth < maxCreateDepth, Machine: machineWrite(ctx),
	}
	if step, ok := audit.FlowStepFrom(ctx); ok && step.Run != "" && !w.Machine {
		w.Skip = flow.RunID(step.Run)
	}
	if !w.Starts && h.predicates[e.Type] {
		slog.WarnContext(ctx, "flows: no run started on a predicate: too many starts in a chain",
			"entity", e.ID, "depth", depth)
	}
	ch, err := box.e.Changed(ctx, w)
	if err != nil || len(ch.Settle) == 0 {
		return nil, err
	}
	return func(c context.Context) { settleStarted(c, box.e, ch.Settle) }, nil
}

// stepWriteKey marks the write a person makes to complete a step of a
// run: [FlowControl] completes it and fires its event itself.
type stepWriteKey struct{}

// withStepWrite marks ctx as carrying the step write of run id.
func withStepWrite(ctx context.Context, id flow.RunID) context.Context {
	return context.WithValue(ctx, stepWriteKey{}, id)
}

// stepWriteOf returns the run whose step write ctx carries, or "".
func stepWriteOf(ctx context.Context) flow.RunID {
	id, _ := ctx.Value(stepWriteKey{}).(flow.RunID)
	return id
}
