package flowengine

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
)

// TestDueRound_CapAndCursor pins that one round advances at most
// MaxDuePerRound runs and reports More, and that the next round resumes
// after the last run, so a caller visits each due run once.
func TestDueRound_CapAndCursor(t *testing.T) {
	h := newHarness(t, "")
	ctx := context.Background()
	const runs = MaxDuePerRound + 1
	for i := range runs {
		id := fmt.Sprintf("PER-%d", i)
		h.f.addPerson(id, map[string]any{"name": "Ada", "status": "hired"})
		_, err := h.eng.Start(ctx, "onboarding", entity.Ref{ID: id})
		require.NoError(t, err)
	}
	// The collect state's stalled timer fires after 7 days.
	*h.clock = h.clock.Add(7*24*time.Hour + time.Minute)

	first, err := h.eng.DueRound(ctx, flowrun.DueCursor{})
	require.NoError(t, err)
	require.Equal(t, MaxDuePerRound, first.Advanced)
	require.Zero(t, first.Failed)
	require.True(t, first.More)

	second, err := h.eng.DueRound(ctx, first.Next)
	require.NoError(t, err)
	require.Equal(t, 1, second.Advanced)
	require.False(t, second.More)

	none, err := h.eng.DueRound(ctx, flowrun.DueCursor{})
	require.NoError(t, err)
	require.Zero(t, none.Advanced, "every run escalated and waits for its next timer")
}

// TestJoinWhen_TestedAgainLater pins the join poll: a join whose `when:`
// is false at the arrival makes the run due after JoinRecheck, and a
// round after the subject changed continues the run (TKT-FWQZGAM).
func TestJoinWhen_TestedAgainLater(t *testing.T) {
	h, run := startFork(t, `join: { when: "arrived >= 1 and entity.documents == true" }`,
		map[string]any{"name": "Ada", "documents": false})
	ctx := context.Background()
	_, err := h.eng.Change(ctx, run.ID, genOf(t, run, "it"), "alice", holdsRoles("hr"))
	require.NoError(t, err)
	require.Equal(t, "input_required", h.f.header(run.ID)["status"])
	run, err = h.eng.Run(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, h.clock.Add(JoinRecheck), run.Instance.NextDueAt, "the waiting join is tested again")

	// Not due yet: nothing changes.
	n, err := h.eng.Due(ctx)
	require.NoError(t, err)
	require.Zero(t, n)

	// Due, but the condition still fails: the run waits for the next poll.
	*h.clock = h.clock.Add(JoinRecheck)
	n, err = h.eng.Due(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	run, err = h.eng.Run(ctx, run.ID)
	require.NoError(t, err)
	require.NotEmpty(t, run.Branches, "the join still waits")
	require.Equal(t, h.clock.Add(JoinRecheck), run.Instance.NextDueAt)

	h.f.addPerson("PER-1", map[string]any{"name": "Ada", "documents": true})
	*h.clock = h.clock.Add(JoinRecheck)
	n, err = h.eng.Due(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, "completed", h.f.header(run.ID)["status"], "the join fired and the run finished")
}

// TestJoinWhen_NoPollWithoutWhen pins that only a join with a `when:` is
// polled: an all join waits for its last arrival without a due time.
func TestJoinWhen_NoPollWithoutWhen(t *testing.T) {
	h, run := startFork(t, "join: all", map[string]any{"name": "Ada"})
	ctx := context.Background()
	_, err := h.eng.Change(ctx, run.ID, genOf(t, run, "it"), "alice", holdsRoles("hr"))
	require.NoError(t, err)
	run, err = h.eng.Run(ctx, run.ID)
	require.NoError(t, err)
	require.True(t, run.Instance.NextDueAt.IsZero())
}

// failingRejoin fails the write that drops a run's branches while fail is
// set: a crash or store error between an arrival and the join firing.
type failingRejoin struct {
	flowrun.Store
	fail bool
}

func (r *failingRejoin) Update(ctx context.Context, id flow.RunID, rev int64, st flowrun.State,
	at time.Time) (flowrun.Run, error) {
	if r.fail && len(st.Branches) == 0 {
		return flowrun.Run{}, errors.New("store down")
	}
	return r.Store.Update(ctx, id, rev, st, at)
}

// TestJoin_FiresOnALaterRoundAfterAFailedRejoin pins that a join that
// holds when its write fails is due again, so a later round fires it.
func TestJoin_FiresOnALaterRoundAfterAFailedRejoin(t *testing.T) {
	h, run := startFork(t, "join: any", map[string]any{"name": "Ada"})
	ctx := context.Background()
	failing := &failingRejoin{Store: h.eng.d.Runs, fail: true}
	h.eng.d.Runs = failing
	_, _ = h.eng.Change(ctx, run.ID, genOf(t, run, "it"), "alice", holdsRoles("hr"))
	stuck, err := h.eng.Run(ctx, run.ID)
	require.NoError(t, err)
	require.NotEmpty(t, stuck.Branches, "the join did not fire")
	require.False(t, stuck.Instance.NextDueAt.IsZero(), "the run is due again")

	failing.fail = false
	*h.clock = h.clock.Add(JoinRecheck)
	n, err := h.eng.Due(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, "completed", h.f.header(run.ID)["status"])
}

// TestJoinWhen_NoPollWhileABranchIsPending pins that the join poll does
// not undercut the backoff of a branch whose outputs failed.
func TestJoinWhen_NoPollWhileABranchIsPending(t *testing.T) {
	h, run := startFork(t, `join: { when: "arrived >= 1 and entity.documents == true" }`,
		map[string]any{"name": "Ada"})
	ctx := context.Background()
	st := cloneState(run)
	st.Branches[0].Arrived = true
	st.Branches[0].Pending, st.Branches[0].Claim = nil, nil
	st.Branches[1].Pending = &flowrun.Pending{State: "legal", Generation: st.Branches[1].Instance.Generation,
		Failures: 3}
	st.Branches[1].Instance.NextDueAt = h.clock.Add(time.Hour)
	_, err := h.eng.put(ctx, run, st, *h.clock)
	require.NoError(t, err)
	got, err := h.eng.Run(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, h.clock.Add(time.Hour), got.Instance.NextDueAt, "the branch's backoff decides")
}
