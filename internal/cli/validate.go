package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate project configuration files",
	Long: `Validate metamodel.yaml and data-entry.yaml configuration files.

Checks for:
- Unknown/misspelled keys
- Invalid cross-references (forms, lists, views)
- Invalid entity types, relations, and properties
- View traversal correctness
- Dashboard and command configuration

Examples:
  rela validate    # Validate all config files`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Determine start directory: flag > env var > cwd
		startDir := projectPath
		if startDir == "" {
			startDir = os.Getenv("RELA_PROJECT")
		}

		result, err := workspace.Validate(startDir)
		if err != nil {
			return err
		}

		hasErrors := false

		// Report metamodel validation
		fmt.Println("Validating metamodel.yaml...")
		if result.MetamodelError != nil {
			fmt.Printf("  ✗ %v\n", result.MetamodelError)
			hasErrors = true
		} else {
			fmt.Println("  ✓ metamodel.yaml is valid")
		}

		// Report data-entry validation
		if result.DataEntrySkipped {
			if result.MetamodelError != nil {
				fmt.Println("  ⚠ Skipping data-entry validation (metamodel has errors)")
			} else {
				fmt.Printf("Skipping %s (file not found)\n", dataentryconfig.ConfigFile)
			}
		} else {
			fmt.Printf("Validating %s...\n", dataentryconfig.ConfigFile)
			if result.DataEntryError != nil {
				fmt.Printf("  ✗ %v\n", result.DataEntryError)
				hasErrors = true
			} else {
				fmt.Printf("  ✓ %s is valid\n", dataentryconfig.ConfigFile)
			}
		}

		if hasErrors {
			fmt.Println("\nValidation failed.")
			os.Exit(1)
		}

		fmt.Println("\nAll configuration files are valid.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
