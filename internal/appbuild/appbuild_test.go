package appbuild_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// minimalProject writes a sufficient project layout (metamodel.yaml +
// .rela cache dir + entities/ + relations/) into a fresh tempdir so
// appbuild.Discover has something to discover.
const metamodelYAML = `version: "1.0"
entities:
  doc:
    label: Doc
    plural: docs
    id_prefix: "DOC-"
    id_type: sequential
    properties:
      title:
        type: string
`

func writeMinimalProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustWrite := func(rel, content string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("metamodel.yaml", metamodelYAML)
	if err := os.MkdirAll(filepath.Join(root, ".rela"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "entities", "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "relations"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestDiscover_BuildsAllServices verifies that appbuild.Discover
// returns a Services with every field populated.
func TestDiscover_BuildsAllServices(t *testing.T) {
	root := writeMinimalProject(t)
	svc, err := appbuild.Discover(root, script.NewEngine())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer svc.Close()

	if svc.FS() == nil {
		t.Error("FS is nil")
	}
	if svc.Paths() == nil {
		t.Error("Paths is nil")
	}
	if svc.Meta() == nil {
		t.Error("Meta is nil")
	}
	if svc.Store() == nil {
		t.Error("Store is nil")
	}
	if svc.Searcher() == nil {
		t.Error("Searcher is nil")
	}
	if svc.EntityManager() == nil {
		t.Error("EntityManager is nil")
	}
	if svc.Tracer() == nil {
		t.Error("Tracer is nil")
	}
	if svc.Validator() == nil {
		t.Error("Validator is nil")
	}
	if svc.Templater() == nil {
		t.Error("Templater is nil")
	}
	if svc.Config() == nil {
		t.Error("Config is nil")
	}
	if svc.State() == nil {
		t.Error("State is nil")
	}
}

// TestDiscover_LuaDepsDerivable verifies LuaReadDeps/LuaWriteDeps
// produce non-empty bundles from the focused services.
func TestDiscover_LuaDepsDerivable(t *testing.T) {
	root := writeMinimalProject(t)
	svc, err := appbuild.Discover(root, script.NewEngine())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer svc.Close()

	read := svc.LuaReadDeps()
	if read.Store == nil || read.Tracer == nil || read.Meta == nil {
		t.Errorf("LuaReadDeps incomplete: %+v", read)
	}
	if read.ProjectRoot == "" {
		t.Error("LuaReadDeps.ProjectRoot is empty")
	}

	write := svc.LuaWriteDeps()
	if write.EntityManager == nil {
		t.Error("LuaWriteDeps.EntityManager is nil")
	}
}

// TestDiscover_MissingProject returns a clear error when startDir
// doesn't contain a project.
func TestDiscover_MissingProject(t *testing.T) {
	_, err := appbuild.Discover(t.TempDir(), script.NewEngine())
	if err == nil {
		t.Fatal("expected error for missing project")
	}
}

// TestClose_Idempotent confirms repeated Close calls are safe — the
// underlying bleve index is closed exactly once, second-and-later
// invocations are no-ops.
func TestClose_Idempotent(t *testing.T) {
	root := writeMinimalProject(t)
	svc, err := appbuild.Discover(root, script.NewEngine())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if err := svc.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := svc.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
	if err := svc.Close(); err != nil {
		t.Errorf("third Close: %v", err)
	}
}

// TestNew_RejectsNilDeps pins the constructor's nil-rejection.
func TestNew_RejectsNilDeps(t *testing.T) {
	root := writeMinimalProject(t)
	svc, err := appbuild.Discover(root, script.NewEngine())
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	defer svc.Close()

	cases := []struct {
		name string
		call func() (*appbuild.Services, error)
		want string
	}{
		{"nil fs", func() (*appbuild.Services, error) {
			return appbuild.New(nil, svc.Paths(), script.NewEngine(), audit.Nop{})
		}, "fs is required"},
		{"nil paths", func() (*appbuild.Services, error) {
			return appbuild.New(svc.FS(), nil, script.NewEngine(), audit.Nop{})
		}, "paths is required"},
		{"nil engine", func() (*appbuild.Services, error) {
			return appbuild.New(svc.FS(), svc.Paths(), nil, audit.Nop{})
		}, "scriptEngine is required"},
		{"nil audit", func() (*appbuild.Services, error) {
			return appbuild.New(svc.FS(), svc.Paths(), script.NewEngine(), nil)
		}, "auditSink is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.call()
			if err == nil {
				t.Fatalf("expected error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want substring %q", err, tc.want)
			}
		})
	}
}
