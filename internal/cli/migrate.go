package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

var (
	migrateCheck bool
)

var migrateCmd = &cobra.Command{
	Use:         "migrate",
	Short:       "Migrate project files to current schema",
	Annotations: map[string]string{skipProjectDiscovery: "true"},
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

		if migrateCheck {
			return runMigrateCheck(startDir)
		}

		return runMigrate(startDir)
	},
}

func runMigrateCheck(startDir string) error {
	detections, err := workspace.DetectMigrations(startDir)
	if err != nil {
		return err
	}

	if len(detections) > 0 {
		for _, d := range detections {
			fmt.Printf("%s needs migration:\n", d.File.Name)
			for _, m := range d.Migrations {
				fmt.Printf("  - %s\n", m.Description)
			}
		}
		fmt.Println("\nRun 'rela migrate' to apply these migrations.")
		os.Exit(1)
	}

	fmt.Println("No migrations needed.")
	return nil
}

func runMigrate(startDir string) error {
	result, err := workspace.Migrate(startDir)
	if err != nil {
		return err
	}

	if result.FilesUpdated == 0 {
		fmt.Println("No migrations needed.")
	} else {
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
	}

	return nil
}

func init() {
	migrateCmd.Flags().BoolVar(&migrateCheck, "check", false, "Check for pending migrations without applying (for CI)")
	rootCmd.AddCommand(migrateCmd)
}
