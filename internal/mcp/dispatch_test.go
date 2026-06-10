package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// These tests exercise the real registration → dispatch → argument-decode
// surface of the mcp-go server, which the handler-level tests in
// tools_test.go bypass (they call s.handle* methods directly with
// pre-decoded argument maps). A tool registered under the wrong name, a
// handler wired to the wrong tool, or an argument schema that stops
// decoding is only visible at this level (TKT-TLQ94B).

// newDispatchServer builds the server through the production NewServer
// constructor so tool registration, the principal middleware, and the
// mcp-go dispatch table are all real.
func newDispatchServer(t *testing.T) *Server {
	t.Helper()

	meta, st := makeTestFixture(t)
	srv, err := NewServer(newTestDeps(t, meta, st), "test",
		WithPrincipal(principal.Principal{User: "tester", Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv
}

// rpcError mirrors the JSON-RPC error object shape on the wire.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// dispatch sends a raw JSON-RPC request through the mcp-go message
// handler — the same entry the stdio transport feeds — and returns the
// decoded result or error.
func dispatch(t *testing.T, s *Server, method, params string) (json.RawMessage, *rpcError) {
	t.Helper()

	raw := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":%q,"params":%s}`, method, params)
	msg := s.mcp.HandleMessage(context.Background(), json.RawMessage(raw))
	if msg == nil {
		t.Fatalf("HandleMessage returned nil for %s", method)
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var resp struct {
		Result json.RawMessage `json:"result"`
		Error  *rpcError       `json:"error"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("decode response %s: %v", data, err)
	}
	return resp.Result, resp.Error
}

// callTool invokes a tool via a real tools/call message with raw JSON
// arguments and returns the decoded CallToolResult fields.
func callTool(t *testing.T, s *Server, name, argsJSON string) (text string, isError bool) {
	t.Helper()

	params := fmt.Sprintf(`{"name":%q,"arguments":%s}`, name, argsJSON)
	result, rpcErr := dispatch(t, s, "tools/call", params)
	if rpcErr != nil {
		t.Fatalf("tools/call %s: JSON-RPC error %d: %s", name, rpcErr.Code, rpcErr.Message)
	}
	var decoded struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(result, &decoded); err != nil {
		t.Fatalf("decode tools/call result %s: %v", result, err)
	}
	if len(decoded.Content) == 0 {
		t.Fatalf("tools/call %s: empty content", name)
	}
	return decoded.Content[0].Text, decoded.IsError
}

// toolCalls is one realistic happy-path invocation per registered tool.
// Arguments are raw JSON so the real schema decode runs — no hand-built
// map[string]interface{} with pre-coerced float64s.
//
// TestDispatch_ToolInventoryMatches diffs this table against tools/list,
// so registering a new tool without adding a case here fails loudly.
var toolCalls = map[string]struct {
	args string
	// wantErr marks calls whose tool-level error result is the expected
	// outcome (the dispatch and decode still succeeded).
	wantErr bool
}{
	"list_entities":       {args: `{"type":"requirement","limit":2}`},
	"show_entity":         {args: `{"id":"REQ-001"}`},
	"search_entities":     {args: `{"query":"requirement","limit":5}`},
	"create_entity":       {args: `{"type":"requirement","properties":{"title":"Created via dispatch"}}`},
	"update_entity":       {args: `{"id":"REQ-001","properties":{"status":"done"}}`},
	"delete_entity":       {args: `{"id":"REQ-002","cascade":true}`},
	"rename_entity":       {args: `{"id":"REQ-003","new_id":"REQ-099"}`},
	"list_relations":      {args: `{}`},
	"create_relation":     {args: `{"from":"DEC-001","type":"addresses","to":"REQ-002"}`},
	"delete_relation":     {args: `{"from":"DEC-001","type":"addresses","to":"REQ-001"}`},
	"trace_from":          {args: `{"id":"REQ-001","max_depth":3}`},
	"trace_to":            {args: `{"id":"REQ-001"}`},
	"find_path":           {args: `{"from":"DEC-001","to":"REQ-001"}`},
	"analyze_orphans":     {args: `{}`},
	"analyze_cardinality": {args: `{}`},
	"analyze_properties":  {args: `{}`},
	"analyze_validations": {args: `{}`},
	"analyze_schema":      {args: `{"threshold":0}`},
	"get_metamodel":       {args: `{}`},
	"list_entity_types":   {args: `{}`},
	"list_relation_types": {args: `{}`},
	"export":              {args: `{"format":"json"}`},
	"lua_eval":            {args: `{"code":"return 1"}`},
	"lua_run":             {args: `{"path":"missing.lua"}`, wantErr: true}, // no scripts dir in fixture
	"lua_list":            {args: `{}`},
}

// TestDispatch_ToolInventoryMatches pins the registered tool set via a
// real tools/list round trip. A tool added to registerTools without a
// dispatch case below (or removed while still listed here) fails this
// test by name.
func TestDispatch_ToolInventoryMatches(t *testing.T) {
	t.Parallel()
	s := newDispatchServer(t)

	result, rpcErr := dispatch(t, s, "tools/list", `{}`)
	if rpcErr != nil {
		t.Fatalf("tools/list: JSON-RPC error %d: %s", rpcErr.Code, rpcErr.Message)
	}
	var decoded struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &decoded); err != nil {
		t.Fatalf("decode tools/list result: %v", err)
	}

	registered := make(map[string]bool, len(decoded.Tools))
	for _, tool := range decoded.Tools {
		registered[tool.Name] = true
	}
	for name := range toolCalls {
		if !registered[name] {
			t.Errorf("tool %q is in the dispatch table but not registered", name)
		}
	}
	var missing []string
	for name := range registered {
		if _, ok := toolCalls[name]; !ok {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	for _, name := range missing {
		t.Errorf("tool %q is registered but has no dispatch test case — add it to toolCalls", name)
	}
}

// TestDispatch_EveryToolDecodesAndRuns drives each registered tool
// through a real tools/call. Each subtest gets a fresh server because
// the write tools mutate the seeded graph.
func TestDispatch_EveryToolDecodesAndRuns(t *testing.T) {
	t.Parallel()
	names := make([]string, 0, len(toolCalls))
	for name := range toolCalls {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		tc := toolCalls[name]
		t.Run(name, func(t *testing.T) {
			s := newDispatchServer(t)
			text, isError := callTool(t, s, name, tc.args)
			if isError != tc.wantErr {
				t.Errorf("isError = %v, want %v (result text: %s)", isError, tc.wantErr, text)
			}
			if text == "" {
				t.Error("empty result text")
			}
		})
	}
}

// TestDispatch_UnknownToolRejected pins that the dispatch layer (not our
// handlers) rejects calls to unregistered tool names.
func TestDispatch_UnknownToolRejected(t *testing.T) {
	t.Parallel()
	s := newDispatchServer(t)

	result, rpcErr := dispatch(t, s, "tools/call", `{"name":"no_such_tool","arguments":{}}`)
	if rpcErr == nil {
		t.Fatalf("expected JSON-RPC error for unknown tool, got result %s", result)
	}
}

// TestDispatch_MalformedArgumentsSurface pins that a required argument
// missing from the wire payload surfaces as an error to the client
// rather than silently running with a zero value. The enforcement
// lives in the handler's RequireString guard (mcp-go does not reject
// missing required arguments at the schema/dispatch layer); this test
// pins the client-visible contract regardless of which layer enforces
// it.
func TestDispatch_MalformedArgumentsSurface(t *testing.T) {
	t.Parallel()
	s := newDispatchServer(t)

	text, isError := callTool(t, s, "show_entity", `{}`)
	if !isError {
		t.Errorf("show_entity without required id should produce an error result, got: %s", text)
	}
}
