package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/encryption"
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
// message. Distinguishes the three encryption-class errors from a
// genuine "not found" so the CLI doesn't lie about the state of the
// repo (e.g., reporting "not found" when the entity exists but the
// local identity can't decrypt it).
func classifyReadError(id string, err error) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return &entityNotFoundError{ID: id}
	case encryption.IsNoMatchingKey(err):
		return fmt.Errorf("%s: not authorized (your identity is not in this repo's recipient list)", id)
	case encryption.IsNoPrivateKey(err):
		return fmt.Errorf("%s: no identity loaded (set $RELA_KEY_FILE or place .rela/key)", id)
	case encryption.IsCorrupted(err):
		return fmt.Errorf("%s: sealed file is corrupted or tampered: %w", id, err)
	default:
		return err
	}
}
