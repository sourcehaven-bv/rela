package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

var detachCmd = &cobra.Command{
	Use:   "detach <entity-id> <property>",
	Short: "Remove the attachment from an entity property",
	Long: `Remove the attachment referenced by an entity property.

Clears the property on the entity and deletes the underlying file
from the attachment store. Each file-type property holds at most one
attachment, so no further disambiguation is needed.

Examples:
  rela detach BUG-042 screenshot
  rela detach DEC-007 supporting-docs`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		entityID := args[0]
		propName := args[1]
		svc := cliWriteFromContext(cmd.Context())

		ctx := context.Background()
		e, err := svc.Store().GetEntity(ctx, entityID)
		if err != nil {
			return fmt.Errorf("entity not found: %s", entityID)
		}

		entityDef, ok := svc.Meta().GetEntityDef(e.Type)
		if !ok {
			return fmt.Errorf("unknown entity type: %s", e.Type)
		}

		propDef, ok := entityDef.Properties[propName]
		if !ok {
			return fmt.Errorf("property %q not defined for entity type %s", propName, e.Type)
		}
		if propDef.Type != metamodel.PropertyTypeFile {
			return fmt.Errorf("property %q is not a file type (is %s)", propName, propDef.Type)
		}

		val, ok := e.Properties[propName]
		if !ok || val == nil || val == "" {
			return fmt.Errorf("property %q has no attachment", propName)
		}

		if err := svc.Store().DeleteAttachment(ctx, entityID, propName); err != nil {
			return fmt.Errorf("delete attachment: %w", err)
		}

		delete(e.Properties, propName)
		if _, err := svc.EntityManager().UpdateEntity(ctx, e); err != nil {
			return fmt.Errorf("update entity: %w", err)
		}

		out.WriteSuccess("Detached attachment from %s.%s", entityID, propName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(detachCmd)
}
