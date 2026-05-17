package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	relaerrors "github.com/Sourcehaven-BV/rela/internal/errors"
	relamcp "github.com/Sourcehaven-BV/rela/internal/mcp"
)

// coverage-ignore: MCP command - requires stdio server
var mcpCmd = &cobra.Command{
	Use:         "mcp",
	Short:       "Start the MCP (Model Context Protocol) server",
	Annotations: map[string]string{skipProjectDiscovery: "true"},
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

	svc, err := newMCPServices(startDir)
	if err != nil {
		// project.Discover signals "no project here" with a distinct
		// error; everything else (metamodel parse error, store open
		// failure, etc.) is a real diagnostic the operator needs.
		if errors.Is(err, relaerrors.ErrNoProject) {
			return errors.New("no project found: run 'rela init' to create one")
		}
		return fmt.Errorf("mcp startup: %w", err)
	}
	defer svc.Close()

	// Override the cli-stamped Principal with Tool=mcp. Every audit
	// record produced by tool handlers within this server inherits
	// this Principal via the request ctx. See plan AC4.
	mcpPrincipal := audit.Principal{
		User: audit.SystemUser(),
		Tool: audit.ToolMCP,
	}
	srv, srvErr := relamcp.NewServer(svc, Version, relamcp.WithPrincipal(mcpPrincipal))
	if srvErr != nil {
		return fmt.Errorf("mcp startup: %w", srvErr)
	}
	return srv.Serve()
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
