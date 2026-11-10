package appbuild_test

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flowview"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// stepProject is the onboarding sample with alice (hr), bob (manager) and
// a buddy role relation that confers hr on one person.
func stepProject(t *testing.T) (*appbuild.Services, *appbuild.FlowControl) {
	t.Helper()
	root := writeFlowProject(t)
	schema, err := os.ReadFile(filepath.Join("..", "flowcfg", "testdata", "schema.yaml"))
	require.NoError(t, err)
	writeMetamodelBody(t, root, string(schema)+"  buddy:\n    label: buddy\n    from: [person]\n    to: [person]\n")
	policy, err := os.ReadFile(filepath.Join(root, "acl.yaml"))
	require.NoError(t, err)
	p := strings.Replace(string(policy), "assignments:\n", "assignments:\n  alice: hr\n  bob: manager\n  carol: manager\n", 1)
	p += "role_relations:\n  buddy:\n    confers: hr\n"
	writePolicy(t, root, p)
	svc, err := appbuildOnDisk(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	ctl := appbuild.FlowControlFor(svc)
	require.NotNil(t, ctl)
	return svc, ctl
}

func flowUser(t *testing.T, svc *appbuild.Services, user string) context.Context {
	t.Helper()
	ctx, err := appbuild.MustIdentityBinder(svc)(principal.With(context.Background(),
		principal.Principal{User: user, Tool: principal.ToolDataEntry}))
	require.NoError(t, err)
	return ctx
}

func addFlowPerson(t *testing.T, svc *appbuild.Services, id string, props map[string]any) {
	t.Helper()
	p := entity.New(id, "person")
	p.Properties = props
	require.NoError(t, svc.Store().CreateEntity(context.Background(), p))
}

// TestFlowControl_CompleteStep pins AC 1: only a holder of a waiting role
// completes a step, it writes only `writes:` properties as the caller, and
// the change branch moves the run; a hidden run is not found.
func TestFlowControl_CompleteStep(t *testing.T) {
	tests := []struct {
		name    string
		user    string
		props   map[string]any
		gen     int64 // added to the run's generation
		wantErr error
		moved   bool
	}{
		{name: "holder moves the run", user: "alice", props: map[string]any{"documents": true}, moved: true},
		{name: "holder, no branch applies", user: "alice", props: map[string]any{"documents": false}},
		{name: "holder, nothing written", user: "alice"},
		{name: "non-holder", user: "bob", props: map[string]any{"documents": true}, wantErr: flowview.ErrForbidden},
		{name: "hidden run", user: "eve", props: map[string]any{"documents": true}, wantErr: flowview.ErrNotFound},
		{name: "property outside writes", user: "alice", props: map[string]any{"stage": "x"},
			wantErr: flowview.ErrInvalid},
		{name: "stale generation", user: "alice", props: map[string]any{"documents": true}, gen: 1,
			wantErr: flowview.ErrConflict},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, ctl := stepProject(t)
			addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada", "status": "hired"})
			alice := flowUser(t, svc, "alice")
			id, err := ctl.Start(alice, "onboarding", "PER-1")
			require.NoError(t, err)
			run, err := ctl.Run(alice, id)
			require.NoError(t, err)
			require.True(t, run.Actions.Complete)
			require.Equal(t, []string{"documents"}, run.Waiting.Writes)

			got, err := ctl.Complete(flowUser(t, svc, tc.user), id, run.Generation+tc.gen, tc.props)
			require.ErrorIs(t, err, tc.wantErr)
			subj, gerr := svc.Store().GetEntity(context.Background(), entity.Ref{ID: "PER-1"})
			require.NoError(t, gerr)
			if tc.wantErr != nil {
				require.Nil(t, subj.Properties["documents"], "a refused step writes nothing")
				return
			}
			require.Equal(t, tc.moved, got.Moved)
			if tc.props != nil {
				require.Equal(t, tc.props["documents"], subj.Properties["documents"])
			}
			after, err := ctl.Run(alice, id)
			require.NoError(t, err)
			if tc.moved {
				require.True(t, got.Finished)
				require.False(t, after.Active)
				require.Equal(t, "completed", after.Status)
				return
			}
			require.Equal(t, "collect", after.State)
			require.Empty(t, after.Waiting.ClaimedBy, "a step that moves nothing releases its claim")
		})
	}
}

// TestFlowControl_RunViewAndActions pins the run view: the waiting reason,
// the timeline, the diagram, and actions computed like the POST checks.
func TestFlowControl_RunViewAndActions(t *testing.T) {
	svc, ctl := stepProject(t)
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada"})
	alice, bob, eve := flowUser(t, svc, "alice"), flowUser(t, svc, "bob"), flowUser(t, svc, "eve")
	id, err := ctl.Start(alice, "onboarding", "PER-1")
	require.NoError(t, err)

	run, err := ctl.Run(alice, id)
	require.NoError(t, err)
	require.True(t, run.Active)
	require.Equal(t, "PER-1", run.Subject)
	require.Equal(t, "Collect documents", run.StateLabel)
	require.Equal(t, []string{"hr"}, run.Waiting.Roles)
	require.True(t, run.Waiting.YouHold)
	require.Len(t, run.Waiting.Timers, 1)
	require.Len(t, run.Waiting.Branches, 1)
	require.False(t, run.Waiting.Branches[0].Holds)
	require.Len(t, run.Log, 1)
	require.Equal(t, "alice", run.Log[0].By)
	require.Contains(t, run.Diagram, "class ")
	require.Equal(t, flowview.Actions{Complete: true, Cancel: true}, run.Actions)

	run, err = ctl.Run(bob, id)
	require.NoError(t, err)
	require.False(t, run.Waiting.YouHold)
	require.Equal(t, flowview.Actions{}, run.Actions)

	_, err = ctl.Run(eve, id)
	require.ErrorIs(t, err, flowview.ErrNotFound)

	runs, err := ctl.SubjectRuns(alice, "PER-1")
	require.NoError(t, err)
	require.Len(t, runs, 1)
	_, err = ctl.SubjectRuns(eve, "PER-1")
	require.ErrorIs(t, err, flowview.ErrNotFound)

	require.ErrorIs(t, ctl.Retry(alice, id), flowview.ErrConflict, "nothing to retry")
	require.ErrorIs(t, ctl.Retry(bob, id), flowview.ErrForbidden)
	require.ErrorIs(t, ctl.Retry(eve, id), flowview.ErrNotFound)

	flows := ctl.Flows()
	require.Len(t, flows, 1)
	require.True(t, flows[0].Manual)
}

