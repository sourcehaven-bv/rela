package appbuild_test

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowview"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// stepProject is the onboarding sample with alice (hr), bob (manager) and
// a buddy role relation that confers hr on one person.
func stepProject(t *testing.T) (*appbuild.Services, *appbuild.FlowControl) {
	t.Helper()
	return stepProjectWith(t, "")
}

// stepProjectWith is stepProject with extra appended to acl.yaml.
func stepProjectWith(t *testing.T, extra string) (*appbuild.Services, *appbuild.FlowControl) {
	t.Helper()
	return stepProjectEdit(t, "name", func(p string) string { return p + extra })
}

// stepProjectAt is stepProjectWith that also returns the project root.
func stepProjectAt(t *testing.T, extra string) (*appbuild.Services, *appbuild.FlowControl, string) {
	t.Helper()
	keep := func(s string) string { return s }
	return stepProjectFlowsAt(t, "name", keep, func(p string) string { return p + extra }, keep)
}

// stepProjectEdit is stepProject with display as the person's display
// property and acl.yaml passed through edit.
func stepProjectEdit(t *testing.T, display string, edit func(string) string) (*appbuild.Services,
	*appbuild.FlowControl) {
	t.Helper()
	return stepProjectSchema(t, display, func(s string) string { return s }, edit)
}

// stepProjectSchema is stepProjectEdit with schema.yaml passed through
// editSchema too.
func stepProjectSchema(t *testing.T, display string, editSchema, edit func(string) string) (*appbuild.Services,
	*appbuild.FlowControl) {
	t.Helper()
	return stepProjectFlows(t, display, editSchema, edit, func(s string) string { return s })
}

// stepProjectFlows is stepProjectSchema with flows.yaml passed through
// editFlows too.
func stepProjectFlows(t *testing.T, display string, editSchema, edit, editFlows func(string) string) (
	*appbuild.Services, *appbuild.FlowControl) {
	t.Helper()
	svc, ctl, _ := stepProjectFlowsAt(t, display, editSchema, edit, editFlows)
	return svc, ctl
}

