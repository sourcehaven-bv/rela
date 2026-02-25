package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	relamcp "github.com/Sourcehaven-BV/rela/internal/mcp"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
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
	// Determine start directory: flag > env var > cwd
	startDir := projectPath
	if startDir == "" {
		startDir = os.Getenv("RELA_PROJECT")
	}

	// Discover project (standalone initialization because mcp is excluded
	// from the root PersistentPreRunE skip list)
	ctx, err := project.Discover(startDir, cliFS)
	if err != nil {
		return fmt.Errorf("no project found: run 'rela init' to create one")
	}

	mcpRepo := repository.New(cliFS, ctx)

	mcpWs, err := workspace.New(mcpRepo)
	if err != nil {
		return fmt.Errorf("failed to initialize workspace: %w", err)
	}

	srv := relamcp.NewServer(mcpWs, Version)
	return srv.Serve()
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
