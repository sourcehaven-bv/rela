//go:build !postgres

package dataentrywire

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

var flowToolNames = []string{"list_flow_tasks", "show_flow_run", "complete_flow_step"}

// flowServices builds the onboarding sample of flowcfg's testdata.
func flowServices(t *testing.T) *appbuild.Services {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"entities", "relations", filepath.Join(".rela", "audit")} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for src, dst := range map[string]string{"schema.yaml": "metamodel.yaml", "acl.yaml": "acl.yaml",
		"flows.yaml": "flows.yaml"} {
		data, err := os.ReadFile(filepath.Join("..", "flowcfg", "testdata", src))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, dst), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(root, fs)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	svc, err := appbuild.New(appbuild.Config{
		FS: fs, Paths: paths, ScriptEngine: script.NewEngine(), Audit: audit.Nop{},
	})
	if err != nil {
		t.Fatalf("appbuild.New: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

// TestRemoteMCPDeps_Flows pins that the remote server lists the flow tools
// for a project with flows, and none without.
func TestRemoteMCPDeps_Flows(t *testing.T) {
	names := remoteToolNames(t, flowServices(t))
	for _, name := range flowToolNames {
		if !slices.Contains(names, name) {
			t.Errorf("remote MCP does not list %s with flows.yaml", name)
		}
	}
	_, plain := channelsServices(t, "")
	names = remoteToolNames(t, plain)
	for _, name := range flowToolNames {
		if slices.Contains(names, name) {
			t.Errorf("remote MCP lists %s without flows.yaml", name)
		}
	}
}
