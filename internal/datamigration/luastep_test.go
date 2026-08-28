package datamigration

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// luaFixture builds a migration file with one lua step and the script FS.
func luaFixture(t *testing.T, entityTarget, script string) (*File, fstest.MapFS) {
	t.Helper()
	fsys := fstest.MapFS{
		"migrations/transform.lua": &fstest.MapFile{Data: []byte(script)},
	}
	data := mustFileYAML(t, metaV1(), metaV2(),
		"  - lua: {entity: '"+entityTarget+"', script: migrations/transform.lua}\n")
	return mustParse(t, "0001-lua.yaml", data), fsys
}

func TestLuaStep_PureTransformPatch(t *testing.T) {
	f, fsys := luaFixture(t, "task", `
function migrate(entity)
  if entity.properties.status == nil then return nil end
  return {
    properties = { state = entity.properties.status .. "-migrated" },
    unset = { "status" },
  }
end`)
	st := seedStore(t)
	r := newTestRunner(t, Deps{Store: st, ScriptFS: fsys})
	res, err := r.Run(t.Context(), []*File{f}, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Files[0].Steps[0].Affected != 3 {
		t.Fatalf("affected = %d, want 3", res.Files[0].Steps[0].Affected)
	}
	e := getEntity(t, st, "TSK-1")
	if got := e.Properties["state"]; got != "open-migrated" {
		t.Errorf("state = %v", got)
	}
	if _, has := e.Properties["status"]; has {
		t.Errorf("unset did not remove status")
	}
	// Idempotency: re-run transforms nothing (status is gone).
	res, err = r.Run(t.Context(), []*File{f}, true)
	if err != nil || res.Files[0].Steps[0].Affected != 0 {
		t.Fatalf("re-run affected %d err %v, want 0", res.Files[0].Steps[0].Affected, err)
	}
}

func TestLuaStep_RunsOnUnknownTypeEntities(t *testing.T) {
	// The transform must reach entities whose type the NEW schema doesn't
	// know — entitymanager would 422 them; the raw runner must not.
	st := seedStore(t)
	ctx := t.Context()
	if err := st.CreateEntity(ctx, &entity.Entity{
		ID: "OLD-1", Type: "legacy", Properties: map[string]any{"x": "1"},
	}); err != nil {
		t.Fatal(err)
	}
	f, fsys := luaFixture(t, "*", `
function migrate(entity)
  if entity.type ~= "legacy" then return nil end
  return { properties = { x = "2" } }
end`)
	r := newTestRunner(t, Deps{Store: st, ScriptFS: fsys})
	res, err := r.Run(ctx, []*File{f}, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Files[0].Steps[0].Affected != 1 {
		t.Fatalf("affected = %d, want 1", res.Files[0].Steps[0].Affected)
	}
	if got := getEntity(t, st, "OLD-1").Properties["x"]; got != "2" {
		t.Errorf("x = %v, want 2", got)
	}
}

func TestLuaStep_SandboxHasNoIO(t *testing.T) {
	f, fsys := luaFixture(t, "task", `
function migrate(entity)
  if os ~= nil or io ~= nil or dofile ~= nil or load ~= nil then
    error("sandbox leak")
  end
  return nil
end`)
	st := seedStore(t)
	r := newTestRunner(t, Deps{Store: st, ScriptFS: fsys})
	if _, err := r.Run(t.Context(), []*File{f}, true); err != nil {
		t.Fatalf("sandbox leaked a dangerous global: %v", err)
	}
}

func TestLuaStep_BadReturnFailsBeforeMarker(t *testing.T) {
	f, fsys := luaFixture(t, "task", `function migrate(entity) return 42 end`)
	st := seedStore(t)
	kv := newFakeKV()
	r := newTestRunner(t, Deps{Store: st, ScriptFS: fsys, State: kv})
	_, err := r.Run(t.Context(), []*File{f}, true)
	if err == nil || !strings.Contains(err.Error(), "patch table") {
		t.Fatalf("err = %v, want patch-table error", err)
	}
	marker, _ := LoadMarker(t.Context(), kv)
	if marker != nil {
		t.Fatalf("marker written despite failed run")
	}
}

func TestLuaStep_MissingMigrateFunction(t *testing.T) {
	f, fsys := luaFixture(t, "task", `local x = 1`)
	r := newTestRunner(t, Deps{Store: seedStore(t), ScriptFS: fsys})
	_, err := r.Run(t.Context(), []*File{f}, true)
	if err == nil || !strings.Contains(err.Error(), "function migrate") {
		t.Fatalf("err = %v, want missing-migrate error", err)
	}
}

func TestLuaStep_PathTraversalRejectedAtParse(t *testing.T) {
	data := mustFileYAML(t, metaV1(), metaV2(),
		"  - lua: {entity: task, script: ../outside.lua}\n")
	_, err := ParseFile("0001-lua.yaml", data)
	if err == nil || !strings.Contains(err.Error(), "project-relative") {
		t.Fatalf("err = %v, want path rejection", err)
	}
}
