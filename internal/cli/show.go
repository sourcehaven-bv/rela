package cli

import (
	"context"
	"errors"

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
			return classifyReadError(entityID, err)
		}

		var incoming, outgoing []*entity.Relation
		inQ := store.RelationQuery{EntityID: entityID, Direction: store.DirectionIncoming}
		for r, err := range st.ListRelations(ctx, inQ) {
			if err != nil {
				break
			}
			incoming = append(incoming, r)
		}
		outQ := store.RelationQuery{EntityID: entityID, Direction: store.DirectionOutgoing}
		for r, err := range st.ListRelations(ctx, outQ) {
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

// classifyReadError maps a store.GetEntity error onto a user-facing
// message. Today it only distinguishes "not found" from every other
// error shape, but the indirection stays so future error classes can
// plug in without touching the showCmd body.
func classifyReadError(id string, err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return &entityNotFoundError{ID: id}
	}
	return err
}
