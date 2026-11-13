package flowengine

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Parallel branches (TKT-ZO9IB6V).
//
// A run is at one place or, after a fork, at several: one per branch. The
// engine names a place by a token: "" for the run itself, or the name of a
// branch, which is the fork target it started in. Every operation that
// used to act on the run's instance acts on a token's instance, pending
// mark and claim instead.
//
// While a run is forked its own instance is inert: its state is empty, so
// [flow.Read] reads it as stranded and every path that does not know
// about branches fails closed. Generations are unique across the run, so a
// person's step names its branch by generation alone.
//
// Three moves change the shape of a run, each in one compare-and-set:
//
//   - the fork starts every branch, each with a pending mark;
//   - a branch that reaches the join, or a final state, waits there
//     (Arrived);
//   - the join fires, or the run ends in the final state a branch reached:
//     the branches are dropped and the run continues at that state with a
//     pending mark.
//
// The join is tested only when no live branch carries a pending mark, so a
// branch is never withdrawn halfway through its outputs. Each branch
// settles on its own: one whose outputs fail is backed off alone, and the
// others go on.

// forked reports a run in parallel branches.
func forked(run flowrun.Run) bool { return len(run.Branches) > 0 }

// branchAt returns the index of the branch named tok, or -1.
func branchAt(run flowrun.Run, tok string) int {
	return slices.IndexFunc(run.Branches, func(b flowrun.Branch) bool { return b.Name == tok })
}

// slot returns the instance, pending mark and claim of token tok. ok is
// false for a branch the run does not carry.
func slot(run flowrun.Run, tok string) (inst flow.Instance, p *flowrun.Pending, c *flowrun.Claim, ok bool) {
	if tok == "" {
		return run.Instance, run.Pending, run.Claim, true
	}
	i := branchAt(run, tok)
	if i < 0 {
		return flow.Instance{}, nil, nil, false
	}
	b := run.Branches[i]
	return b.Instance, b.Pending, b.Claim, true
}

// instOf returns the instance of token tok in run.
func instOf(run flowrun.Run, tok string) flow.Instance {
	inst, _, _, _ := slot(run, tok) //nolint:dogsled // the one place that picks a field of slot
	return inst
}

// pendingOf returns the pending mark of token tok in run.
func pendingOf(run flowrun.Run, tok string) *flowrun.Pending {
	_, p, _, _ := slot(run, tok) //nolint:dogsled // the one place that picks a field of slot
	return p
}

// maxGen returns the highest generation in run: a move takes the next.
func maxGen(run flowrun.Run) int64 {
	g := run.Instance.Generation
	for _, b := range run.Branches {
		g = max(g, b.Instance.Generation)
	}
	return g
}

// tokenAt returns the live token whose instance is at generation gen.
func tokenAt(run flowrun.Run, gen int64) (string, bool) {
	if !forked(run) {
		return "", run.Instance.Generation == gen
	}
	for _, b := range run.Branches {
		if !b.Arrived && b.Instance.Generation == gen {
			return b.Name, true
		}
	}
	return "", false
}

// StepAt returns the branch whose step is at generation gen: "" for the
// run's own step. ok is false when no step of run is at gen.
func StepAt(run flowrun.Run, gen int64) (branch string, ok bool) { return tokenAt(run, gen) }

// RunsByID returns the stored runs among ids, in one store read per
// [flowrun.MaxPage] ids. An id without a stored run is left out.
func (e *Engine) RunsByID(ctx context.Context, ids []flow.RunID) ([]flowrun.Run, error) {
	var out []flowrun.Run
	for batch := range slices.Chunk(ids, flowrun.MaxPage) {
		page, err := e.d.Runs.List(ctx, flowrun.Query{IDs: batch, Limit: flowrun.MaxPage})
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
	}
	return out, nil
}

// pendingToken returns a token with a pending mark that skip does not
// hold: first if it has one, else the first branch that does.
func pendingToken(run flowrun.Run, skip map[string]bool, first string) (string, bool) {
	if !forked(run) {
		return "", run.Pending != nil
	}
	if i := branchAt(run, first); i >= 0 && run.Branches[i].Pending != nil && !skip[first] {
		return first, true
	}
	for _, b := range run.Branches {
		if b.Pending != nil && !skip[b.Name] {
			return b.Name, true
		}
	}
	return "", false
}

// Pending reports whether a token of run carries a pending mark: the run
// has a step to settle.
func Pending(run flowrun.Run) bool {
	_, ok := pendingToken(run, nil, "")
	return ok
}

