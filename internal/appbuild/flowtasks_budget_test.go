package appbuild_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// TestFlowControl_TasksBudget pins AC 4: the task list and the run list
// read the store a fixed number of times, whatever the number of runs, for a global holder
// and for one who holds the role through a role relation. Each also reads
// the same rows for 10 runs whatever the number of other people.
func TestFlowControl_TasksBudget(t *testing.T) {
	// cost lists the tasks and the runs of user, with runs onboarding runs
	// and pad people that no run is about.
	cost := func(t *testing.T, runs, pad int, user string) (tasks, list storetest.Cost) {
		t.Helper()
		svc, ctl := stepProject(t)
		addFlowPerson(t, svc, "carol", map[string]any{"name": "Carol"})
		alice := flowUser(t, svc, "alice")
		for i := range pad {
			id := fmt.Sprintf("PAD-%d", i+1)
			addFlowPerson(t, svc, id, map[string]any{"name": id})
		}
		for i := range runs {
			id := fmt.Sprintf("PER-%d", i+1)
			addFlowPerson(t, svc, id, map[string]any{"name": id})
			_, err := svc.Store().CreateRelation(context.Background(), entity.RelationKey{
				From: "carol", Type: "buddy", To: id,
			}, &store.RelationData{})
			require.NoError(t, err)
			_, err = ctl.Start(alice, "onboarding", id)
			require.NoError(t, err)
		}
		counting := storetest.NewCounting(svc.Store())
		budget := appbuild.FlowControlOver(svc, counting)
		got, err := budget.Tasks(flowUser(t, svc, user), "", 1, 50)
		require.NoError(t, err)
		require.Equal(t, runs, got.Total)
		tasks = counting.Cost()
		counting.Reset()
		runList, err := budget.RunList(flowUser(t, svc, user), "", "", 1, 50)
		require.NoError(t, err)
		require.Equal(t, runs, runList.Total)
		return tasks, counting.Cost()
	}
	for _, user := range []string{"alice", "carol"} {
		t.Run(user, func(t *testing.T) {
			smallTasks, smallList := cost(t, 10, 0, user)
			largeTasks, largeList := cost(t, 50, 0, user)
			require.Equal(t, smallTasks.Calls, largeTasks.Calls, "task list reads grow with the number of runs")
			require.Equal(t, smallList.Calls, largeList.Calls, "run list reads grow with the number of runs")
			// The fixture is on disk, so the padding is smaller than
			// storetest.ScaleSmall/ScaleLarge; 20 against 200 still
			// tells a scan from a page.
			const padSmall, padLarge = 20, 200
			ts, ls := cost(t, 10, padSmall, user)
			tl, ll := cost(t, 10, padLarge, user)
			storetest.AssertSameCost(t, padSmall, ts, "task list", padLarge, tl, "task list")
			storetest.AssertSameCost(t, padSmall, ls, "run list", padLarge, ll, "run list")
		})
	}
}
