package projectsetup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func testFS() storage.FS { return storage.NewSafeFS(storage.NewOsFS()) }

// TestInitializeRefusesExistingSchema is the regression for the data-loss bug
// found in design review (RR-NRFBZE). A guard that stats only schema.yaml would
// see a legacy project as an empty directory and write a DEFAULT schema.yaml
// beside the operator's real metamodel.yaml. Since discovery prefers the new
// name, the project would then come up on the empty default with the real
// schema silently ignored — every entity failing type resolution.
//
// The assertion that no schema.yaml was created is the load-bearing half: an
// error alone would not prove nothing was written.
func TestInitializeRefusesExistingSchema(t *testing.T) {
	tests := []struct {
		name        string
		existing    string
		wantInError string
	}{
		{
			name:        "legacy name points at rela migrate",
			existing:    project.LegacySchemaFile,
			wantInError: "rela migrate",
		},
		{
			name:        "new name reports already initialized",
			existing:    project.SchemaFile,
			wantInError: "already initialized",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			original := "version: '1.0'\nentities:\n  ticket:\n    properties: {}\n"
			if err := os.WriteFile(filepath.Join(dir, tc.existing), []byte(original), 0o644); err != nil {
				t.Fatal(err)
			}

			_, err := InitializeWithFS(dir, testFS())
			if err == nil {
				t.Fatal("InitializeWithFS must refuse a directory that already holds a schema file")
			}
			if !strings.Contains(err.Error(), tc.wantInError) {
				t.Errorf("error %q should mention %q", err, tc.wantInError)
			}

			// The operator's file must be untouched...
			got, readErr := os.ReadFile(filepath.Join(dir, tc.existing))
			if readErr != nil {
				t.Fatalf("existing schema was removed: %v", readErr)
			}
			if string(got) != original {
				t.Error("existing schema file was overwritten")
			}

			// ...and no second schema file may appear next to it.
			if tc.existing == project.LegacySchemaFile {
				if _, statErr := os.Stat(filepath.Join(dir, project.SchemaFile)); statErr == nil {
					t.Fatal("wrote a default schema.yaml alongside a legacy metamodel.yaml")
				}
			}
		})
	}
}

// TestInitializeCreatesNewName pins that fresh projects get the new filename.
func TestInitializeCreatesNewName(t *testing.T) {
	dir := t.TempDir()

	result, err := InitializeWithFS(dir, testFS())
	if err != nil {
		t.Fatalf("InitializeWithFS: %v", err)
	}
	if filepath.Base(result.SchemaPath) != project.SchemaFile {
		t.Errorf("SchemaPath = %s, want %s", result.SchemaPath, project.SchemaFile)
	}
	if _, err := os.Stat(filepath.Join(dir, project.SchemaFile)); err != nil {
		t.Errorf("%s was not created: %v", project.SchemaFile, err)
	}
}

// TestMigrateRenamesLegacySchema covers the ordering bug from design review
// (RR-ZB1FJB): the rename must happen before the migration file list is built,
// or the loop's skip-if-missing guard silently drops every content migration
// for a project that was just renamed.
func TestMigrateRenamesLegacySchema(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, project.LegacySchemaFile)
	if err := os.WriteFile(legacy, []byte("version: '1.0'\nentities: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := MigrateWithFS(dir, testFS())
	if err != nil {
		t.Fatalf("MigrateWithFS: %v", err)
	}
	if result.SchemaRenamedFrom != project.LegacySchemaFile {
		t.Errorf("SchemaRenamedFrom = %q, want %q", result.SchemaRenamedFrom, project.LegacySchemaFile)
	}
	if _, statErr := os.Stat(filepath.Join(dir, project.SchemaFile)); statErr != nil {
		t.Errorf("%s missing after migrate: %v", project.SchemaFile, statErr)
	}
	if _, statErr := os.Stat(legacy); statErr == nil {
		t.Error("legacy file still present after migrate")
	}

	// The schema file must still be in the migrated set under its new name —
	// this is what proves content migrations were not skipped past.
	var sawSchema bool
	for _, fr := range result.FileResults {
		if fr.File.Name == project.SchemaFile {
			sawSchema = true
		}
	}
	if !sawSchema {
		t.Error("renamed schema file was not processed by the migration loop")
	}

	// Second run is a no-op.
	again, err := MigrateWithFS(dir, testFS())
	if err != nil {
		t.Fatalf("second MigrateWithFS: %v", err)
	}
	if again.SchemaRenamedFrom != "" {
		t.Errorf("second run reported a rename: %q", again.SchemaRenamedFrom)
	}
}

// TestMigrateRefusesWhenBothPresent pins the refuse-don't-overwrite decision.
// storage.FS.Rename wraps os.Rename, which replaces an existing target
// silently on POSIX, so proceeding would destroy the operator's schema.yaml.
func TestMigrateRefusesWhenBothPresent(t *testing.T) {
	dir := t.TempDir()
	keep := "version: '1.0'\nentities: {}\n# the real one\n"
	if err := os.WriteFile(filepath.Join(dir, project.SchemaFile), []byte(keep), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, project.LegacySchemaFile), []byte("version: '1.0'\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Discovery prefers schema.yaml, so this project is not flagged legacy and
	// migrate proceeds without renaming. Either way the invariant is the same:
	// schema.yaml must survive byte-for-byte.
	if _, err := MigrateWithFS(dir, testFS()); err != nil {
		t.Logf("migrate refused (acceptable): %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, project.SchemaFile))
	if err != nil {
		t.Fatalf("schema.yaml was removed: %v", err)
	}
	if string(got) != keep {
		t.Error("schema.yaml was overwritten by the legacy file")
	}
}
