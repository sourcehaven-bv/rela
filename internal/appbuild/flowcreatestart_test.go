package appbuild_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// createStartServices builds the onboarding sample, whose flow starts on
// the create of a hired person, with a user "clerk" who may create persons
// and codes.
func createStartServices(t *testing.T) *appbuild.Services {
	t.Helper()
	root := writeFlowProject(t)
	policy, err := os.ReadFile(filepath.Join(root, "acl.yaml"))
	require.NoError(t, err)
	extra := strings.Replace(string(policy), "roles:\n",
		"roles:\n  clerk:\n    read: [person, task, code]\n    create: [person, code]\n", 1)
	extra = strings.Replace(extra, "assignments:\n", "assignments:\n  clerk: clerk\n", 1)
	require.NoError(t, os.WriteFile(filepath.Join(root, "acl.yaml"), []byte(extra), 0o600))
	svc, err := appbuildOnDisk(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

// flowHeadersFor returns the run headers that carry subject.
func flowHeadersFor(t *testing.T, svc *appbuild.Services, subject string) []*entity.Entity {
	t.Helper()
	ctx := context.Background()
	var out []*entity.Entity
	for _, r := range collectRelations(t, svc.Store(), store.RelationQuery{Type: entity.CarriesRelation, To: subject}) {
		hdr, err := svc.Store().GetEntity(ctx, entity.Ref{ID: r.From})
		require.NoError(t, err)
		out = append(out, hdr)
	}
	return out
}

// TestFlowCreateStart pins FEAT-J0O10QI: a manager create of a subject
// that satisfies an `on: create` start starts one settled run, on every
// backend; one that does not starts none.
func TestFlowCreateStart(t *testing.T) {
	tests := []struct {
		name   string
		status string
		runs   int
	}{
		{name: "when holds", status: "hired", runs: 1},
		{name: "when fails", status: "new", runs: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := createStartServices(t)
			ctx := flowUser(t, svc, "clerk")
			p := entity.New("", "person")
			p.Properties = map[string]any{"name": "Ada", "status": tc.status}
			res, err := svc.EntityManager().CreateEntity(ctx, p, entity.CreateOptions{})
			require.NoError(t, err)
			id := res.Entity.ID

			hdrs := flowHeadersFor(t, svc, id)
			require.Len(t, hdrs, tc.runs)
			if tc.runs == 0 {
				return
			}
			hdr := hdrs[0]
			require.Equal(t, "onboarding", hdr.Properties[metamodel.FlowRunPropFlow])
			require.Equal(t, "collect", hdr.Properties[metamodel.FlowRunPropState])

			got, err := svc.Store().GetEntity(context.Background(), entity.Ref{ID: id})
			require.NoError(t, err)
			require.Equal(t, "collecting", got.Properties["stage"], "the run settled once the create committed")
			produced := collectRelations(t, svc.Store(), store.RelationQuery{From: hdr.ID, Type: entity.ProducedRelation})
			require.Len(t, produced, 1, "the run's create output ran as run_as, which may create tasks")

			// A due round finds nothing left to do and starts no second run.
			_, err = appbuild.FlowEngine(svc).Due(context.Background())
			require.NoError(t, err)
			require.Len(t, flowHeadersFor(t, svc, id), 1)
			produced = collectRelations(t, svc.Store(), store.RelationQuery{From: hdr.ID, Type: entity.ProducedRelation})
			require.Len(t, produced, 1, "the due round does not settle the run again")
		})
	}
}

// TestFlowCreateStart_Lua: a create through rela.create_entity, as an
// action or a webhook action makes it, starts the run too.
func TestFlowCreateStart_Lua(t *testing.T) {
	svc := createStartServices(t)
	ctx := flowUser(t, svc, "clerk")
	var out strings.Builder
	rt := lua.NewWriter(svc.Scheduler().ScheduledLuaWriteDeps(), &out, lua.WithContext(ctx),
		lua.WithPrincipal(principal.From(ctx)))
	defer rt.Close()
	require.NoError(t, rt.RunString(
		`rela.create_entity("person", {name = "Ada", status = "hired"})`))
	var id string
	for e, lErr := range svc.Store().ListEntities(context.Background(),
		store.EntityQuery{Type: "person", Faces: store.AllFaces()}) {
		require.NoError(t, lErr)
		id = e.ID
	}
	require.NotEmpty(t, id)

	hdrs := flowHeadersFor(t, svc, id)
	require.Len(t, hdrs, 1)
	got, err := svc.Store().GetEntity(context.Background(), entity.Ref{ID: id})
	require.NoError(t, err)
	require.Equal(t, "collecting", got.Properties["stage"])
}

// TestFlowCreateStart_RollbackStartsNoRun: when the create rolls back, so
// does its run, on a store whose transactions roll back.
func TestFlowCreateStart_RollbackStartsNoRun(t *testing.T) {
	svc := createStartServices(t)
	if !store.TxRollsBack(svc.Store()) {
		t.Skip("this store does not roll back a failed transaction")
	}
	ctx := flowUser(t, svc, "clerk")
	taken := entity.New("C-1", "code")
	taken.Properties = map[string]any{"title": "taken"}
	require.NoError(t, svc.Store().CreateEntity(context.Background(), taken))

	p := entity.New("", "person")
	p.Properties = map[string]any{"name": "Ada", "status": "hired"}
	dup := entity.New("", "code")
	dup.Properties = map[string]any{"title": "again"}
	_, err := entitymanager.CreateBatch(ctx, svc.EntityManager(), entitymanager.Batch{Entities: []entitymanager.BatchEntity{
		{Entity: p},
		{Entity: dup, Options: entity.CreateOptions{ID: "C-1"}},
	}})
	require.Error(t, err)

	for e, lErr := range svc.Store().ListEntities(context.Background(),
		store.EntityQuery{Type: entity.FlowRunType, Faces: store.AllFaces()}) {
		require.NoError(t, lErr)
		t.Fatalf("a rolled-back create left run %s", e.ID)
	}
	for e, lErr := range svc.Store().ListEntities(context.Background(),
		store.EntityQuery{Type: "person", Faces: store.AllFaces()}) {
		require.NoError(t, lErr)
		t.Fatalf("the batch rolled back, but person %s stayed", e.ID)
	}
}

// createChainFlows is a flow whose output creates an entity of its own subject
// type, so every run it starts on create would start the next.
const createChainFlows = `flows:
  echo:
    subject: task
    run_as: system:onboarding-bot
    start:
      - on: create
    initial: copy
    states:
      copy:
        outputs:
          - create:
              type: task
              properties: { title: "copy" }
        on:
          done: { to: finished }
      finished:
        final: true
`

// headerDepth is the stored depth of a run header; absent is 0.
func headerDepth(hdr *entity.Entity) int {
	v, ok := hdr.Properties[metamodel.FlowRunPropDepth]
	if !ok {
		return 0
	}
	var d int
	_, _ = fmt.Sscan(fmt.Sprint(v), &d)
	return d
}

// TestFlowCreateStart_ChainIsBounded: a chain of create starts stops at
// the depth bound instead of running without end. The depth is stored on
// each header, so the bound holds whoever settles the run that makes the
// next create.
func TestFlowCreateStart_ChainIsBounded(t *testing.T) {
	root := writeFlowProject(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, "flows.yaml"), []byte(createChainFlows), 0o600))
	policy, err := os.ReadFile(filepath.Join(root, "acl.yaml"))
	require.NoError(t, err)
	extra := strings.Replace(string(policy), "roles:\n",
		"roles:\n  clerk:\n    read: [task]\n    create: [task]\n", 1)
	extra = strings.Replace(extra, "assignments:\n", "assignments:\n  clerk: clerk\n", 1)
	require.NoError(t, os.WriteFile(filepath.Join(root, "acl.yaml"), []byte(extra), 0o600))
	svc, err := appbuildOnDisk(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	task := entity.New("", "task")
	task.SetString("title", "first")
	_, err = svc.EntityManager().CreateEntity(flowUser(t, svc, "clerk"), task, entity.CreateOptions{})
	require.NoError(t, err)

	var depths []int
	tasks := 0
	for e, lErr := range svc.Store().ListEntities(context.Background(),
		store.EntityQuery{Type: entity.FlowRunType, Faces: store.AllFaces()}) {
		require.NoError(t, lErr)
		depths = append(depths, headerDepth(e))
	}
	for _, lErr := range svc.Store().ListEntities(context.Background(),
		store.EntityQuery{Type: "task", Faces: store.AllFaces()}) {
		require.NoError(t, lErr)
		tasks++
	}
	slices.Sort(depths)
	require.Equal(t, []int{0, 1, 2, 3, 4, 5, 6, 7}, depths, "a chain starts runs up to the depth bound")
	require.Equal(t, 9, tasks, "the last run's copy starts nothing")

	// A create made under the deepest run, as a flow worker settling it
	// later would make it, starts nothing either.
	var deepest string
	for e, lErr := range svc.Store().ListEntities(context.Background(),
		store.EntityQuery{Type: entity.FlowRunType, Faces: store.AllFaces()}) {
		require.NoError(t, lErr)
		if headerDepth(e) == 7 {
			deepest = e.ID
		}
	}
	require.NotEmpty(t, deepest)
	late := entity.New("", "task")
	late.SetString("title", "late")
	ctx := audit.WithFlowStep(flowUser(t, svc, "clerk"), audit.FlowStep{Flow: "echo", Run: deepest})
	res, err := svc.EntityManager().CreateEntity(ctx, late, entity.CreateOptions{})
	require.NoError(t, err)
	require.Empty(t, flowHeadersFor(t, svc, res.Entity.ID), "a create under the deepest run starts no run")
}

