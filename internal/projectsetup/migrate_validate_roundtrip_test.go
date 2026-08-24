package projectsetup_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/projectsetup"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// TestMigrateThenValidate_RoundTrip asserts the general contract that `rela
// migrate` must never rewrite a valid project into an invalid one.
//
// This is a guard for every migration, not just the one that motivated it: a
// migration edits config, and `rela validate` (plus rela-server startup, which
// runs the same dataentryconfig.ValidateConfig) decides whether that config is
// loadable. Nothing structurally couples the two, so a migration can strip a key
// the validator requires. That is exactly what the dataentry-cleanup migration
// did to `direction: incoming` — and because migrate is idempotent, re-running
// it reported "No migrations needed" instead of repairing the damage, leaving a
// project that could not start.
func TestMigrateThenValidate_RoundTrip(t *testing.T) {
	// A to-side form binding: `category` is only ever the TO of belongs-to, so
	// the binding needs an explicit `direction: incoming` to be valid.
	const schema = `version: "1.0"
namespace: "https://example.com/test#"
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    id_type: manual
    properties:
      title:
        type: string
  category:
    label: Category
    id_prefix: CAT
    id_type: manual
    properties:
      title:
        type: string
relations:
  belongs-to:
    label: belongs to
    from: [ticket]
    to: [category]
`

	const dataEntry = `forms:
  edit_category:
    entity_type: category
    title: Edit category
    fields:
      - property: title
    relations:
      - relation: belongs-to
        direction: incoming
        widget: cards
`

	fs := storage.NewMemFS()
	root := "/proj"
	if _, err := projectsetup.InitializeWithFS(root, fs); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := fs.WriteFile(filepath.Join(root, "schema.yaml"), []byte(schema), 0o644); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	if err := fs.WriteFile(filepath.Join(root, "data-entry.yaml"), []byte(dataEntry), 0o644); err != nil {
		t.Fatalf("write data-entry: %v", err)
	}

	// Precondition: the project is valid before migrating. If this fails the
	// test fixture is wrong, not the migration.
	before, err := projectsetup.ValidateWithFS(root, fs)
	if err != nil {
		t.Fatalf("validate before: %v", err)
	}
	if before.HasErrors() {
		t.Fatalf("fixture is not valid before migrate: metamodel=%v data-entry=%v",
			before.MetamodelError, before.DataEntryError)
	}

	if _, migrateErr := projectsetup.MigrateWithFS(root, fs); migrateErr != nil {
		t.Fatalf("migrate: %v", migrateErr)
	}

	after, err := projectsetup.ValidateWithFS(root, fs)
	if err != nil {
		t.Fatalf("validate after: %v", err)
	}
	if after.HasErrors() {
		migrated, _ := fs.ReadFile(filepath.Join(root, "data-entry.yaml"))
		t.Errorf("migrate turned a valid project into an invalid one.\n"+
			"metamodel error: %v\ndata-entry error: %v\nmigrated data-entry.yaml:\n%s",
			after.MetamodelError, after.DataEntryError, migrated)
	}

	// Idempotence. The original bug's worst property was not the corruption
	// itself but that re-running the tool reported "No migrations needed" while
	// leaving the damage — so a second pass must be a fixed point, for every
	// registered migration, not just the one this fixture exercises.
	firstPass, err := fs.ReadFile(filepath.Join(root, "data-entry.yaml"))
	if err != nil {
		t.Fatalf("read after first migrate: %v", err)
	}
	if _, secondErr := projectsetup.MigrateWithFS(root, fs); secondErr != nil {
		t.Fatalf("second migrate: %v", secondErr)
	}
	secondPass, err := fs.ReadFile(filepath.Join(root, "data-entry.yaml"))
	if err != nil {
		t.Fatalf("read after second migrate: %v", err)
	}
	if !bytes.Equal(firstPass, secondPass) {
		t.Errorf("migrate is not idempotent — a second pass changed the file.\nfirst:\n%s\nsecond:\n%s",
			firstPass, secondPass)
	}
}
