package flowengine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
)

// forkFlow forks at once into an IT branch of two states, with a timer,
// and a legal branch that either joins or rejects.
const forkFlow = `
flows:
  hire:
    subject: person
    run_as: system:onboarding-bot
    start: [{ on: manual, permission: onboarding.start }]
    initial: start
    states:
      start:
        on:
          done: { to: [it, legal] }
      it:
        waiting_for: [hr]
        outputs:
          - set: { properties: { stage: it } }
        on:
          change: { to: it_done, roles: [hr] }
        after:
          slow: { period: 1d, to: it_done }
      it_done:
        on:
          done: { to: joined }
      legal:
        waiting_for: [manager]
        outputs:
          - create: { type: task, properties: { title: legal } }
        on:
          change:
            - { to: joined, roles: [manager], when: "entity.documents == true" }
            - { to: rejected, roles: [manager] }
      joined:
        join: all
        outputs:
          - set: { properties: { stage: joined } }
        on:
          done: { to: finished }
      finished: { final: true }
      rejected: { final: true, status: failed }
`

func startFork(t *testing.T, join string, props map[string]any) (harness, flowrun.Run) {
	t.Helper()
	h := newHarness(t, strings.Replace(forkFlow, "join: all", join, 1))
	h.f.addPerson("PER-1", props)
	id, err := h.eng.Start(context.Background(), "hire", entity.Ref{ID: "PER-1"})
	require.NoError(t, err)
	run, err := h.eng.Run(context.Background(), id)
	require.NoError(t, err)
	return h, run
}

// genOf returns the generation of run's branch name.
func genOf(t *testing.T, run flowrun.Run, name string) int64 {
	t.Helper()
	i := branchAt(run, name)
	require.GreaterOrEqual(t, i, 0, "no branch %s", name)
	return run.Branches[i].Instance.Generation
}

func TestFork_StartsEveryBranch(t *testing.T) {
	h, run := startFork(t, "join: all", map[string]any{"name": "Ada"})

	require.Len(t, run.Branches, 2)
	require.Equal(t, "it", run.Branches[0].Name)
	require.Equal(t, "legal", run.Branches[1].Name)
	// The run's own instance is inert: it reads as stranded.
	require.Empty(t, run.Instance.State)
	require.Equal(t, "start", run.Instance.Previous)
	require.Nil(t, run.Pending)
	gens := map[int64]bool{run.Instance.Generation: true}
	for _, b := range run.Branches {
		require.Nil(t, b.Pending, "branch %s settled", b.Name)
		require.False(t, gens[b.Instance.Generation], "generation %d twice", b.Instance.Generation)
		gens[b.Instance.Generation] = true
	}
	// Both branches wrote their outputs.
	require.Equal(t, "it", h.f.subjects["PER-1"].Properties["stage"])
	require.Len(t, h.f.creates, 1)
	// The header names the join and waits for a person.
	require.Equal(t, "joined", h.f.header(run.ID)["state"])
	require.Equal(t, "input_required", h.f.header(run.ID)["status"])
	// The due hint is the IT timer.
	require.Equal(t, h.clock.Add(24*time.Hour), run.Instance.NextDueAt)
	// The log names each branch.
	last := run.Log[len(run.Log)-2:]
	require.Equal(t, "it", last[0].Branch)
	require.Equal(t, "legal", last[1].Branch)
}

func TestFork_JoinAllWaitsForEveryBranch(t *testing.T) {
	h, run := startFork(t, "join: all", map[string]any{"name": "Ada", "documents": true})
	ctx := context.Background()

	got, err := h.eng.Change(ctx, run.ID, genOf(t, run, "it"), "alice", holdsRoles("hr"))
	require.NoError(t, err)
	require.True(t, got.Moved)
	run, err = h.eng.Run(ctx, run.ID)
	require.NoError(t, err)
	require.True(t, run.Branches[0].Arrived, "it arrived")
	require.False(t, run.Branches[1].Arrived)
	require.Equal(t, "joined", h.f.header(run.ID)["state"])

	xs, err := h.eng.Explain(ctx, run)
	require.NoError(t, err)
	require.Equal(t, "joined", xs[0].State)
	require.Zero(t, xs[0].Generation)
	require.Len(t, xs[0].Parallel, 2)
	require.True(t, xs[0].Parallel[0].Arrived)
	require.Empty(t, xs[0].Parallel[0].WaitingFor)
	require.Equal(t, []string{"manager"}, xs[0].Parallel[1].WaitingFor)

	// The arrived branch has no step left.
	_, err = h.eng.Change(ctx, run.ID, genOf(t, run, "it"), "alice", holdsRoles("hr"))
	require.ErrorIs(t, err, ErrStale)

	got, err = h.eng.Change(ctx, run.ID, genOf(t, run, "legal"), "bob", holdsRoles("manager"))
	require.NoError(t, err)
	require.True(t, got.Finished)
	require.Equal(t, "completed", h.f.header(run.ID)["status"])
	require.Equal(t, "finished", h.f.header(run.ID)["state"])
	require.Equal(t, "joined", h.f.subjects["PER-1"].Properties["stage"])
}

