package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

var (
	gcDryRun      bool
	gcAttachments bool
)

var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Garbage collect unreferenced files",
	Long: `Remove unreferenced files from the project.

Currently supports cleaning up attachments that are no longer referenced
by any entity.

Examples:
  rela gc --attachments           # Remove unreferenced attachment files
  rela gc --attachments --dry-run # Show what would be removed`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !gcAttachments {
			return fmt.Errorf("specify --attachments to clean up attachment files")
		}

		if gcAttachments {
			return gcAttachmentFiles()
		}

		return nil
	},
}

func gcAttachmentFiles() error {
	referencedPaths := collectReferencedAttachmentPaths()
	store := attachment.NewStore(cliFS, projectCtx.Root)

	result, err := store.GC(referencedPaths)
	if err != nil {
		return fmt.Errorf("gc failed: %w", err)
	}

	if len(result.Removed) == 0 {
		out.WriteMessage("No unreferenced attachments found")
		return nil
	}

	if gcDryRun {
		out.WriteMessage("Would remove %d unreferenced attachment(s) (%s):",
			len(result.Removed), attachment.FormatSize(result.Reclaimed))
		for _, path := range result.Removed {
			out.WriteMessage("  %s", path)
		}
		return nil
	}

	if err := store.RemoveUnreferenced(result); err != nil {
		return fmt.Errorf("failed to remove files: %w", err)
	}

	out.WriteSuccess("Removed %d unreferenced attachment(s), reclaimed %s",
		len(result.Removed), attachment.FormatSize(result.Reclaimed))

	return nil
}

// collectReferencedAttachmentPaths returns all attachment paths referenced by entities.
func collectReferencedAttachmentPaths() []string {
	var paths []string

	for _, entity := range g.AllNodes() {
		entityDef, ok := meta.GetEntityDef(entity.Type)
		if !ok {
			continue
		}

		for propName, propDef := range entityDef.Properties {
			if propDef.Type != metamodel.PropertyTypeFile {
				continue
			}

			paths = append(paths, extractAttachmentPaths(entity.Properties[propName])...)
		}
	}

	return paths
}

// extractAttachmentPaths extracts attachment paths from a property value.
func extractAttachmentPaths(val interface{}) []string {
	if val == nil {
		return nil
	}

	switch v := val.(type) {
	case string:
		if v != "" {
			return []string{v}
		}
	case []interface{}:
		var paths []string
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				paths = append(paths, s)
			}
		}
		return paths
	case []string:
		return v
	}

	return nil
}

func init() {
	gcCmd.Flags().BoolVar(&gcDryRun, "dry-run", false, "Show what would be removed without actually removing")
	gcCmd.Flags().BoolVar(&gcAttachments, "attachments", false, "Clean up unreferenced attachment files")

	rootCmd.AddCommand(gcCmd)
}
