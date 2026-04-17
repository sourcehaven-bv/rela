package cli

import (
	"github.com/spf13/cobra"
)

var (
	syncForce bool
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize graph from markdown files",
	Long: `Rebuilds the in-memory graph and cache from markdown files.

This command is useful after manually editing markdown files or when
the cache appears to be out of sync.

Examples:
  rela sync          # Sync from files
  rela sync --force  # Force full rebuild`,
	RunE: func(cmd *cobra.Command, args []string) error {
		out.WriteMessage("Syncing from markdown files...")

		result, err := ws.Sync()
		if err != nil {
			return err
		}

		// Report errors
		for _, syncErr := range result.Errors {
			out.WriteWarning("%v", syncErr)
		}

		out.WriteSuccess("Synced %d entities and %d relations", result.EntitiesLoaded, result.RelationsLoaded)

		if len(result.Errors) > 0 {
			out.WriteWarning("%d warnings during sync", len(result.Errors))
		}

		return nil
	},
}

func init() {
	syncCmd.Flags().BoolVar(&syncForce, "force", false, "Force full rebuild")

	rootCmd.AddCommand(syncCmd)
}
