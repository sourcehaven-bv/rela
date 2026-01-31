package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	relamcp "github.com/Sourcehaven-BV/rela/internal/mcp"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
)

// coverage-ignore: MCP command - requires stdio server
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the MCP (Model Context Protocol) server",
	Long: `Starts a Model Context Protocol server on stdio.

The MCP server exposes rela's capabilities to AI assistants and other MCP clients.
It provides tools for entity/relation management, graph analysis, and tracing.

Setup with claude mcp add (recommended):
  claude mcp add rela -s local -- /path/to/rela mcp

Setup with .mcp.json (for sharing via git):
  {
    "mcpServers": {
      "rela": {
        "command": "rela",
        "args": ["mcp"]
      }
    }
  }

Claude Code launches MCP servers with the project directory as cwd,
so rela mcp finds metamodel.yaml automatically.

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

	mcpRepo := repository.New(cliFS, ctx)

	m, err := mcpRepo.LoadMetamodel()
	if err != nil {
		return fmt.Errorf("failed to load metamodel: %w", err)
	}

	gr := graph.New()
	if mcpRepo.CacheExists() {
		if cacheErr := mcpRepo.LoadCache(gr); cacheErr != nil {
			if _, syncErr := mcpRepo.Sync(m, gr); syncErr != nil {
				return fmt.Errorf("failed to sync: %w", syncErr)
			}
		}
	} else {
		if _, syncErr := mcpRepo.Sync(m, gr); syncErr != nil {
			return fmt.Errorf("failed to sync: %w", syncErr)
		}
	}

	srv := relamcp.NewServer(gr, m, ctx, Version)
	return srv.Serve()
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
