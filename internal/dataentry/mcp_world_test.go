package dataentry

import (
	"context"
	"errors"
	"testing"
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
		wantDenied  bool
		wantErr     error
		wantAnyErr  bool
	}{
		{name: "no default world configured", gate: fakeGate{}, wantDefault: true},
		{name: "explicit default world", configured: defaultWorldName, gate: fakeGate{}, wantDefault: true},
		{name: "granted world", configured: "published", gate: fakeGate{}},
		{
			name: "world without a grant binds as denied", configured: "published",
			gate: fakeGate{denyWorlds: map[string]bool{"published": true}}, wantDenied: true,
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
			if got.Denied != tc.wantDenied {
				t.Errorf("Denied = %v, want %v", got.Denied, tc.wantDenied)
			}
			if !tc.wantDenied && got.Scope.IsDefaultWorld() != tc.wantDefault {
				t.Errorf("default world = %v, want %v", got.Scope.IsDefaultWorld(), tc.wantDefault)
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
	if err != nil || !before.Scope.IsDefaultWorld() {
		t.Fatalf("before reload: %+v, %v; want the default world", before, err)
	}
	cfg := &Config{}
	cfg.App.DefaultWorld = "published"
	a.schema.Publish(&Schema{Cfg: cfg})

	after, err := source(ctx)
	if err != nil || after.Scope.IsDefaultWorld() {
		t.Fatalf("after reload: %+v, %v; want the published world", after, err)
	}
}

func TestMCPHost_CarriesAWorldSource(t *testing.T) {
	t.Parallel()
	if mcpHost(appWithDefaultWorld(t, "")).ReadWorld == nil {
		t.Error("mcpHost left ReadWorld nil; the remote MCP wiring would refuse to start")
	}
}