// stepProjectFlowsAt is stepProjectFlows that also returns the project root.
func stepProjectFlowsAt(t *testing.T, display string, editSchema, edit, editFlows func(string) string) (
	*appbuild.Services, *appbuild.FlowControl, string) {
	t.Helper()
	root := writeFlowProject(t)
	flows, err := os.ReadFile(filepath.Join(root, "flows.yaml"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "flows.yaml"), []byte(editFlows(string(flows))), 0o600))
	schema, err := os.ReadFile(filepath.Join("..", "flowcfg", "testdata", "schema.yaml"))
	require.NoError(t, err)
	named := strings.Replace(editSchema(string(schema)), "    label: Person\n",
		"    label: Person\n    display_property: "+display+"\n", 1)
	writeMetamodelBody(t, root, named+"  buddy:\n    label: buddy\n    from: [person]\n    to: [person]\n")
	policy, err := os.ReadFile(filepath.Join(root, "acl.yaml"))
	require.NoError(t, err)
	p := strings.Replace(string(policy), "assignments:\n", "assignments:\n  alice: hr\n  bob: manager\n  carol: manager\n", 1)
	p += "role_relations:\n  buddy:\n    confers: hr\n"
	writePolicy(t, root, edit(p))
	svc, err := appbuildOnDisk(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	ctl := appbuild.FlowControlFor(svc)
	require.NotNil(t, ctl)
	return svc, ctl, root
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
// completes a step, it writes only the fields of the state's `form:` as the
// caller, and
// the change branch moves the run; a hidden run is not found.
func TestFlowControl_CompleteStep(t *testing.T) {
	tests := []struct {
		name    string
		user    string
		props   map[string]any
		gen     int64 // added to the run's generation
		wantErr error
		moved   bool
		problem string
	}{
		{name: "holder moves the run", user: "alice", props: map[string]any{"documents": true}, moved: true},
		{name: "holder, no branch applies", user: "alice", props: map[string]any{"documents": false},
			problem: flowview.ProblemBlocked},
		{name: "holder, nothing written", user: "alice", problem: flowview.ProblemBlocked},
		{name: "non-holder", user: "bob", props: map[string]any{"documents": true}, wantErr: flowview.ErrForbidden},
		{name: "hidden run", user: "eve", props: map[string]any{"documents": true}, wantErr: flowview.ErrNotFound},
		{name: "property outside the form", user: "alice", props: map[string]any{"stage": "x"},
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
			require.Equal(t, "collect_documents", run.Waiting.Form)
			require.Equal(t, []string{"documents"}, run.Waiting.Fields)

			got, err := ctl.Complete(flowUser(t, svc, tc.user), id, run.Generation+tc.gen, tc.props)
			require.ErrorIs(t, err, tc.wantErr)
			subj, gerr := svc.Store().GetEntity(context.Background(), entity.Ref{ID: "PER-1"})
			require.NoError(t, gerr)
			if tc.wantErr != nil {
				require.Nil(t, subj.Properties["documents"], "a refused step writes nothing")
				return
			}
			require.Equal(t, tc.moved, got.Moved)
			require.Equal(t, tc.problem, got.Problem)
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

// TestFlowControl_StepFormReadPerStep pins that a step reads its form from
// data-entry.yaml as it is now: an edit applies without a restart, and a
// form that can no longer be used refuses the step instead of completing it
// with nothing written (RR-YNSWLUM, RR-DWWR4SB).
func TestFlowControl_StepFormReadPerStep(t *testing.T) {
	tests := []struct {
		name      string
		dataEntry string
		props     map[string]any
		fields    []string
		wantErr   error
	}{
		{name: "a field added after start",
			dataEntry: "forms:\n  collect_documents:\n    entity_type: person\n    fields:\n" +
				"      - property: documents\n      - property: stage\n",
			props: map[string]any{"stage": "x"}, fields: []string{"documents", "stage"}},
		{name: "a field removed after start",
			dataEntry: "forms:\n  collect_documents:\n    entity_type: person\n    fields: []\n",
			props:     map[string]any{"documents": true}, fields: []string{}, wantErr: flowview.ErrInvalid},
		{name: "the form removed after start", dataEntry: "forms: {}\n", wantErr: flowview.ErrConflict},
		{name: "the form made a wizard after start",
			dataEntry: "forms:\n  collect_documents:\n    entity_type: person\n    steps: [{title: a}]\n",
			wantErr:   flowview.ErrConflict},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, ctl, root := stepProjectAt(t, "")
			addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada"})
			alice := flowUser(t, svc, "alice")
			id, err := ctl.Start(alice, "onboarding", "PER-1")
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(root, "data-entry.yaml"), []byte(tc.dataEntry), 0o600))

			run, err := ctl.Run(alice, id)
			require.NoError(t, err)
			unusable := errors.Is(tc.wantErr, flowview.ErrConflict)
			require.Equal(t, unusable, run.Waiting.FormProblem != "", run.Waiting.FormProblem)
			require.Equal(t, !unusable, run.Actions.Complete)
			if !unusable && len(tc.fields) > 0 {
				require.Equal(t, tc.fields, run.Waiting.Fields)
			}

			_, err = ctl.Complete(alice, id, run.Generation, tc.props)
			require.ErrorIs(t, err, tc.wantErr)
			after, err := ctl.Run(alice, id)
			require.NoError(t, err)
			if tc.wantErr != nil {
				require.Equal(t, "collect", after.State, "a refused step does not move the run")
			}
		})
	}
}