// TestFlowCreateStart_ACL pins the access checks around a create start,
// with the declarative ACL: a caller who may not create the subject starts
// nothing, and a run_as that may not write an output is refused when
// flows.yaml loads, so a create never starts a run that writes as it.
func TestFlowCreateStart_ACL(t *testing.T) {
	t.Run("caller may not create the subject", func(t *testing.T) {
		svc := createStartServices(t)
		p := entity.New("", "person")
		p.Properties = map[string]any{"name": "Ada", "status": "hired"}
		_, err := svc.EntityManager().CreateEntity(flowUser(t, svc, "nobody"), p, entity.CreateOptions{})
		require.Error(t, err)
		for e, lErr := range svc.Store().ListEntities(context.Background(),
			store.EntityQuery{Type: entity.FlowRunType, Faces: store.AllFaces()}) {
			require.NoError(t, lErr)
			t.Fatalf("a refused create started run %s", e.ID)
		}
	})

	t.Run("run_as may not write an output", func(t *testing.T) {
		root := writeFlowProject(t)
		policy, err := os.ReadFile(filepath.Join(root, "acl.yaml"))
		require.NoError(t, err)
		extra := strings.Replace(string(policy), "    create: [task]\n", "", 1)
		extra = strings.Replace(extra, "roles:\n",
			"roles:\n  clerk:\n    read: [person]\n    create: [person]\n", 1)
		extra = strings.Replace(extra, "assignments:\n", "assignments:\n  clerk: clerk\n", 1)
		require.NoError(t, os.WriteFile(filepath.Join(root, "acl.yaml"), []byte(extra), 0o600))
		_, err = appbuildOnDiskNoBackend(t, root)
		require.ErrorContains(t, err, "system:onboarding-bot cannot create task",
			"a run_as that may not write an output refuses the boot, so no create can start its run")
	})
}
