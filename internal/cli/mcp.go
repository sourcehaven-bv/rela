package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/mcpwire"

	relaerrors "github.com/Sourcehaven-BV/rela/internal/errors"
	relamcp "github.com/Sourcehaven-BV/rela/internal/mcp"
)

// openMCPServices opens the project for `rela mcp` and warns when acl.yaml
// assigns the identity no role.
func openMCPServices(startDir string, identity localIdentity) (*mcpServices, error) {
	svc, err := newMCPServices(startDir, identity.runAs())
	if err != nil {
		if errors.Is(err, relaerrors.ErrNoProject) {
			return nil, errors.New("no project found: run 'rela init' to create one")
		}
		return nil, fmt.Errorf("mcp startup: %w", err)
	}
	warnUnassignedUser(svc.current(), identity.Principal.User)
	return svc, nil
}

// McpCmd starts the MCP (Model Context Protocol) server on stdio.
//
// coverage-ignore: MCP command - requires stdio server
type McpCmd struct{}

// Run dispatches `rela mcp`.
func (c *McpCmd) Run(identity localIdentity) error {
	// coverage-ignore-start: external-tool: starts the MCP stdio server (blocking Serve loop over os.Stdin/os.Stdout);
	// requires a real project +
	// stdio transport
	startDir := projectPath
	if startDir == "" {
		startDir = os.Getenv("RELA_PROJECT")
	}

	svc, err := openMCPServices(startDir, identity)
	if err != nil {
		return err
	}
	defer svc.Close()

	srv, srvErr := relamcp.NewServer(svc.Deps(), Version,
		relamcp.WithPrincipal(identity.Principal), relamcp.WithAssertedBy(identity.AssertedBy),
		relamcp.WithLuaTools(),
		// Bound to the startup piles service. A schema reload shares its
		// backend (ForReassembly) and rebuilds only the owner check, which
		// under the MCP server's NopACL never has person mapping to change.
		mcpwire.Piles(svc.current()))
	if srvErr != nil {
		return fmt.Errorf("mcp startup: %w", srvErr)
	}

	// Pick up schema.yaml edits without a restart (TKT-NU247U). Never fails
	// startup: a project or backend that cannot be watched just serves the
	// schema it booted with.
	svc.watchSchema(srv)

	// The lease watcher lives as long as Serve: it wakes lease subscribers
	// for claims made by other processes and for expiries. It runs on the
	// startup assembly, whose lease backend and hub every reload shares.
	life, stop := context.WithCancel(context.Background())
	watched := appbuild.WatchLeases(life, svc.current())
	defer func() {
		stop()
		<-watched
	}()
	return srv.Serve(life)
	// coverage-ignore-end
}
