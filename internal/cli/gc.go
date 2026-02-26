package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/output"
)

var (
	gcDryRun      bool
	gcAttachments bool
	gcTempFiles   bool
)

var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Garbage collect unreferenced files",
	Long: `Remove unreferenced files from the project.

Supports cleaning up:
  --attachments  Remove attachment files no longer referenced by entities
  --temp-files   Remove orphaned .new files from interrupted transactions

Examples:
  rela gc --attachments           # Remove unreferenced attachment files
  rela gc --temp-files            # Remove orphaned temp files
  rela gc --attachments --dry-run # Show what would be removed`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !gcAttachments && !gcTempFiles {
			return fmt.Errorf("specify at least one of --attachments or --temp-files")
		}

		if gcTempFiles {
			if err := gcOrphanedTempFiles(); err != nil {
				return err
			}
		}

		if gcAttachments {
			if err := gcAttachmentFiles(); err != nil {
				return err
			}
		}

		return nil
	},
}

func gcAttachmentFiles() error {
	result, err := ws.GCAttachments(gcDryRun)
	if err != nil {
		return err
	}

	if len(result.Removed) == 0 {
		out.WriteMessage("No unreferenced attachments found")
		return nil
	}

	if gcDryRun {
		out.WriteMessage("Would remove %d unreferenced attachment(s) (%s):",
			len(result.Removed), output.FormatSize(result.Reclaimed))
		for _, path := range result.Removed {
			out.WriteMessage("  %s", path)
		}
		return nil
	}

	out.WriteSuccess("Removed %d unreferenced attachment(s), reclaimed %s",
		len(result.Removed), output.FormatSize(result.Reclaimed))

	return nil
}

func gcOrphanedTempFiles() error {
	orphaned, err := ws.FindOrphanedTempFiles()
	if err != nil {
		return fmt.Errorf("find orphaned files: %w", err)
	}

	if len(orphaned) == 0 {
		out.WriteMessage("No orphaned temp files found")
		return nil
	}

	if gcDryRun {
		out.WriteMessage("Would remove %d orphaned temp file(s):", len(orphaned))
		for _, path := range orphaned {
			out.WriteMessage("  %s", path)
		}
		return nil
	}

	count, err := ws.CleanupOrphanedTempFiles()
	if err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}

	out.WriteSuccess("Removed %d orphaned temp file(s)", count)
	return nil
}

func init() {
	gcCmd.Flags().BoolVar(&gcDryRun, "dry-run", false, "Show what would be removed without actually removing")
	gcCmd.Flags().BoolVar(&gcAttachments, "attachments", false, "Clean up unreferenced attachment files")
	gcCmd.Flags().BoolVar(&gcTempFiles, "temp-files", false, "Clean up orphaned .new files from interrupted transactions")

	rootCmd.AddCommand(gcCmd)
}
