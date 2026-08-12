package renametype_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/renametype"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// TestRenameTypeWritesBackToLegacyName pins the reason Context carries a
// RESOLVED schema path rather than a bool: rename-type edits the schema file
// in place. If it rejoined project.SchemaFile instead of using ctx.SchemaPath,
// a legacy project would keep its stale metamodel.yaml and grow a second,
// partial schema.yaml — which discovery then prefers, so the rename would
// appear to have destroyed every other type.
//
// Discovery is exercised for real here (not a hand-built Context) because the
// guarantee under test is precisely that the two agree.
func TestRenameTypeWritesBackToLegacyName(t *testing.T) {
	fs := storage.NewMemFS()
	root := "/proj"
	legacy := filepath.Join(root, project.LegacySchemaFile)

	// Reuses the shared fixture so this test tracks the real schema shape.
	const schema = renametypeTestMetamodel
	if err := fs.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fs.WriteFile(legacy, []byte(schema), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, err := project.Discover(root, fs)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if !ctx.SchemaIsLegacy {
		t.Fatalf("expected a legacy-named project, got SchemaPath=%s", ctx.SchemaPath)
	}

	meta, err := metamodel.Parse([]byte(schema))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	svc, err := renametype.New(renametype.Deps{FS: fs, Meta: meta, Paths: ctx})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if _, renameErr := svc.Rename("ticket", "issue", "issues"); renameErr != nil {
		t.Fatalf("rename: %v", renameErr)
	}

	// The edit must land in the file that already existed...
	got, err := fs.ReadFile(legacy)
	if err != nil {
		t.Fatalf("legacy schema disappeared: %v", err)
	}
	if !strings.Contains(string(got), "issue:") {
		t.Error("rename did not update the legacy schema file")
	}
	if strings.Contains(string(got), "ticket:") {
		t.Error("legacy schema still contains the old type name")
	}

	// ...and must NOT create a competing schema.yaml, which discovery would
	// then prefer over the real (legacy) file.
	if _, err := fs.Stat(filepath.Join(root, project.SchemaFile)); err == nil {
		t.Fatal("rename-type created a stray schema.yaml next to metamodel.yaml")
	}
}
