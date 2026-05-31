package cli

import (
	"errors"
	"fmt"
	"os"

	relaerrors "github.com/Sourcehaven-BV/rela/internal/errors"
	relamcp "github.com/Sourcehaven-BV/rela/internal/mcp"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// McpCmd starts the MCP (Model Context Protocol) server on stdio.
//
// coverage-ignore: MCP command - requires stdio server
type McpCmd struct{}

// Run dispatches `rela mcp`.
func (c *McpCmd) Run() error {
	startDir := projectPath
	if startDir == "" {
		startDir = os.Getenv("RELA_PROJECT")
	}

	svc, err := newMCPServices(startDir)
	if err != nil {
		if errors.Is(err, relaerrors.ErrNoProject) {
			return errors.New("no project found: run 'rela init' to create one")
		}
		return fmt.Errorf("mcp startup: %w", err)
	}
	defer svc.Close()

	mcpPrincipal := principal.Principal{
		User: principal.SystemUser(),
		Tool: principal.ToolMCP,
	}
	srv, srvErr := relamcp.NewServer(svc.Deps(), Version, relamcp.WithPrincipal(mcpPrincipal))
	if srvErr != nil {
		return fmt.Errorf("mcp startup: %w", srvErr)
	}
	return srv.Serve()
}