// cloneState returns run's state with its own branch slice, so a change to
// one branch does not reach the caller's run.
func cloneState(run flowrun.Run) flowrun.State {
	st := run.State
	st.Branches = slices.Clone(run.Branches)
	return st
}

// JoinRecheck is how long a join that waits on its `when:` waits before
// the engine tests the condition again. Only an arrival or a settle tests
// it otherwise, and the subject can change in between.
const JoinRecheck = 5 * time.Minute

// put stores st over run. It is the one way the engine updates a run
// (RR-SHLMJUL); every caller builds st from the stored state, so the
// timeline carries over. For a forked run it first sets the run's due
// hint, the one value the store lists due runs by: the earliest due time
// of a live branch or of the join (see joinDue).
func (e *Engine) put(ctx context.Context, run flowrun.Run, st flowrun.State, at time.Time) (flowrun.Run, error) {
	if len(st.Branches) > 0 {
		var due time.Time
		earlier := func(d time.Time) {
			if !d.IsZero() && (due.IsZero() || d.Before(due)) {
				due = d
			}
		}
		for _, b := range st.Branches {
			if !b.Arrived {
				earlier(b.Instance.NextDueAt)
			}
		}
		earlier(joinDue(ctx, e.flowOf, run, st, at))
		st.Instance.NextDueAt = due
	}
	return e.d.Runs.Update(ctx, run.ID, run.Rev, st, at)
}

// joinDue returns when the join of the forked state st of run, stored at
// at, must be tested again, or zero when only an arrival or a branch's
// own due time can make it hold.
//
// An arrival and the join firing are two writes. A join that holds by its
// shape (every branch arrived, one arrived at a final state, or an `any`
// join with an arrival) is due minBackoff after at, so a crash or a failed
// write between the two is picked up by a later due round. A `when:` join
// that some branches but not all reached is due JoinRecheck after at,
// because the subject can change in between. Neither applies while a live
// branch carries a pending mark: the join is not tested until it settles,
// and the branch's own due time keeps its backoff. A flow it cannot read is
// logged and gives zero.
func joinDue(ctx context.Context, flowOf func(context.Context, flowrun.Run) (*flowcfg.Flow, error),
	run flowrun.Run, st flowrun.State, at time.Time) time.Time {
	arrived := 0
	for _, b := range st.Branches {
		switch {
		case b.Arrived:
			arrived++
		case b.Pending != nil:
			return time.Time{}
		}
	}
	if arrived == 0 {
		return time.Time{}
	}
	if arrived == len(st.Branches) {
		return at.Add(minBackoff)
	}
	f, err := flowOf(ctx, run)
	if err != nil {
		slog.WarnContext(ctx, "flowengine: a waiting join is not tested again", "run", run.ID, "error", err)
		return time.Time{}
	}
	for _, b := range st.Branches {
		if s, _ := f.State(b.Instance.State); b.Arrived && s.Final {
			return at.Add(minBackoff)
		}
	}
	fk, _, ok := f.Region(st.Branches[0].Name)
	if !ok {
		return time.Time{}
	}
	js, _ := f.State(fk.Join)
	switch j, _ := js.Join(); {
	case j.Any:
		return at.Add(minBackoff)
	case j.When != nil:
		return at.Add(JoinRecheck)
	}
	return time.Time{}
}

// fork moves the run from cur into the branches that start in targets, in
// one write, and records the fork's join as the header's state.
func (e *Engine) fork(ctx context.Context, f *flowcfg.Flow, run flowrun.Run, cur flow.Current, targets []string,
	step flowrun.Step, now time.Time) (flowrun.Run, error) {
	fk, _, ok := f.Region(targets[0])
	if !ok || len(targets) > flowrun.MaxBranches {
		return flowrun.Run{}, fmt.Errorf("%w: %q forks to no declared branches", ErrStranded, cur.State)
	}
	def := f.Definition()
	g := maxGen(run) + 1
	st := flowrun.State{
		Instance: flow.Instance{Generation: g, EnteredAt: now, Previous: cur.State},
		Log:      run.Log,
		Branches: make([]flowrun.Branch, len(targets)),
	}
	for i, t := range targets {
		inst := cur.Enter(flow.Move{From: cur.State, To: t}, now, 0)
		inst.Generation = g + 1 + int64(i)
		schedule(def, cur, &inst, run.CreatedAt, now)
		st.Branches[i] = flowrun.Branch{Name: t, Instance: inst,
			Pending: &flowrun.Pending{State: t, Generation: inst.Generation}}
		s := step
		s.At, s.From, s.State, s.Branch = now, cur.State, t, t
		st.Log = flowrun.AppendLog(st.Log, s)
	}
	moved, err := e.put(ctx, run, st, now)
	if err != nil {
		return flowrun.Run{}, err
	}
	status := flow.StatusWorking
	if fk.Waits {
		status = flow.StatusInputRequired
	}
	e.patchHeader(ctx, run.ID, map[string]any{
		metamodel.FlowRunPropState:  fk.Join,
		metamodel.FlowRunPropStatus: string(status),
	})
	return moved, nil
}

