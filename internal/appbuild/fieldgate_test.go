package appbuild_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

const fieldGateMetamodel = `version: "1.0"
entities:
  task:
    label: Task
    plural: tasks
    id_prefix: "TASK-"
    id_type: sequential
    properties:
      title: {type: string}
      status: {type: string}
`

// alice may write title, and status only on a task titled "editable". A
// `fields:` block is a closed list: a field it does not name is read-only.
const fieldGatePolicy = `roles:
  editor:
    read: [task]
    create: [task]
    update: [task]
    fields:
      task:
        - field: title
        - field: status
          when: "entity.title == 'editable'"
assignments:
  alice: editor
`

func asAlice() context.Context {
	return principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolMCP})
}

func fieldGateServices(t *testing.T) *appbuild.Services {
	t.Helper()
	root := t.TempDir()
	writeMetamodelBody(t, root, fieldGateMetamodel)
	writePolicy(t, root, fieldGatePolicy)
	svc, err := appbuild.Discover(root, script.NewEngine(), backendtest.Options(t)...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

func createTask(t *testing.T, svc *appbuild.Services, title string) string {
	t.Helper()
	e := entity.New("", "task")
	e.SetString("title", title)
	res, err := svc.EntityManager().CreateEntity(asAlice(), e, entity.CreateOptions{})
	require.NoError(t, err)
	return res.Entity.ID
}

func requireReadOnlyStatus(t *testing.T, err error) {
	t.Helper()
	var d *affordances.FieldWriteError
	require.ErrorAs(t, err, &d)
	require.Equal(t, "field-affordance:read-only:status", d.RuleID())
}

// TestFieldGate_PolicyHoldsOnGatedHandle: the field-gated handle, which
// MCP on rela-server writes through, refuses a field write the caller's
// `fields:` grant forbids, on patch and on create (TKT-0XL8MF).
func TestFieldGate_PolicyHoldsOnGatedHandle(t *testing.T) {
	svc := fieldGateServices(t)
	mgr := entitymanager.FieldGated(svc.EntityManager())

	locked := createTask(t, svc, "locked")
	_, err := mgr.PatchEntity(asAlice(), locked, entity.Patch{Properties: map[string]any{"status": "done"}})
	requireReadOnlyStatus(t, err)
	_, err = mgr.PatchEntity(asAlice(), locked, entity.Patch{MetaUnset: []string{"status"}})
	requireReadOnlyStatus(t, err)

	editable := createTask(t, svc, "editable")
	_, err = mgr.PatchEntity(asAlice(), editable, entity.Patch{Properties: map[string]any{"status": "done"}})
	require.NoError(t, err)

	e := entity.New("", "task")
	e.SetString("title", "locked")
	e.SetString("status", "done")
	_, err = mgr.CreateEntity(asAlice(), e, entity.CreateOptions{})
	requireReadOnlyStatus(t, err)

	_, err = mgr.PatchEntity(asAlice(), locked, entity.Patch{Properties: map[string]any{"nonexistent": "x"}})
	var d *affordances.FieldWriteError
	require.ErrorAs(t, err, &d)
	require.Equal(t, "field-affordance:hidden:nonexistent", d.RuleID())
}

// TestFieldGate_LuaUpdateRaises: a scheduled script writing as alice gets
// the same refusal as an error.
func TestFieldGate_LuaUpdateRaises(t *testing.T) {
	svc := fieldGateServices(t)
	locked := createTask(t, svc, "locked")
	var out strings.Builder
	rt := lua.NewWriter(svc.ScheduledLuaWriteDeps(), &out, lua.WithContext(asAlice()),
		lua.WithPrincipal(principal.From(asAlice())))
	defer rt.Close()
	err := rt.RunString(`rela.update_entity("` + locked + `", {status = "done"})`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "field-affordance:read-only:status")
}

// TestFieldGate_OperatorSurfacesUngated: the default handle and the CLI's
// Lua deps write any field the row grant allows, while the row grants still
// refuse a principal without update.
func TestFieldGate_OperatorSurfacesUngated(t *testing.T) {
	svc := fieldGateServices(t)
	locked := createTask(t, svc, "locked")
	_, err := svc.EntityManager().PatchEntity(asAlice(), locked, entity.Patch{Properties: map[string]any{"status": "done"}})
	require.NoError(t, err)

	var out strings.Builder
	rt := lua.NewWriter(svc.LuaWriteDeps(), &out, lua.WithContext(asAlice()),
		lua.WithPrincipal(principal.From(asAlice())))
	defer rt.Close()
	require.NoError(t, rt.RunString(`rela.update_entity("`+locked+`", {status = "again"})`))

	bob := principal.With(context.Background(), principal.Principal{User: "bob", Tool: principal.ToolCLI})
	_, err = svc.EntityManager().PatchEntity(bob, locked, entity.Patch{Properties: map[string]any{"status": "x"}})
	require.Error(t, err, "row grants must still refuse a principal without update")
}
