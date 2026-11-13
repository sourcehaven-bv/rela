package appbuild_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowengine"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// writeFlowProject writes the onboarding sample (flowcfg's testdata) as a
// project: schema, acl.yaml and flows.yaml.
func writeFlowProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join("..", "flowcfg", "testdata", name))
		require.NoError(t, err)
		return string(data)
	}
	writeMetamodelBody(t, root, read("schema.yaml"))
	writePolicy(t, root, read("acl.yaml"))
	require.NoError(t, os.WriteFile(filepath.Join(root, "flows.yaml"), []byte(read("flows.yaml")), 0o600))
	return root
}

// TestFlowEngine_SampleRunsOnEveryBackend starts the onboarding sample and
// checks what it wrote to the graph, on whichever backend this build runs:
// the header as system:flow, the subject edge, and the initial outputs as
// run_as (TKT-9IT3LN8).
func TestFlowEngine_SampleRunsOnEveryBackend(t *testing.T) {
	svc, err := appbuildOnDisk(t, writeFlowProject(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	ctx := context.Background()

	require.True(t, svc.Meta().HasFlowRuns(), "flows.yaml declares flows, so the schema carries _flow-run")
	eng := appbuild.FlowEngine(svc)
	require.NotNil(t, eng)

	person := entity.New("PER-1", "person")
	person.Properties = map[string]any{"name": "Ada", "status": "hired"}
	require.NoError(t, svc.Store().CreateEntity(ctx, person))

	id, err := eng.Start(ctx, "onboarding", person.Ref())
	require.NoError(t, err)

	hdr, err := svc.Store().GetEntity(ctx, entity.Ref{ID: string(id)})
	require.NoError(t, err)
	require.Equal(t, entity.FlowRunType, hdr.Type)
	require.Equal(t, "onboarding", hdr.Properties[metamodel.FlowRunPropFlow])
	require.Equal(t, "collect", hdr.Properties[metamodel.FlowRunPropState])
	require.Equal(t, "input_required", hdr.Properties[metamodel.FlowRunPropStatus])

	got, err := svc.Store().GetEntity(ctx, person.Ref())
	require.NoError(t, err)
	require.Equal(t, "collecting", got.Properties["stage"], "the set output wrote the subject as run_as")

	rels := collectRelations(t, svc.Store(), store.RelationQuery{From: string(id)})
	var carries, produced string
	for _, r := range rels {
		switch r.Type {
		case entity.CarriesRelation:
			carries = r.To
		case entity.ProducedRelation:
			produced = r.To
		}
	}
	require.Equal(t, "PER-1", carries)
	require.NotEmpty(t, produced, "the create output is linked to its run")
	task, err := svc.Store().GetEntity(ctx, entity.Ref{ID: produced})
	require.NoError(t, err)
	require.Equal(t, "task", task.Type)
	require.Equal(t, "Collect documents for Ada", task.Properties["title"])
	about := collectRelations(t, svc.Store(), store.RelationQuery{From: produced, Type: "about"})
	require.Len(t, about, 1)
	require.Equal(t, "PER-1", about[0].To)

	require.NoError(t, eng.Cancel(ctx, id))
	hdr, err = svc.Store().GetEntity(ctx, entity.Ref{ID: string(id)})
	require.NoError(t, err)
	require.Equal(t, "cancelled", hdr.Properties[metamodel.FlowRunPropStatus])
	require.NotNil(t, hdr.Properties[metamodel.FlowRunPropCompleted])
}

// TestFlowEngine_NoFlowsNoEngine: without flows.yaml nothing changes.
func TestFlowEngine_NoFlowsNoEngine(t *testing.T) {
	root := t.TempDir()
	writeMetamodel(t, root)
	svc, err := appbuildOnDisk(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	require.Nil(t, appbuild.FlowEngine(svc))
	require.False(t, svc.Meta().HasFlowRuns())
}

// TestFlowEngine_InvalidFlowsFailBoot: a flows.yaml that does not compile
// fails the boot, as realtime.yaml does.
func TestFlowEngine_InvalidFlowsFailBoot(t *testing.T) {
	root := writeFlowProject(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, "flows.yaml"),
		[]byte("flows:\n  bad:\n    subject: nothing\n"), 0o600))
	_, err := appbuildOnDiskNoBackend(t, root)
	require.ErrorContains(t, err, "flows.yaml")
}

// TestFlowEngine_InjectedACLChecksFlowsAgainstACLYAML: a host that injects
// an ACL without a policy (rela mcp and the desktop inject NopACL) still
// compiles flows.yaml against acl.yaml, so the roles it names resolve. The
// engine compiles a run's older version against the same policy
// (BUG-E9A72XB).
func TestFlowEngine_InjectedACLChecksFlowsAgainstACLYAML(t *testing.T) {
	ctx := context.Background()
	root := writeFlowProject(t)
	dsn := backendtest.DSN(t) // both boots open the same store
	svc, err := newOnDisk(t, dsn, root, appbuild.WithACL(acl.NopACL{}))
	require.NoError(t, err, "flows.yaml names roles acl.yaml declares")
	eng := appbuild.FlowEngine(svc)
	require.NotNil(t, eng)
	person := entity.New("PER-1", "person")
	person.Properties = map[string]any{"name": "Ada", "status": "hired"}
	require.NoError(t, svc.Store().CreateEntity(ctx, person))
	id, err := eng.Start(ctx, "onboarding", person.Ref())
	require.NoError(t, err)
	require.NoError(t, svc.Close())

	// A new flows.yaml version leaves the run on the old one.
	path := filepath.Join(root, "flows.yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	changed := strings.Replace(string(data), "label: Collect documents", "label: Collect the documents", 1)
	require.NotEqual(t, string(data), changed)
	require.NoError(t, os.WriteFile(path, []byte(changed), 0o600))

	svc, err = newOnDisk(t, dsn, root, appbuild.WithACL(acl.NopACL{}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	eng = appbuild.FlowEngine(svc)
	run, err := eng.Run(ctx, id)
	require.NoError(t, err)
	cur, ok := eng.Flow("onboarding")
	require.True(t, ok)
	require.NotEqual(t, cur.Version(), run.Version, "the run follows the older version")
	_, err = eng.FlowOf(ctx, run)
	require.NoError(t, err, "the older version compiles against acl.yaml")
}

// TestFlowEngine_InjectedACLConfigPolicyEdges pins the edges of the flows
// check under an injected ACL without a policy (BUG-E9A72XB): a missing
// acl.yaml is named, and an acl.yaml that does not fit the schema does not
// stop a host that does not enforce it, as for realtime.yaml, but is logged
// as a warning (GH-2226).
func TestFlowEngine_InjectedACLConfigPolicyEdges(t *testing.T) {
	t.Run("no acl.yaml names it", func(t *testing.T) {
		root := writeFlowProject(t)
		require.NoError(t, os.Remove(filepath.Join(root, "acl.yaml")))
		_, err := newOnDisk(t, "", root, appbuild.WithACL(acl.NopACL{}))
		require.ErrorContains(t, err, `names role "hr", but the project has no acl.yaml`)
	})
	t.Run("acl.yaml that does not fit the schema still boots", func(t *testing.T) {
		root := writeFlowProject(t)
		path := filepath.Join(root, "acl.yaml")
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		data = append(data, "user_entity_type: ghost\nprincipal_property: email\n"...)
		require.NoError(t, os.WriteFile(path, data, 0o600))
		logs := captureWarnings(t)
		svc, err := newOnDisk(t, backendtest.DSN(t), root, appbuild.WithACL(acl.NopACL{}))
		require.NoError(t, err)
		t.Cleanup(func() { _ = svc.Close() })
		require.NotNil(t, appbuild.FlowEngine(svc))
		require.Contains(t, logs(), "acl.yaml does not fit the schema")
	})
}

func collectRelations(t *testing.T, st store.Store, q store.RelationQuery) []*entity.Relation {
	t.Helper()
	var out []*entity.Relation
	for r, err := range st.ListRelations(context.Background(), q) {
		require.NoError(t, err)
		out = append(out, r)
	}
	return out
}

// TestFlowControl_GatesStartCancelAndRuns pins the access checks of
// `rela flow start|runs|cancel`: a subject the caller cannot read is not
// found, a start or cancel needs the manual permission, and the run list
// follows the header's read gate (RR-R5I7UE0).
func TestFlowControl_GatesStartCancelAndRuns(t *testing.T) {
	root := writeFlowProject(t)
	policy, err := os.ReadFile(filepath.Join(root, "acl.yaml"))
	require.NoError(t, err)
	extra := strings.Replace(string(policy), "assignments:\n",
		"assignments:\n  alice: hr\n  bob: manager\n", 1)
	require.NoError(t, os.WriteFile(filepath.Join(root, "acl.yaml"), []byte(extra), 0o600))

	svc, err := appbuildOnDisk(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	ctl := appbuild.FlowControlFor(svc)
	require.NotNil(t, ctl)

	person := entity.New("PER-1", "person")
	person.Properties = map[string]any{"name": "Ada", "status": "hired"}
	require.NoError(t, svc.Store().CreateEntity(context.Background(), person))

	as := func(user string) context.Context {
		ctx, berr := appbuild.MustIdentityBinder(svc)(principal.With(context.Background(),
			principal.Principal{User: user, Tool: principal.ToolCLI}))
		require.NoError(t, berr)
		return ctx
	}
	alice, bob, eve := as("alice"), as("bob"), as("eve")

	_, err = ctl.Start(eve, "onboarding", "PER-1")
	require.ErrorIs(t, err, flowengine.ErrSubjectNotFound, "eve cannot read the subject")
	_, err = ctl.Start(bob, "onboarding", "PER-1")
	require.ErrorIs(t, err, appbuild.ErrFlowPermission, "bob reads the subject but lacks onboarding.start")
	_, err = ctl.Start(alice, "nope", "PER-1")
	require.ErrorIs(t, err, flowengine.ErrUnknownFlow)

	id, err := ctl.Start(alice, "onboarding", "PER-1")
	require.NoError(t, err)

	runs, err := ctl.Runs(bob, "onboarding")
	require.NoError(t, err)
	require.Len(t, runs, 1, "bob reads the subject, so he reads its run")
	require.Equal(t, string(id), runs[0].ID)
	require.Equal(t, "collect", runs[0].State)
	runs, err = ctl.Runs(eve, "")
	require.NoError(t, err)
	require.Empty(t, runs)
	runs, err = ctl.Runs(alice, "other")
	require.NoError(t, err)
	require.Empty(t, runs)

	require.ErrorIs(t, ctl.Cancel(eve, id), flowengine.ErrRunNotFound)
	require.ErrorIs(t, ctl.Cancel(bob, id), appbuild.ErrFlowPermission)
	require.NoError(t, ctl.Cancel(alice, id))
	require.ErrorIs(t, ctl.Cancel(alice, id), flowengine.ErrRunNotFound, "a finished run cannot be cancelled again")

	runs, err = ctl.Runs(alice, "")
	require.NoError(t, err)
	require.Len(t, runs, 1)
	require.Equal(t, "cancelled", runs[0].Status)
	require.NotEmpty(t, runs[0].Completed)

	// A header whose run was never stored: a crash between the writes.
	orphan := func(id string, started time.Time) flow.RunID {
		hdr := entity.New(id, entity.FlowRunType)
		hdr.Properties = map[string]any{
			metamodel.FlowRunPropFlow: "onboarding", metamodel.FlowRunPropVersion: "v",
			metamodel.FlowRunPropState: "collect", metamodel.FlowRunPropStatus: "input_required",
			metamodel.FlowRunPropStarted: started,
		}
		require.NoError(t, svc.Store().CreateEntity(context.Background(), hdr))
		_, rerr := svc.Store().CreateRelation(context.Background(), entity.RelationKey{
			From: id, FromFace: entity.ImplicitFace, Type: entity.CarriesRelation, To: "PER-1",
		}, &store.RelationData{})
		require.NoError(t, rerr)
		return flow.RunID(id)
	}
	fresh := orphan("RUN-FRESH01", time.Now())
	require.ErrorContains(t, ctl.Cancel(alice, fresh), "still starting")
	old := orphan("RUN-OLD0001", time.Now().Add(-time.Hour))
	require.ErrorIs(t, ctl.Cancel(bob, old), appbuild.ErrFlowPermission)
	require.NoError(t, ctl.Cancel(alice, old))
	hdr, err := svc.Store().GetEntity(context.Background(), entity.Ref{ID: string(old), Face: entity.ImplicitFace})
	require.NoError(t, err)
	require.Equal(t, "cancelled", hdr.Properties[metamodel.FlowRunPropStatus])
	require.ErrorIs(t, ctl.Cancel(alice, old), flowengine.ErrRunNotFound)
}
