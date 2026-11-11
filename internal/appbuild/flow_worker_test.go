package appbuild_test

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowengine"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// workerFlows adds two flows to the sample for the worker tests: retry,
// whose one output creates a task titled with the subject's stage, which
// fails while another task has that title (see workerProject), and review,
// which forks and joins when the subject has its documents.
const workerFlows = `
  retry:
    subject: person
    run_as: system:onboarding-bot
    per_subject: many
    start: [{ on: manual, permission: onboarding.start }]
    initial: mark
    states:
      mark:
        outputs:
          - create: { type: task, properties: { title: "{{subject.stage}}" } }
        on:
          done: { to: marked }
      marked: { final: true }
  review:
    subject: person
    run_as: system:onboarding-bot
    per_subject: many
    start: [{ on: manual, permission: onboarding.start }]
    initial: start
    states:
      start:
        on:
          done: { to: [it, legal] }
      it:
        waiting_for: [hr]
        on:
          change: { to: joined, roles: [hr] }
      legal:
        waiting_for: [manager]
        on:
          change: { to: joined, roles: [manager] }
      joined:
        join: { when: "arrived >= 1 and entity.documents == true" }
        on:
          done: { to: finished }
      finished: { final: true }
`

// testClock is a clock a test moves by hand.
type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// workerProject builds the services of a long-running host over the sample
// project plus workerFlows, on whichever backend this build runs, with the
// flow engine on clock.
func workerProject(t *testing.T, clock *testClock) *appbuild.Services {
	t.Helper()
	root := writeFlowProject(t)
	flows, err := os.ReadFile(filepath.Join(root, "flows.yaml"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "flows.yaml"), append(flows, workerFlows...), 0o600))
	// A task title is unique, so a create output can fail for a while.
	schema, err := os.ReadFile(filepath.Join(root, "metamodel.yaml"))
	require.NoError(t, err)
	unique := strings.Replace(string(schema), "        required: true\n", "        required: true\n        unique: true\n", 1)
	require.NotEqual(t, string(schema), unique)
	require.NoError(t, os.WriteFile(filepath.Join(root, "metamodel.yaml"), []byte(unique), 0o600))
	svc, err := appbuildOnDiskWithOpts(t, root, appbuild.WithBackgroundAutomationJobs(), appbuild.WithClock(clock.Now))
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

// tick runs one tick of svc's flow worker and returns how many runs it
// advanced.
func tick(t *testing.T, svc *appbuild.Services) int {
	t.Helper()
	advanced, failed, ok := appbuild.FlowWorkerTick(context.Background(), svc)
	require.True(t, ok, "a long-running host with flows has a flow worker")
	require.Zero(t, failed)
	return advanced
}

// header returns the header properties of run id.
func header(t *testing.T, svc *appbuild.Services, id flow.RunID) map[string]any {
	t.Helper()
	h, err := svc.Store().GetEntity(context.Background(), entity.Ref{ID: string(id)})
	require.NoError(t, err)
	return h.Properties
}

// setPerson writes props on person id straight into the store, past the
// entity manager. It stands in for an edit the flow engine has no hook for
// yet (TKT-GYXOTQC); production code writes through the manager.
func setPerson(t *testing.T, svc *appbuild.Services, id string, props map[string]any) {
	t.Helper()
	ctx := context.Background()
	p, err := svc.Store().GetEntity(ctx, entity.Ref{ID: id})
	require.NoError(t, err)
	maps.Copy(p.Properties, props)
	require.NoError(t, svc.Store().UpdateEntity(ctx, p))
}

// TestFlowWorker_TimerEscalates pins that a tick fires a due timer: the
// sample run in collect escalates after a week without anyone acting.
func TestFlowWorker_TimerEscalates(t *testing.T) {
	clock := &testClock{now: time.Now().UTC()}
	svc := workerProject(t, clock)
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada", "status": "hired"})
	id, err := appbuild.FlowEngine(svc).Start(context.Background(), "onboarding", entity.Ref{ID: "PER-1"})
	require.NoError(t, err)

	require.Zero(t, tick(t, svc), "nothing is due before the timer")
	require.Equal(t, "collect", header(t, svc, id)[metamodel.FlowRunPropState])

	clock.Add(7*24*time.Hour + time.Minute)
	require.Equal(t, 1, tick(t, svc))
	require.Equal(t, "escalated", header(t, svc, id)[metamodel.FlowRunPropState])
}

