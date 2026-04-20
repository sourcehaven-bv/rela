package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	gcDryRun    bool
	gcTempFiles bool
)

var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Garbage collect orphaned files",
	Long: `Remove orphaned files left behind by interrupted writes.

Supports cleaning up:
  --temp-files   Remove orphaned .new files from interrupted writes

Attachments live 1:1 with their owning entity — deleting an entity
takes its attachments with it, so there is no separate attachment GC
pass to run.

Examples:
  rela gc --temp-files            # Remove orphaned temp files
  rela gc --temp-files --dry-run  # Show what would be removed`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !gcTempFiles {
			return errors.New("specify --temp-files")
		}
		return gcOrphanedTempFiles()
	},
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
	gcCmd.Flags().BoolVar(&gcTempFiles, "temp-files", false, "Clean up orphaned .new files from interrupted transactions")

	rootCmd.AddCommand(gcCmd)
}
