package dataentry

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestMCPReadWorld resolves the remote MCP world the way attachWorld resolves
// a data-entry read that names no world: the operator's `app.default_world`,
// with the caller's world grant checked (BUG-6XTX0G).
func TestMCPReadWorld(t *testing.T) {
	t.Parallel()
	gateDown := errors.New("store is down")
	tests := []struct {
		name        string
		configured  string
		gate        readGate
		wantDefault bool
		wantErr     error
		wantAnyErr  bool
	}{
		{name: "no default world configured", gate: fakeGate{}, wantDefault: true},
		{name: "explicit default world", configured: defaultWorldName, gate: fakeGate{}, wantDefault: true},
		{name: "granted world", configured: "published", gate: fakeGate{}},
		{
			name: "world without a grant falls back to the default world", configured: "published",
			gate: fakeGate{denyWorlds: map[string]bool{"published": true}}, wantDefault: true,
		},
		{
			name: "a gate failure is an error, not a denial", configured: "published",
			gate: fakeGate{worldErr: gateDown}, wantErr: gateDown,
		},
		{name: "an undeclared world is an error", configured: "nope", gate: fakeGate{}, wantAnyErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := appWithDefaultWorld(t, tc.configured)
			ctx := withReadGate(context.Background(), tc.gate)

			got, err := mcpReadWorld(a)(ctx)
			switch {
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			case tc.wantAnyErr:
				if err == nil {
					t.Fatal("err = nil, want an error")
				}
				return
			case err != nil:
				t.Fatalf("err = %v", err)
			}
			if got.IsDefaultWorld() != tc.wantDefault {
				t.Errorf("default world = %v, want %v", got.IsDefaultWorld(), tc.wantDefault)
			}
		})
	}
}

// The configuration is read per call, so an operator's edit to
// `app.default_world` reaches MCP without a restart.
func TestMCPReadWorld_FollowsAConfigReload(t *testing.T) {
	t.Parallel()
	a := appWithDefaultWorld(t, "")
	source := mcpReadWorld(a)
	ctx := withReadGate(context.Background(), fakeGate{})

	before, err := source(ctx)
	if err != nil || !before.IsDefaultWorld() {
		t.Fatalf("before reload: %+v, %v; want the default world", before, err)
	}
	cfg := &Config{}
	cfg.App.DefaultWorld = "published"
	a.schema.Publish(&Schema{Cfg: cfg})

	after, err := source(ctx)
	if err != nil || after.IsDefaultWorld() {
		t.Fatalf("after reload: %+v, %v; want the published world", after, err)
	}
}

func TestMCPHost_CarriesAWorldSource(t *testing.T) {
	t.Parallel()
	if mcpHost(appWithDefaultWorld(t, "")).ReadWorld == nil {
		t.Error("mcpHost left ReadWorld nil; the remote MCP wiring would refuse to start")
	}
}

// Within one MCP request the world resolves once, so a configuration reload
// part-way through a tool call cannot switch the world between its reads.
func TestMCPReadWorld_ResolvesOncePerRequest(t *testing.T) {
	t.Parallel()
	a := appWithDefaultWorld(t, "")
	source := mcpReadWorld(a)

	var inRequest []bool
	h := withMCPWorldMemo(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ctx := withReadGate(r.Context(), fakeGate{})
		first, err := source(ctx)
		if err != nil {
			t.Fatal(err)
		}
		cfg := &Config{}
		cfg.App.DefaultWorld = "published"
		a.schema.Publish(&Schema{Cfg: cfg})
		second, err := source(ctx)
		if err != nil {
			t.Fatal(err)
		}
		inRequest = []bool{first.IsDefaultWorld(), second.IsDefaultWorld()}
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, MCPPath, http.NoBody))

	if !inRequest[0] || !inRequest[1] {
		t.Errorf("default world within the request = %v, want both reads in the world it started in", inRequest)
	}
	next, err := source(withReadGate(context.Background(), fakeGate{}))
	if err != nil || next.IsDefaultWorld() {
		t.Errorf("a later request = %v, %v; want the reloaded world", next, err)
	}
}

// TestMCPReadWorld_ThroughTheRouter drives the MCP route through the real
// middleware chain, so the world grant is checked by the real ACL gate that
// attachACLRequest puts on the ctx, not by a fake.
func TestMCPReadWorld_ThroughTheRouter(t *testing.T) {
	for _, tc := range []struct {
		name        string
		read        []string
		wantDefault bool
	}{
		{"granted world is applied", []string{"ticket", "world:published"}, false},
		{"ungranted world falls back to the default world", []string{"ticket"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			app.acl = mustNewACL(t, &acl.Policy{
				Roles:       map[string]acl.RoleDef{"viewer": {Read: tc.read}},
				Assignments: map[string]string{"alice": "viewer"},
			}, app.store)
			app.SetWorlds(stubWorlds{names: map[string]bool{"published": true}})
			app.SetPrincipalResolver(func(*http.Request) principal.Principal {
				return principal.Principal{User: "alice", Tool: principal.ToolMCP}
			})
			state := *app.State()
			cfg := *state.Cfg
			cfg.App.DefaultWorld = "published"
			state.Cfg = &cfg
			app.schema.Publish(&state)

			source := mcpReadWorld(app)
			var got store.WorldScope
			var gotErr error
			app.mcpHandler = withMCPWorldMemo(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got, gotErr = source(r.Context())
				w.WriteHeader(http.StatusOK)
			}))
			rec := httptest.NewRecorder()
			app.NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, MCPPath, http.NoBody))
			if rec.Code != http.StatusOK {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
			if gotErr != nil {
				t.Fatalf("source: %v", gotErr)
			}
			if got.IsDefaultWorld() != tc.wantDefault {
				t.Errorf("default world = %v, want %v", got.IsDefaultWorld(), tc.wantDefault)
			}
		})
	}
}
