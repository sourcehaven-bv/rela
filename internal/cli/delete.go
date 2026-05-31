package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// DeleteCmd deletes an entity and (optionally) its relations.
type DeleteCmd struct {
	ID      string `arg:"" help:"Entity ID."`
	Force   bool   `short:"f" help:"Skip confirmation prompt."`
	Cascade bool   `help:"Also delete related links."`
}

// Run dispatches `rela delete <id>`.
func (c *DeleteCmd) Run(ctx context.Context, svc *cliServices) error {
	st := svc.Store()

	entity, err := st.GetEntity(ctx, c.ID)
	if err != nil {
		return &entityNotFoundError{ID: c.ID}
	}

	totalRelations, _ := st.CountRelations(ctx, store.RelationQuery{
		EntityID:  c.ID,
		Direction: store.DirectionBoth,
	})

	if totalRelations > 0 && !c.Cascade {
		return fmt.Errorf("entity %s has %d relation(s); use --cascade to delete them too", c.ID, totalRelations)
	}

	if !c.Force {
		fmt.Printf("Delete %s '%s'", entity.Type, entity.Title())
		if totalRelations > 0 {
			fmt.Printf(" and %d relation(s)", totalRelations)
		}
		fmt.Print("? [y/N] ")

		reader := bufio.NewReader(os.Stdin)
		response, readErr := reader.ReadString('\n')
		if readErr != nil {
			return fmt.Errorf("failed to read input: %w", readErr)
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			out.WriteMessage("Cancelled")
			return nil
		}
	}

	result, err := svc.EntityManager().DeleteEntity(ctx, c.ID, c.Cascade)
	if err != nil {
		if errors.Is(err, entitymanager.ErrHasRelations) {
			return fmt.Errorf("entity %s has relation(s); use --cascade to delete them too", c.ID)
		}
		return err
	}

	out.WriteSuccess("Deleted %s", c.ID)
	if c.Cascade && len(result.DeletedRelations) > 0 {
		out.WriteMessage("  Also deleted %d relation(s)", len(result.DeletedRelations))
	}
	return nil
}
