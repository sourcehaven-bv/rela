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
// and for one who holds the role through a role relation.
func TestFlowControl_TasksBudget(t *testing.T) {
	reads := func(t *testing.T, runs int, user string) int {
		t.Helper()
		svc, ctl := stepProject(t)
		addFlowPerson(t, svc, "carol", map[string]any{"name": "Carol"})
		alice := flowUser(t, svc, "alice")
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
		tasks := counting.Reads()
		counting.Reset()
		list, err := budget.RunList(flowUser(t, svc, user), "", "", 1, 50)
		require.NoError(t, err)
		require.Equal(t, runs, list.Total)
		return tasks*1000 + counting.Reads()
	}
	for _, user := range []string{"alice", "carol"} {
		t.Run(user, func(t *testing.T) {
			small, large := reads(t, 10, user), reads(t, 50, user)
			require.Equal(t, small, large, "store reads grow with the number of runs")
		})
	}
}