func TestFork_FinalBranchEndsTheRun(t *testing.T) {
	h, run := startFork(t, "join: all", map[string]any{"name": "Ada", "documents": false})
	ctx := context.Background()

	got, err := h.eng.Change(ctx, run.ID, genOf(t, run, "legal"), "bob", holdsRoles("manager"))
	require.NoError(t, err)
	require.True(t, got.Finished)
	require.Equal(t, "failed", h.f.header(run.ID)["status"])
	require.Equal(t, "rejected", h.f.header(run.ID)["state"])
	_, err = h.eng.Run(ctx, run.ID)
	require.ErrorIs(t, err, ErrRunNotFound)
}

func TestFork_JoinAnyWithdrawsTheOthers(t *testing.T) {
	h, run := startFork(t, "join: any", map[string]any{"name": "Ada"})
	ctx := context.Background()

	_, err := h.eng.Change(ctx, run.ID, genOf(t, run, "it"), "alice", holdsRoles("hr"))
	require.NoError(t, err)
	require.Equal(t, "completed", h.f.header(run.ID)["status"])
	require.Equal(t, "joined", h.f.subjects["PER-1"].Properties["stage"])
}

func TestFork_JoinWhen(t *testing.T) {
	tests := []struct {
		name string
		docs bool
		want string
	}{
		{"holds", true, "completed"},
		{"does not hold", false, "input_required"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, run := startFork(t, `join: { when: "arrived >= 1 and entity.documents == true" }`,
				map[string]any{"name": "Ada", "documents": tc.docs})
			_, err := h.eng.Change(context.Background(), run.ID, genOf(t, run, "it"), "alice", holdsRoles("hr"))
			require.NoError(t, err)
			require.Equal(t, tc.want, h.f.header(run.ID)["status"])
		})
	}
}

