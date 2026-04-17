package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
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
		ctx := context.Background()
		st := ws.Store()

		e, err := st.GetEntity(ctx, entityID)
		if err != nil {
			return &entityNotFoundError{ID: entityID}
		}

		var incoming, outgoing []*entity.Relation
		for r, err := range st.ListRelations(ctx, store.RelationQuery{EntityID: entityID, Direction: store.DirectionIncoming}) {
			if err != nil {
				break
			}
			incoming = append(incoming, r)
		}
		for r, err := range st.ListRelations(ctx, store.RelationQuery{EntityID: entityID, Direction: store.DirectionOutgoing}) {
			if err != nil {
				break
			}
			outgoing = append(outgoing, r)
		}

		return out.WriteEntity(e, incoming, outgoing)
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