// rejoin drops the branches and moves the run itself into state to, with
// a pending mark, in one write: the join firing, or a branch reaching a
// final state. from is the state the run is read to leave.
func (e *Engine) rejoin(ctx context.Context, f *flowcfg.Flow, run flowrun.Run, from flow.Current,
	step flowrun.Step, now time.Time) (flowrun.Run, error) {
	def := f.Definition()
	to := step.State
	inst := from.Enter(flow.Move{From: from.State, To: to}, now, 0)
	inst.Generation = maxGen(run) + 1
	schedule(def, from, &inst, run.CreatedAt, now)
	step.At = now
	if step.From == "" {
		step.From = from.State
	}
	st := flowrun.State{
		Instance: inst,
		Pending:  &flowrun.Pending{State: to, Generation: inst.Generation},
		Log:      flowrun.AppendLog(run.Log, step),
	}
	moved, err := e.put(ctx, run, st, now)
	if err != nil {
		return flowrun.Run{}, err
	}
	if ts, _ := f.State(to); !ts.Final {
		e.patchHeader(ctx, run.ID, map[string]any{
			metamodel.FlowRunPropState:  to,
			metamodel.FlowRunPropStatus: string(ts.Status),
		})
	}
	return moved, nil
}

// join fires the join of a forked run when it holds: every branch has
// arrived, or one has and the join is `any`, or its `when:` holds. A
// branch that arrived at a final state ends the run there instead. The
// caller has settled every mark it could: a branch that waits for its job
// has not arrived, so it holds a join that needs it and not one that does
// not. It returns the run as stored and whether the join fired; a run that
// is not forked does not join, and a lost write returns a zero run.
func (e *Engine) join(ctx context.Context, f *flowcfg.Flow, run flowrun.Run) (flowrun.Run, bool, error) {
	if !forked(run) {
		return run, false, nil
	}
	fk, _, ok := f.Region(run.Branches[0].Name)
	if !ok {
		return run, false, fmt.Errorf("%w: branch %q is in no fork", ErrStranded, run.Branches[0].Name)
	}
	to := ""
	arrived := 0
	for _, b := range run.Branches {
		if !b.Arrived {
			continue
		}
		arrived++
		if st, _ := f.State(b.Instance.State); st.Final && to == "" {
			to = st.Name
		}
	}
	if to == "" {
		if arrived == 0 || !e.g.joinHolds(ctx, f, run, fk.Join, arrived) {
			return run, false, nil
		}
		to = fk.Join
	}
	// The run leaves the state that forked, as far as the schedule goes:
	// nothing a branch carried crosses the join.
	from := flow.Current{Instance: flow.Instance{State: run.Instance.Previous}, CreatedAt: run.CreatedAt}
	step := flowrun.Step{State: to, From: fk.State, Via: flowrun.ViaJoin}
	moved, err := e.rejoin(ctx, f, run, from, step, e.now())
	if isLost(err) {
		return flowrun.Run{}, false, nil
	}
	if err != nil {
		return flowrun.Run{}, false, err
	}
	return moved, true, nil
}