func TestFork_BranchTimer(t *testing.T) {
	h, run := startFork(t, "join: all", map[string]any{"name": "Ada"})
	ctx := context.Background()

	*h.clock = h.clock.Add(24*time.Hour + time.Minute)
	n, err := h.eng.Due(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	run, err = h.eng.Run(ctx, run.ID)
	require.NoError(t, err)
	require.True(t, run.Branches[0].Arrived)
	// Only legal is left, and it has no timer.
	require.True(t, run.Instance.NextDueAt.IsZero())
	require.Equal(t, "it", run.Log[len(run.Log)-2].Branch)
	require.Equal(t, flowrun.ViaTimer, run.Log[len(run.Log)-2].Via)
}

// TestFork_JoinWaitsForPendingBranch pins that the join never fires while
// a branch is halfway through its outputs: an any join waits for legal's
// create to land before it withdraws legal.
func TestFork_JoinWaitsForPendingBranch(t *testing.T) {
	h := newHarness(t, strings.Replace(forkFlow, "join: all", "join: any", 1))
	ctx := context.Background()
	h.f.addPerson("PER-1", map[string]any{"name": "Ada"})
	h.f.failCreate = errors.New("refused")
	id, err := h.eng.Start(ctx, "hire", entity.Ref{ID: "PER-1"})
	require.Error(t, err)
	run, err := h.eng.Run(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, run.Branches[1].Pending, "legal is unsettled")

	// IT is settled, so its step can be taken while legal is not.
	// Legal's create still fails, but that is not IT's step's error:
	// legal is backed off alone.
	failures := run.Branches[1].Pending.Failures
	_, err = h.eng.Change(ctx, id, genOf(t, run, "it"), "alice", holdsRoles("hr"))
	require.NoError(t, err)
	run, err = h.eng.Run(ctx, id)
	require.NoError(t, err)
	require.True(t, run.Branches[0].Arrived)
	require.Equal(t, failures+1, run.Branches[1].Pending.Failures)
	require.False(t, run.Branches[1].Instance.NextDueAt.IsZero(), "legal is due again later")
	require.Equal(t, "joined", h.f.header(id)["state"])
	require.NotEqual(t, "completed", h.f.header(id)["status"])

	// The legal step is refused while it settles.
	_, err = h.eng.Change(ctx, id, genOf(t, run, "legal"), "bob", holdsRoles("manager"))
	require.ErrorIs(t, err, ErrSettling)

	h.f.failCreate = nil
	require.NoError(t, h.eng.Retry(ctx, id))
	require.Equal(t, "completed", h.f.header(id)["status"])
	require.Len(t, h.f.creates, 1, "legal's create was written once")
}

func TestFork_ClaimsArePerBranch(t *testing.T) {
	h, run := startFork(t, "join: all", map[string]any{"name": "Ada"})
	ctx := context.Background()

	require.NoError(t, h.eng.Claim(ctx, run.ID, genOf(t, run, "it"), "alice"))
	require.ErrorIs(t, h.eng.Claim(ctx, run.ID, genOf(t, run, "it"), "bob"), ErrClaimed)
	require.NoError(t, h.eng.Claim(ctx, run.ID, genOf(t, run, "legal"), "bob"))
	// The run's own generation is no step while it is forked.
	require.ErrorIs(t, h.eng.Claim(ctx, run.ID, run.Instance.Generation, "bob"), ErrStale)

	h.eng.ReleaseClaim(ctx, run.ID, genOf(t, run, "it"), "alice")
	require.NoError(t, h.eng.Claim(ctx, run.ID, genOf(t, run, "it"), "bob"))
	stored, err := h.eng.Run(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, "bob", stored.Branches[0].Claim.By)
	require.Equal(t, "bob", stored.Branches[1].Claim.By)
	require.Nil(t, stored.Claim)
}

func TestFork_CancelKeepsTheJoinOnTheHeader(t *testing.T) {
	h, run := startFork(t, "join: all", map[string]any{"name": "Ada"})
	require.NoError(t, h.eng.Cancel(context.Background(), run.ID))
	require.Equal(t, "cancelled", h.f.header(run.ID)["status"])
	require.Equal(t, "joined", h.f.header(run.ID)["state"])
}

func TestFork_BackoffPerBranch(t *testing.T) {
	h := newHarness(t, forkFlow)
	ctx := context.Background()
	h.f.addPerson("PER-1", map[string]any{"name": "Ada"})
	h.f.failCreate = errors.New("refused")
	id, err := h.eng.Start(ctx, "hire", entity.Ref{ID: "PER-1"})
	require.Error(t, err)
	run, err := h.eng.Run(ctx, id)
	require.NoError(t, err)
	require.Equal(t, 1, run.Branches[1].Pending.Failures)
	require.Equal(t, h.clock.Add(minBackoff), run.Instance.NextDueAt)
	require.Nil(t, run.Branches[0].Pending)
}

// countingRuns counts the List calls of a run store.
type countingRuns struct {
	flowrun.Store
	lists int
}

func (c *countingRuns) List(ctx context.Context, q flowrun.Query) ([]flowrun.Run, error) {
	c.lists++
	return c.Store.List(ctx, q)
}

// TestRunsByID_OneReadPerPage pins the budget of the task list's run
// read: one store read per [flowrun.MaxPage] ids, however many runs.
func TestRunsByID_OneReadPerPage(t *testing.T) {
	h := newHarness(t, forkFlow)
	ctx := context.Background()
	var ids []flow.RunID
	for i := range 3 {
		person := fmt.Sprintf("PER-%d", i)
		h.f.addPerson(person, map[string]any{"name": "Ada"})
		id, err := h.eng.Start(ctx, "hire", entity.Ref{ID: person})
		require.NoError(t, err)
		ids = append(ids, id)
	}
	counted := &countingRuns{Store: h.eng.d.Runs}
	h.eng.d.Runs = counted
	runs, err := h.eng.RunsByID(ctx, append(ids, "RUN-404"))
	require.NoError(t, err)
	require.Len(t, runs, 3)
	require.Equal(t, 1, counted.lists)

	many := make([]flow.RunID, flowrun.MaxPage+1)
	for i := range many {
		many[i] = flow.RunID(fmt.Sprintf("RUN-%d", i+100))
	}
	counted.lists = 0
	_, err = h.eng.RunsByID(ctx, many)
	require.NoError(t, err)
	require.Equal(t, 2, counted.lists)
}

// TestFork_LoopForksAgain pins that a run that comes back to the state
// that forked forks again, with fresh generations.
func TestFork_LoopForksAgain(t *testing.T) {
	flows := strings.Replace(forkFlow, "join: all", "join: any", 1)
	flows = strings.Replace(flows, "          done: { to: finished }",
		"          done:\n            - { to: finished, when: \"entity.documents == true\" }\n            - { to: start }", 1)
	h := newHarness(t, flows)
	ctx := context.Background()
	h.f.addPerson("PER-1", map[string]any{"name": "Ada", "documents": false})
	id, err := h.eng.Start(ctx, "hire", entity.Ref{ID: "PER-1"})
	require.NoError(t, err)
	first, err := h.eng.Run(ctx, id)
	require.NoError(t, err)

	_, err = h.eng.Change(ctx, id, genOf(t, first, "it"), "alice", holdsRoles("hr"))
	require.NoError(t, err)
	again, err := h.eng.Run(ctx, id)
	require.NoError(t, err)
	require.Len(t, again.Branches, 2)
	require.False(t, again.Branches[0].Arrived)
	require.Greater(t, genOf(t, again, "it"), genOf(t, first, "legal"))
	require.Equal(t, "joined", h.f.header(id)["state"])
}

// TestFork_FinalWaitsForPendingBranch: a branch that reaches a final state
// ends the run only once its siblings have written their outputs.
func TestFork_FinalWaitsForPendingBranch(t *testing.T) {
	h := newHarness(t, forkFlow)
	ctx := context.Background()
	h.f.addPerson("PER-1", map[string]any{"name": "Ada", "documents": false})
	h.f.failSet = errors.New("refused")
	id, err := h.eng.Start(ctx, "hire", entity.Ref{ID: "PER-1"})
	require.Error(t, err)
	run, err := h.eng.Run(ctx, id)
	require.NoError(t, err)

	_, err = h.eng.Change(ctx, id, genOf(t, run, "legal"), "bob", holdsRoles("manager"))
	require.NoError(t, err)
	run, err = h.eng.Run(ctx, id)
	require.NoError(t, err, "IT still writes its outputs: the run has not ended")
	require.True(t, run.Branches[1].Arrived)
	require.Equal(t, "rejected", run.Branches[1].Instance.State)

	h.f.failSet = nil
	require.NoError(t, h.eng.Retry(ctx, id))
	require.Equal(t, "failed", h.f.header(id)["status"])
	require.Equal(t, "rejected", h.f.header(id)["state"])
	require.Equal(t, []map[string]any{{"stage": "it"}}, h.f.sets)
}

// racingRuns lets another write land on the run just before the first
// update the engine makes.
type racingRuns struct {
	flowrun.Store
	race func()
}

func (r *racingRuns) Update(ctx context.Context, id flow.RunID, rev int64, st flowrun.State,
	at time.Time) (flowrun.Run, error) {
	if race := r.race; race != nil {
		r.race = nil
		race()
	}
	return r.Store.Update(ctx, id, rev, st, at)
}

// TestFork_ChangeRetriesAfterASiblingWrite: a write to another branch
// between the read and the move does not make a step stale.
func TestFork_ChangeRetriesAfterASiblingWrite(t *testing.T) {
	h, run := startFork(t, "join: all", map[string]any{"name": "Ada"})
	ctx := context.Background()
	racing := &racingRuns{Store: h.eng.d.Runs}
	h.eng.d.Runs = racing
	racing.race = func() {
		require.NoError(t, h.eng.Claim(ctx, run.ID, genOf(t, run, "legal"), "bob"))
	}

	got, err := h.eng.Change(ctx, run.ID, genOf(t, run, "it"), "alice", holdsRoles("hr"))
	require.NoError(t, err)
	require.True(t, got.Moved)
	after, err := h.eng.Run(ctx, run.ID)
	require.NoError(t, err)
	require.True(t, after.Branches[0].Arrived)
	require.NotNil(t, after.Branches[1].Claim, "bob's claim on legal stands")
}
