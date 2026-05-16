package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

var (
	normalizeDryRun bool
)

var normalizeCmd = &cobra.Command{
	Use:   "normalize [type]",
	Short: "Normalize markdown headers in entity files",
	Long: `Normalizes markdown headers in entity files to start at level 2 (##).

This command adjusts header levels so the minimum header level in each entity
is ##, preserving the relative hierarchy. For example, if an entity has:
  # Overview
  ## Details
  ### Subsection

It will be normalized to:
  ## Overview
  ### Details
  #### Subsection

Setext-style headers (underlined with === or ---) are converted to ATX style (##).

If headers already start at ## or deeper, no changes are made.

Examples:
  rela normalize                # Normalize all entities
  rela normalize requirements   # Normalize only requirements
  rela normalize req            # Alias works too
  rela normalize --dry-run      # Preview changes without writing`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		svc := cliWriteFromContext(cmd.Context())
		st := svc.Store()

		q := store.EntityQuery{}
		if len(args) > 0 {
			resolvedType, _, err := resolveEntityType(svc.Meta(), args[0])
			if err != nil {
				return err
			}
			q.Type = resolvedType
		}

		var entities []*entity.Entity
		for e, err := range st.ListEntities(ctx, q) {
			if err != nil {
				return err
			}
			entities = append(entities, e)
		}

		if len(entities) == 0 {
			out.WriteMessage("No entities found")
			return nil
		}

		modified := 0
		for _, e := range entities {
			normalized := markdown.NormalizeHeaders(e.Content)
			if normalized == e.Content {
				continue
			}

			if normalizeDryRun {
				out.WriteMessage("Would normalize: %s", e.ID)
				modified++
				continue
			}

			e.Content = normalized
			if err := st.UpdateEntity(ctx, e); err != nil {
				out.WriteWarning("Failed to write %s: %v", e.ID, err)
				continue
			}

			modified++

			if verbose {
				out.WriteMessage("Normalized: %s", e.ID)
			}
		}

		if normalizeDryRun {
			out.WriteMessage("Dry run: %d entities would be modified", modified)
		} else {
			out.WriteSuccess("Normalized %d entities", modified)
		}

		return nil
	},
}

func init() {
	normalizeCmd.Flags().BoolVar(&normalizeDryRun, "dry-run", false, "Preview changes without writing")

	rootCmd.AddCommand(normalizeCmd)
}
