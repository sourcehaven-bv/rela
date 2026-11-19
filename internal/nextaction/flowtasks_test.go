package nextaction_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/nextaction"
	"github.com/Sourcehaven-BV/rela/internal/userstate"
	"github.com/Sourcehaven-BV/rela/internal/userstate/memuserstate"
)

func flowTaskConfig(src dataentryconfig.NextActionSource) *dataentryconfig.Config {
	src.Band = "banner"
	src.Source = dataentryconfig.NextActionSourceFlowTasks
	if src.Suggest == "" {
		src.Suggest = "Complete {step} for {subject}"
	}
	return &dataentryconfig.Config{
		NextActionBands: []dataentryconfig.NextActionBand{{ID: "banner", Prominence: "banner"}},
		NextActions:     map[string]dataentryconfig.NextActionSource{"tasks": src},
	}
}

func task(run, subject string) nextaction.Candidate {
	return nextaction.Candidate{
		Entity: ent(subject, "account", map[string]any{"name": "Northwind " + subject, "step": "own"}),
		Task: &nextaction.FlowTask{
			Run: run, State: "details", Flow: "Signup", Step: "Company details",
			Subject: "Northwind " + subject, SubjectType: "account",
			Link: "/entity/account/" + subject + "?run=" + run,
		},
	}
}

// tasksFunc serves the candidates in the given order, oldest first, as the
// wiring site does.
func tasksFunc(cands *[]nextaction.Candidate) nextaction.CandidateFunc {
	return func(context.Context, string, dataentryconfig.NextActionSource, nextaction.Want) ([]nextaction.Candidate, error) {
		return *cands, nil
	}
}

func TestFlowTasks_MessageAndLink(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		src         dataentryconfig.NextActionSource
		wantMessage string
		wantNav     []string
	}{
		{
			name:        "default link to the step",
			src:         dataentryconfig.NextActionSource{},
			wantMessage: "Complete Company details for Northwind ACC-1",
			wantNav:     []string{"/entity/account/ACC-1?run=RUN-1"},
		},
		{
			name: "declared navigate is filled, no default added",
			src: dataentryconfig.NextActionSource{
				Suggest: "{flow}: {step} ({name}, {missing})",
				Actions: []dataentryconfig.NextActionOffer{
					{Navigate: "/entity/{type}/{id}?run={run}"}, {Snooze: []string{"1d"}},
				},
			},
			wantMessage: "Signup: Company details (Northwind ACC-1, {missing})",
			wantNav:     []string{"/entity/account/ACC-1?run=RUN-1", ""},
		},
		{
			name: "task placeholders win over subject properties",
			src: dataentryconfig.NextActionSource{
				Suggest: "{step}",
				Actions: []dataentryconfig.NextActionOffer{{Dismiss: true}},
			},
			wantMessage: "Company details",
			wantNav:     []string{"/entity/account/ACC-1?run=RUN-1", ""},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cands := []nextaction.Candidate{task("RUN-1", "ACC-1")}
			cfg := flowTaskConfig(tc.src)
			eng, _ := newEngine(t, cfg, tasksFunc(&cands))

			got, ok, err := eng.Resolve(context.Background(), testUser, base)
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, tc.wantMessage, got.Message)
			require.Equal(t, "ACC-1", got.EntityID)
			require.Equal(t, "run=RUN-1;state=details", got.Key.Variant)
			nav := make([]string, 0, len(got.Actions))
			for _, o := range got.Actions {
				nav = append(nav, o.Navigate)
			}
			require.Equal(t, tc.wantNav, nav)
			for _, o := range cfg.NextActions["tasks"].Actions {
				require.NotContains(t, o.Navigate, "RUN-1", "the config's offers are never rewritten")
			}
		})
	}
}

// A placeholder is escaped for where it lands: a path segment before the
// `?`, a query value after it.
func TestFlowTasks_LinkEscaping(t *testing.T) {
	t.Parallel()
	c := task("R&x=1/2", "A B")
	cands := []nextaction.Candidate{c}
	cfg := flowTaskConfig(dataentryconfig.NextActionSource{
		Actions: []dataentryconfig.NextActionOffer{{Navigate: "/entity/{type}/{id}?run={run}&tab=flows"}},
	})
	eng, _ := newEngine(t, cfg, tasksFunc(&cands))
	got, ok, err := eng.Resolve(context.Background(), testUser, base)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "/entity/account/A%20B?run=R%26x%3D1%2F2&tab=flows", got.Actions[0].Navigate)
}

