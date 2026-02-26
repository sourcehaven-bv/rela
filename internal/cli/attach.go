package cli

import (
	"fmt"
	"os/user"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

var attachProperty string

var attachCmd = &cobra.Command{
	Use:   "attach <entity-id> <file>...",
	Short: "Attach file(s) to an entity",
	Long: `Attach one or more files to an entity.

Files are stored in a content-addressable store using SHA-256 hashes.
Duplicate files are automatically deduplicated.

The --property flag specifies which property to attach the file(s) to.
If not specified, uses the first file-type property defined for the entity type.

Examples:
  rela attach BUG-042 screenshot.png
  rela attach BUG-042 screenshot.png --property screenshot
  rela attach DEC-007 *.pdf --property supporting-docs
  rela attach REQ-001 diagram.png spec.pdf`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		entityID := args[0]
		filePaths := args[1:]

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

		// Determine which property to use
		propName := attachProperty
		if propName == "" {
			// Find first file-type property
			propName = findFileProperty(entityDef)
			if propName == "" {
				return fmt.Errorf("no file property defined for entity type %s; use --property to specify", entity.Type)
			}
		}

		// Validate property exists and is file type
		propDef, ok := entityDef.Properties[propName]
		if !ok {
			return fmt.Errorf("property %q not defined for entity type %s", propName, entity.Type)
		}
		if propDef.Type != metamodel.PropertyTypeFile {
			return fmt.Errorf("property %q is not a file type (is %s)", propName, propDef.Type)
		}

		// Get current user for metadata
		addedBy := ""
		if u, err := user.Current(); err == nil {
			addedBy = u.Username
		}

		// Create attachment store
		store := attachment.NewStore(ws.FS(), ws.Paths().Root)

		// Process each file
		var attachments []*attachment.Attachment
		for _, filePath := range filePaths {
			// Expand globs
			matches, err := filepath.Glob(filePath)
			if err != nil {
				return fmt.Errorf("invalid glob pattern %q: %w", filePath, err)
			}
			if len(matches) == 0 {
				// No glob match, try as literal path
				matches = []string{filePath}
			}

			for _, match := range matches {
				// Convert to absolute path for reading
				absPath, err := filepath.Abs(match)
				if err != nil {
					return fmt.Errorf("invalid path %q: %w", match, err)
				}

				// Add to store
				att, err := store.Add(absPath, addedBy)
				if err != nil {
					return fmt.Errorf("failed to attach %q: %w", match, err)
				}
				attachments = append(attachments, att)
				out.WriteSuccess("Attached %s → %s", filepath.Base(match), att.Path)
			}
		}

		if len(attachments) == 0 {
			return fmt.Errorf("no files matched")
		}

		// Clone before mutation so workspace can diff old vs new.
		oldEntity := entity.Clone()

		// Update entity property
		// For single file, store as string; for multiple, store as list
		if len(attachments) == 1 {
			entity.SetString(propName, attachments[0].Path)
		} else {
			// Get existing values if any
			var paths []string
			if existing := entity.Properties[propName]; existing != nil {
				switch v := existing.(type) {
				case string:
					if v != "" {
						paths = append(paths, v)
					}
				case []interface{}:
					for _, item := range v {
						if s, ok := item.(string); ok {
							paths = append(paths, s)
						}
					}
				case []string:
					paths = append(paths, v...)
				}
			}

			// Add new paths
			for _, att := range attachments {
				paths = append(paths, att.Path)
			}
			entity.Properties[propName] = paths
		}

		// Write through workspace (validates, persists, updates graph+cache).
		if _, err := ws.UpdateEntity(entity, oldEntity); err != nil {
			return fmt.Errorf("failed to update entity: %w", err)
		}

		return nil
	},
}

// findFileProperty returns the first file-type property name for an entity definition.
func findFileProperty(entityDef *metamodel.EntityDef) string {
	for name, prop := range entityDef.Properties {
		if prop.Type == metamodel.PropertyTypeFile {
			return name
		}
	}
	return ""
}

func init() {
	attachCmd.Flags().StringVarP(&attachProperty, "property", "P", "", "Property to attach file(s) to")
	rootCmd.AddCommand(attachCmd)
}
