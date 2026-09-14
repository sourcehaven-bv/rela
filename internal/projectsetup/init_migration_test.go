package projectsetup_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/projectsetup"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// TestInitializeWithFS_NeedsNoMigration pins that a freshly initialized project
// is already in the current syntax. This is not cosmetic: metamodel.FSLoader
// refuses to load a file with pending detections, so a generated schema that
// trips a migration leaves every command failing until the user runs
// `rela migrate` on a file rela itself just wrote.
//
// It goes through DetectMigrations rather than checking schema.yaml against one
// named migration, so it covers every file type `rela migrate --check` scans and
// the whole registered migration set. A future migration that forgets the init
// template — or a starter data-entry.yaml/acl.yaml that falls behind — fails
// here at introduction.
func TestInitializeWithFS_NeedsNoMigration(t *testing.T) {
	fs := storage.NewMemFS()
	root := "/proj"

	if _, err := projectsetup.InitializeWithFS(root, fs); err != nil {
		t.Fatalf("InitializeWithFS: %v", err)
	}

	detections, err := projectsetup.DetectMigrationsWithFS(root, fs)
	if err != nil {
		t.Fatalf("DetectMigrationsWithFS: %v", err)
	}

	for _, d := range detections {
		for _, det := range d.Migrations {
			t.Errorf("generated %s needs migration %q: %s",
				d.File.Name, det.Migration.Name(), det.Description)
		}
	}
}
