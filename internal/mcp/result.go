package mcp

import (
	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Result shims for the go-sdk migration (TKT-UIR41P).
//
// The 34 handlers in this package build results through exactly two helpers,
// which mark3labs supplied as mcp.NewToolResultText / mcp.NewToolResultError
// (117 call sites between them). The go-sdk has no direct equivalent: its
// generic AddTool expects a handler to return `(nil, out, err)` and packs the
// error itself.
//
// Rather than rewrite 117 call sites into two different control-flow shapes —
// which is where a migration silently changes behavior — these shims keep the
// existing "build and return a result value" style and produce the go-sdk
// result explicitly. The wire output is identical to the previous library's:
// a single text content block, with IsError set for the error case.
//
// Handlers therefore return (*mcpgo.CallToolResult, error) with a nil error,
// and are registered through the RAW Server.AddTool, whose contract treats a
// returned error as a PROTOCOL error (tool.go:27-29). That is what we want:
// tool-level failures stay in-band as IsError results, exactly as before, and
// only genuine dispatch faults become JSON-RPC errors.

// textResult returns a successful single-text-block tool result.
func textResult(text string) *mcpgo.CallToolResult {
	return &mcpgo.CallToolResult{
		Content: []mcpgo.Content{&mcpgo.TextContent{Text: text}},
	}
}

// errorResult returns a tool-level error result: the failure is reported to
// the model in-band (IsError) rather than as a JSON-RPC protocol error, so the
// assistant can read the message and correct its call.
func errorResult(text string) *mcpgo.CallToolResult {
	return &mcpgo.CallToolResult{
		Content: []mcpgo.Content{&mcpgo.TextContent{Text: text}},
		IsError: true,
	}
}
