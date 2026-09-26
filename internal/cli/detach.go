package cli

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/attachment"
)

// DetachCmd removes an attachment from an entity property. When the
// property holds several attachments, --file selects which one.
type DetachCmd struct {
	File     string `short:"f" help:"File name to detach (required when the property holds more than one)."`
	EntityID string `arg:"" name:"entity-id" help:"Target entity ID."`
	Property string `arg:"" help:"Property name."`
}

// Run dispatches `rela detach <entity-id> <property> [--file <name>]`.
func (c *DetachCmd) Run(ctx context.Context, att *attachment.Service) error {
	removed, err := att.Detach(ctx, c.EntityID, c.Property, c.File)
	if err != nil {
		return err
	}
	if removed == "" {
		out.WriteSuccess("%s was not attached to %s.%s; nothing to detach", c.File, c.EntityID, c.Property)
		return nil
	}
	out.WriteSuccess("Detached %s from %s.%s", removed, c.EntityID, c.Property)
	return nil
}