// The source offers its oldest task that is not snoozed, and a snooze hides
// only that task.
func TestFlowTasks_OldestNotSnoozed(t *testing.T) {
	t.Parallel()
	cands := []nextaction.Candidate{task("RUN-1", "ACC-1"), task("RUN-2", "ACC-2"), task("RUN-3", "ACC-3")}
	eng, st := newEngine(t, flowTaskConfig(dataentryconfig.NextActionSource{}), tasksFunc(&cands))
	ctx := context.Background()

	got, ok, err := eng.Resolve(ctx, testUser, base)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "ACC-1", got.EntityID)

	// Seeing it does not hide it: a flow task is required work.
	require.NoError(t, eng.MarkShown(ctx, got, base))
	got, _, err = eng.Resolve(ctx, testUser, base.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, "ACC-1", got.EntityID)

	require.NoError(t, st.SetSnooze(ctx, got.Key, base.Add(24*time.Hour)))
	got, ok, err = eng.Resolve(ctx, testUser, base.Add(time.Minute))
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "ACC-2", got.EntityID)

	// Completing the oldest task drops it from the next resolve.
	cands = cands[1:]
	got, _, err = eng.Resolve(ctx, testUser, base.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, "ACC-2", got.EntityID)
}

// Snoozed tasks do not use up the candidate cap: past twenty snoozed tasks,
// the twenty-first still shows.
func TestFlowTasks_SnoozedTasksDoNotUseTheCap(t *testing.T) {
	t.Parallel()
	var cands []nextaction.Candidate
	for i := range 21 {
		cands = append(cands, task(fmt.Sprintf("RUN-%02d", i), fmt.Sprintf("ACC-%02d", i)))
	}
	eng, st := newEngine(t, flowTaskConfig(dataentryconfig.NextActionSource{}), tasksFunc(&cands))
	ctx := context.Background()
	for _, c := range cands[:20] {
		key := userstate.Key{User: testUser, Source: "tasks", EntityID: c.Entity.ID,
			Variant: "run=" + c.Task.Run + ";state=details"}
		require.NoError(t, st.SetSnooze(ctx, key, base.Add(24*time.Hour)))
	}
	got, ok, err := eng.Resolve(ctx, testUser, base)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "ACC-20", got.EntityID)
}

// A run that moves on to another waiting state is a new suggestion: the old
// snooze was for the old step.
func TestFlowTasks_NewStateIsNewSuggestion(t *testing.T) {
	t.Parallel()
	cands := []nextaction.Candidate{task("RUN-1", "ACC-1")}
	eng, st := newEngine(t, flowTaskConfig(dataentryconfig.NextActionSource{}), tasksFunc(&cands))
	ctx := context.Background()

	got, _, err := eng.Resolve(ctx, testUser, base)
	require.NoError(t, err)
	require.NoError(t, st.SetSnooze(ctx, got.Key, base.Add(24*time.Hour)))
	_, ok, err := eng.Resolve(ctx, testUser, base)
	require.NoError(t, err)
	require.False(t, ok)

	moved := task("RUN-1", "ACC-1")
	moved.Task.State = "contract"
	cands = []nextaction.Candidate{moved}
	got, ok, err = eng.Resolve(ctx, testUser, base)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "run=RUN-1;state=contract", got.Key.Variant)
}

// A declared cooldown applies as on any source; the suggestion on screen
// keeps its place over an older task.
func TestFlowTasks_CooldownAndCurrent(t *testing.T) {
	t.Parallel()
	cands := []nextaction.Candidate{task("RUN-1", "ACC-1"), task("RUN-2", "ACC-2")}
	cfg := flowTaskConfig(dataentryconfig.NextActionSource{Cooldown: "1d"})
	eng, _ := newEngine(t, cfg, tasksFunc(&cands))
	ctx := context.Background()

	got, _, err := eng.Resolve(ctx, testUser, base)
	require.NoError(t, err)
	require.NoError(t, eng.MarkShown(ctx, got, base))
	got, _, err = eng.Resolve(ctx, testUser, base.Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, "ACC-2", got.EntityID, "a declared cooldown hides the shown task")

	st := memuserstate.New()
	t.Cleanup(func() { _ = st.Close() })
	cur := &userstate.Key{Source: "tasks", EntityID: "ACC-2", Variant: "run=RUN-2;state=details"}
	eng2, err := nextaction.New(cfg, st, tasksFunc(&cands), nextaction.WithCurrent(cur))
	require.NoError(t, err)
	got, _, err = eng2.Resolve(ctx, testUser, base)
	require.NoError(t, err)
	require.Equal(t, "ACC-2", got.EntityID, "the task on screen stays put")
}

// A source-scoped flow_tasks source keys on the source alone.
func TestFlowTasks_SourceScope(t *testing.T) {
	t.Parallel()
	cands := []nextaction.Candidate{task("RUN-1", "ACC-1")}
	eng, _ := newEngine(t,
		flowTaskConfig(dataentryconfig.NextActionSource{DeferScope: dataentryconfig.DeferScopeSource}),
		tasksFunc(&cands))
	got, _, err := eng.Resolve(context.Background(), testUser, base)
	require.NoError(t, err)
	require.Equal(t, userstate.Key{User: testUser, Source: "tasks"}, got.Key)
	require.Equal(t, "ACC-1", got.EntityID)
}
