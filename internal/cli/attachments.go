package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/output"
)

var attachmentsCmd = &cobra.Command{
	Use:   "attachments <entity-id>",
	Short: "List attachments for an entity",
	Long: `List all file attachments for an entity.

Shows the property name, path, and size for each attachment.

Examples:
  rela attachments BUG-042
  rela attachments DEC-007`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		entityID := args[0]
		svc := cliAnalyzeFromContext(cmd.Context())

		infos, err := svc.ListAttachments(cmd.Context(), entityID)
		if err != nil {
			return err
		}

		if len(infos) == 0 {
			out.WriteMessage("No attachments found for %s", entityID)
			return nil
		}

		out.WriteMessage("Attachments for %s:\n", entityID)

		propWidth := len("PROPERTY")
		pathWidth := len("PATH")
		for _, info := range infos {
			if len(info.Property) > propWidth {
				propWidth = len(info.Property)
			}
			if len(info.Path) > pathWidth {
				pathWidth = len(info.Path)
			}
		}

		format := fmt.Sprintf("  %%-%ds  %%-%ds  %%s\n", propWidth, pathWidth)
		out.WriteMessage(format, "PROPERTY", "PATH", "SIZE")
		out.WriteMessage(format,
			strings.Repeat("-", propWidth),
			strings.Repeat("-", pathWidth),
			"----")

		for _, info := range infos {
			size := "-"
			if info.Size > 0 {
				size = output.FormatSize(info.Size)
			}
			out.WriteMessage(format, info.Property, info.Path, size)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(attachmentsCmd)
}
