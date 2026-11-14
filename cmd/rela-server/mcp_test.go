package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// Without -mcp, wireRemoteMCP does nothing at all — no handler is built, and
// crucially SetRemoteMCP is never called, so an upgraded server that does not
// opt in cannot fail to start because of MCP (AC #2).
func TestWireRemoteMCP_DisabledIsANoop(t *testing.T) {
	// A zero App would panic if wireRemoteMCP touched it; that it does not is
	// the assertion. No JWT gate is configured either, so if the flag were
	// ignored this would return the SetRemoteMCP refusal instead of nil.
	if err := wireRemoteMCP(nil, nil, &serverFlags{}); err != nil {
		t.Fatalf("wireRemoteMCP with -mcp unset = %v, want nil (and no App access)", err)
	}
}

// AC #6 at the wiring layer: -mcp without the JWT identity flags must stop
// startup. The refusal itself lives in dataentry.SetRemoteMCP (and is tested
// there); this pins that rela-server actually surfaces it rather than logging
// and continuing, which would serve a CSRF-exempt endpoint with no
// authentication.
func TestWireRemoteMCP_WithoutJWTGateIsRefused(t *testing.T) {
	app := newBareApp(t)

	err := wireRemoteMCP(app, nil, &serverFlags{remoteMCP: toggleOn})
	if err == nil {
		t.Fatal("wireRemoteMCP() error = nil, want the missing-JWT refusal " +
			"propagated to the caller so startup can exit")
	}
	if !strings.Contains(err.Error(), "verified JWT identity") {
		t.Errorf("error = %q, want it to name the missing requirement", err)
	}
}

// newBareApp builds the minimum App that SetRemoteMCP can be called against.
// It needs no store or metamodel: the JWT-gate check runs before the factory,
// which is exactly what these tests exercise.
func newBareApp(t *testing.T) *dataentry.App {
	t.Helper()
	return &dataentry.App{}
}

// TestResolveRemoteMCP covers every combination of the three sources: the
// endpoint is served only when none turns it off and at least one turns it
// on, and a conflict names the source that turned it off.
func TestResolveRemoteMCP(t *testing.T) {
	t.Parallel()

	says := []toggle{toggleUnset, toggleOn, toggleOff}
	label := map[toggle]string{toggleUnset: "unset", toggleOn: "on", toggleOff: "off"}
	for _, cfg := range says {
		for _, env := range says {
			for _, fl := range says {
				all := []toggle{cfg, env, fl}
				t.Run(fmt.Sprintf("config=%s/env=%s/flag=%s", label[cfg], label[env], label[fl]), func(t *testing.T) {
					t.Parallel()
					anyOn, firstOff := false, ""
					names := []string{"config", "env", "flag"}
					for i, s := range all {
						anyOn = anyOn || s == toggleOn
						if s == toggleOff && firstOff == "" {
							firstOff = names[i]
						}
					}
					wantOn := anyOn && firstOff == ""
					wantBy := ""
					if anyOn && firstOff != "" {
						wantBy = firstOff
					}

					on, by := resolveRemoteMCP(
						mcpSource{"config", cfg}, mcpSource{"env", env}, mcpSource{"flag", fl})
					if on != wantOn || by != wantBy {
						t.Errorf("resolveRemoteMCP = (%v, %q), want (%v, %q)", on, by, wantOn, wantBy)
					}
				})
			}
		}
	}
}

func TestEnvToggle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in      string
		want    toggle
		wantErr bool
	}{
		{in: "", want: toggleUnset},
		{in: "1", want: toggleOn},
		{in: "0", want: toggleOff},
		{in: "true", wantErr: true},
		{in: "yes", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got, err := envToggle(tc.in)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Errorf("envToggle(%q) = (%v, %v), want (%v, error %v)", tc.in, got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestToggleFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		args    []string
		want    toggle
		wantErr bool
	}{
		{args: nil, want: toggleUnset},
		{args: []string{"-mcp"}, want: toggleOn},
		{args: []string{"-mcp=true"}, want: toggleOn},
		{args: []string{"-mcp=false"}, want: toggleOff},
		{args: []string{"-mcp=sure"}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			t.Parallel()
			var got toggle
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.SetOutput(io.Discard)
			fs.Var(toggleFlag{&got}, "mcp", "")
			err := fs.Parse(tc.args)
			if (err != nil) != tc.wantErr || (!tc.wantErr && got != tc.want) {
				t.Errorf("parse %v = (%v, %v), want (%v, error %v)", tc.args, got, err, tc.want, tc.wantErr)
			}
		})
	}
}

// writeRemoteMCPConfig writes body as the project's .rela/config.yaml and
// returns its paths.
func writeRemoteMCPConfig(t *testing.T, body string) *project.Context {
	t.Helper()
	cache := filepath.Join(t.TempDir(), ".rela")
	if err := os.MkdirAll(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	if body != "" {
		if err := os.WriteFile(filepath.Join(cache, project.ConfigFile), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return &project.Context{CacheDir: cache}
}

func TestPlanRemoteMCP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		config    string
		flag      toggle
		env       string
		wantServe bool
		wantOpts  int
		wantErr   string
	}{
		{name: "nothing set", wantServe: false},
		{name: "flag alone", flag: toggleOn, wantServe: true},
		{name: "env alone", env: "1", wantServe: true},
		{name: "config alone", config: "remote_mcp:\n  enabled: true\n", wantServe: true},
		{name: "config off beats flag on", config: "remote_mcp:\n  enabled: false\n", flag: toggleOn},
		{name: "config off beats env on", config: "remote_mcp:\n  enabled: false\n", env: "1"},
		{name: "env off beats config on", config: "remote_mcp:\n  enabled: true\n", env: "0"},
		{name: "flag off beats config on", config: "remote_mcp:\n  enabled: true\n", flag: toggleOff},
		{
			name:      "tool lists become a filter",
			config:    "remote_mcp:\n  enabled: true\n  tools:\n    deny: [attach_file]\n",
			wantServe: true, wantOpts: 1,
		},
		{
			name:   "tool lists without the endpoint do nothing",
			config: "remote_mcp:\n  tools:\n    deny: [attach_file]\n",
		},
		{name: "unknown env value stops startup", env: "true", wantErr: "RELA_MCP"},
		{name: "misspelled config key stops startup", config: "remote_mcp:\n  enable: false\n", flag: toggleOn,
			wantErr: "config.yaml"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			paths := writeRemoteMCPConfig(t, tc.config)
			plan, err := planRemoteMCP(&serverFlags{remoteMCP: tc.flag, remoteMCPEnv: tc.env}, paths)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("planRemoteMCP error = %v, want one naming %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("planRemoteMCP: %v", err)
			}
			if plan.serve != tc.wantServe || len(plan.opts) != tc.wantOpts {
				t.Errorf("plan = serve %v with %d options, want serve %v with %d",
					plan.serve, len(plan.opts), tc.wantServe, tc.wantOpts)
			}
		})
	}
}
