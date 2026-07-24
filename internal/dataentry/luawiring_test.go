package dataentry

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TKT-ZF2DTV: the WIRING is the security control. internal/lua's tests
// prove the reader gates correctly; these prove the App actually hands a
// gated reader to script runtimes. Without this, repointing
// App.luaWriteDeps at the raw store passes every other test in the repo
// (RR-QS4WQV).

// runScriptAsAlice executes src through the App's real luaWriteDeps bundle
// under alice's principal + read gate, and returns what the script emitted.
func runScriptAsAlice(t *testing.T, app *App, d *acl.Declarative, src string) string {
	t.Helper()
	ctx := gateCtxFor(aliceCtx(), t, d)
	var out strings.Builder
	rt := lua.NewWriter(app.luaWriteDeps(), &out,
		lua.WithContext(ctx), lua.WithPrincipal(principal.From(ctx)))
	defer rt.Close()
	if err := rt.RunString(src); err != nil {
		t.Fatalf("script error: %v (output: %q)", err, out.String())
	}
	return out.String()
}

// TestLuaWiring_ScriptReadsAreACLBound pins that App.luaWriteDeps supplies
// an ACL-BOUND reader — the wiring every data-entry script path depends on
// (actions, export_render, MCP-invoked). A hidden entity must be invisible
// to a script even though the raw store holds it.
func TestLuaWiring_ScriptReadsAreACLBound(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "Readable"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature",
		Properties: map[string]any{"title": "Classified"}})

	// alice may read tickets, NOT features.
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	out := runScriptAsAlice(t, app, d, `
rela.output("ticket=" .. tostring(rela.get_entity("TKT-001") ~= nil))
rela.output("feature=" .. tostring(rela.get_entity("FEAT-001") ~= nil))
`)

	if !strings.Contains(out, "ticket=true") {
		t.Errorf("readable entity should be visible to the script: %q", out)
	}
	if !strings.Contains(out, "feature=false") {
		t.Errorf("App.luaWriteDeps is NOT wired to an ACL-bound reader — "+
			"a script read an entity its principal cannot see: %q", out)
	}
}

// TestLuaWiring_ScriptTraversalIsACLBound pins the tracer half of the same
// wiring: the decorator must be injected, not the bare tracer.
func TestLuaWiring_ScriptTraversalIsACLBound(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "Readable"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature",
		Properties: map[string]any{"title": "Classified"}})
	seedRelation(app, &entity.Relation{From: "TKT-001", Type: "implements", To: "FEAT-001"})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	out := runScriptAsAlice(t, app, d, `
local function walk(n, acc)
  if n == nil then return acc end
  acc[#acc+1] = n.id
  for _, c in ipairs(n.children or {}) do walk(c, acc) end
  return acc
end
rela.output("nodes=" .. table.concat(walk(rela.trace_from("TKT-001", 2), {}), ","))
`)

	if !strings.Contains(out, "TKT-001") {
		t.Errorf("visible root missing from trace: %q", out)
	}
	if strings.Contains(out, "FEAT-001") {
		t.Errorf("App.luaWriteDeps is NOT wired to the visibility tracer — "+
			"a hidden node leaked into a script's traversal: %q", out)
	}
}

// TestLuaWiring_WritePrepStoreStaysRaw pins the other half of the split at
// the wiring level: the App must hand scripts a RAW write-prep handle, or
// rela.update_entity would erase hidden properties on save.
func TestLuaWiring_WritePrepStoreStaysRaw(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "T"}})

	// With a Declarative ACL configured, the two handles must DIFFER: reads
	// gated, write-prep raw. (Without a policy both are the raw store — the
	// NopACL path — which is why this test configures one.)
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	deps := app.luaWriteDeps()
	if deps.WritePrepStore == nil {
		t.Fatal("WritePrepStore is nil — rela.update_entity would fail")
	}
	if deps.WritePrepStore != app.store {
		t.Errorf("WritePrepStore is not the raw store (%T) — a redacted "+
			"read-before-write erases hidden properties on save", deps.WritePrepStore)
	}
	if deps.VisibleReader == lua.EntityReader(app.store) {
		t.Error("VisibleReader IS the raw store under a configured policy — " +
			"script reads are not ACL-bound")
	}
}
