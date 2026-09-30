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
func (c *LinkCmd) Run(ctx context.Context, svc *writeServices) error {
	// `rela link` takes no face yet (TKT-2RQMV4), so it writes the
	// implicit-tail edge.
	key := entity.RelationKey{From: c.From, FromFace: entity.ImplicitFace, Type: c.Relation, To: c.To}
	_, err := svc.EntityManager.CreateRelation(ctx, key, entity.RelationOptions{})
	if err != nil {
		return err
	}
	out.WriteSuccess("Created link: %s --%s--> %s", c.From, c.Relation, c.To)
	return nil
}
