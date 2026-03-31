package cli

import (
	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

var (
	fmtDryRun bool
	fmtCheck  bool
)

var fmtCmd = &cobra.Command{
	Use:   "fmt [type]",
	Short: "Format entity and relation files",
	Long: `Formats entity and relation files to ensure consistent formatting.

This command normalizes:
- Frontmatter property ordering (id/type first for entities, from/relation/to for relations)
- Markdown content formatting (headings, lists, whitespace)

Exit codes:
- 0: All files formatted (or already formatted with --check)
- 1: Files need formatting (with --check)

Examples:
  rela fmt                # Format all entities and relations
  rela fmt requirements   # Format only requirements (entities)
  rela fmt --dry-run      # Preview changes without writing
  rela fmt --check        # Check if files need formatting (for CI)`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// For --check mode, use dry-run behavior internally
		dryRun := fmtDryRun || fmtCheck

		modifiedEntities := 0
		modifiedRelations := 0

		// Format entities
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

		for _, entity := range entities {
			changed, err := ws.FormatEntity(entity, dryRun)
			if err != nil {
				out.WriteWarning("Failed to format %s: %v", entity.ID, err)
				continue
			}

			if !changed {
				continue
			}

			modifiedEntities++

			if fmtCheck {
				out.WriteMessage("Needs formatting: %s", entity.ID)
			} else if fmtDryRun {
				out.WriteMessage("Would format: %s", entity.ID)
			} else if verbose {
				out.WriteMessage("Formatted: %s", entity.ID)
			}
		}

		// Format relations (only when no specific type is specified)
		if len(args) == 0 {
			relations := ws.AllRelations()
			for _, relation := range relations {
				changed, err := ws.FormatRelation(relation, dryRun)
				if err != nil {
					out.WriteWarning("Failed to format relation %s--%s--%s: %v",
						relation.From, relation.Type, relation.To, err)
					continue
				}

				if !changed {
					continue
				}

				modifiedRelations++

				relationID := relation.From + "--" + relation.Type + "--" + relation.To
				if fmtCheck {
					out.WriteMessage("Needs formatting: %s", relationID)
				} else if fmtDryRun {
					out.WriteMessage("Would format: %s", relationID)
				} else if verbose {
					out.WriteMessage("Formatted: %s", relationID)
				}
			}
		}

		totalModified := modifiedEntities + modifiedRelations

		if fmtCheck {
			if totalModified > 0 {
				out.WriteMessage("%d files need formatting (%d entities, %d relations)",
					totalModified, modifiedEntities, modifiedRelations)
				return errors.NewExitError(1)
			}
			out.WriteSuccess("All files are properly formatted")
			return nil
		}

		if fmtDryRun {
			out.WriteMessage("Dry run: %d files would be formatted (%d entities, %d relations)",
				totalModified, modifiedEntities, modifiedRelations)
		} else {
			out.WriteSuccess("Formatted %d files (%d entities, %d relations)",
				totalModified, modifiedEntities, modifiedRelations)
		}

		return nil
	},
}

func init() {
	fmtCmd.Flags().BoolVar(&fmtDryRun, "dry-run", false, "Preview changes without writing")
	fmtCmd.Flags().BoolVar(&fmtCheck, "check", false, "Check if files need formatting (exits 1 if they do)")

	rootCmd.AddCommand(fmtCmd)
}
