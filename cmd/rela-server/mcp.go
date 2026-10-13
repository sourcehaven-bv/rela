package main

import (
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/dataentrywire"
)

// mcpServerVersion is the version rela reports in the MCP `initialize` /
// `server/discover` response. It is a distinct var from the CLI's Version
// (which rela-server does not import — it has no CLI command surface) and is
// overridable at build time via
// -ldflags "-X main.mcpServerVersion=$(git describe --tags)".
//
// It is informational: MCP clients log it, they do not gate on it. "dev" in
// an unstamped build is honest rather than a wrong number.
var mcpServerVersion = "dev"

// wireRemoteMCP enables the HTTP MCP endpoint when -mcp is set. The server it
// mounts is built by [dataentrywire.RemoteMCP], which says how identity, reads
// and tools are wired.
//
// It returns an error rather than exiting so the caller owns the exit, the
// same shape as validateIdentityFlags. A failure here MUST stop startup: the
// operator asked for MCP, and booting without it would serve a server that
// silently lacks the feature they enabled.
func wireRemoteMCP(app *dataentry.App, svc *appbuild.Services, f *serverFlags) error {
	if !f.remoteMCP {
		return nil
	}

	return dataentrywire.RemoteMCP(app, svc, mcpServerVersion)
}
