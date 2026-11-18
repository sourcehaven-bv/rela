package dataentry

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flowview"
)

// taskFlows serves fixed tasks per flow, as FlowControl.Tasks does: the
// flow filter keeps one flow, "" keeps all.
type taskFlows struct {
	fakeFlows
	tasks []flowview.Task
}

func (f *taskFlows) Flows() []flowview.Flow {
	return []flowview.Flow{
		{Name: "signup", Label: "Signup", Subject: "ticket"},
		{Name: "review", Subject: "ticket"},
	}
}

func (f *taskFlows) Tasks(_ context.Context, name string, page, perPage int) (flowview.Tasks, error) {
	f.rec("tasks %s %d %d", name, page, perPage)
	out := flowview.Tasks{Tasks: []flowview.Task{}}
	for _, t := range f.tasks {
		if name == "" || t.Flow == name {
			out.Tasks = append(out.Tasks, t)
		}
	}
	return out, f.err
}

func flowTaskApp(t *testing.T, src dataentryconfig.NextActionSource, tasks ...flowview.Task) (*App, *taskFlows) {
	t.Helper()
	app := newTestAppV1(t)
	seedNextActionTickets(t, app, "TKT-001", "TKT-002")
	svc := &taskFlows{tasks: tasks}
	require.NoError(t, SetFlows(app, svc))
	src.Band = "banner"
	src.Source = dataentryconfig.NextActionSourceFlowTasks
	if src.Suggest == "" {
		src.Suggest = "Complete {step} for {subject}"
	}
	withNextActions(t, app,
		[]dataentryconfig.NextActionBand{{ID: "banner", Prominence: dataentryconfig.ProminenceBanner}},
		map[string]dataentryconfig.NextActionSource{"flow-tasks": src})
	return app, svc
}

var t0 = time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)

func waiting(run, flowName, subject string, started time.Time) flowview.Task {
	label := ""
	if flowName == "signup" {
		label = "Signup"
	}
	return flowview.Task{
		Run: run, Flow: flowName, FlowLabel: label, State: "details", StateLabel: "Company details",
		Subject: subject, Started: started,
	}
}

func TestNextAction_FlowTasksOldestWithTitleAndLink(t *testing.T) {
	app, _ := flowTaskApp(t, dataentryconfig.NextActionSource{},
		waiting("RUN-2", "signup", "TKT-002", t0.Add(time.Hour)),
		waiting("RUN-1", "review", "TKT-001", t0),
	)

	got, code := getNextAction(aliceCtx(), t, app)
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, got.Suggestion)
	require.Equal(t, "TKT-001", got.Suggestion.EntityID, "the oldest task across flows")
	require.Equal(t, "Complete Company details for Ticket TKT-001", got.Suggestion.Message)
	require.Equal(t, "run=RUN-1;state=details", got.Suggestion.Variant)
	require.Len(t, got.Suggestion.Actions, 1)
	require.Equal(t, "/entity/ticket/TKT-001?run=RUN-1", got.Suggestion.Actions[0].Navigate)
	require.Equal(t, []string{entity.FlowRunType}, got.RefreshTypes,
		"a completed step changes the run, so the page re-resolves on run changes")
	require.Empty(t, got.ContextType)
}

func TestNextAction_FlowTasksFlowFilter(t *testing.T) {
	app, svc := flowTaskApp(t, dataentryconfig.NextActionSource{
		Flows: []string{"signup", "gone"}, Suggest: "{flow}: {step}",
	},
		waiting("RUN-1", "review", "TKT-001", t0),
		waiting("RUN-2", "signup", "TKT-002", t0.Add(time.Hour)),
	)

	got, code := getNextAction(aliceCtx(), t, app)
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, got.Suggestion)
	require.Equal(t, "TKT-002", got.Suggestion.EntityID)
	require.Equal(t, "Signup: Company details", got.Suggestion.Message)
	require.Equal(t, []string{"tasks signup 1 500"}, svc.calls,
		"one call per declared flow; an undeclared flow is skipped")
}

// A subject the read does not return still yields its task, named by its
// address, so the hint names the same tasks as the Flow tasks list.
func TestNextAction_FlowTasksUnreadSubjectUsesAddress(t *testing.T) {
	app, _ := flowTaskApp(t, dataentryconfig.NextActionSource{},
		waiting("RUN-1", "review", "TKT-404", t0))

	got, code := getNextAction(aliceCtx(), t, app)
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, got.Suggestion)
	require.Equal(t, "Complete Company details for TKT-404", got.Suggestion.Message)
}

