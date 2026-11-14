package apptest

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowengine"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Flows in a case (TKT-HB313W5). A case's world has no job queue or flow
// worker. The flow engine puts job step attempts on the case's flowQueue,
// and t.flows.work() runs them and the engine's due rounds, on the case's
// goroutine and clock. A retry waits for its backoff on the case's clock,
// so a case moves the clock with t.advance before the next t.flows.work().

// maxWorkRounds bounds one t.flows.work(), so a flow that keeps queueing
// work at the same instant fails the case instead of hanging it.
const maxWorkRounds = 100

// flowQueue holds the attempts the engine queued and the case has not run.
type flowQueue struct {
	mu   sync.Mutex
	refs []flowengine.AttemptRef
}

// EnqueueAttempt implements [flowengine.JobQueue]. The attempt runs at the
// next t.flows.work(); its queue deadline is checked then, on the case's
// clock, by the engine. The dedupe key is ignored: running an attempt twice
// is safe, because [flowengine.AttemptHandler.HandleAttempt] is idempotent.
func (q *flowQueue) EnqueueAttempt(_ context.Context, ref flowengine.AttemptRef, _ string, _ time.Time) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.refs = append(q.refs, ref)
	return nil
}

// requeue puts refs back in front of the queue.
func (q *flowQueue) requeue(refs []flowengine.AttemptRef) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.refs = append(slices.Clone(refs), q.refs...)
}

// take removes and returns the queued attempts.
func (q *flowQueue) take() []flowengine.AttemptRef {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := q.refs
	q.refs = nil
	return out
}

// flowsModule holds the t.flows functions of a file run.
type flowsModule struct{ fr *fileRun }

// flowsTable builds t.flows.
func (m flowsModule) flowsTable(ls *lua.LState) *lua.LTable {
	t := ls.NewTable()
	t.RawSetString("work", ls.NewFunction(m.luaFlowsWork))
	t.RawSetString("run", ls.NewFunction(m.luaFlowsRun))
	return t
}

// engine returns the world's flow engine, or raises when there is none.
func (m flowsModule) engine(ls *lua.LState, w *world) *flowengine.Engine {
	fr := m.fr
	fr.check(ls, w.ensure())
	eng := appbuild.FlowEngine(w.svc)
	if eng == nil {
		fr.raise(ls, errors.New("the project declares no flows in flows.yaml"))
	}
	return eng
}

// luaFlowsWork is t.flows.work(): it runs the queued job attempts and the
// engine's due rounds until nothing is left to do at the case's clock, and
// returns how many attempts ran.
func (m flowsModule) luaFlowsWork(ls *lua.LState) int {
	fr := m.fr
	w := fr.world(ls, "flows.work")
	eng := m.engine(ls, w)
	h, err := flowengine.NewAttemptHandler(eng)
	fr.check(ls, err)
	ran := 0
	for range maxWorkRounds {
		refs := w.flowJobs.take()
		for i, ref := range refs {
			if err := h.HandleAttempt(w.ctx, ref); err != nil {
				// Keep the attempts not run, so a later t.flows.work()
				// after a pcall still runs them.
				w.flowJobs.requeue(refs[i+1:])
				fr.raise(ls, err)
			}
			ran++
		}
		advanced, derr := eng.Due(w.ctx)
		fr.check(ls, derr)
		if len(refs) == 0 && advanced == 0 {
			ls.Push(lua.LNumber(ran))
			return 1
		}
	}
	fr.raise(ls, fmt.Errorf("t.flows.work: the flows still had work after %d rounds at %s",
		maxWorkRounds, w.now().Format(time.RFC3339)))
	return 0
}

// luaFlowsRun is t.flows.run(id): the run as a table, or nil when there is
// no such run. It reads past the ACL, as given state is written.
//
//	id, flow, state, status  from the run's _flow-run entity
//	job                      the job of the run's own step while one is
//	                         pending: phase, attempts (queued so far),
//	                         failed (attempts that failed), fail_kind and
//	                         next_at; nil otherwise
func (m flowsModule) luaFlowsRun(ls *lua.LState) int {
	fr := m.fr
	w := fr.world(ls, "flows.run")
	eng := m.engine(ls, w)
	id := ls.CheckString(1)
	ctx := w.systemContext()
	// _flow-run is a faceless type.
	hdr, err := w.svc.Store().GetEntity(ctx, entity.Ref{ID: id, Face: entity.ImplicitFace})
	if errors.Is(err, store.ErrNotFound) || (err == nil && hdr.Type != entity.FlowRunType) {
		ls.Push(lua.LNil)
		return 1
	}
	fr.check(ls, err)
	t := ls.NewTable()
	t.RawSetString("id", lua.LString(hdr.ID))
	for _, p := range []string{metamodel.FlowRunPropFlow, metamodel.FlowRunPropState, metamodel.FlowRunPropStatus} {
		if v, ok := hdr.Properties[p].(string); ok {
			t.RawSetString(p, lua.LString(v))
		}
	}
	run, err := eng.Run(ctx, flow.RunID(id))
	if err != nil && !errors.Is(err, flowengine.ErrRunNotFound) {
		fr.raise(ls, err)
	}
	if err == nil && run.Pending != nil && run.Pending.Job != nil {
		j := run.Pending.Job
		jt := ls.NewTable()
		jt.RawSetString("phase", lua.LString(j.Phase))
		jt.RawSetString("attempts", lua.LNumber(j.Seq))
		jt.RawSetString("failed", lua.LNumber(j.Failed))
		if j.FailKind != "" {
			jt.RawSetString("fail_kind", lua.LString(j.FailKind))
		}
		if !j.NextAt.IsZero() {
			jt.RawSetString("next_at", lua.LString(j.NextAt.UTC().Format(time.RFC3339)))
		}
		t.RawSetString("job", jt)
	}
	ls.Push(t)
	return 1
}

// startFlow is actor:start_flow(name, ref): it starts a run of the named
// flow on the subject, as `rela flow start` does for the actor, and returns
// the run's id. The run's first step runs at once; its job attempts wait for
// t.flows.work().
func (fr *fileRun) startFlow(ctx context.Context, ls *lua.LState, w *world) int {
	name := ls.CheckString(2)
	addr, _, _ := address(ls, 3)
	fc := appbuild.FlowControlFor(w.svc)
	if fc == nil {
		fr.raise(ls, errors.New("the project declares no flows in flows.yaml"))
	}
	id, err := fc.Start(ctx, name, addr)
	fr.check(ls, err)
	ls.Push(lua.LString(id))
	return 1
}
