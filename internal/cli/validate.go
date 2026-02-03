package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
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
		// Discover project (we need the paths)
		ctx, err := project.Discover("", cliFS)
		if err != nil {
			return fmt.Errorf("no project found: run 'rela init' to create one")
		}

		hasErrors := false

		// Initialize repository
		repoInstance := repository.New(cliFS, ctx)

		// 1. Validate metamodel
		fmt.Println("Validating metamodel.yaml...")
		mm, err := repoInstance.LoadMetamodel()
		if err != nil {
			fmt.Printf("  ✗ %v\n", err)
			hasErrors = true
			mm = nil
		} else {
			fmt.Println("  ✓ metamodel.yaml is valid")
		}

		// 2. Validate data-entry.yaml if it exists
		dataEntryPath := filepath.Join(ctx.Root, dataentry.ConfigFile)
		if _, statErr := cliFS.Stat(dataEntryPath); statErr == nil {
			fmt.Printf("Validating %s...\n", dataentry.ConfigFile)

			if mm == nil {
				fmt.Println("  ⚠ Skipping data-entry validation (metamodel has errors)")
			} else {
				if err := validateDataEntryConfig(dataEntryPath, mm); err != nil {
					fmt.Printf("  ✗ %v\n", err)
					hasErrors = true
				} else {
					fmt.Printf("  ✓ %s is valid\n", dataentry.ConfigFile)
				}
			}
		} else {
			fmt.Printf("Skipping %s (file not found)\n", dataentry.ConfigFile)
		}

		if hasErrors {
			fmt.Println("\nValidation failed.")
			os.Exit(1)
		}

		fmt.Println("\nAll configuration files are valid.")
		return nil
	},
}

// validateDataEntryConfig validates the data-entry.yaml file.
func validateDataEntryConfig(path string, mm *metamodel.Metamodel) error {
	data, err := cliFS.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	var cfg dataentry.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parsing YAML: %w", err)
	}

	return dataentry.ValidateConfig(data, &cfg, mm)
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
