package cli

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// LinkCmd creates a relation between entities.
type LinkCmd struct {
	From     string `arg:"" help:"Source entity ID, or ID@face for a content-scoped relation on a faced type."`
	Relation string `arg:"" help:"Relation type."`
	To       string `arg:"" help:"Target entity ID."`
}

// Run dispatches `rela link <from> <relation> <to>`.
func (c *LinkCmd) Run(ctx context.Context, svc *writeServices) error {
	from, err := relationTail(ctx, &svc.readServices, c.From, c.Relation)
	if err != nil {
		return err
	}
	key := entity.RelationKey{From: from.ID, FromFace: from.Face, Type: c.Relation, To: c.To}
	_, err = svc.EntityManager.CreateRelation(ctx, key, entity.RelationOptions{})
	if err != nil {
		return err
	}
	out.WriteSuccess("Created link: %s --%s--> %s", c.From, c.Relation, c.To)
	return nil
}