// TestFlowWorker_RecoversPendingOutput pins that a tick settles a run whose
// output failed, once its backoff has passed and the cause is gone.
func TestFlowWorker_RecoversPendingOutput(t *testing.T) {
	clock := &testClock{now: time.Now().UTC()}
	svc := workerProject(t, clock)
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada", "stage": "Ada's papers"})
	ctx := context.Background()
	blocker := entity.New("TSK-900", "task")
	blocker.Properties = map[string]any{"title": "Ada's papers"}
	require.NoError(t, svc.Store().CreateEntity(ctx, blocker))
	id, err := appbuild.FlowEngine(svc).Start(ctx, "retry", entity.Ref{ID: "PER-1"})
	require.Error(t, err, "a second task with the same title is refused")
	require.NotEmpty(t, id, "the run exists with its output pending")

	blocker.Properties["title"] = "Someone else's papers"
	require.NoError(t, svc.Store().UpdateEntity(ctx, blocker))
	require.Zero(t, tick(t, svc), "the run waits for its backoff")

	clock.Add(time.Minute)
	require.Equal(t, 1, tick(t, svc))
	require.Equal(t, "completed", header(t, svc, id)[metamodel.FlowRunPropStatus])
	var titles []any
	for _, r := range collectRelations(t, svc.Store(), store.RelationQuery{From: string(id)}) {
		if r.Type != entity.ProducedRelation {
			continue
		}
		task, gerr := svc.Store().GetEntity(ctx, entity.Ref{ID: r.To})
		require.NoError(t, gerr)
		titles = append(titles, task.Properties["title"])
	}
	require.Equal(t, []any{"Ada's papers"}, titles, "the output was written once, as run_as")
}

// TestFlowWorker_JoinWhenBecomesTrue pins that a join whose `when:` was
// false at the arrival continues on a later tick once the subject changed,
// without another arrival.
func TestFlowWorker_JoinWhenBecomesTrue(t *testing.T) {
	clock := &testClock{now: time.Now().UTC()}
	svc := workerProject(t, clock)
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada"})
	ctx := context.Background()
	eng := appbuild.FlowEngine(svc)
	id, err := eng.Start(ctx, "review", entity.Ref{ID: "PER-1"})
	require.NoError(t, err)
	run, err := eng.Run(ctx, id)
	require.NoError(t, err)
	i := slices.IndexFunc(run.Branches, func(b flowrun.Branch) bool { return b.Name == "it" })
	require.GreaterOrEqual(t, i, 0)
	_, err = eng.Change(ctx, id, run.Branches[i].Instance.Generation, "alice",
		func(r string) bool { return r == "hr" })
	require.NoError(t, err)
	require.Equal(t, "joined", header(t, svc, id)[metamodel.FlowRunPropState], "the run waits at the join")
	require.Empty(t, header(t, svc, id)[metamodel.FlowRunPropCompleted])

	setPerson(t, svc, "PER-1", map[string]any{"documents": true})
	require.Zero(t, tick(t, svc), "the join is not tested again before its poll")

	clock.Add(flowengine.JoinRecheck)
	require.Equal(t, 1, tick(t, svc))
	require.Equal(t, "completed", header(t, svc, id)[metamodel.FlowRunPropStatus])
}

// TestFlowWorker_OneLeader pins that the flow worker lock admits one
// holder at a time: the database lock on postgres, the lock file on fs
// and sqlite.
func TestFlowWorker_OneLeader(t *testing.T) {
	svc := workerProject(t, &testClock{now: time.Now().UTC()})
	ctx := context.Background()
	release, ok, err := appbuild.TryFlowWorkerLock(ctx, svc)
	require.NoError(t, err)
	require.True(t, ok)
	_, ok, err = appbuild.TryFlowWorkerLock(ctx, svc)
	require.NoError(t, err)
	require.False(t, ok, "a second worker does not lead while the first holds the lock")
	release()
	again, ok, err := appbuild.TryFlowWorkerLock(ctx, svc)
	require.NoError(t, err)
	require.True(t, ok, "the lock is free once released")
	again()
}

// TestFlowWorker_OnlyLongRunningHosts pins that a one-shot process builds
// no flow worker.
func TestFlowWorker_OnlyLongRunningHosts(t *testing.T) {
	svc, err := appbuildOnDisk(t, writeFlowProject(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	_, _, ok := appbuild.FlowWorkerTick(context.Background(), svc)
	require.False(t, ok)
	done := appbuild.RunFlowWorker(context.Background(), svc)
	<-done
}
