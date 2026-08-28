package appbuild_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/app"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// minimalMetamodel is just enough YAML to make appbuild.New succeed.
// PR 3's tests exercise the ACL wiring, not the metamodel.
const minimalMetamodel = `version: "1.0"
entities:
  ticket:
    label: Ticket
    plural: tickets
    id_prefix: "TKT-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
relations: {}
`

// AC3.1: acl.yaml at the project root → loaded into a Declarative.
func TestDiscover_ACLPresent_LoadsDeclarative(t *testing.T) {
	root := t.TempDir()
	writeMetamodel(t, root)
	writePolicy(t, root, `roles:
  admin:
    create: ["*"]
    update: ["*"]
    delete: ["*"]
    read: ["*"]
assignments:
  jeroen: admin
`)

	svc, err := appbuildOnDisk(t, root)
	if err != nil {
		t.Fatalf("appbuild.New: %v", err)
	}
	defer svc.Close()

	if _, ok := svc.ACL().(*acl.Declarative); !ok {
		t.Errorf("svc.ACL() is %T, want *acl.Declarative", svc.ACL())
	}
}

// AC3.1: missing acl.yaml → NopACL fallback so existing projects
// remain backwards-compatible.
func TestDiscover_ACLMissing_UsesNop(t *testing.T) {
	root := t.TempDir()
	writeMetamodel(t, root)
	// no acl.yaml

	svc, err := appbuildOnDisk(t, root)
	if err != nil {
		t.Fatalf("appbuild.New: %v", err)
	}
	defer svc.Close()

	if _, ok := svc.ACL().(acl.NopACL); !ok {
		t.Errorf("svc.ACL() is %T, want acl.NopACL", svc.ACL())
	}
}

// AC3.2: WithACL(ReadOnlyACL{}) wins over a loaded policy. This is
// how `rela-server --read-only` overrides the project's acl.yaml.
func TestWithACL_OverridesLoadedPolicy(t *testing.T) {
	root := t.TempDir()
	writeMetamodel(t, root)
	writePolicy(t, root, `roles:
  admin:
    create: ["*"]
    update: ["*"]
    delete: ["*"]
    read: ["*"]
assignments:
  jeroen: admin
`)

	svc, err := appbuildOnDiskWithOpts(t, root, appbuild.WithACL(acl.ReadOnlyACL{}))
	if err != nil {
		t.Fatalf("appbuild.New: %v", err)
	}
	defer svc.Close()

	if _, ok := svc.ACL().(acl.ReadOnlyACL); !ok {
		t.Errorf("svc.ACL() is %T, want acl.ReadOnlyACL", svc.ACL())
	}
}

// Malformed acl.yaml → boot fails loud (RR-72OJ). A parse-error
// fallback to NopACL would silently invert the operator's intent
// (writing a policy means "enforce something"); booting allow-all
// on a typo is a security regression. Operator sees the error
// immediately, fixes the file, retries.
func TestDiscover_MalformedACL_FailsBoot(t *testing.T) {
	root := t.TempDir()
	writeMetamodel(t, root)
	writePolicy(t, root, "roles:\n  admin:\n    create: [not-closed\n")

	svc, err := appbuildOnDiskNoBackend(t, root)
	if err == nil {
		svc.Close()
		t.Fatalf("appbuild.New: expected error on malformed acl.yaml, got nil")
	}
	if !strings.Contains(err.Error(), "acl.yaml") {
		t.Errorf("error should mention acl.yaml so the operator can find the file; got %q", err.Error())
	}
}

// principalPropertyMetamodel declares a persoon type with a unique email
// and a non-unique nickname, for the principal_property boot checks.
const principalPropertyMetamodel = `version: "1.0"
entities:
  persoon:
    label: Persoon
    plural: personen
    id_type: manual
    properties:
      email:
        type: string
        unique: true
      nickname:
        type: string
relations: {}
`

// principal_property referencing a non-unique property must fail boot —
// a non-unique identity key admits duplicates and makes resolution
// ambiguous (the ValidateAgainstMetamodel gate, wired via buildACL).
func TestDiscover_PrincipalPropertyNotUnique_FailsBoot(t *testing.T) {
	root := t.TempDir()
	writeMetamodelBody(t, root, principalPropertyMetamodel)
	writePolicy(t, root, "user_entity_type: persoon\nprincipal_property: nickname\n")

	svc, err := appbuildOnDisk(t, root)
	if err == nil {
		svc.Close()
		t.Fatalf("expected boot to fail on non-unique principal_property, got nil")
	}
	if !strings.Contains(err.Error(), "unique") {
		t.Errorf("error should explain the uniqueness requirement; got %q", err.Error())
	}
}

