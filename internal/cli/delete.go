package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

var (
	deleteForce   bool
	deleteCascade bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete an entity",
	Long: `Deletes an entity and optionally its relations.

Examples:
  rela delete REQ-001
  rela delete REQ-001 --cascade  # Also delete related links
  rela delete REQ-001 --force    # Skip confirmation`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		entityID := args[0]
		ctx := context.Background()
		st := ws.Store()

		entity, err := st.GetEntity(ctx, entityID)
		if err != nil {
			return &entityNotFoundError{ID: entityID}
		}

		// Check for relations
		totalRelations, _ := st.CountRelations(ctx, store.RelationQuery{
			EntityID:  entityID,
			Direction: store.DirectionBoth,
		})

		if totalRelations > 0 && !deleteCascade {
			return fmt.Errorf("entity %s has %d relation(s); use --cascade to delete them too", entityID, totalRelations)
		}

		// Confirm deletion
		if !deleteForce {
			fmt.Printf("Delete %s '%s'", entity.Type, entity.Title())
			if totalRelations > 0 {
				fmt.Printf(" and %d relation(s)", totalRelations)
			}
			fmt.Print("? [y/N] ")

			reader := bufio.NewReader(os.Stdin)
			response, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}
			response = strings.TrimSpace(strings.ToLower(response))

			if response != "y" && response != "yes" {
				out.WriteMessage("Cancelled")
				return nil
			}
		}

		result, err := ws.EntityManager().DeleteEntity(ctx, entityID, deleteCascade)
		if err != nil {
			if errors.Is(err, workspace.ErrHasRelations) {
				return fmt.Errorf("entity %s has relation(s); use --cascade to delete them too", entityID)
			}
			return err
		}

		out.WriteSuccess("Deleted %s", entityID)
		if deleteCascade && len(result.DeletedRelations) > 0 {
			out.WriteMessage("  Also deleted %d relation(s)", len(result.DeletedRelations))
		}

		return nil
	},
}

func init() {
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "Skip confirmation prompt")
	deleteCmd.Flags().BoolVar(&deleteCascade, "cascade", false, "Also delete related links")

	rootCmd.AddCommand(deleteCmd)
}
