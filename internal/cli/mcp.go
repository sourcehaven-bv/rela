package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
	relamcp "github.com/Sourcehaven-BV/rela/internal/mcp"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// coverage-ignore: MCP command - requires stdio server
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the MCP (Model Context Protocol) server",
	Long: `Starts a Model Context Protocol server on stdio.

The MCP server exposes rela's capabilities to AI assistants and other MCP clients.
It provides tools for entity/relation management, graph analysis, and tracing.

Configuration for Claude Code (.claude.json):
  {
    "mcpServers": {
      "rela": {
        "command": "rela",
        "args": ["mcp"],
        "cwd": "/path/to/project"
      }
    }
  }

The server communicates via JSON-RPC over stdin/stdout.
Logs are written to stderr.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMCPServer()
	},
}

func runMCPServer() error {
	// Discover project (standalone initialization because mcp is excluded
	// from the root PersistentPreRunE skip list)
	ctx, err := project.Discover("")
	if err != nil {
		return fmt.Errorf("no project found: run 'rela init' to create one")
	}

	m, err := metamodel.Load(ctx.MetamodelPath)
	if err != nil {
		return fmt.Errorf("failed to load metamodel: %w", err)
	}

	gr := graph.New()
	if graph.CacheExists(ctx.CachePath) {
		if cacheErr := gr.LoadCache(ctx.CachePath); cacheErr != nil {
			if _, syncErr := markdown.SyncFromFiles(ctx, m, gr); syncErr != nil {
				return fmt.Errorf("failed to sync: %w", syncErr)
			}
		}
	} else {
		if _, syncErr := markdown.SyncFromFiles(ctx, m, gr); syncErr != nil {
			return fmt.Errorf("failed to sync: %w", syncErr)
		}
	}

	srv := relamcp.NewServer(gr, m, ctx, Version)
	return srv.Serve()
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
