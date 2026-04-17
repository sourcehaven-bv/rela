package cli

import (
	"context"
	"fmt"

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
		ctx := context.Background()

		// Check if relation exists (for better error message)
		if _, err := ws.Store().GetRelation(ctx, fromID, relationType, toID); err != nil {
			return fmt.Errorf("relation not found: %s --%s--> %s", fromID, relationType, toID)
		}

		if err := ws.EntityManager().DeleteRelation(ctx, fromID, relationType, toID); err != nil {
			return err
		}

		out.WriteSuccess("Removed link: %s --%s--> %s", fromID, relationType, toID)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(unlinkCmd)
}
