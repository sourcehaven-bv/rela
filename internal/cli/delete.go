package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/markdown"
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

		entity, ok := g.GetNode(entityID)
		if !ok {
			return &entityNotFoundError{ID: entityID}
		}

		// Check for relations
		incoming := g.IncomingEdges(entityID)
		outgoing := g.OutgoingEdges(entityID)
		totalRelations := len(incoming) + len(outgoing)

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

		// Delete relations first if cascade
		if deleteCascade {
			for _, rel := range incoming {
				if err := markdown.DeleteRelation(rel.FilePath); err != nil && !os.IsNotExist(err) {
					out.WriteWarning("Failed to delete relation file: %v", err)
				}
				g.RemoveEdge(rel.From, rel.Type, rel.To)
			}
			for _, rel := range outgoing {
				if err := markdown.DeleteRelation(rel.FilePath); err != nil && !os.IsNotExist(err) {
					out.WriteWarning("Failed to delete relation file: %v", err)
				}
				g.RemoveEdge(rel.From, rel.Type, rel.To)
			}
		}

		// Delete entity file
		filePath := entity.FilePath
		if filePath == "" {
			// Use proper plural from metamodel if available
			entityDef, _ := meta.GetEntityDef(entity.Type)
			if entityDef != nil {
				plural := entityDef.GetDirPlural(entity.Type)
				filePath = projectCtx.EntityFilePathWithPlural(plural, entityID)
			} else {
				filePath = projectCtx.EntityFilePath(entity.Type, entityID)
			}
		}

		if err := markdown.DeleteEntity(filePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete entity file: %w", err)
		}

		// Remove from graph
		g.RemoveNode(entityID)

		// Save cache
		if err := saveCache(); err != nil {
			out.WriteWarning("Failed to save cache: %v", err)
		}

		out.WriteSuccess("Deleted %s", entityID)
		if deleteCascade && totalRelations > 0 {
			out.WriteMessage("  Also deleted %d relation(s)", totalRelations)
		}

		return nil
	},
}

func init() {
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "Skip confirmation prompt")
	deleteCmd.Flags().BoolVar(&deleteCascade, "cascade", false, "Also delete related links")

	rootCmd.AddCommand(deleteCmd)
}
