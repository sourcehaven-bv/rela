package appbuild_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowview"
)

// forkFlows forks at once into two branches that both wait for hr, joined
// when both are done (TKT-ZO9IB6V).
const forkFlows = `flows:
  hire:
    label: Hire
    subject: person
    run_as: system:onboarding-bot
    start: [{ on: manual, permission: onboarding.start }]
    initial: start
    states:
      start:
        on:
          done: { to: [it, legal] }
      it:
        label: IT
        waiting_for: [hr]
        writes: [documents]
        on:
          change: { to: joined, roles: [hr], when: "entity.documents == true" }
      legal:
        label: Legal
        waiting_for: [hr]
        on:
          change: { to: joined, roles: [hr] }
      joined:
        label: Joined
        join: all
        on:
          done: { to: finished }
      finished: { final: true }
`

type forkFixture struct {
	ctl *appbuild.FlowControl
	svc *appbuild.Services
	id  flow.RunID
}

// as returns a context bound to user.
func (fx forkFixture) as(t *testing.T, user string) context.Context {
	t.Helper()
	return flowUser(t, fx.svc, user)
}

// view reads the run as user.
func (fx forkFixture) view(t *testing.T, user string) flowview.Run {
	t.Helper()
	run, err := fx.ctl.Run(fx.as(t, user), fx.id)
	require.NoError(t, err)
	return run
}

func forkProject(t *testing.T) forkFixture {
	t.Helper()
	root := writeFlowProject(t)
	policy, err := os.ReadFile(filepath.Join(root, "acl.yaml"))
	require.NoError(t, err)
	writePolicy(t, root, strings.Replace(string(policy), "assignments:\n", "assignments:\n  alice: hr\n  bob: manager\n", 1))
	require.NoError(t, os.WriteFile(filepath.Join(root, "flows.yaml"), []byte(forkFlows), 0o600))
	svc, err := appbuildOnDisk(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	ctl := appbuild.FlowControlFor(svc)
	require.NotNil(t, ctl)
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada"})
	fx := forkFixture{ctl: ctl, svc: svc}
	fx.id, err = ctl.Start(fx.as(t, "alice"), "hire", "PER-1")
	require.NoError(t, err)
	return fx
}

func TestFlowControl_ForkedRun(t *testing.T) {
	fx := forkProject(t)

	run := fx.view(t, "alice")
	require.Equal(t, "joined", run.State)
	require.Equal(t, "input_required", run.Status)
	require.Zero(t, run.Generation)
	require.False(t, run.Actions.Complete, "a forked run has no step of its own")
	require.Len(t, run.Parallel, 2)
	it, legal := run.Parallel[0], run.Parallel[1]
	require.Equal(t, "it", it.Branch)
	require.Equal(t, "IT", it.StateLabel)
	require.True(t, it.Complete)
	require.Equal(t, []string{"documents"}, it.Waiting.Writes)
	require.Len(t, it.Waiting.Branches, 1)
	require.True(t, legal.Complete)
	require.Contains(t, run.Diagram, "class s1,s2 current")
	require.Equal(t, "it", run.Log[len(run.Log)-2].Branch)

	// bob holds no waiting role: he sees the branches but no step.
	for _, p := range fx.view(t, "bob").Parallel {
		require.False(t, p.Complete)
	}
}

func TestFlowControl_ForkedTasksAndSteps(t *testing.T) {
	fx := forkProject(t)
	run := fx.view(t, "alice")

	tasks, err := fx.ctl.Tasks(fx.as(t, "alice"), "", 1, 50)
	require.NoError(t, err)
	require.Len(t, tasks.Tasks, 2)
	require.Equal(t, "it", tasks.Tasks[0].Branch)
	require.Equal(t, "legal", tasks.Tasks[1].Branch)
	require.Equal(t, "IT", tasks.Tasks[0].StateLabel)
	tasks, err = fx.ctl.Tasks(fx.as(t, "bob"), "", 1, 50)
	require.NoError(t, err)
	require.Empty(t, tasks.Tasks)

	// A property outside the branch's writes is refused.
	_, err = fx.ctl.Complete(fx.as(t, "alice"), fx.id, run.Parallel[1].Generation, map[string]any{"documents": true})
	require.ErrorIs(t, err, flowview.ErrInvalid)
	// The run's own generation is no step.
	_, err = fx.ctl.Complete(fx.as(t, "alice"), fx.id, 0, nil)
	require.ErrorIs(t, err, flowview.ErrConflict)

	got, err := fx.ctl.Complete(fx.as(t, "alice"), fx.id, run.Parallel[0].Generation, map[string]any{"documents": true})
	require.NoError(t, err)
	require.True(t, got.Moved)
	require.Equal(t, "joined", got.State)
	after := fx.view(t, "alice")
	require.True(t, after.Parallel[0].Arrived)
	require.False(t, after.Parallel[0].Complete)

	tasks, err = fx.ctl.Tasks(fx.as(t, "alice"), "", 1, 50)
	require.NoError(t, err)
	require.Len(t, tasks.Tasks, 1)
	require.Equal(t, "legal", tasks.Tasks[0].Branch)

	got, err = fx.ctl.Complete(fx.as(t, "alice"), fx.id, after.Parallel[1].Generation, nil)
	require.NoError(t, err)
	require.True(t, got.Finished)
	done := fx.view(t, "alice")
	require.False(t, done.Active)
	require.Equal(t, "completed", done.Status)
}
