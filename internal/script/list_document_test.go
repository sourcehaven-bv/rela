package script

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// testRows is a ListRows over a fixed slice, mirroring the adapter the
// data-entry export handler supplies.
type testRows []*entity.Entity

func (r testRows) Len() int { return len(r) }

func (r testRows) At(i int) *entity.Entity {
	if i < 0 || i >= len(r) {
		return nil
	}
	return r[i]
}

func listCtx() lua.ListRenderContext {
	return lua.ListRenderContext{
		ListID: "tickets",
		Rows: testRows{
			{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "First"}},
			{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "Second"}},
		},
		Query: lua.ListQuery{
			ListID: "tickets", EntityType: "ticket", Total: 2, Rendered: 2,
		},
	}
}

// TestExecuteListDocument_CapturesStdout is the core contract: the script
// renders the rows it was handed and its print() output is the markdown.
func TestExecuteListDocument_CapturesStdout(t *testing.T) {
	root := writeDocScript(t, "list.lua", `print("# " .. rela.document.list_id)
for _, row in rela.document.rows() do
  print("- " .. row.properties.title)
end`)

	var stdout bytes.Buffer
	engine := NewEngine()
	err := engine.ExecuteListDocument(t.Context(), "list.lua", testWriteDeps(root), &stdout,
		"export:list:tickets", listCtx(), 0)
	if err != nil {
		t.Fatalf("ExecuteListDocument failed: %v", err)
	}

	want := "# tickets\n- First\n- Second\n"
	if got := stdout.String(); got != want {
		t.Errorf("stdout mismatch:\n got: %q\nwant: %q", got, want)
	}
}

// TestExecuteListDocument_PrincipalThreaded mirrors the entity path: the
// render runs under the caller's identity, which is what binds the script's
// own reads to the caller's ACL (TKT-ZF2DTV).
func TestExecuteListDocument_PrincipalThreaded(t *testing.T) {
	root := writeDocScript(t, "who.lua", `print(rela.principal.user .. "/" .. rela.principal.tool)`)

	ctx := principal.With(t.Context(), principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	var stdout bytes.Buffer
	engine := NewEngine()
	err := engine.ExecuteListDocument(ctx, "who.lua", testWriteDeps(root), &stdout,
		"export:list:tickets", listCtx(), 0)
	if err != nil {
		t.Fatalf("ExecuteListDocument failed: %v", err)
	}
	if got, want := stdout.String(), "alice/"+principal.ToolDataEntry+"\n"; got != want {
		t.Errorf("principal not threaded:\n got: %q\nwant: %q", got, want)
	}
}

// TestExecuteListDocument_ScriptErrorSurfaced verifies a Lua failure comes
// back as a *lua.ScriptError naming the list, so the HTTP layer can branch
// via errors.As exactly as it does for entity renders.
func TestExecuteListDocument_ScriptErrorSurfaced(t *testing.T) {
	root := writeDocScript(t, "boom.lua", `error("kaboom")`)

	var stdout bytes.Buffer
	engine := NewEngine()
	err := engine.ExecuteListDocument(t.Context(), "boom.lua", testWriteDeps(root), &stdout,
		"export:list:tickets", listCtx(), 0)
	if err == nil {
		t.Fatal("expected an error from a failing script")
	}
	var se *lua.ScriptError
	if !errors.As(err, &se) {
		t.Fatalf("want *lua.ScriptError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "kaboom") {
		t.Errorf("error should carry the Lua message: %v", err)
	}
}

// TestExecuteListDocument_BadPath pins that path validation runs before any
// execution — the same loadScript guard ExecuteDocument uses.
func TestExecuteListDocument_BadPath(t *testing.T) {
	root := writeDocScript(t, "ok.lua", `print("hi")`)

	var stdout bytes.Buffer
	engine := NewEngine()
	err := engine.ExecuteListDocument(t.Context(), "../escape.lua", testWriteDeps(root), &stdout,
		"export:list:tickets", listCtx(), 0)
	if err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
	if stdout.Len() != 0 {
		t.Errorf("nothing should have been rendered, got %q", stdout.String())
	}
}
