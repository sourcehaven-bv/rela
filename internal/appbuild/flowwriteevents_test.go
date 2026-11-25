package appbuild_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// writeEventProject is the onboarding sample with alice (hr) and dora, a
// clerk who may update persons but holds no flow role.
func writeEventProject(t *testing.T) (*appbuild.Services, *appbuild.FlowControl) {
	t.Helper()
	return stepProjectEdit(t, "name", func(p string) string {
		p = strings.Replace(p, "roles:\n", "roles:\n  clerk:\n    read: [person, task]\n    update: [person]\n", 1)
		return strings.Replace(p, "assignments:\n", "assignments:\n  dora: clerk\n", 1)
	})
}

// subjectProp reads property name of subject from the store.
func subjectProp(t *testing.T, svc *appbuild.Services, subject, name string) any {
	t.Helper()
	got, err := svc.Store().GetEntity(context.Background(), entity.Ref{ID: subject})
	require.NoError(t, err)
	return got.Properties[name]
}

// TestFlowWriteEvents_PatchMovesTheRun pins TKT-GYXOTQC: an ordinary
// manager write fires the `change` event of the subject's run, as its
// author, on every backend. Only an author who holds a role of the branch
// moves it.
func TestFlowWriteEvents_PatchMovesTheRun(t *testing.T) {
	tests := []struct {
		name  string
		user  string
		state string
	}{
		{name: "hr moves the run", user: "alice", state: "finished"},
		{name: "a clerk does not", user: "dora", state: "collect"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, ctl := writeEventProject(t)
			addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada", "status": "hired"})
			alice := flowUser(t, svc, "alice")
			id, err := ctl.Start(alice, "onboarding", "PER-1")
			require.NoError(t, err)

			_, err = svc.EntityManager().PatchEntity(flowUser(t, svc, tc.user), "PER-1",
				entity.Patch{Properties: map[string]any{"documents": true}})
			require.NoError(t, err)
			require.Equal(t, true, subjectProp(t, svc, "PER-1", "documents"))

			hdr, err := svc.Store().GetEntity(context.Background(), entity.Ref{ID: string(id)})
			require.NoError(t, err)
			require.Equal(t, tc.state, hdr.Properties[metamodel.FlowRunPropState])
		})
	}
}

// TestFlowWriteEvents_Lua pins that a write a script makes, as a sync
// connector does, fires the event as the script's principal.
func TestFlowWriteEvents_Lua(t *testing.T) {
	svc, ctl := writeEventProject(t)
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada", "status": "hired"})
	alice := flowUser(t, svc, "alice")
	id, err := ctl.Start(alice, "onboarding", "PER-1")
	require.NoError(t, err)

	var out strings.Builder
	rt := lua.NewWriter(svc.Scheduler().ScheduledLuaWriteDeps(), &out, lua.WithContext(alice),
		lua.WithPrincipal(principal.From(alice)))
	defer rt.Close()
	require.NoError(t, rt.RunString(`rela.update_entity("PER-1", {documents = true})`))

	hdr, err := svc.Store().GetEntity(context.Background(), entity.Ref{ID: string(id)})
	require.NoError(t, err)
	require.Equal(t, "finished", hdr.Properties[metamodel.FlowRunPropState])
}

// TestFlowWriteEvents_PredicateStart pins that a write that makes the
// subject satisfy an `on: predicate` start starts one settled run, and
// that a later write while it still holds starts no second one.
func TestFlowWriteEvents_PredicateStart(t *testing.T) {
	svc, _ := writeEventProject(t)
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada", "status": "new"})
	dora := flowUser(t, svc, "dora")
	_, err := svc.EntityManager().PatchEntity(dora, "PER-1",
		entity.Patch{Properties: map[string]any{"name": "Ada B"}})
	require.NoError(t, err)
	require.Empty(t, flowHeadersFor(t, svc, "PER-1"), "the start does not hold")

	_, err = svc.EntityManager().PatchEntity(dora, "PER-1",
		entity.Patch{Properties: map[string]any{"status": "hired"}})
	require.NoError(t, err)
	hdrs := flowHeadersFor(t, svc, "PER-1")
	require.Len(t, hdrs, 1)
	require.Equal(t, "collect", hdrs[0].Properties[metamodel.FlowRunPropState])
	require.Equal(t, "collecting", subjectProp(t, svc, "PER-1", "stage"),
		"the run settled once the write committed, and its own output started nothing")

	_, err = svc.EntityManager().PatchEntity(dora, "PER-1",
		entity.Patch{Properties: map[string]any{"name": "Ada C"}})
	require.NoError(t, err)
	require.Len(t, flowHeadersFor(t, svc, "PER-1"), 1)
}
