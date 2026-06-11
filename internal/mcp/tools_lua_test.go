package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// luaCallToolReq builds a minimal MCP CallToolRequest carrying named
// string args, mirroring how the real MCP harness deserialises a client
// invocation. Used to drive handleLuaEval / handleLuaRun in tests.
func luaCallToolReq(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

// decodeScriptError extracts the JSON envelope returned by the lua tools
// on failure. Errors out the test if anything about the shape is off.
func decodeScriptError(t *testing.T, result *mcp.CallToolResult) *lua.ScriptError {
	t.Helper()
	if !result.IsError {
		t.Fatalf("expected IsError=true, got false; content=%v", result.Content)
	}
	if len(result.Content) == 0 {
		t.Fatal("result has no content")
	}
	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] is not TextContent: %T", result.Content[0])
	}
	var se lua.ScriptError
	if err := json.Unmarshal([]byte(text.Text), &se); err != nil {
		t.Fatalf("envelope is not valid JSON: %v\nbody: %s", err, text.Text)
	}
	return &se
}

func TestHandleLuaEval_ReturnsScriptErrorEnvelope(t *testing.T) {
	t.Parallel()
	s := makeTestServer(t)

	result, err := s.handleLuaEval(context.Background(),
		luaCallToolReq(map[string]any{"code": "print('hello')\nerror('kaboom')"}))
	if err != nil {
		t.Fatalf("handler returned Go error: %v", err)
	}
	se := decodeScriptError(t, result)

	if se.Surface != lua.SurfaceLuaEval {
		t.Errorf("Surface=%q, want %q", se.Surface, lua.SurfaceLuaEval)
	}
	if se.Path != "<inline>" {
		t.Errorf("Path=%q, want <inline>", se.Path)
	}
	if !strings.Contains(se.LuaMessage, "kaboom") {
		t.Errorf("LuaMessage=%q, want contains kaboom", se.LuaMessage)
	}
	if se.LuaLine != 2 {
		t.Errorf("LuaLine=%d, want 2", se.LuaLine)
	}
	// CapturedOutput is intentionally empty for lua_eval/lua_run: see
	// runtime.go:256 — print() routes to os.Stdout outside document/
	// action modes so MCP / scheduler / CLI behave like a normal terminal.
	if se.CapturedOutput != "" {
		t.Errorf("lua_eval CapturedOutput=%q, want empty (print goes to terminal)", se.CapturedOutput)
	}
	// lua_eval has no on-disk source — Source slice must be empty.
	if len(se.Source) != 0 {
		t.Errorf("lua_eval Source non-empty: %+v", se.Source)
	}
}

func TestHandleLuaEval_PreservesIsErrorFlag(t *testing.T) {
	t.Parallel()
	s := makeTestServer(t)
	result, _ := s.handleLuaEval(context.Background(),
		luaCallToolReq(map[string]any{"code": "error('x')"}))
	if !result.IsError {
		t.Error("IsError flag must remain true on Lua failure for MCP clients to branch")
	}
}
