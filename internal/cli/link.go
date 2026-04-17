package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
)

var linkCmd = &cobra.Command{
	Use:   "link <from> <relation> <to>",
	Short: "Create a relation between entities",
	Long: `Creates a directed relation between two entities.

Examples:
  rela link DEC-001 addresses REQ-001
  rela link SOL-001 implements DEC-001
  rela link COMP-001 realizes SOL-001`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		fromID := args[0]
		relationType := args[1]
		toID := args[2]

		_, err := ws.EntityManager().CreateRelation(
			context.Background(), fromID, relationType, toID, entitymanager.RelationOptions{})
		if err != nil {
			return err
		}

		out.WriteSuccess("Created link: %s --%s--> %s", fromID, relationType, toID)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(linkCmd)
}
