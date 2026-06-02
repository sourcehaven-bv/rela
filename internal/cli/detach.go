package cli

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// DetachCmd removes the attachment from an entity property.
type DetachCmd struct {
	EntityID string `arg:"" name:"entity-id" help:"Target entity ID."`
	Property string `arg:"" help:"Property name."`
}

// Run dispatches `rela detach <entity-id> <property>`.
func (c *DetachCmd) Run(ctx context.Context, svc *cliServices) error {
	e, err := svc.Store().GetEntity(ctx, c.EntityID)
	if err != nil {
		return fmt.Errorf("entity not found: %s", c.EntityID)
	}
	entityDef, ok := svc.Meta().GetEntityDef(e.Type)
	if !ok {
		return fmt.Errorf("unknown entity type: %s", e.Type)
	}
	propDef, ok := entityDef.Properties[c.Property]
	if !ok {
		return fmt.Errorf("property %q not defined for entity type %s", c.Property, e.Type)
	}
	if propDef.Type != metamodel.PropertyTypeFile {
		return fmt.Errorf("property %q is not a file type (is %s)", c.Property, propDef.Type)
	}
	val, ok := e.Properties[c.Property]
	if !ok || val == nil || val == "" {
		return fmt.Errorf("property %q has no attachment", c.Property)
	}
	if err := svc.Store().DeleteAttachment(ctx, c.EntityID, c.Property); err != nil {
		return fmt.Errorf("delete attachment: %w", err)
	}
	delete(e.Properties, c.Property)
	if _, err := svc.EntityManager().UpdateEntity(ctx, e); err != nil {
		return fmt.Errorf("update entity: %w", err)
	}
	out.WriteSuccess("Detached attachment from %s.%s", c.EntityID, c.Property)
	return nil
}
