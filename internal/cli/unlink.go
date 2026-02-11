package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var unlinkCmd = &cobra.Command{
	Use:   "unlink <from> <relation> <to>",
	Short: "Remove a relation between entities",
	Long: `Removes a directed relation between two entities.

Examples:
  rela unlink DEC-001 addresses REQ-001
  rela unlink SOL-001 implements DEC-001`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		fromID := args[0]
		relationType := args[1]
		toID := args[2]

		// Check if relation exists
		_, exists := g.GetEdge(fromID, relationType, toID)
		if !exists {
			return fmt.Errorf("relation not found: %s --%s--> %s", fromID, relationType, toID)
		}

		// Delete file
		if err := repo.DeleteRelation(fromID, relationType, toID); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete relation file: %w", err)
		}

		// Remove from graph
		g.RemoveEdge(fromID, relationType, toID)

		// Save cache
		if err := saveCache(); err != nil {
			out.WriteWarning("Failed to save cache: %v", err)
		}

		out.WriteSuccess("Removed link: %s --%s--> %s", fromID, relationType, toID)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(unlinkCmd)
}
