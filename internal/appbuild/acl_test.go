package appbuild_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// testFS returns an in-memory FS + project context suitable for
// appbuild.NewForTest tests that exercise the EntityManager
// templater path.
func testFS(t *testing.T) (storage.FS, *project.Context) {
	t.Helper()
	fs := storage.NewMemFS()
	paths := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	if err := fs.MkdirAll(paths.CacheDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	return fs, paths
}

// AC1.8 (production wiring): an `appbuild.WithACL(acl.ReadOnlyACL{})`
// option produces a Services whose EntityManager refuses every write.
// This is the seam `rela-server --read-only` plugs into; the flag
// itself is tested at the integration level (see cmd/rela-server)
// because flag parsing in main() is excluded from coverage.
func TestWithACL_ReadOnlyDeniesWrites(t *testing.T) {
	meta, err := metamodel.Parse([]byte(`version: "1.0"
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
`))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}

	fs, paths := testFS(t)
	svc := appbuild.NewForTest(meta,
		appbuild.WithFS(fs, paths),
		appbuild.WithTestACL(acl.ReadOnlyACL{}),
	)
	defer svc.Close()

	e := entity.New("", "ticket")
	e.SetString("title", "Should be denied")
	_, err = svc.EntityManager().CreateEntity(context.Background(), e, entity.CreateOptions{})

	if err == nil {
		t.Fatal("CreateEntity returned nil error under ReadOnlyACL; want forbidden")
	}
	if !errors.Is(err, acl.ErrForbidden) {
		t.Errorf("errors.Is(err, ErrForbidden) = false, want true. err = %v", err)
	}
	var fe *acl.ForbiddenError
	if !errors.As(err, &fe) {
		t.Fatalf("errors.As(err, *ForbiddenError) = false. err = %v", err)
	}
	if fe.Decision.RuleKind != "read-only" {
		t.Errorf("RuleKind = %q, want %q", fe.Decision.RuleKind, "read-only")
	}
}

// AC1.4 / default: omitting WithTestACL produces a NopACL-backed
// Services where writes succeed. Pins backwards-compat: existing
// tests that don't think about ACL keep working.
func TestNoACLOption_DefaultsToAllowAll(t *testing.T) {
	meta, err := metamodel.Parse([]byte(`version: "1.0"
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
`))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}

	fs, paths := testFS(t)
	svc := appbuild.NewForTest(meta, appbuild.WithFS(fs, paths))
	defer svc.Close()

	e := entity.New("", "ticket")
	e.SetString("title", "Allowed")
	if _, err := svc.EntityManager().CreateEntity(context.Background(), e, entity.CreateOptions{}); err != nil {
		t.Fatalf("CreateEntity under default NopACL: %v", err)
	}
}
