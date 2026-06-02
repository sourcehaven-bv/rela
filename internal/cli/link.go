package cli

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// LinkCmd creates a relation between entities.
type LinkCmd struct {
	From     string `arg:"" help:"Source entity ID."`
	Relation string `arg:"" help:"Relation type."`
	To       string `arg:"" help:"Target entity ID."`
}

// Run dispatches `rela link <from> <relation> <to>`.
func (c *LinkCmd) Run(ctx context.Context, svc *cliServices) error {
	_, err := svc.EntityManager().CreateRelation(
		ctx, c.From, c.Relation, c.To, entity.RelationOptions{})
	if err != nil {
		return err
	}
	out.WriteSuccess("Created link: %s --%s--> %s", c.From, c.Relation, c.To)
	return nil
}
