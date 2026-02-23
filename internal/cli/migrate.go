package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/migration"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

var (
	migrateCheck bool
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate project files to current schema",
	Long: `Migrate project files (metamodel.yaml, etc.) to the current schema format.

This command detects deprecated syntax patterns and transforms them to the
current format while preserving comments and formatting.

Examples:
  rela migrate         # Apply all pending migrations
  rela migrate --check # Check for pending migrations (for CI)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Determine start directory: flag > env var > cwd
		startDir := projectPath
		if startDir == "" {
			startDir = os.Getenv("RELA_PROJECT")
		}

		// Discover project (we need the paths but not the metamodel)
		ctx, err := project.Discover(startDir, cliFS)
		if err != nil {
			return fmt.Errorf("no project found: run 'rela init' to create one")
		}

		// Load metamodel for context-aware migrations (ignore errors - metamodel may need migration)
		mm, _ := metamodel.LoadWithoutMigrationCheck(ctx.MetamodelPath, cliFS)

		// Define files to check
		dataEntryPath := filepath.Join(ctx.Root, dataentryconfig.ConfigFile)

		files := []struct {
			path     string
			fileType migration.FileType
			name     string
		}{
			{ctx.MetamodelPath, migration.FileTypeMetamodel, "metamodel.yaml"},
			{dataEntryPath, migration.FileTypeDataEntry, dataentryconfig.ConfigFile},
		}

		if migrateCheck {
			return runMigrateCheck(files, mm)
		}

		return runMigrate(files, mm)
	},
}

func runMigrateCheck(files []struct {
	path     string
	fileType migration.FileType
	name     string
}, meta *metamodel.Metamodel) error {
	needsMigration := false

	for _, f := range files {
		// Skip files that don't exist
		if _, err := cliFS.Stat(f.path); os.IsNotExist(err) {
			continue
		}

		detections, err := migration.DetectWithMetamodel(f.path, f.fileType, cliFS, meta)
		if err != nil {
			return fmt.Errorf("checking %s: %w", f.name, err)
		}

		if len(detections) > 0 {
			needsMigration = true
			fmt.Printf("%s needs migration:\n", f.name)
			for _, d := range detections {
				fmt.Printf("  - %s\n", d.Description)
			}
		}
	}

	if needsMigration {
		fmt.Println("\nRun 'rela migrate' to apply these migrations.")
		os.Exit(1)
	}

	fmt.Println("No migrations needed.")
	return nil
}

func runMigrate(files []struct {
	path     string
	fileType migration.FileType
	name     string
}, meta *metamodel.Metamodel) error {
	filesUpdated := 0
	totalMigrations := 0

	for _, f := range files {
		// Skip files that don't exist
		if _, err := cliFS.Stat(f.path); os.IsNotExist(err) {
			continue
		}

		result, err := migration.ApplyWithMetamodel(f.path, f.fileType, cliFS, meta)
		if err != nil {
			return fmt.Errorf("migrating %s: %w", f.name, err)
		}

		if result.HasErrors() {
			return fmt.Errorf("migrating %s: %w", f.name, result.Error)
		}

		if result.NeedsMigration() {
			filesUpdated++
			fmt.Printf("Migrating %s...\n", f.name)
			for _, mr := range result.Results {
				if mr.Applied {
					totalMigrations++
					fmt.Printf("  ✓ %s: %s\n", mr.Migration.Name(), mr.Migration.Description())
				}
			}
		}
	}

	if filesUpdated == 0 {
		fmt.Println("No migrations needed.")
	} else {
		fmt.Printf("\nDone. %d file(s) updated with %d migration(s).\n", filesUpdated, totalMigrations)
	}

	return nil
}

func init() {
	migrateCmd.Flags().BoolVar(&migrateCheck, "check", false, "Check for pending migrations without applying (for CI)")
	rootCmd.AddCommand(migrateCmd)
}