// TestFlowControl_Tasks pins the task list: runs waiting for a role the
// caller holds globally or through a role relation, and nothing else.
func TestFlowControl_Tasks(t *testing.T) {
	svc, ctl := stepProject(t)
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada"})
	addFlowPerson(t, svc, "PER-2", map[string]any{"name": "Bea"})
	addFlowPerson(t, svc, "carol", map[string]any{"name": "Carol"})
	_, err := svc.Store().CreateRelation(context.Background(), entity.RelationKey{
		From: "carol", Type: "buddy", To: "PER-2",
	}, &store.RelationData{})
	require.NoError(t, err)
	alice := flowUser(t, svc, "alice")
	r1, err := ctl.Start(alice, "onboarding", "PER-1")
	require.NoError(t, err)
	r2, err := ctl.Start(alice, "onboarding", "PER-2")
	require.NoError(t, err)

	tasks, err := ctl.Tasks(alice, "", 1, 50)
	require.NoError(t, err)
	require.Equal(t, 2, tasks.Total)
	require.ElementsMatch(t, []string{string(r1), string(r2)}, []string{tasks.Tasks[0].Run, tasks.Tasks[1].Run})

	tasks, err = ctl.Tasks(flowUser(t, svc, "carol"), "", 1, 50)
	require.NoError(t, err)
	require.Len(t, tasks.Tasks, 1, "carol holds hr on PER-2 only")
	require.Equal(t, string(r2), tasks.Tasks[0].Run)
	require.Equal(t, "PER-2", tasks.Tasks[0].Subject)

	for _, user := range []string{"bob", "eve"} {
		tasks, err = ctl.Tasks(flowUser(t, svc, user), "", 1, 50)
		require.NoError(t, err)
		require.Empty(t, tasks.Tasks, user)
	}
	tasks, err = ctl.Tasks(alice, "other", 1, 50)
	require.NoError(t, err)
	require.Empty(t, tasks.Tasks)
	tasks, err = ctl.Tasks(alice, "", 2, 1)
	require.NoError(t, err)
	require.Len(t, tasks.Tasks, 1)
	require.Equal(t, 2, tasks.Total)

	// A completed step leaves the list.
	run, err := ctl.Run(alice, r1)
	require.NoError(t, err)
	_, err = ctl.Complete(alice, r1, run.Generation, map[string]any{"documents": true})
	require.NoError(t, err)
	tasks, err = ctl.Tasks(alice, "", 1, 50)
	require.NoError(t, err)
	require.Len(t, tasks.Tasks, 1)
	require.Equal(t, string(r2), tasks.Tasks[0].Run)
}

// TestFlowControl_RunList pins the run list: the runs whose headers the
// caller can read, newest first, filtered and paged.
func TestFlowControl_RunList(t *testing.T) {
	svc, ctl := stepProject(t)
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada"})
	addFlowPerson(t, svc, "PER-2", map[string]any{"name": "Bea"})
	alice := flowUser(t, svc, "alice")
	r1, err := ctl.Start(alice, "onboarding", "PER-1")
	require.NoError(t, err)
	_, err = ctl.Start(alice, "onboarding", "PER-2")
	require.NoError(t, err)
	require.NoError(t, ctl.Cancel(alice, r1))

	tests := []struct {
		name           string
		user           string
		flow, status   string
		page, perPage  int
		want, wantRows int
	}{
		{name: "all", user: "alice", want: 2, wantRows: 2},
		{name: "by status", user: "alice", status: "cancelled", want: 1, wantRows: 1},
		{name: "by flow", user: "alice", flow: "other", want: 0, wantRows: 0},
		{name: "second page", user: "alice", page: 2, perPage: 1, want: 2, wantRows: 1},
		{name: "past the end", user: "alice", page: 3, perPage: 1, want: 2, wantRows: 0},
		{name: "huge page", user: "alice", page: math.MaxInt, perPage: 50, want: 2, wantRows: 0},
		{name: "huge page size", user: "alice", page: 1, perPage: math.MaxInt, want: 2, wantRows: 2},
		{name: "hidden runs", user: "eve", want: 0, wantRows: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ctl.RunList(flowUser(t, svc, tc.user), tc.flow, tc.status, tc.page, tc.perPage)
			require.NoError(t, err)
			require.Equal(t, tc.want, got.Total)
			require.Len(t, got.Runs, tc.wantRows)
			require.False(t, got.Truncated)
			for _, r := range got.Runs {
				require.NotEmpty(t, r.Subject)
				require.Equal(t, r.Status != "cancelled", r.Active)
			}
		})
	}
}
