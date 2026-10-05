package dataentry

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func TestMCPSelectWorld(t *testing.T) {
	t.Parallel()
	gateDown := errors.New("store is down")
	tests := []struct {
		name        string
		configured  string
		world       string
		gate        readGate
		wantDefault bool
		wantErr     string
		wantIs      error
	}{
		{name: "a granted world is read", world: "published", gate: fakeGate{}},
		{name: "the generated default world needs no grant", world: metamodel.DefaultWorldName,
			gate: fakeGate{denyWorlds: map[string]bool{"published": true}}, wantDefault: true},
		{
			name: "a denied world is refused", world: "published",
			gate: fakeGate{denyWorlds: map[string]bool{"published": true}}, wantErr: "not readable",
		},
		{name: "an undeclared world is refused", world: "nope", gate: fakeGate{}, wantErr: "no such world"},
		{name: "a gate failure is an error", world: "published", gate: fakeGate{worldErr: gateDown}, wantIs: gateDown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := appWithDefaultWorld(t, tc.configured)
			ctx := withReadGate(context.Background(), tc.gate)

			got, err := mcpHost(a).SelectWorld(ctx, tc.world)
			switch {
			case tc.wantIs != nil:
				if !errors.Is(err, tc.wantIs) {
					t.Fatalf("err = %v, want %v", err, tc.wantIs)
				}
				return
			case tc.wantErr != "":
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want one containing %q", err, tc.wantErr)
				}
				return
			case err != nil:
				t.Fatalf("SelectWorld: %v", err)
			}
			if got.IsTrivial() != tc.wantDefault {
				t.Errorf("trivial world = %v, want %v", got.IsTrivial(), tc.wantDefault)
			}
		})
	}
}

func TestMCPWorldReadable(t *testing.T) {
	t.Parallel()
	gateDown := errors.New("store is down")
	tests := []struct {
		name    string
		world   string
		gate    readGate
		want    bool
		wantErr error
	}{
		{name: "default", world: metamodel.DefaultWorldName, gate: fakeGate{denyWorlds: map[string]bool{"published": true}}, want: true},
		{name: "granted", world: "published", gate: fakeGate{}, want: true},
		{name: "denied", world: "published", gate: fakeGate{denyWorlds: map[string]bool{"published": true}}},
		{name: "undeclared", world: "nope", gate: fakeGate{}},
		{name: "gate failure", world: "published", gate: fakeGate{worldErr: gateDown}, wantErr: gateDown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := appWithDefaultWorld(t, "")
			got, err := mcpHost(a).WorldReadable(withReadGate(context.Background(), tc.gate), tc.world)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("readable = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMCPHost_DefaultWorldName(t *testing.T) {
	t.Parallel()
	for configured, want := range map[string]string{"": metamodel.DefaultWorldName, "published": "published"} {
		if got := mcpHost(appWithDefaultWorld(t, configured)).DefaultWorld(); got != want {
			t.Errorf("configured %q: DefaultWorld() = %q, want %q", configured, got, want)
		}
	}
}
