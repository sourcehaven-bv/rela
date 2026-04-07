package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/ai"
	"github.com/Sourcehaven-BV/rela/internal/lua"
)

var scriptOutputDir string

var scriptCmd = &cobra.Command{
	Use:   "script <file.lua> [args...]",
	Short: "Execute a Lua script against the graph",
	Long: `Execute a Lua script with access to the rela graph.

Scripts can query entities and relations, apply filters, trace dependencies,
create/update/delete entities and relations, and output results.

Query functions:
  rela.get_entity(id)              Get entity by ID (returns table or nil)
  rela.list_entities(type, filter) List entities with optional filter
  rela.search(query, limit?)       Full-text search (default limit: 20)
  rela.get_relations(opts)         Get relations (opts: {from, type, to})
  rela.trace_from(id, depth)       Trace outgoing dependencies
  rela.trace_to(id, depth)         Trace incoming dependencies
  rela.find_path(from, to)         Find shortest path between entities

Mutation functions:
  rela.create_entity(type, props, content?, id?)  Create new entity
  rela.update_entity(id, props, content?)         Update entity properties
  rela.delete_entity(id, cascade?)                Delete entity
  rela.create_relation(from, type, to)            Create relation
  rela.delete_relation(from, type, to)            Delete relation
  rela.refresh()                                  Reload graph from disk

Schema introspection:
  rela.get_entity_types()            Get all entity types with properties
  rela.get_relation_types()          Get all relation types with constraints

Output functions:
  rela.output(data)                Output data as JSON to stdout
  rela.write_file(path, content)   Write content to file (relative to --output-dir)

Context:
  rela.args                        Script arguments (table)
  rela.project_root                Project root path

Scripts can include a shebang line (#!/usr/bin/env -S rela script) for direct
execution. The shebang is automatically stripped before running.

Example:
  rela script scripts/export.lua
  rela script scripts/report.lua --format=json
  rela script scripts/docs.lua --output-dir=/path/to/docs`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		scriptPath := args[0]
		scriptArgs := args[1:]

		opts := []lua.Option{lua.WithContext(cmd.Context())}
		if scriptOutputDir != "" {
			opts = append(opts, lua.WithOutputDir(scriptOutputDir))
		}
		// AI is often the whole point of running a script, so a
		// misconfigured ai.yaml should surface immediately rather
		// than silently disable AI and let the script blow up later
		// with a not_configured error. ErrConfigNotFound is the
		// normal "no AI" state and is not propagated.
		provider, err := ai.LoadProvider(projectCtx.CacheDir)
		switch {
		case errors.Is(err, ai.ErrConfigNotFound):
			// no AI configured; the Lua bindings will return
			// not_configured if the script tries to call ai.*
		case err != nil:
			return fmt.Errorf("ai: %w", err)
		default:
			opts = append(opts, lua.WithAIProvider(provider))
		}

		runtime := lua.New(ws, meta, projectCtx.Root, os.Stdout, opts...)
		defer runtime.Close()

		return runtime.RunFile(scriptPath, scriptArgs)
	},
}

func init() {
	scriptCmd.Flags().StringVar(&scriptOutputDir, "output-dir", "",
		"Directory for write_file output (default: {project}/output)")
	rootCmd.AddCommand(scriptCmd)
}
