package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

var attachmentsCmd = &cobra.Command{
	Use:   "attachments <entity-id>",
	Short: "List attachments for an entity",
	Long: `List all file attachments for an entity.

Shows the property name, file path, original filename, and size for each attachment.

Examples:
  rela attachments BUG-042
  rela attachments DEC-007`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		entityID := args[0]

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

		// Create attachment store for metadata lookup
		store := attachment.NewStore(ws.FS(), ws.Paths().Root)

		// Collect attachments from all file properties
		type attachmentInfo struct {
			Property string
			Path     string
			Original string
			Size     string
		}

		var infos []attachmentInfo

		for propName, propDef := range entityDef.Properties {
			if propDef.Type != metamodel.PropertyTypeFile {
				continue
			}

			val, ok := entity.Properties[propName]
			if !ok || val == nil {
				continue
			}

			// Handle single value or list
			var paths []string
			switch v := val.(type) {
			case string:
				if v != "" {
					paths = append(paths, v)
				}
			case []interface{}:
				for _, item := range v {
					if s, ok := item.(string); ok && s != "" {
						paths = append(paths, s)
					}
				}
			case []string:
				paths = append(paths, v...)
			}

			for _, path := range paths {
				info := attachmentInfo{
					Property: propName,
					Path:     path,
					Original: "-",
					Size:     "-",
				}

				// Try to get metadata
				if meta, err := store.GetMetadata(path); err == nil {
					info.Original = meta.OriginalName
					info.Size = attachment.FormatSize(meta.Size)
				}

				infos = append(infos, info)
			}
		}

		if len(infos) == 0 {
			out.WriteMessage("No attachments found for %s", entityID)
			return nil
		}

		// Print table
		out.WriteMessage("Attachments for %s:\n", entityID)

		// Calculate column widths
		propWidth := len("PROPERTY")
		pathWidth := len("PATH")
		origWidth := len("ORIGINAL")

		for _, info := range infos {
			if len(info.Property) > propWidth {
				propWidth = len(info.Property)
			}
			if len(info.Path) > pathWidth {
				pathWidth = len(info.Path)
			}
			if len(info.Original) > origWidth {
				origWidth = len(info.Original)
			}
		}

		// Print header
		format := fmt.Sprintf("  %%-%ds  %%-%ds  %%-%ds  %%s\n", propWidth, pathWidth, origWidth)
		out.WriteMessage(format, "PROPERTY", "PATH", "ORIGINAL", "SIZE")
		out.WriteMessage(format,
			strings.Repeat("-", propWidth),
			strings.Repeat("-", pathWidth),
			strings.Repeat("-", origWidth),
			"----")

		// Print rows
		for _, info := range infos {
			out.WriteMessage(format, info.Property, info.Path, info.Original, info.Size)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(attachmentsCmd)
}
