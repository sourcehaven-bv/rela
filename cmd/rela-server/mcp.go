package main

import (
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	relamcp "github.com/Sourcehaven-BV/rela/internal/mcp"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// mcpServerVersion is the version rela reports in the MCP `initialize` /
// `server/discover` response. It is a distinct var from the CLI's Version
// (which rela-server does not import — it has no cobra/kong surface) and is
// overridable at build time via
// -ldflags "-X main.mcpServerVersion=$(git describe --tags)".
//
// It is informational: MCP clients log it, they do not gate on it. "dev" in
// an unstamped build is honest rather than a wrong number.
var mcpServerVersion = "dev"

// wireRemoteMCP enables the HTTP MCP endpoint when -mcp is set.
//
// It returns an error rather than exiting so the caller owns the exit, the
// same shape as validateIdentityFlags. A failure here MUST stop startup: the
// operator asked for MCP, and booting without it would serve a server that
// silently lacks the feature they enabled.
//
// **Identity comes from the request, not from here.** Unlike `rela mcp`
// (stdio), which stamps one process-wide system principal, the remote server
// serves many callers. The MCP server's principal middleware preserves an
// identity already on the ctx, and the transport hands it the *http.Request
// ctx that `requireVerifiedJWT` has stamped — so `WithPrincipal` below is
// only the fallback for a request that somehow carries none. It is a real,
// non-zero system principal (NewServer requires one) rather than a
// placeholder, so an unattributed write is recorded as the server itself
// rather than as a guessed user.
//
// **Reads are ACL-gated.** The read handles come from
// [appbuild.Services.GatedReads], which resolves the ctx principal per call.
// This is the opposite of the stdio wiring's deliberate NopACL: there the
// filesystem is the trust boundary (anyone who can run `rela mcp` can edit
// the files directly), so a gate would defend nothing. A remote caller has no
// filesystem access, so the ACL is the ONLY boundary.
func wireRemoteMCP(app *dataentry.App, svc *appbuild.Services, f *serverFlags) error {
	if !f.remoteMCP {
		return nil
	}

	factory := func() (http.Handler, error) {
		reads := svc.GatedReads()

		deps := relamcp.Deps{
			Store:         reads.Reader,
			Meta:          svc.Meta(),
			Tracer:        reads.Tracer,
			Searcher:      svc.Searcher(),
			Validator:     reads.Validator,
			EntityManager: svc.EntityManager(),
			Config:        svc.Config(),
			LuaWriteDeps:  svc.LuaWriteDeps(),
			LuaCache:      svc.ScriptEngine().LuaCache(),
			Watcher:       noopWatcher{},
			ProjectRoot:   svc.Paths().Root,
		}

		srv, err := relamcp.NewServer(deps, mcpServerVersion,
			relamcp.WithPrincipal(principal.Principal{
				User: principal.SystemUser(),
				Tool: principal.ToolMCP,
			}))
		if err != nil {
			return nil, err
		}

		return srv.HTTPHandler(), nil
	}

	return app.SetRemoteMCP(factory)
}

// noopWatcher satisfies [relamcp.Watcher] for the HTTP transport, which has
// no use for file-change callbacks.
//
// Stateless streamable HTTP cannot deliver `resources/list_changed`: there is
// no session to push to, and server→client requests are rejected outright.
// Remote clients re-read on demand instead, which is correct because every
// read goes to the store. Starting a real filesystem watcher here would burn
// an inotify handle to feed a callback whose notification can never be sent.
//
// This is a no-op, NOT a silent degradation of a working feature — the
// notification does not exist on this transport in the first place.
type noopWatcher struct{}

func (noopWatcher) Start(func()) error { return nil }
func (noopWatcher) Stop()              {}
func (noopWatcher) Pause()             {}
func (noopWatcher) Resume()            {}