// TestFlowControl_TimelineNamesActor pins that a timeline entry names the
// person who caused it by the title the reader may see. Only a principal
// mapped onto a person (principal_property) is named; a login that
// happens to equal an entity id, or a person whose title the reader may
// not see, keeps only its id.
func TestFlowControl_TimelineNamesActor(t *testing.T) {
	// The viewer sees every person field but the email.
	hidden := "        - field: name\n        - field: status\n        - field: documents\n" +
		"        - field: stage\n        - field: started\n        - field: salary\n        - field: task_count\n"
	mapped := func(p string) string {
		p = strings.Replace(p, "assignments:\n", "assignments:\n  PER-9: hr\n  dan: viewer\n", 1)
		p = strings.Replace(p, "roles:\n", "roles:\n  viewer:\n    read: [person, task]\n"+
			"    visible:\n      person:\n"+hidden, 1)
		return "user_entity_type: person\nprincipal_property: email\n" + p
	}
	withEmail := func(s string) string {
		return strings.Replace(s, "      name:\n        type: string\n",
			"      name:\n        type: string\n      email:\n        type: string\n        unique: true\n", 1)
	}
	tests := []struct {
		name, reader, want string
		policy             func(string) string
	}{
		{name: "mapped person, reader sees the title", reader: "alice", want: "nina@example.org", policy: mapped},
		{name: "mapped person, title hidden from the reader", reader: "dan", want: "", policy: mapped},
		{name: "no person mapping: a login is not looked up", reader: "alice", want: "",
			policy: func(p string) string { return strings.Replace(p, "assignments:\n", "assignments:\n  PER-9: hr\n", 1) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// The email is the title, so a reader without it sees only the id.
			svc, ctl := stepProjectSchema(t, "email", withEmail, tc.policy)
			addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada", "email": "ada@example.org"})
			addFlowPerson(t, svc, "PER-9", map[string]any{"name": "Nina", "email": "nina@example.org"})
			// The data-entry edge stamps a matched principal as its entity
			// id (access.Restamp), so the run records PER-9.
			id, err := ctl.Start(flowUser(t, svc, "PER-9"), "onboarding", "PER-1")
			require.NoError(t, err)

			run, err := ctl.Run(flowUser(t, svc, tc.reader), id)
			require.NoError(t, err)
			require.Equal(t, "PER-9", run.Log[0].By)
			require.Equal(t, tc.want, run.Log[0].ByTitle)
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
	require.Equal(t, flowview.Actions{Complete: true, Manage: true, Cancel: true}, run.Actions)

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
	require.Equal(t, "one", flows[0].PerSubject)
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

// TestFlowControl_RoleRelationWithInheritance pins that a role held
// through a role relation on the subject reads the run, lists its task and
// completes its step when acl.yaml declares inherit_roles_through
// (BUG-A7QDXU1). dan holds no global role, so the role relation is the
// only route to the subject.
func TestFlowControl_RoleRelationWithInheritance(t *testing.T) {
	svc, ctl := stepProjectWith(t, "inherit_roles_through: [mentions]\n")
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada"})
	addFlowPerson(t, svc, "dan", map[string]any{"name": "Dan"})
	_, err := svc.Store().CreateRelation(context.Background(), entity.RelationKey{
		From: "dan", Type: "buddy", To: "PER-1",
	}, &store.RelationData{})
	require.NoError(t, err)
	id, err := ctl.Start(flowUser(t, svc, "alice"), "onboarding", "PER-1")
	require.NoError(t, err)

	dan := flowUser(t, svc, "dan")
	run, err := ctl.Run(dan, id)
	require.NoError(t, err)
	require.True(t, run.Waiting.YouHold)
	runs, err := ctl.SubjectRuns(dan, "PER-1")
	require.NoError(t, err)
	require.Len(t, runs, 1)
	tasks, err := ctl.Tasks(dan, "", 1, 50)
	require.NoError(t, err)
	require.Len(t, tasks.Tasks, 1)
	require.Equal(t, string(id), tasks.Tasks[0].Run)
	got, err := ctl.Complete(dan, id, run.Generation, map[string]any{"documents": true})
	require.NoError(t, err)
	require.True(t, got.Moved)
}

// TestFlowControl_Startable pins which flows a caller may start on a
// subject: those whose manual start permission the caller holds for it,
// also while a run is open; a subject the caller cannot read is not found.
func TestFlowControl_Startable(t *testing.T) {
	svc, ctl := stepProject(t)
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada"})
	_, err := ctl.Start(flowUser(t, svc, "alice"), "onboarding", "PER-1")
	require.NoError(t, err)
	tests := []struct {
		user    string
		want    []string
		wantErr error
	}{
		{user: "alice", want: []string{"onboarding"}},
		{user: "bob", want: []string{}},
		{user: "eve", wantErr: flowview.ErrNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.user, func(t *testing.T) {
			ctx := flowUser(t, svc, tc.user)
			got, err := ctl.Startable(ctx, "PER-1")
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
			// Start agrees: it refuses a flow left out for a missing
			// permission, and only per_subject: one refuses a listed one.
			_, err = ctl.Start(ctx, "onboarding", "PER-1")
			if len(got) == 0 {
				require.ErrorIs(t, err, flowview.ErrForbidden)
			} else {
				require.NotErrorIs(t, err, flowview.ErrForbidden)
			}
		})
	}
}

// TestFlowControl_ManualStartWhen pins that a manual start's `when:`
// decides whether the flow is offered and started on a subject: Startable
// leaves the flow out and Start refuses it as a conflict while the
// condition does not hold (finding 15).
func TestFlowControl_ManualStartWhen(t *testing.T) {
	id := func(s string) string { return s }
	svc, ctl := stepProjectFlows(t, "name", id, id, func(s string) string {
		return strings.Replace(s, "permission: onboarding.start",
			"permission: onboarding.start\n        when: \"entity.status ~= 'left'\"", 1)
	})
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada", "status": "left"})
	addFlowPerson(t, svc, "PER-2", map[string]any{"name": "Bea", "status": "new"})
	alice := flowUser(t, svc, "alice")

	got, err := ctl.Startable(alice, "PER-1")
	require.NoError(t, err)
	require.Empty(t, got, "the condition does not hold, so the flow is not offered")
	_, err = ctl.Start(alice, "onboarding", "PER-1")
	require.ErrorIs(t, err, flowview.ErrConflict)

	got, err = ctl.Startable(alice, "PER-2")
	require.NoError(t, err)
	require.Equal(t, []string{"onboarding"}, got)
	_, err = ctl.Start(alice, "onboarding", "PER-2")
	require.NoError(t, err)

	// A caller without the permission is still refused as forbidden, not
	// told whether the condition holds.
	_, err = ctl.Start(flowUser(t, svc, "bob"), "onboarding", "PER-1")
	require.ErrorIs(t, err, flowview.ErrForbidden)
}

// TestFlowControl_SubjectTitle pins that the run list, the tasks, the run
// detail and the subject's runs name the subject by its title; the list
// does so for an active and a finished run.
func TestFlowControl_SubjectTitle(t *testing.T) {
	svc, ctl := stepProject(t)
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada"})
	addFlowPerson(t, svc, "PER-2", map[string]any{"name": "Bea"})
	alice := flowUser(t, svc, "alice")
	r1, err := ctl.Start(alice, "onboarding", "PER-1")
	require.NoError(t, err)
	r2, err := ctl.Start(alice, "onboarding", "PER-2")
	require.NoError(t, err)
	require.NoError(t, ctl.Cancel(alice, r2))
	want := map[string]string{string(r1): "Ada", string(r2): "Bea"}

	list, err := ctl.RunList(alice, "", "", 1, 50)
	require.NoError(t, err)
	require.Len(t, list.Runs, 2)
	for _, r := range list.Runs {
		require.Equal(t, want[r.ID], r.SubjectTitle, "list %s", r.ID)
	}
	tasks, err := ctl.Tasks(alice, "", 1, 50)
	require.NoError(t, err)
	require.Len(t, tasks.Tasks, 1)
	require.Equal(t, "Ada", tasks.Tasks[0].SubjectTitle)
	run, err := ctl.Run(alice, r1)
	require.NoError(t, err)
	require.Equal(t, "Ada", run.SubjectTitle)
	runs, err := ctl.SubjectRuns(alice, "PER-1")
	require.NoError(t, err)
	require.Len(t, runs, 1)
	require.Equal(t, "Ada", runs[0].SubjectTitle)
}

// TestFlowControl_SubjectTitleHidden pins that a subject is named by its
// address when `visible:` hides a property its title is made of, also one
// placeholder of a templated title, in the run list, the tasks and the run.
func TestFlowControl_SubjectTitleHidden(t *testing.T) {
	// The flow's guards and templates read every field but salary, so
	// only salary may be hidden. hr (alice) holds the waiting role and
	// does not see salary; manager (bob) sees every field.
	var shown strings.Builder
	for _, f := range []string{"name", "status", "documents", "stage", "started", "task_count"} {
		shown.WriteString("        - field: " + f + "\n")
	}
	hide := func(p string) string {
		return strings.Replace(p, "  hr:\n    read: [person, task]\n",
			"  hr:\n    read: [person, task]\n    visible:\n      person:\n"+shown.String(), 1)
	}
	tests := []struct {
		name, display, full string
	}{
		{name: "property", display: "salary", full: "5000"},
		{name: "template", display: `"{name} {salary}"`, full: "Ada 5000"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, ctl := stepProjectEdit(t, tc.display, hide)
			addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada", "salary": 5000})
			alice := flowUser(t, svc, "alice")
			id, err := ctl.Start(alice, "onboarding", "PER-1")
			require.NoError(t, err)

			for user, want := range map[string]string{"alice": "PER-1", "bob": tc.full} {
				ctx := flowUser(t, svc, user)
				list, lerr := ctl.RunList(ctx, "", "", 1, 50)
				require.NoError(t, lerr)
				require.Len(t, list.Runs, 1, user)
				require.Equal(t, want, list.Runs[0].SubjectTitle, "list %s", user)
				run, rerr := ctl.Run(ctx, id)
				require.NoError(t, rerr)
				require.Equal(t, want, run.SubjectTitle, "run %s", user)
			}
			tasks, err := ctl.Tasks(alice, "", 1, 50)
			require.NoError(t, err)
			require.Len(t, tasks.Tasks, 1)
			require.Equal(t, "PER-1", tasks.Tasks[0].SubjectTitle)
		})
	}
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

// TestFlowControl_CompleteStepFieldGate pins that a step form does not widen
// a role's field grants: a field of the form the caller may not write is
// refused on Complete and the run does not move.
func TestFlowControl_CompleteStepFieldGate(t *testing.T) {
	root := writeFlowProject(t)
	schema, err := os.ReadFile(filepath.Join("..", "flowcfg", "testdata", "schema.yaml"))
	require.NoError(t, err)
	writeMetamodelBody(t, root, string(schema))
	policy, err := os.ReadFile(filepath.Join(root, "acl.yaml"))
	require.NoError(t, err)
	hr := "  hr:\n    read: [person, task]\n    update: [person]\n"
	require.Contains(t, string(policy), hr)
	p := strings.Replace(string(policy), hr,
		hr+"    fields:\n      person:\n        - field: name\n          when: \"true\"\n", 1)
	p = strings.Replace(p, "assignments:\n", "assignments:\n  alice: hr\n", 1)
	writePolicy(t, root, p)
	svc, err := appbuildOnDisk(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	ctl := appbuild.FlowControlFor(svc)
	require.NotNil(t, ctl)

	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada", "status": "hired"})
	alice := flowUser(t, svc, "alice")
	id, err := ctl.Start(alice, "onboarding", "PER-1")
	require.NoError(t, err)
	run, err := ctl.Run(alice, id)
	require.NoError(t, err)
	require.Equal(t, []string{"documents"}, run.Waiting.Fields)

	_, err = ctl.Complete(alice, id, run.Generation, map[string]any{"documents": true})
	require.ErrorIs(t, err, flowview.ErrForbidden)
	subj, err := svc.Store().GetEntity(context.Background(), entity.Ref{ID: "PER-1"})
	require.NoError(t, err)
	require.Nil(t, subj.Properties["documents"], "a refused field writes nothing")
	after, err := ctl.Run(alice, id)
	require.NoError(t, err)
	require.Equal(t, "collect", after.State)
}

// TestFlowControl_FinishedRunNamesSubject pins BUG-TVU9HRT: a finished run
// keeps only its header, and its detail still names the subject for a
// reader of the subject, from the `_carries` edge as the run list does.
// The run page links the subject with it.
func TestFlowControl_FinishedRunNamesSubject(t *testing.T) {
	svc, ctl := stepProject(t)
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada"})
	alice, bob := flowUser(t, svc, "alice"), flowUser(t, svc, "bob")
	id, err := ctl.Start(alice, "onboarding", "PER-1")
	require.NoError(t, err)
	require.NoError(t, ctl.Cancel(alice, id))

	for _, user := range []context.Context{alice, bob} {
		run, rerr := ctl.Run(user, id)
		require.NoError(t, rerr)
		require.False(t, run.Active)
		require.Equal(t, "PER-1", run.Subject)
		runs, rerr := ctl.SubjectRuns(user, "PER-1")
		require.NoError(t, rerr)
		require.Len(t, runs, 1)
		require.Equal(t, "PER-1", runs[0].Subject)
	}
	_, err = ctl.Run(flowUser(t, svc, "eve"), id)
	require.ErrorIs(t, err, flowview.ErrNotFound)
}

// TestFlowControl_FinishedRunKeepsTimeline pins BUG-A4BFVNW: a run keeps its log
// in the header body when it ends, so a manager still sees its timeline, its
// diagram at the final state and the manage view. Cancel and retry are not
// offered on a run that has ended. A cancel adds a step that names who
// cancelled. The run list reads headers without a body, so it carries no
// timeline.
func TestFlowControl_FinishedRunKeepsTimeline(t *testing.T) {
	type ender func(t *testing.T, ctl *appbuild.FlowControl, alice context.Context, id flow.RunID)
	tests := []struct {
		name   string
		end    ender
		status string
		states []string
		marked string
	}{
		{name: "completed", status: "completed", states: []string{"collect", "review", "finished"},
			marked: "finished",
			end: func(t *testing.T, ctl *appbuild.FlowControl, alice context.Context, id flow.RunID) {
				t.Helper()
				run, err := ctl.Run(alice, id)
				require.NoError(t, err)
				got, err := ctl.Complete(alice, id, run.Generation, map[string]any{"documents": true})
				require.NoError(t, err)
				require.True(t, got.Finished)
			}},
		{name: "cancelled", status: "cancelled", states: []string{"collect", "collect"},
			marked: "collect",
			end: func(t *testing.T, ctl *appbuild.FlowControl, alice context.Context, id flow.RunID) {
				t.Helper()
				require.NoError(t, ctl.Cancel(alice, id))
			}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, ctl := stepProject(t)
			addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada"})
			alice, bob := flowUser(t, svc, "alice"), flowUser(t, svc, "bob")
			id, err := ctl.Start(alice, "onboarding", "PER-1")
			require.NoError(t, err)
			tc.end(t, ctl, alice, id)

			run, err := ctl.Run(alice, id)
			require.NoError(t, err)
			require.False(t, run.Active)
			require.Equal(t, tc.status, run.Status)
			states := make([]string, len(run.Log))
			for i, e := range run.Log {
				states[i] = e.State
			}
			require.Equal(t, tc.states, states)
			require.Equal(t, "alice", run.Log[0].By)
			require.Equal(t, "Collect documents", run.Log[0].Label)
			if tc.status == "cancelled" {
				last := run.Log[len(run.Log)-1]
				require.Equal(t, "cancel", last.Via)
				require.Equal(t, "alice", last.By)
			}
			require.Equal(t, []string{tc.marked}, markedStates(run.Diagram))
			require.Equal(t, flowview.Actions{Manage: true}, run.Actions)

			run, err = ctl.Run(bob, id)
			require.NoError(t, err)
			require.Equal(t, flowview.Actions{}, run.Actions, "a reader who does not manage the flow")
			require.Len(t, run.Log, len(tc.states), "a reader sees the timeline, as on an active run")

			runs, err := ctl.SubjectRuns(alice, "PER-1")
			require.NoError(t, err)
			require.Len(t, runs, 1)
			require.Len(t, runs[0].Log, len(tc.states), "the subject's runs carry the timeline")
			require.True(t, runs[0].Actions.Manage)

			list, err := ctl.RunList(alice, "", "", 1, 10)
			require.NoError(t, err)
			require.Len(t, list.Runs, 1)
			require.Empty(t, list.Runs[0].Log, "the run list carries no timeline")
			require.False(t, list.Runs[0].Actions.Manage)
		})
	}
}

// TestFlowControl_FinishedRunWithoutLog pins that a header with no log, from
// a run that ended before the header kept one, or with a log that does not
// parse, shows the run without a timeline instead of failing the page.
func TestFlowControl_FinishedRunWithoutLog(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{name: "no body", body: ""},
		{name: "unreadable body", body: "not a log"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, ctl := stepProject(t)
			addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada"})
			alice := flowUser(t, svc, "alice")
			id, err := ctl.Start(alice, "onboarding", "PER-1")
			require.NoError(t, err)
			require.NoError(t, ctl.Cancel(alice, id))

			ctx := context.Background()
			h, err := svc.Store().GetEntity(ctx, entity.Ref{ID: string(id)})
			require.NoError(t, err)
			h.Content = tc.body
			require.NoError(t, svc.Store().UpdateEntity(ctx, h))

			run, err := ctl.Run(alice, id)
			require.NoError(t, err)
			require.Empty(t, run.Log)
			require.Equal(t, []string{"collect"}, markedStates(run.Diagram))
			require.True(t, run.Actions.Manage)
		})
	}
}

var diagramState = regexp.MustCompile(`^state "(.*)" as (\S+)$`)

// markedStates returns the names of the states diagram marks as current.
func markedStates(diagram string) []string {
	names := map[string]string{}
	var marked []string
	for line := range strings.SplitSeq(diagram, "\n") {
		line = strings.TrimSpace(line)
		if m := diagramState.FindStringSubmatch(line); m != nil {
			names[m[2]] = m[1]
		}
		if rest, ok := strings.CutPrefix(line, "class "); ok {
			for id := range strings.SplitSeq(strings.TrimSuffix(rest, " current"), ",") {
				marked = append(marked, names[id])
			}
		}
	}
	return marked
}

// TestFlowControl_CompleteStepFieldFeedback pins BUG-53GP5F5: a step that
// writes a value breaking its type's pattern is written and answers the
// pattern's message as a warning at that field, and a step a write rule
// refuses fails with the rule's fields and description, so the form can
// show it at a field.
func TestFlowControl_CompleteStepFieldFeedback(t *testing.T) {
	editSchema := func(s string) string {
		s = strings.Replace(s, "entities:\n", "types:\n  initials:\n    validations:\n"+
			"      - pattern: '^[A-Z]+$'\n        error: Write capitals only.\n"+"entities:\n", 1)
		s = strings.Replace(s, "      name:\n        type: string\n", "      name:\n        type: initials\n", 1)
		return strings.Replace(s, "relations:\n", "validations:\n  - name: documents-need-stage\n"+
			"    description: Check the documents first.\n    entity_type: person\n"+
			"    enforce: write\n    when_condition: \"entity.documents == true\"\n    then: [\"stage=checked\"]\n"+"relations:\n", 1)
	}
	keep := func(s string) string { return s }
	svc, ctl, root := stepProjectFlowsAt(t, "name", editSchema, keep, keep)
	// The step's form lets it write name and stage too.
	forms, err := os.ReadFile(filepath.Join(root, "data-entry.yaml"))
	require.NoError(t, err)
	widened := strings.Replace(string(forms), "        help: Tick once every document is in.\n",
		"        help: Tick once every document is in.\n      - property: name\n      - property: stage\n", 1)
	require.NotEqual(t, string(forms), widened)
	require.NoError(t, os.WriteFile(filepath.Join(root, "data-entry.yaml"), []byte(widened), 0o600))
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "ADA", "status": "hired"})
	alice := flowUser(t, svc, "alice")
	id, err := ctl.Start(alice, "onboarding", "PER-1")
	require.NoError(t, err)
	run, err := ctl.Run(alice, id)
	require.NoError(t, err)
	require.Equal(t, "Onboarding: ADA", run.Title, "the view titles the run by the flow label and the subject title")
	h, err := svc.Store().GetEntity(context.Background(), entity.Ref{ID: string(id)})
	require.NoError(t, err)
	require.Equal(t, "Onboarding", h.Properties[metamodel.FlowRunPropTitle], "the stored title copies nothing of the subject")
	require.Empty(t, run.FailedMessage, "only a failed job step carries the flow's text")
	require.Equal(t, []string{"documents"}, run.Waiting.Branches[0].Fields,
		"the guard reads documents, which the step writes")

	got, err := ctl.Complete(alice, id, run.Generation, map[string]any{"name": "ada"})
	require.NoError(t, err)
	require.Equal(t, flowview.ProblemBlocked, got.Problem)
	require.Equal(t, []flowview.FieldNote{{Field: "name", Message: "Write capitals only."}}, got.Warnings)

	_, err = ctl.Complete(alice, id, run.Generation, map[string]any{"documents": true})
	var invalid *entitymanager.ValidationError
	require.ErrorAs(t, err, &invalid)
	require.Len(t, invalid.Errors, 1)
	require.Equal(t, []string{"stage"}, invalid.Errors[0].Fields)
	require.Equal(t, "Check the documents first.", invalid.Errors[0].Description)
}
