package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

var (
	fmtDryRun bool
	fmtCheck  bool
)

var fmtCmd = &cobra.Command{
	Use:   "fmt [type]",
	Short: "Format entity files",
	Long: `Formats entity files to ensure consistent formatting.

Currently this command:
- Orders frontmatter properties according to the metamodel definition
- Ensures id and type appear first, followed by properties in metamodel order
- Places any extra properties (not in metamodel) at the end, sorted alphabetically

Exit codes:
- 0: All files formatted (or already formatted with --check)
- 1: Files need formatting (with --check)

Examples:
  rela fmt                # Format all entities
  rela fmt requirements   # Format only requirements
  rela fmt --dry-run      # Preview changes without writing
  rela fmt --check        # Check if files need formatting (for CI)`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var entities []*model.Entity

		if len(args) > 0 {
			typeName := args[0]
			resolvedType, _, err := resolveEntityType(typeName)
			if err != nil {
				return err
			}
			entities = ws.EntitiesByType(resolvedType)
		} else {
			entities = ws.AllEntities()
		}

		if len(entities) == 0 {
			out.WriteMessage("No entities found")
			return nil
		}

		modified := 0
		for _, entity := range entities {
			// Get property order from metamodel
			var propertyOrder []string
			if entityDef, ok := meta.GetEntityDef(entity.Type); ok {
				propertyOrder = entityDef.GetPropertyOrder()
			}

			// Generate formatted content
			formatted, err := markdown.FormatEntity(entity, propertyOrder)
			if err != nil {
				out.WriteWarning("Failed to format %s: %v", entity.ID, err)
				continue
			}

			// Read current file content
			currentContent, err := os.ReadFile(entity.FilePath)
			if err != nil {
				out.WriteWarning("Failed to read %s: %v", entity.ID, err)
				continue
			}

			// Compare
			if formatted == string(currentContent) {
				continue
			}

			modified++

			if fmtCheck {
				out.WriteMessage("Needs formatting: %s", entity.ID)
				continue
			}

			if fmtDryRun {
				out.WriteMessage("Would format: %s", entity.ID)
				continue
			}

			// Write formatted content
			if err := os.WriteFile(entity.FilePath, []byte(formatted), 0644); err != nil {
				out.WriteWarning("Failed to write %s: %v", entity.ID, err)
				continue
			}

			if verbose {
				out.WriteMessage("Formatted: %s", entity.ID)
			}
		}

		if fmtCheck {
			if modified > 0 {
				out.WriteMessage("%d entities need formatting", modified)
				return errors.NewExitError(1)
			}
			out.WriteSuccess("All entities are properly formatted")
			return nil
		}

		if fmtDryRun {
			out.WriteMessage("Dry run: %d entities would be formatted", modified)
		} else {
			out.WriteSuccess("Formatted %d entities", modified)
		}

		return nil
	},
}

func init() {
	fmtCmd.Flags().BoolVar(&fmtDryRun, "dry-run", false, "Preview changes without writing")
	fmtCmd.Flags().BoolVar(&fmtCheck, "check", false, "Check if files need formatting (exits 1 if they do)")

	rootCmd.AddCommand(fmtCmd)
}
