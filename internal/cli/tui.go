package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/tui"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive terminal UI",
	Long: `Launches an interactive terminal user interface for browsing
and managing entities.

The TUI provides:
  - Entity browser with type filtering
  - Entity detail view with relationships
  - Create and link entities
  - Search across all entities
  - Relationship graph visualization
  - Analysis checks

Keyboard shortcuts:
  ? - Show help
  / - Search
  q - Quit or go back
  Esc - Go back`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Try to discover project
		ctx, err := project.Discover("")
		if err != nil {
			// No project found - launch TUI in init mode
			cwd, cwdErr := os.Getwd()
			if cwdErr != nil {
				return cwdErr
			}
			return tui.RunInit(cwd)
		}

		// Load metamodel
		mm, err := metamodel.Load(ctx.MetamodelPath)
		if err != nil {
			return err
		}

		// Initialize graph
		gr := graph.New()

		// Try to load from cache first
		if graph.CacheExists(ctx.CachePath) {
			if err := gr.LoadCache(ctx.CachePath); err != nil {
				// Cache load failed, sync from files
				if _, err := markdown.SyncFromFiles(ctx, mm, gr); err != nil {
					return err
				}
			}
		} else {
			// No cache, sync from files
			if _, err := markdown.SyncFromFiles(ctx, mm, gr); err != nil {
				return err
			}
		}

		return tui.Run(ctx, mm, gr)
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
