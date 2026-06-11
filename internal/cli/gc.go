package cli

import (
	"errors"
	"fmt"
)

// GcCmd garbage-collects orphaned files (e.g. interrupted-write temp files).
type GcCmd struct {
	DryRun    bool `name:"dry-run" help:"Show what would be removed without actually removing."`
	TempFiles bool `name:"temp-files" help:"Clean up orphaned .new files from interrupted transactions."`
}

// Run dispatches `rela gc`.
func (c *GcCmd) Run(svc *cliServices) error {
	if !c.TempFiles {
		return errors.New("specify --temp-files")
	}
	return gcOrphanedTempFiles(svc, c.DryRun)
}

func gcOrphanedTempFiles(svc *cliServices, dryRun bool) error {
	orphaned, err := svc.FindOrphanedTempFiles()
	if err != nil {
		return fmt.Errorf("find orphaned files: %w", err)
	}
	if len(orphaned) == 0 {
		out.WriteMessage("No orphaned temp files found")
		return nil
	}
	if dryRun {
		out.WriteMessage("Would remove %d orphaned temp file(s):", len(orphaned))
		for _, path := range orphaned {
			out.WriteMessage("  %s", path)
		}
		return nil
	}
	count, err := svc.CleanupOrphanedTempFiles()
	if err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}
	out.WriteSuccess("Removed %d orphaned temp file(s)", count)
	return nil
}
