package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/attachment"
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

		infos, err := ws.ListAttachments(entityID)
		if err != nil {
			return err
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
			if len(info.OriginalName) > origWidth {
				origWidth = len(info.OriginalName)
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
			original := info.OriginalName
			if original == "" {
				original = "-"
			}
			size := "-"
			if info.Size > 0 {
				size = attachment.FormatSize(info.Size)
			}
			out.WriteMessage(format, info.Property, info.Path, original, size)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(attachmentsCmd)
}
