package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	traceMaxDepth int
)

var traceCmd = &cobra.Command{
	Use:   "trace",
	Short: "Trace dependencies between entities",
	Long: `Trace dependencies between entities in various directions.

Subcommands:
  from  - Trace downstream dependencies (what depends on this)
  to    - Trace upstream dependencies (what this depends on)
  path  - Find a path between two entities`,
}

var traceFromCmd = &cobra.Command{
	Use:   "from <id>",
	Short: "Trace downstream dependencies",
	Long: `Shows all entities that are reachable from the given entity
by following outgoing relations.

Examples:
  rela trace from REQ-001
  rela trace from REQ-001 --depth 2`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		entityID := args[0]

		if _, ok := g.GetNode(entityID); !ok {
			return &entityNotFoundError{ID: entityID}
		}

		result := g.TraceFrom(entityID, traceMaxDepth)
		if result == nil {
			out.WriteMessage("No downstream dependencies found")
			return nil
		}

		return out.WriteTrace(result)
	},
}

var traceToCmd = &cobra.Command{
	Use:   "to <id>",
	Short: "Trace upstream dependencies",
	Long: `Shows all entities that lead to the given entity
by following incoming relations.

Examples:
  rela trace to COMP-001
  rela trace to COMP-001 --depth 3`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		entityID := args[0]

		if _, ok := g.GetNode(entityID); !ok {
			return &entityNotFoundError{ID: entityID}
		}

		result := g.TraceTo(entityID, traceMaxDepth)
		if result == nil {
			out.WriteMessage("No upstream dependencies found")
			return nil
		}

		return out.WriteTrace(result)
	},
}

var tracePathCmd = &cobra.Command{
	Use:   "path <from> <to>",
	Short: "Find a path between two entities",
	Long: `Finds the shortest path between two entities.

Examples:
  rela trace path REQ-001 COMP-001`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fromID := args[0]
		toID := args[1]

		if _, ok := g.GetNode(fromID); !ok {
			return fmt.Errorf("source entity not found: %s", fromID)
		}
		if _, ok := g.GetNode(toID); !ok {
			return fmt.Errorf("target entity not found: %s", toID)
		}

		path := g.FindPath(fromID, toID)
		if path == nil {
			out.WriteMessage("No path found between %s and %s", fromID, toID)
			return nil
		}

		return out.WritePath(path)
	},
}

func init() {
	traceCmd.PersistentFlags().IntVar(&traceMaxDepth, "depth", 0, "Maximum depth to trace (0 = unlimited)")

	traceCmd.AddCommand(traceFromCmd)
	traceCmd.AddCommand(traceToCmd)
	traceCmd.AddCommand(tracePathCmd)

	rootCmd.AddCommand(traceCmd)
}
