package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

var detachCmd = &cobra.Command{
	Use:   "detach <entity-id> <property> [hash-prefix]",
	Short: "Remove attachment reference from an entity",
	Long: `Remove an attachment reference from an entity.

This removes the reference from the entity's property but does NOT delete the
actual file. Use 'rela gc --attachments' to clean up unreferenced files.

If the property contains multiple attachments, provide a hash prefix to specify
which one to remove. Without a prefix, removes all attachments from the property.

Examples:
  rela detach BUG-042 screenshot
  rela detach DEC-007 supporting-docs ab3f
  rela detach DEC-007 supporting-docs --all`,
	Args: cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		entityID := args[0]
		propName := args[1]
		hashPrefix := ""
		if len(args) > 2 {
			hashPrefix = args[2]
		}

		// Get entity from graph
		entity, ok := g.GetNode(entityID)
		if !ok {
			return fmt.Errorf("entity not found: %s", entityID)
		}

		// Get entity definition
		entityDef, ok := meta.GetEntityDef(entity.Type)
		if !ok {
			return fmt.Errorf("unknown entity type: %s", entity.Type)
		}

		// Validate property exists and is file type
		propDef, ok := entityDef.Properties[propName]
		if !ok {
			return fmt.Errorf("property %q not defined for entity type %s", propName, entity.Type)
		}
		if propDef.Type != metamodel.PropertyTypeFile {
			return fmt.Errorf("property %q is not a file type (is %s)", propName, propDef.Type)
		}

		// Get current value
		val, ok := entity.Properties[propName]
		if !ok || val == nil {
			return fmt.Errorf("property %q has no attachments", propName)
		}

		// Handle single value or list
		var currentPaths []string
		switch v := val.(type) {
		case string:
			if v != "" {
				currentPaths = append(currentPaths, v)
			}
		case []interface{}:
			for _, item := range v {
				if s, ok := item.(string); ok && s != "" {
					currentPaths = append(currentPaths, s)
				}
			}
		case []string:
			currentPaths = append(currentPaths, v...)
		}

		if len(currentPaths) == 0 {
			return fmt.Errorf("property %q has no attachments", propName)
		}

		// Find attachments to remove
		var remaining []string
		var removed []string

		for _, path := range currentPaths {
			if hashPrefix == "" {
				// Remove all
				removed = append(removed, path)
			} else {
				// Check if path contains the hash prefix
				// Path format: attachments/ab/ab3f8c2e9d1a.png
				// Hash is between last / and last .
				parts := strings.Split(path, "/")
				if len(parts) >= 1 {
					filename := parts[len(parts)-1]
					hash := strings.TrimSuffix(filename, filepath.Ext(filename))
					if strings.HasPrefix(hash, hashPrefix) {
						removed = append(removed, path)
						continue
					}
				}
				remaining = append(remaining, path)
			}
		}

		if len(removed) == 0 {
			if hashPrefix != "" {
				return fmt.Errorf("no attachment found with hash prefix %q", hashPrefix)
			}
			return fmt.Errorf("no attachments to remove")
		}

		// Clone before mutation so workspace can diff old vs new.
		oldEntity := entity.Clone()

		// Update entity property
		if len(remaining) == 0 {
			delete(entity.Properties, propName)
		} else if len(remaining) == 1 {
			entity.Properties[propName] = remaining[0]
		} else {
			entity.Properties[propName] = remaining
		}

		// Write through workspace (validates, persists, updates graph+cache).
		if _, err := ws.UpdateEntity(entity, oldEntity); err != nil {
			return fmt.Errorf("failed to update entity: %w", err)
		}

		for _, path := range removed {
			out.WriteSuccess("Detached %s from %s.%s", path, entityID, propName)
		}
		out.WriteMessage("Note: Files remain in storage. Use 'rela gc --attachments' to clean up.")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(detachCmd)
}
