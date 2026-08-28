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

// TestMigrateReportsOrphanedLegacySchema covers the both-files-present case.
//
// Discovery prefers schema.yaml, so the stale metamodel.yaml is not "pending a
// rename" — there is nothing to rename it to. Without an explicit report it is
// invisible: every command reads past it silently, forever. The earlier version
// of this test accepted either an error or no error and asserted only that
// schema.yaml was unchanged, which held trivially and pinned nothing (RR-66X91V).
func TestMigrateReportsOrphanedLegacySchema(t *testing.T) {
	dir := t.TempDir()
	keep := "version: '1.0'\nentities: {}\n# the real one\n"
	if err := os.WriteFile(filepath.Join(dir, project.SchemaFile), []byte(keep), 0o644); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(dir, project.LegacySchemaFile)
	if err := os.WriteFile(legacy, []byte("version: '1.0'\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := MigrateWithFS(dir, testFS())
	if err != nil {
		t.Fatalf("MigrateWithFS: %v", err)
	}

	if result.OrphanedLegacySchema != legacy {
		t.Errorf("OrphanedLegacySchema = %q, want %q", result.OrphanedLegacySchema, legacy)
	}
	if result.SchemaRenamedFrom != "" {
		t.Errorf("must not rename when schema.yaml already exists, got %q", result.SchemaRenamedFrom)
	}

	// The live file must survive byte-for-byte: os.Rename replaces silently on
	// POSIX, so a stray rename here would be an unrecoverable overwrite.
	got, readErr := os.ReadFile(filepath.Join(dir, project.SchemaFile))
	if readErr != nil {
		t.Fatalf("schema.yaml was removed: %v", readErr)
	}
	if string(got) != keep {
		t.Error("schema.yaml was overwritten by the legacy file")
	}
	// migrate reports the orphan; deleting the operator's file is not its call.
	if _, statErr := os.Stat(legacy); statErr != nil {
		t.Error("migrate deleted the orphaned legacy file; it should only report it")
	}
}

// TestSchemaNameStatus pins what `rela migrate --check` sees, so a legacy or
// orphaned file fails CI instead of passing quietly.
func TestSchemaNameStatus(t *testing.T) {
	t.Run("orphan is flagged", func(t *testing.T) {
		dir := t.TempDir()
		for _, n := range []string{project.SchemaFile, project.LegacySchemaFile} {
			if err := os.WriteFile(filepath.Join(dir, n), []byte("version: '1.0'\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		ctx, err := project.Discover(dir, testFS())
		if err != nil {
			t.Fatal(err)
		}
		st := SchemaName(ctx, testFS())
		if st.RenamePending {
			t.Error("RenamePending must be false when schema.yaml is live")
		}
		if st.Orphaned == "" || !st.NeedsAttention() {
			t.Error("an orphaned metamodel.yaml must be reported")
		}
	})

	t.Run("clean project needs nothing", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, project.SchemaFile), []byte("version: '1.0'\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		ctx, err := project.Discover(dir, testFS())
		if err != nil {
			t.Fatal(err)
		}
		if SchemaName(ctx, testFS()).NeedsAttention() {
			t.Error("a schema.yaml-only project needs no attention")
		}
	})
}