func TestNextAction_FlowTasksSnoozeHidesOnlyTheHint(t *testing.T) {
	app, svc := flowTaskApp(t, dataentryconfig.NextActionSource{},
		waiting("RUN-1", "review", "TKT-001", t0))
	ctx := aliceCtx()

	got, _ := getNextAction(ctx, t, app)
	require.NotNil(t, got.Suggestion)
	require.Equal(t, http.StatusNoContent, postFeedback(ctx, t, app,
		`{"source":"flow-tasks","entity_id":"TKT-001","variant":"run=RUN-1;state=details","kind":"snooze","duration":"1d"}`))

	got, _ = getNextAction(ctx, t, app)
	require.Nil(t, got.Suggestion)
	tasks, err := svc.Tasks(ctx, "", 1, 50)
	require.NoError(t, err)
	require.Len(t, tasks.Tasks, 1, "the task itself stays")
}

func TestNextAction_FlowTasksWithoutIdentityContributeNothing(t *testing.T) {
	app, svc := flowTaskApp(t, dataentryconfig.NextActionSource{},
		waiting("RUN-1", "review", "TKT-001", t0))
	svc.err = acl.ErrNoIdentity

	got, code := getNextAction(aliceCtx(), t, app)
	require.Equal(t, http.StatusOK, code)
	require.Nil(t, got.Suggestion)
}

func TestNextAction_FlowTasksNoFlowsWired(t *testing.T) {
	app := newTestAppV1(t)
	withNextActions(t, app,
		[]dataentryconfig.NextActionBand{{ID: "banner"}},
		map[string]dataentryconfig.NextActionSource{"flow-tasks": {
			Band: "banner", Source: dataentryconfig.NextActionSourceFlowTasks, Suggest: "x",
		}})

	got, code := getNextAction(aliceCtx(), t, app)
	require.Equal(t, http.StatusOK, code)
	require.Nil(t, got.Suggestion)
}

// nextActionFlowTasksBudget is the store reads of one resolve with a
// flow_tasks source: the subjects of the kept tasks in one id-bounded read
// (which faces match, then the rows of that face), and the two membership
// reads that resolve the principal's roles. The task list's own reads are
// pinned by the flow tests; here the flow service is a fake.
const nextActionFlowTasksBudget = 4

// TestNextActionBudget_FlowTasksReadSubjectsOnce pins that the subjects of
// a flow_tasks source are read in one batch, not one read per task.
func TestNextActionBudget_FlowTasksReadSubjectsOnce(t *testing.T) {
	op := func(t *testing.T, app *App, d *acl.Declarative, ctx context.Context) {
		t.Helper()
		tasks := make([]flowview.Task, 0, budgetSizes[1])
		for i := 1; i <= budgetSizes[1]; i++ {
			tasks = append(tasks, waiting(fmt.Sprintf("RUN-%04d", i), "review",
				fmt.Sprintf("TKT-%04d", i), t0.Add(time.Duration(i)*time.Minute)))
		}
		app.flows = nil
		require.NoError(t, SetFlows(app, &taskFlows{tasks: tasks}))
		withNextActions(t, app,
			[]dataentryconfig.NextActionBand{{ID: "work"}},
			map[string]dataentryconfig.NextActionSource{"tasks": {
				Band: "work", Source: dataentryconfig.NextActionSourceFlowTasks, Suggest: "{subject}",
			}})
		got, code := getNextAction(gateCtxFor(ctx, t, d), t, app)
		require.Equal(t, http.StatusOK, code)
		require.NotNil(t, got.Suggestion)
		require.Equal(t, "TKT-0001", got.Suggestion.EntityID)
	}
	c, detail := readsFor(t, op)
	assertBudget(t, "next-action flow_tasks source", c, nextActionFlowTasksBudget, detail)
}

// The page names every type whose change can settle its answer: the
// context type, and the run type when a flow_tasks source competes.
func TestNextActionPage_RefreshTypes(t *testing.T) {
	t.Parallel()
	contextSrc := dataentryconfig.NextActionSource{Band: "b", Context: "ticket", Suggest: "x"}
	tasksSrc := dataentryconfig.NextActionSource{Band: "b", Source: dataentryconfig.NextActionSourceFlowTasks}
	entityPage := nextActionPage{surface: "entity", view: "ticket", contextType: "ticket", entityID: "TKT-1"}
	listPage := nextActionPage{surface: dataentryconfig.SurfaceList, view: "tickets"}
	tests := []struct {
		name    string
		sources map[string]dataentryconfig.NextActionSource
		page    nextActionPage
		want    []string
	}{
		{"context source on its entity page", map[string]dataentryconfig.NextActionSource{"c": contextSrc},
			entityPage, []string{"ticket"}},
		{"context and tasks on an entity page",
			map[string]dataentryconfig.NextActionSource{"c": contextSrc, "t": tasksSrc},
			entityPage, []string{"ticket", entity.FlowRunType}},
		{"tasks on a list", map[string]dataentryconfig.NextActionSource{"c": contextSrc, "t": tasksSrc},
			listPage, []string{entity.FlowRunType}},
		{"nothing to refresh", map[string]dataentryconfig.NextActionSource{"c": contextSrc}, listPage, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := &dataentryconfig.Config{NextActions: tc.sources}
			require.Equal(t, tc.want, tc.page.refreshTypes(cfg))
		})
	}
}
