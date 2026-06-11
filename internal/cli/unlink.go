package cli

import (
	"context"
	"fmt"
)

// UnlinkCmd removes a relation between entities.
type UnlinkCmd struct {
	From     string `arg:"" help:"Source entity ID."`
	Relation string `arg:"" help:"Relation type."`
	To       string `arg:"" help:"Target entity ID."`
}

// Run dispatches `rela unlink <from> <relation> <to>`.
func (c *UnlinkCmd) Run(ctx context.Context, svc *cliServices) error {
	if _, err := svc.Store().GetRelation(ctx, c.From, c.Relation, c.To); err != nil {
		return fmt.Errorf("relation not found: %s --%s--> %s", c.From, c.Relation, c.To)
	}
	if err := svc.EntityManager().DeleteRelation(ctx, c.From, c.Relation, c.To); err != nil {
		return err
	}
	out.WriteSuccess("Removed link: %s --%s--> %s", c.From, c.Relation, c.To)
	return nil
}
