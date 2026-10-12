package projectsetup_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/projectsetup"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func TestDetectMigrationsWithFS_NoProject(t *testing.T) {
	fs := storage.NewMemFS()
	_, err := projectsetup.DetectMigrationsWithFS("/missing", fs)
	if err == nil {
		t.Fatal("expected error for missing project, got nil")
	}
}

func TestMigrateWithFS_NoProject(t *testing.T) {
	fs := storage.NewMemFS()
	_, err := projectsetup.MigrateWithFS("/missing", fs)
	if err == nil {
		t.Fatal("expected error for missing project, got nil")
	}
}

func TestDetectMigrationsWithFS_RunsSuccessfully(t *testing.T) {
	fs := storage.NewMemFS()
	root := "/proj"
	if _, err := projectsetup.InitializeWithFS(root, fs); err != nil {
		t.Fatalf("init: %v", err)
	}

	if _, err := projectsetup.DetectMigrationsWithFS(root, fs); err != nil {
		t.Fatalf("DetectMigrationsWithFS: %v", err)
	}
}

func TestMigrateWithFS_AppliesPending(t *testing.T) {
	fs := storage.NewMemFS()
	root := "/proj"
	if _, err := projectsetup.InitializeWithFS(root, fs); err != nil {
		t.Fatalf("init: %v", err)
	}

	result, err := projectsetup.MigrateWithFS(root, fs)
	if err != nil {
		t.Fatalf("MigrateWithFS: %v", err)
	}
	if result == nil {
		t.Fatal("nil result")
	}

	after, err := projectsetup.DetectMigrationsWithFS(root, fs)
	if err != nil {
		t.Fatalf("DetectMigrationsWithFS after migrate: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("expected no pending migrations after Migrate, got %d", len(after))
	}
}

// A schema that fails to load must not reach the migrations as a typed nil:
// the data-entry cleanup would pass its nil check and dereference it.
func TestDetectMigrationsWithFS_UnloadableSchemaUsesNoMetamodelPath(t *testing.T) {
	fs := storage.NewMemFS()
	root := "/proj"
	if _, err := projectsetup.InitializeWithFS(root, fs); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := fs.WriteFile(root+"/schema.yaml", []byte("entities: 5\n"), 0o644); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	dataEntry := "forms:\n  ticket:\n    entity_type: ticket\n    fields:\n      - property: title\n        widget: text\n"
	if err := fs.WriteFile(root+"/data-entry.yaml", []byte(dataEntry), 0o644); err != nil {
		t.Fatalf("write data-entry: %v", err)
	}

	detections, err := projectsetup.DetectMigrationsWithFS(root, fs)
	if err != nil {
		t.Fatalf("DetectMigrationsWithFS: %v", err)
	}
	for _, d := range detections {
		for _, m := range d.Migrations {
			if strings.Contains(m.Description, "using metamodel") {
				t.Errorf("%s: %q ran with a metamodel the schema could not provide", d.File.Name, m.Description)
			}
		}
	}
}