// advanceBranches runs one due forked run: it settles a branch with a
// pending mark, else fires the first due timer of a branch, else stores
// the right due times.
func (e *Engine) advanceBranches(ctx context.Context, f *flowcfg.Flow, run flowrun.Run,
	now time.Time) (flowrun.Run, error) {
	// A branch that waits for its job does not hold back the others'
	// timers.
	if settleable(run, now) {
		return e.settle(ctx, f, run, "")
	}
	def := f.Definition()
	st := cloneState(run)
	for i, b := range run.Branches {
		// A branch that waits for its job keeps the due time its job set.
		if b.Arrived || b.Pending != nil {
			continue
		}
		cur := flow.Read(def, b.Instance, run.CreatedAt)
		if cur.Stranded {
			return run, fmt.Errorf("%w: branch %s state %q", ErrStranded, b.Name, cur.State)
		}
		t, due, ok := flow.NextTimer(def, cur)
		if ok && !due.After(now) {
			moved, err := e.take(ctx, f, run, b.Name, cur, []string{t.To},
				flowrun.Step{Via: flowrun.ViaTimer, Timer: t.Name}, now)
			if err != nil {
				if isLost(err) {
					return flowrun.Run{}, nil
				}
				return run, err
			}
			return e.settle(ctx, f, moved, b.Name)
		}
		st.Branches[i].Instance.NextDueAt = flow.NextDue(def, cur)
		if !st.Branches[i].Instance.NextDueAt.After(now) {
			st.Branches[i].Instance.NextDueAt = time.Time{}
		}
	}
	// No timer was due: the stored due times were stale. A join waiting
	// on its `when:` is tested again on the way out.
	saved, err := e.put(ctx, run, st, now)
	if err != nil {
		if isLost(err) {
			return flowrun.Run{}, nil
		}
		return run, err
	}
	return e.settle(ctx, f, saved, "")
}

// schedule sets the timer schedule of inst, which a move from cur made at
// now, as a move does.
func schedule(def *flow.Definition, cur flow.Current, inst *flow.Instance, createdAt, now time.Time) {
	sched := flow.StepSchedule(def, cur, flow.Read(def, *inst, createdAt))
	inst.NextDueAt, inst.Restarts = now, sched.Restarts
	if sched.Reminders != nil {
		inst.Reminders = sched.Reminders
	}
}

// arrived returns the state of run with branch tok, at cur, arrived at the
// join step names; ok is false when run has no such branch.
func arrived(run flowrun.Run, tok string, cur flow.Current, step flowrun.Step,
	now time.Time) (st flowrun.State, ok bool) {
	i := branchAt(run, tok)
	if i < 0 {
		return st, false
	}
	st = cloneState(run)
	inst := cur.Enter(flow.Move{From: cur.State, To: step.State}, now, 0)
	inst.Generation = maxGen(run) + 1
	st.Branches[i] = flowrun.Branch{Name: tok, Instance: inst, Arrived: true}
	step.At, step.From, step.Branch = now, cur.State, tok
	st.Log = flowrun.AppendLog(run.Log, step)
	return st, true
}

// branchesBackedOff returns the state of a forked run that failed to
// advance, with every live branch backed off at now: one with a pending
// mark by its failures, one whose due time has passed by minBackoff. A
// branch whose outputs failed is backed off alone, by branchBackedOff,
// while the others settle.
func branchesBackedOff(run flowrun.Run, now time.Time) flowrun.State {
	st := cloneState(run)
	for i, b := range st.Branches {
		switch {
		case b.Arrived, jobParked(b.Pending, now):
		case b.Pending != nil:
			st.Branches[i] = backedOff(b, now)
		case !b.Instance.NextDueAt.IsZero() && !b.Instance.NextDueAt.After(now):
			st.Branches[i].Instance.NextDueAt = now.Add(minBackoff)
		}
	}
	return st
}

// branchBackedOff returns the state of run with branch tok, whose outputs
// failed, backed off at now: it is due again after its failures, and the
// other branches are untouched. ok is false when tok has no pending mark.
func branchBackedOff(run flowrun.Run, tok string, now time.Time) (st flowrun.State, ok bool) {
	i := branchAt(run, tok)
	if i < 0 || run.Branches[i].Pending == nil {
		return st, false
	}
	st = cloneState(run)
	st.Branches[i] = backedOff(st.Branches[i], now)
	return st, true
}

// backedOff returns b, which has a pending mark, due again after its
// failures, with one failure more.
func backedOff(b flowrun.Branch, now time.Time) flowrun.Branch {
	cp := *b.Pending
	b.Instance.NextDueAt = now.Add(backoffDelay(cp.Failures))
	cp.Failures = min(cp.Failures+1, flowrun.MaxPendingFailures)
	b.Pending = &cp
	return b
}

// notLost returns err, or nil when err is a lost compare-and-set: the
// winner owns the run.
func notLost(err error) error {
	if isLost(err) {
		return nil
	}
	return err
}

// backoffDelay is the wait after failures failed attempts in a row:
// minBackoff, doubling with each, up to maxBackoff.
func backoffDelay(failures int) time.Duration {
	delay := minBackoff
	for range failures {
		if delay *= 2; delay >= maxBackoff {
			return maxBackoff
		}
	}
	return delay
}
