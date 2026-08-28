package appbuild_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
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

// discover is appbuild.Discover with whatever the current build needs to reach
// a store. The tests below assert build-agnostic composition-root behavior, so
// they must not hard-code a backend; on the postgres build backendtest supplies
// a private migrated schema (and skips when no database is configured).
func discover(t *testing.T, root string) (*appbuild.Services, error) {
	t.Helper()
	return appbuild.Discover(root, script.NewEngine(), backendtest.Options(t)...)
}

// discoverer returns a discover func pinned to ONE backend, for a test that
// builds twice over the same project and needs the second build to observe what
// the first wrote.
//
// Calling discover twice would not do that on postgres: backendtest hands out a
// fresh schema per call, so the second build would come up empty and the test
// would assert against state it never planted.
func discoverer(t *testing.T) func(string) (*appbuild.Services, error) {
	t.Helper()
	backend := backendtest.Options(t)
	return func(root string) (*appbuild.Services, error) {
		return appbuild.Discover(root, script.NewEngine(), backend...)
	}
}

// TestDiscover_BuildsAllServices verifies that appbuild.Discover
// returns a Services with every field populated.
func TestDiscover_BuildsAllServices(t *testing.T) {
	root := writeMinimalProject(t)
	svc, err := discover(t, root)
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
	svc, err := discover(t, root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer svc.Close()

	read := svc.LuaReadDeps()
	if read.VisibleReader == nil || read.Tracer == nil || read.Meta == nil {
		t.Errorf("LuaReadDeps incomplete: %+v", read)
	}
	// The nil check above no longer proves the reader is wired to a store:
	// visibility.Unrestricted always returns a non-nil value, so a bundle
	// built over a nil store would still pass it (TKT-1WV50C). Assert the
	// reader actually reaches the store instead.
	if _, err := read.VisibleReader.GetEntity(t.Context(), "does-not-exist"); err == nil {
		t.Error("LuaReadDeps.VisibleReader did not reach a real store: " +
			"a missing entity should surface a not-found error")
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
	svc, err := discover(t, root)
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
	svc, err := discover(t, root)
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
			return appbuild.New(appbuild.Config{
				Paths: svc.Paths(), ScriptEngine: script.NewEngine(), Audit: audit.Nop{},
			})
		}, "Config.FS is required"},
		{"nil paths", func() (*appbuild.Services, error) {
			return appbuild.New(appbuild.Config{
				FS: svc.FS(), ScriptEngine: script.NewEngine(), Audit: audit.Nop{},
			})
		}, "Config.Paths is required"},
		{"nil engine", func() (*appbuild.Services, error) {
			return appbuild.New(appbuild.Config{
				FS: svc.FS(), Paths: svc.Paths(), Audit: audit.Nop{},
			})
		}, "Config.ScriptEngine is required"},
		{"nil audit", func() (*appbuild.Services, error) {
			return appbuild.New(appbuild.Config{
				FS: svc.FS(), Paths: svc.Paths(), ScriptEngine: script.NewEngine(),
			})
		}, "Config.Audit is required"},
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

// TestCorruptAliasTableDoesNotBrickTheBinary: the alias table lives in the
// gitignored cache dir, and it is built on EVERY appbuild path — `rela list`,
// `analyze`, the MCP server, the desktop app — almost none of which serve
// CalDAV.
//
// Treating a corrupt table as fatal there meant one truncated cache file killed
// every command on a project with no `caldav:` block at all, naming a subsystem
// the user had never enabled. Reproduced with `rela list` before this test.
//
// The fail-loud rule still holds where it matters: registerCalDAVRoutes refuses
// to mount without a healthy table, so a synced client cannot re-create its
// entries as new entities. The failure just lands on the path that can actually
// cause that damage.
// The table is corrupted through the state.KV rather than by writing the file
// directly. Where it LIVES is backend-specific — a file under .rela/caldav/ on
// the fs build, a state_kv row on postgres (TKT-VC27L3) — but the key is the
// same on both, so going through the KV asserts the degrade-don't-die behavior
// on whichever backend is compiled in. Writing the file instead made this a
// filesystem test wearing a wiring test's name: under -tags postgres it
// corrupted a file nothing reads, the alias table loaded clean, and the
// assertion failed.
func TestCorruptAliasTableDoesNotBrickTheBinary(t *testing.T) {
	root := writeMinimalProject(t)
	build := discoverer(t) // both builds must share one backend

	// First build: obtain the state store this project resolves to, and plant
	// the corrupt table in it.
	seed, err := build(root)
	if err != nil {
		t.Fatalf("seed build: %v", err)
	}
	const aliasKey = "caldav/aliases.json" // caldavalias.stateKey
	if err = seed.State().Put(t.Context(), aliasKey, []byte("this is not json")); err != nil {
		t.Fatalf("plant corrupt table: %v", err)
	}
	if err = seed.Close(); err != nil {
		t.Fatalf("close seed: %v", err)
	}

	// Second build over the same project now reads the corrupt table.
	svc, err := build(root)
	if err != nil {
		t.Fatalf("a corrupt CalDAV alias table must not fail project build: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	// Degraded, not silently healthy: nil is the signal registerCalDAVRoutes
	// keys on to refuse to serve.
	if svc.CalDAVAliases() != nil {
		t.Error("alias service is non-nil after a corrupt load; CalDAV would " +
			"serve from an empty table and duplicate every synced entry")
	}
}
