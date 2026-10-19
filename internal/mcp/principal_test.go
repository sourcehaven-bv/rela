package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestNewServer_RejectsZeroPrincipal verifies that NewServer refuses
// construction without WithPrincipal — silently degrading to
// unknown/unknown audit attribution in production would be an
// invisible bug.
func TestNewServer_RejectsZeroPrincipal(t *testing.T) {
	t.Parallel()
	_, err := NewServer(Deps{World: store.TrivialScope(), Families: store.TrivialScope()}, "0.0.0")
	if err == nil {
		t.Fatal("expected error when WithPrincipal is omitted")
	}
}

// TestNewServer_RejectsIncompleteDeps verifies that NewServer rejects a
// Deps missing any required field, with a valid Principal supplied so
// the Deps validation (not the Principal gate) is what fails. A zero
// field deferred to request time would nil-deref in a handler.
func TestNewServer_RejectsIncompleteDeps(t *testing.T) {
	t.Parallel()
	withPrincipal := WithPrincipal(principal.Principal{User: "test", Tool: principal.ToolMCP})

	// makeTestServer builds a complete Deps; reuse it as the baseline.
	complete := makeTestServer(t).deps()
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
		"World":         func(d *Deps) { d.World = store.WorldScope{} },
		"Families":      func(d *Deps) { d.Families = store.WorldScope{} },
	}
	for field, zero := range mutators {
		t.Run(field, func(t *testing.T) {
			t.Parallel()
			deps := makeTestServer(t).deps()
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

// TestPrincipalMiddleware_CtxIdentityWins pins the property the remote
// HTTP transport depends on (TKT-BDG8U9): when the ctx ALREADY carries a
// principal, the server's construction-time one must not overwrite it.
//
// Over HTTP the SDK hands handlers the *http.Request ctx, which the router
// chain has already stamped with the JWT-verified caller. Overwriting that
// would attribute every remote caller's writes to a single process-wide
// identity AND give the ACL the wrong subject to gate reads against — one
// caller would see another's rows. Under stdio no principal is ever on the
// ctx, so the fallback below still applies there.
func TestPrincipalMiddleware_CtxIdentityWins(t *testing.T) {
	t.Parallel()
	serverPrincipal := principal.Principal{User: "server-identity", Tool: principal.ToolMCP}
	s := &Server{principal: serverPrincipal}

	// capture takes *testing.T so each subtest can run in parallel: the
	// captured principal is local to the call, not shared across subtests.
	capture := func(t *testing.T, ctx context.Context) principal.Principal {
		t.Helper()
		var got principal.Principal
		h := s.principalMiddleware(
			func(ctx context.Context, _ string, _ mcpgo.Request) (mcpgo.Result, error) {
				got = principal.From(ctx)
				return &mcpgo.CallToolResult{}, nil
			})
		_, _ = h(ctx, "tools/call", &mcpgo.CallToolRequest{})
		return got
	}

	t.Run("a request-scoped principal is preserved", func(t *testing.T) {
		t.Parallel()
		caller := principal.Principal{User: "usr_alice", Tool: principal.ToolMCP}
		got := capture(t, principal.With(context.Background(), caller))

		if !got.Equal(caller) {
			t.Errorf("Principal = %+v, want the ctx principal %+v — the "+
				"construction-time identity must not overwrite a verified caller",
				got, caller)
		}
	})

	t.Run("two callers stay distinct", func(t *testing.T) {
		t.Parallel()
		// The multi-tenant property in miniature: whatever the middleware
		// does, it must not collapse two callers onto one identity.
		a := principal.Principal{User: "usr_alice", Tool: principal.ToolMCP}
		b := principal.Principal{User: "usr_bob", Tool: principal.ToolMCP}

		gotA := capture(t, principal.With(context.Background(), a))
		gotB := capture(t, principal.With(context.Background(), b))

		if gotA.Equal(gotB) {
			t.Fatalf("both callers resolved to %+v — identities collapsed", gotA)
		}
		if gotA.User != "usr_alice" || gotB.User != "usr_bob" {
			t.Errorf("got %q and %q, want usr_alice and usr_bob", gotA.User, gotB.User)
		}
	})

	t.Run("an empty ctx still falls back to the server principal", func(t *testing.T) {
		t.Parallel()
		// The stdio path. Without this the change would trade one bug for
		// another: unattributed writes instead of misattributed ones.
		got := capture(t, context.Background())

		if !got.Equal(serverPrincipal) {
			t.Errorf("Principal = %+v, want the server's %+v when the ctx has none",
				got, serverPrincipal)
		}
	})
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

// TestPrincipalMiddleware_BindsWithThePinnedSnapshot pins that one call binds
// its identity and runs its handler against the SAME published snapshot. A
// reload landing mid-call must not pair a Request with readers from another
// assembly; that would fail the call with acl.ErrForeignIdentity.
func TestPrincipalMiddleware_BindsWithThePinnedSnapshot(t *testing.T) {
	t.Parallel()
	s, st := makeTestServerWithStore(t)
	s.principal = principal.Principal{User: "alice", Tool: principal.ToolMCP}

	type boundBy struct{}
	first := s.deps()
	first.BindIdentity = func(ctx context.Context) (context.Context, error) {
		return context.WithValue(ctx, boundBy{}, "first"), nil
	}
	if err := s.ReloadDeps(first); err != nil {
		t.Fatalf("ReloadDeps: %v", err)
	}
	pinned := s.state.current()

	second := newTestDeps(t, metaWithRisk(), st)
	second.BindIdentity = func(ctx context.Context) (context.Context, error) {
		return context.WithValue(ctx, boundBy{}, "second"), nil
	}
	var gotBinder string
	var gotSnap *snapshot
	h := s.principalMiddleware(func(ctx context.Context, _ string, _ mcpgo.Request) (mcpgo.Result, error) {
		// A reload lands between the bind and the handler's reads.
		if err := s.ReloadDeps(second); err != nil {
			t.Errorf("ReloadDeps: %v", err)
		}
		gotBinder, _ = ctx.Value(boundBy{}).(string)
		gotSnap = snapshotFor(ctx, s)
		return &mcpgo.CallToolResult{}, nil
	})
	if _, err := h(context.Background(), "tools/call", &mcpgo.CallToolRequest{}); err != nil {
		t.Fatalf("call: %v", err)
	}
	if gotBinder != "first" {
		t.Errorf("bound by %q, want the binder of the snapshot the call started on", gotBinder)
	}
	if gotSnap != pinned {
		t.Error("the handler read a snapshot other than the one the call pinned")
	}
}

// TestPrincipalMiddleware_ResolveErrorIsFixed pins that a resolve failure
// reaches the client as a fixed message; the cause is only logged.
func TestPrincipalMiddleware_ResolveErrorIsFixed(t *testing.T) {
	t.Parallel()
	s, _ := makeTestServerWithStore(t)
	s.principal = principal.Principal{User: "alice", Tool: principal.ToolMCP}
	d := s.deps()
	d.BindIdentity = func(ctx context.Context) (context.Context, error) {
		return ctx, errors.New("store down at db.internal:5432 for alice")
	}
	if err := s.ReloadDeps(d); err != nil {
		t.Fatalf("ReloadDeps: %v", err)
	}
	h := s.principalMiddleware(func(_ context.Context, _ string, _ mcpgo.Request) (mcpgo.Result, error) {
		t.Error("the handler ran without a resolved identity")
		return &mcpgo.CallToolResult{}, nil
	})
	_, err := h(context.Background(), "tools/call", &mcpgo.CallToolRequest{})
	if !errors.Is(err, errIdentityUnresolved) || err.Error() != errIdentityUnresolved.Error() {
		t.Errorf("err = %v, want exactly %v", err, errIdentityUnresolved)
	}
}
