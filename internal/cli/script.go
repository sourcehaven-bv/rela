package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/lua"
)

var scriptCmd = &cobra.Command{
	Use:   "script <file.lua> [args...]",
	Short: "Execute a Lua script against the graph",
	Long: `Execute a Lua script with access to the rela graph.

Scripts can query entities and relations, apply filters, trace dependencies,
and output results as JSON or write files.

Available functions in the rela module:
  rela.get_entity(id)              Get entity by ID (returns table or nil)
  rela.list_entities(type, filter) List entities with optional filter
  rela.get_relations(opts)         Get relations (opts: {from, type, to})
  rela.trace_from(id, depth)       Trace outgoing dependencies
  rela.trace_to(id, depth)         Trace incoming dependencies
  rela.output(data)                Output data as JSON to stdout
  rela.write_file(path, content)   Write content to file

Context:
  rela.args                        Script arguments (table)
  rela.project_root                Project root path

Example:
  rela script scripts/export.lua
  rela script scripts/report.lua --format=json`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		scriptPath := args[0]
		scriptArgs := args[1:]

		runtime := lua.New(ws, meta, projectCtx.Root, os.Stdout)
		defer runtime.Close()

		return runtime.RunFile(scriptPath, scriptArgs)
	},
}

func init() {
	rootCmd.AddCommand(scriptCmd)
}
