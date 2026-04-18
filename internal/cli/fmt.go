package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/store"
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
		st := ws.Store()
		f, ok := st.(store.Formatter)
		if !ok {
			out.WriteMessage("The active storage backend does not support formatting.")
			return nil
		}

		dryRun := fmtDryRun || fmtCheck
		ctx := context.Background()

		modifiedEntities := 0
		modifiedRelations := 0

		// Collect entity IDs from store
		q := store.EntityQuery{}
		if len(args) > 0 {
			resolvedType, _, err := resolveEntityType(args[0])
			if err != nil {
				return err
			}
			q.Type = resolvedType
		}

		var entityIDs []string
		for e, err := range st.ListEntities(ctx, q) {
			if err != nil {
				return err
			}
			entityIDs = append(entityIDs, e.ID)
		}

		for _, id := range entityIDs {
			changed, err := f.FormatEntity(ctx, id, dryRun)
			if err != nil {
				out.WriteWarning("Failed to format %s: %v", id, err)
				continue
			}
			if !changed {
				continue
			}
			modifiedEntities++

			if fmtCheck {
				out.WriteMessage("Needs formatting: %s", id)
			} else if fmtDryRun {
				out.WriteMessage("Would format: %s", id)
			} else if verbose {
				out.WriteMessage("Formatted: %s", id)
			}
		}

		// Format relations (only when no specific type is specified)
		if len(args) == 0 {
			type relKey struct{ from, typ, to string }
			var relKeys []relKey
			for r, err := range st.ListRelations(ctx, store.RelationQuery{}) {
				if err != nil {
					return err
				}
				relKeys = append(relKeys, relKey{r.From, r.Type, r.To})
			}

			for _, k := range relKeys {
				changed, err := f.FormatRelation(ctx, k.from, k.typ, k.to, dryRun)
				if err != nil {
					out.WriteWarning("Failed to format relation %s--%s--%s: %v", k.from, k.typ, k.to, err)
					continue
				}
				if !changed {
					continue
				}
				modifiedRelations++

				relationID := k.from + "--" + k.typ + "--" + k.to
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
