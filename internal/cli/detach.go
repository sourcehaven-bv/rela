package cli

import (
	"context"
)

// DetachCmd removes an attachment from an entity property. When the
// property holds several attachments, --file selects which one.
type DetachCmd struct {
	File     string `short:"f" help:"File name to detach (required when the property holds more than one)."`
	EntityID string `arg:"" name:"entity-id" help:"Target entity ID."`
	Property string `arg:"" help:"Property name."`
}

// Run dispatches `rela detach <entity-id> <property> [--file <name>]`.
func (c *DetachCmd) Run(ctx context.Context, svc *cliServices) error {
	if err := svc.DetachFile(ctx, c.EntityID, c.Property, c.File); err != nil {
		return err
	}
	if c.File != "" {
		out.WriteSuccess("Detached %s from %s.%s", c.File, c.EntityID, c.Property)
	} else {
		out.WriteSuccess("Detached attachment from %s.%s", c.EntityID, c.Property)
	}
	return nil
}
