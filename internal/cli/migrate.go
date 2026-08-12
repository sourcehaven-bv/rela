package cli

import (
	"fmt"
	"os"

	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/projectsetup"
)

// MigrateCmd migrates project files (schema.yaml, etc.) to current
// schema, and renames a legacy metamodel.yaml. Self-discovers the
// project root.
type MigrateCmd struct {
	Check bool `help:"Check for pending migrations without applying (for CI)."`
}

// Run executes `rela migrate [--check]`.
func (c *MigrateCmd) Run() error {
	startDir := projectPath
	if startDir == "" {
		startDir = os.Getenv("RELA_PROJECT")
	}

	if c.Check {
		return runMigrateCheck(startDir)
	}
	return runMigrate(startDir)
}

func runMigrateCheck(startDir string) error {
	detections, schemaName, err := projectsetup.CheckPending(startDir)
	if err != nil {
		return err
	}

	if len(detections) > 0 || schemaName.NeedsAttention() {
		if schemaName.RenamePending {
			fmt.Printf("%s needs renaming to %s\n", project.LegacySchemaFile, project.SchemaFile)
		}
		if schemaName.Orphaned != "" {
			fmt.Printf("%s is being IGNORED because %s exists; merge and delete it\n",
				project.LegacySchemaFile, project.SchemaFile)
		}
		for _, d := range detections {
			fmt.Printf("%s needs migration:\n", d.File.Name)
			for _, m := range d.Migrations {
				fmt.Printf("  - %s\n", m.Description)
			}
		}
		// An orphaned legacy file is the one condition `rela migrate` cannot
		// resolve on its own — it only reports it — so don't point at the
		// command as if it were the whole fix.
		if len(detections) > 0 || schemaName.RenamePending {
			fmt.Println("\nRun 'rela migrate' to apply these migrations.")
		}
		os.Exit(1)
	}

	fmt.Println("No migrations needed.")
	return nil
}

func runMigrate(startDir string) error {
	result, err := projectsetup.Migrate(startDir)
	if err != nil {
		return err
	}

	// Report the rename before the per-file results: it is a change to the
	// project layout the operator must know about (their tooling, scripts and
	// .gitignore may reference the old name), and it happens even when no
	// content migration applies.
	if result.SchemaRenamedFrom != "" {
		fmt.Printf("Renamed %s → %s\n", result.SchemaRenamedFrom, project.SchemaFile)
	}
	if result.OrphanedLegacySchema != "" {
		fmt.Printf("Note: %s is being IGNORED because %s exists. "+
			"Merge anything you still need, then delete it.\n",
			project.LegacySchemaFile, project.SchemaFile)
	}

	if result.FilesUpdated == 0 {
		if result.SchemaRenamedFrom == "" && result.OrphanedLegacySchema == "" {
			fmt.Println("No migrations needed.")
		}
		return nil
	}
	for _, fr := range result.FileResults {
		appliedCount := 0
		for _, mr := range fr.Results {
			if mr.Applied {
				appliedCount++
			}
		}
		if appliedCount > 0 {
			fmt.Printf("Migrating %s...\n", fr.File.Name)
			for _, mr := range fr.Results {
				if mr.Applied {
					fmt.Printf("  ✓ %s: %s\n", mr.Migration.Name(), mr.Migration.Description())
				}
			}
		}
	}
	fmt.Printf("\nDone. %d file(s) updated with %d migration(s).\n", result.FilesUpdated, result.TotalMigrations)
	return nil
}
