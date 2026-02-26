package cli

import (
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show entity details",
	Long: `Shows detailed information about an entity, including its relations.

Examples:
  rela show REQ-001
  rela show DEC-042 -o json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		entityID := args[0]

		entity, ok := ws.GetEntity(entityID)
		if !ok {
			return &entityNotFoundError{ID: entityID}
		}

		incoming := ws.IncomingRelations(entityID)
		outgoing := ws.OutgoingRelations(entityID)

		return out.WriteEntity(entity, incoming, outgoing)
	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}

type entityNotFoundError struct {
	ID string
}

func (e *entityNotFoundError) Error() string {
	return "entity not found: " + e.ID
}
