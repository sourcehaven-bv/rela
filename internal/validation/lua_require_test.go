package validation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func writeLib(t *testing.T, root, name, content string) {
	t.Helper()
	p := filepath.Join(root, "lib", filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// One runtime serves every entity of a rule, so a module loads once. A
// module function that takes the entity as an argument sees each entity in
// turn.
func TestLuaValidation_RequireAcrossEntities(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeLib(t, root, "rules/status.lua", `
loads = (loads or 0) + 1
return { draft = function(e) return e.properties.status == "draft" end }`)
	meta := ticketMeta([]metamodel.ValidationRule{{
		Name:       "no-drafts",
		EntityType: "ticket",
		Lua: `if require("rules.status").draft(entity) then
  return { message = entity.id .. " is a draft (loads=" .. loads .. ")" }
end`,
	}}, "status")
	entities := []*entity.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"status": "draft"}},
		{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"status": "ready"}},
		{ID: "TKT-003", Type: "ticket", Properties: map[string]any{"status": "draft"}},
	}
	result := New(meta, newMockWorkspace().services(root)).Check(context.Background(), entities, nil)
	if len(result.ScriptErrors) != 0 {
		t.Fatalf("script errors: %v", result.ScriptErrors)
	}
	var got []string
	for _, v := range result.Violations {
		got = append(got, v.Message)
	}
	want := "TKT-001 is a draft (loads=1),TKT-003 is a draft (loads=1)"
	if strings.Join(got, ",") != want {
		t.Errorf("messages = %q, want %q", got, want)
	}
}

// An inline rule's excerpt comes from the rule text, but a failure inside a
// module it required is excerpted from lib/.
func TestLuaValidation_InlineRuleErrorInModule(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeLib(t, root, "fail.lua", "local M = {}\nfunction M.run()\n  error(\"inside module\")\nend\nreturn M\n")
	meta := ticketMeta([]metamodel.ValidationRule{{
		Name: "uses-module", EntityType: "ticket", Lua: `require("fail").run()`,
	}})
	entities := []*entity.Entity{{ID: "TKT-001", Type: "ticket", Properties: map[string]any{}}}
	result := New(meta, newMockWorkspace().services(root)).Check(context.Background(), entities, nil)
	if len(result.ScriptErrors) != 1 {
		t.Fatalf("got %d ScriptErrors, want 1", len(result.ScriptErrors))
	}
	se := result.ScriptErrors[0]
	if se.LuaPath != "lib/fail.lua" || se.LuaLine != 3 {
		t.Errorf("LuaPath:LuaLine = %s:%d, want lib/fail.lua:3", se.LuaPath, se.LuaLine)
	}
	found := false
	for _, l := range se.Source {
		found = found || (l.Highlight && strings.Contains(l.Text, "inside module"))
	}
	if !found {
		t.Errorf("source = %+v, want the module line highlighted", se.Source)
	}
}