// principal_property referencing an unknown property must fail boot.
func TestDiscover_PrincipalPropertyUnknown_FailsBoot(t *testing.T) {
	root := t.TempDir()
	writeMetamodelBody(t, root, principalPropertyMetamodel)
	writePolicy(t, root, "user_entity_type: persoon\nprincipal_property: ghost\n")

	svc, err := appbuildOnDiskNoBackend(t, root)
	if err == nil {
		svc.Close()
		t.Fatalf("expected boot to fail on unknown principal_property, got nil")
	}
}

// principal_property referencing a unique property boots successfully.
func TestDiscover_PrincipalPropertyUnique_Boots(t *testing.T) {
	root := t.TempDir()
	writeMetamodelBody(t, root, principalPropertyMetamodel)
	writePolicy(t, root, "user_entity_type: persoon\nprincipal_property: email\n"+
		"roles:\n  everyone:\n    read: [\"*\"]\n")

	svc, err := appbuildOnDisk(t, root)
	if err != nil {
		t.Fatalf("expected boot to succeed with unique principal_property, got %v", err)
	}
	svc.Close()
}

// --- helpers ---

func writeMetamodel(t *testing.T, root string) {
	t.Helper()
	writeMetamodelBody(t, root, minimalMetamodel)
}

func writeMetamodelBody(t *testing.T, root, body string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metamodel.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".rela", "audit"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "entities"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "relations"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func writePolicy(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "acl.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// appbuildOnDisk builds a Services bundle from a real on-disk project
// rooted at root. Uses appbuild.New directly so we can supply our own
// audit sink without disturbing the test filesystem.
//
// The backend comes from backendtest, so on the postgres build this needs — and
// waits for — a database. Use [appbuildOnDiskNoBackend] for a case that asserts
// boot FAILS before any store is opened; it must not be gated on a database it
// never reaches.
func appbuildOnDisk(t *testing.T, root string) (*appbuild.Services, error) {
	t.Helper()
	return appbuildOnDiskWithOpts(t, root)
}

func appbuildOnDiskWithOpts(t *testing.T, root string, opts ...appbuild.Option) (*appbuild.Services, error) {
	t.Helper()
	// backendtest.DSN supplies whatever the current build needs to reach a store
	// — "" on fs/memory, a private migrated schema on postgres. These are
	// ACL-wiring assertions, not backend assertions, so they must not hard-code
	// one.
	//
	// DSN rather than Options because this path calls appbuild.New, which reads
	// Config.DatabaseURL and ignores WithDatabaseURL by design.
	return newOnDisk(t, backendtest.DSN(t), root, opts...)
}

// appbuildOnDiskNoBackend builds without asking backendtest for a backend.
//
// The config-validation gates (malformed acl.yaml, an unknown
// principal_property) reject the project BEFORE the recipe opens a store, so
// they assert the same thing on every build with no database present. Routing
// them through appbuildOnDisk would turn genuine coverage into a skip whenever
// the postgres build runs without a server — the tests would still be green,
// but they would no longer be checking anything.
func appbuildOnDiskNoBackend(t *testing.T, root string) (*appbuild.Services, error) {
	t.Helper()
	return newOnDisk(t, "", root)
}

// newOnDisk builds a Services over the project at root. dsn is empty on the
// fs/memory builds, where the recipe ignores Config.DatabaseURL entirely.
func newOnDisk(t *testing.T, dsn, root string, opts ...appbuild.Option) (*appbuild.Services, error) {
	t.Helper()
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(root, fs)
	if err != nil {
		return nil, err
	}
	// Use audit.Nop so tests don't write JSONL files into the temp
	// directory and pollute the next run.
	_ = app.FSFactory{} // silence the import; required for the package boundary check
	return appbuild.New(appbuild.Config{
		FS:           fs,
		Paths:        paths,
		ScriptEngine: script.NewEngine(),
		Audit:        audit.Nop{},
		DatabaseURL:  dsn,
	}, opts...)
}
