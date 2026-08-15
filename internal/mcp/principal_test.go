package mcp

import (
	"context"
	"encoding/json"
	"testing"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestNewServer_RejectsZeroPrincipal verifies that NewServer refuses
// construction without WithPrincipal — silently degrading to
// unknown/unknown audit attribution in production would be an
// invisible bug.
func TestNewServer_RejectsZeroPrincipal(t *testing.T) {
	t.Parallel()
	_, err := NewServer(Deps{}, "0.0.0")
	if err == nil {
		t.Fatal("expected error when WithPrincipal is omitted")
	}
}

// TestNewServer_RejectsIncompleteDeps verifies that NewServer rejects a
// Deps missing any required field, with a valid Principal supplied so
// the Deps validation (not the Principal gate) is what fails. A zero
// field deferred to request time would either nil-deref in a handler or
// — for ProjectRoot — make lua_list silently walk the process CWD.
func TestNewServer_RejectsIncompleteDeps(t *testing.T) {
	t.Parallel()
	withPrincipal := WithPrincipal(principal.Principal{User: "test", Tool: principal.ToolMCP})

	// makeTestServer builds a complete Deps; reuse it as the baseline.
	complete := makeTestServer(t).deps
	if _, err := NewServer(complete, "0.0.0", withPrincipal); err != nil {
		t.Fatalf("complete Deps should construct: %v", err)
	}

	mutators := map[string]func(*Deps){
		"Store":         func(d *Deps) { d.Store = nil },
		"Meta":          func(d *Deps) { d.Meta = nil },
		"Tracer":        func(d *Deps) { d.Tracer = nil },
		"Searcher":      func(d *Deps) { d.Searcher = nil },
		"Validator":     func(d *Deps) { d.Validator = nil },
		"EntityManager": func(d *Deps) { d.EntityManager = nil },
		"Config":        func(d *Deps) { d.Config = nil },
		"Watcher":       func(d *Deps) { d.Watcher = nil },
		"ProjectRoot":   func(d *Deps) { d.ProjectRoot = "" },
	}
	for field, zero := range mutators {
		t.Run(field, func(t *testing.T) {
			t.Parallel()
			deps := makeTestServer(t).deps
			zero(&deps)
			if _, err := NewServer(deps, "0.0.0", withPrincipal); err == nil {
				t.Fatalf("expected error when %s is zero", field)
			}
		})
	}
}

// TestPrincipalMiddleware_WithOption verifies AC4 for the MCP entry
// point — the configured Principal flows into every tool ctx via the
// middleware. Registered handlers (including new write tools added
// later) inherit the stamp automatically; no per-handler opt-in.
func TestPrincipalMiddleware_WithOption(t *testing.T) {
	t.Parallel()
	want := principal.Principal{User: "alice", Tool: principal.ToolMCP}
	s := &Server{principal: want}

	var captured principal.Principal
	handler := s.principalMiddleware(
		func(ctx context.Context, _ string, _ mcpgo.Request) (mcpgo.Result, error) {
			captured = principal.From(ctx)
			return &mcpgo.CallToolResult{}, nil
		})

	_, _ = handler(context.Background(), "tools/call", &mcpgo.CallToolRequest{})

	if !captured.Equal(want) {
		t.Errorf("Principal = %+v, want %+v", captured, want)
	}
}

// TestPrincipalMiddleware_RegisteredOnEveryTool is the regression
// guard for the cranky-reviewer finding: handlers that don't
// explicitly call s.principalContext (lua_eval, lua_run, future
// write tools) still inherit the Principal stamp because the
// middleware sits in front of every registered handler.
func TestPrincipalMiddleware_RegisteredOnEveryTool(t *testing.T) {
	t.Parallel()
	want := principal.Principal{User: "alice", Tool: principal.ToolMCP}
	s := &Server{principal: want}

	srv := mcpgo.NewServer(&mcpgo.Implementation{Name: "test", Version: "0.0.0"}, nil)
	srv.AddReceivingMiddleware(s.principalMiddleware)

	var captured principal.Principal
	srv.AddTool(
		&mcpgo.Tool{
			Name:        "any-write-tool",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
		func(ctx context.Context, _ *mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			captured = principal.From(ctx)
			return &mcpgo.CallToolResult{}, nil
		})

	// Drive a real client→server call so the middleware runs in its
	// production position rather than being invoked directly.
	ctx := context.Background()
	serverTransport, clientTransport := mcpgo.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer func() { _ = serverSession.Close() }()

	client := mcpgo.NewClient(&mcpgo.Implementation{Name: "test-client", Version: "0.0.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer func() { _ = clientSession.Close() }()

	if _, err := clientSession.CallTool(ctx, &mcpgo.CallToolParams{Name: "any-write-tool"}); err != nil {
		t.Fatalf("call tool: %v", err)
	}

	if !captured.Equal(want) {
		t.Errorf("Principal stamped on handler ctx = %+v, want %+v", captured, want)
	}
}
