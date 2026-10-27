package dataentrywire

import (
	"errors"
	"net/http"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	relamcp "github.com/Sourcehaven-BV/rela/internal/mcp"
	"github.com/Sourcehaven-BV/rela/internal/mcpwire"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// RemoteMCP mounts the HTTP MCP endpoint on app, served by the server
// [RemoteMCPServer] builds over svc. rela-server calls it when -mcp is set;
// the golden corpus calls it to record the endpoint as served.
//
// **Identity comes from the request, not from here.** Unlike `rela mcp`
// (stdio), which stamps one process-wide system principal, the remote server
// serves many callers. The MCP server's principal middleware preserves an
// identity already on the ctx, and the transport hands it the *http.Request
// ctx that `requireVerifiedJWT` has stamped — so `WithPrincipal` in [RemoteMCPServer] is
// only the fallback for a request that somehow carries none. It is a real,
// non-zero system principal (NewServer requires one) rather than a
// placeholder, so an unattributed write is recorded as the server itself
// rather than as a guessed user.
//
// **Reads are ACL-gated.** Every read handle comes from
// [appbuild.Services.GatedReads] via [RemoteMCPDeps], which resolves the ctx
// principal per call.
// This is the opposite of the stdio wiring's deliberate NopACL: there the
// filesystem is the trust boundary (anyone who can run `rela mcp` can edit
// the files directly), so a gate would defend nothing. A remote caller has no
// filesystem access, so the ACL is the ONLY boundary.
//
// **No Lua tools.** The server is built without [relamcp.WithLuaTools], so
// lua_eval and lua_run do not exist here. A remote caller may not
// run scripts in the server process.
func RemoteMCP(app *dataentry.App, svc *appbuild.Services, version string) error {
	return app.SetRemoteMCP(func(host dataentry.MCPHost) (http.Handler, error) {
		srv, err := RemoteMCPServer(svc, host, version)
		if err != nil {
			return nil, err
		}
		return srv.HTTPHandler(), nil
	})
}

// remoteAttachmentDeps wires the MCP attachment tools onto the web upload
// path's policy: the App's live schema (so an operator's edit to `accept:`,
// `scan:` or `max_attachment_bytes` applies to MCP uploads immediately), its
// command runner, and the manager's attachment surface (stamp and lock). The snapshot is rebuilt per tool
// call, which costs one struct allocation.
func remoteAttachmentDeps(svc *appbuild.Services, host dataentry.MCPHost) relamcp.AttachmentDeps {
	return relamcp.AttachmentDeps{
		Snapshot: func() (relamcp.AttachmentSnapshot, error) {
			meta, limit := host.AttachmentPolicy()
			return relamcp.NewAttachmentSnapshot(
				svc.Store(), host.Attachments, svc.ACL(), meta, host.AttachmentRunner, limit)
		},
		Uploads:    host.AttachmentUploads,
		Authorizer: svc.ACL(),
		Audit:      svc.Audit(),
	}
}

// RemoteMCPServer builds the remote MCP server, reporting version in its
// `initialize` response. It does not pass [relamcp.WithLuaTools]; see
// [RemoteMCP].
func RemoteMCPServer(svc *appbuild.Services, host dataentry.MCPHost, version string) (*relamcp.Server, error) {
	deps, err := RemoteMCPDeps(svc, host)
	if err != nil {
		return nil, err
	}
	return relamcp.NewServer(deps, version,
		relamcp.WithPrincipal(principal.Principal{
			User: principal.SystemUser(),
			Tool: principal.ToolMCP,
		}),
		mcpwire.Piles(svc))
}

// RemoteMCPDeps builds the MCP dependencies for the remote endpoint. Every
// read handle, including search, comes from [appbuild.Services.GatedReads].
// Lua stays zero because the remote server has no Lua tools.
//
// A bare id resolves in the compiled default world, as on the data-entry API
// (BUG-6XTX0G), and a read tool's `world` argument selects another through
// the host, which applies the world grant.
//
// Nil: the host's world functions are rejected, because without them a
// `world` argument could not be authorized.
func RemoteMCPDeps(svc *appbuild.Services, host dataentry.MCPHost) (relamcp.Deps, error) {
	if host.SelectWorld == nil || host.WorldReadable == nil || host.DefaultWorld == nil {
		return relamcp.Deps{}, errors.New("remote MCP: the host's world functions are required")
	}
	reads := svc.GatedReads()
	deps := relamcp.Deps{
		Store:     reads.Reader,
		Meta:      svc.Meta(),
		Tracer:    reads.Tracer,
		Searcher:  reads.Searcher,
		Validator: reads.Validator,
		// The field-gated handle: an MCP client's writes honor acl.yaml's
		// field grants, as the data-entry API's do (TKT-0XL8MF).
		EntityManager: appbuild.FieldGatedEntityManager(svc),
		Config:        svc.Config(),
		Watcher:       noopWatcher{},
		Watch: relamcp.WatchDeps{
			Changes: svc.Store(), Rebind: reads.Rebind, ChannelPoll: remoteChannelPoll,
		},
		Attachments: remoteAttachmentDeps(svc, host),
		World:       reads.LuaReads.World,
		Families:    appbuild.CompiledWorlds(svc).Families(),
		Worlds:      host.Worlds(),
	}
	if reads.Traversals != nil {
		deps.Related.Traversals = reads.Traversals
	}
	if reads.Rollups != nil {
		deps.Related.Rollups = reads.Rollups
	}
	// Assigned only when set: a nil *MCPComments in the interface would
	// register the tools over an absent service.
	if host.Comments != nil {
		deps.Comments = relamcp.CommentDeps{Reader: host.Comments, Writer: host.Comments}
	}
	if leases := appbuild.Leases(svc); leases != nil {
		deps.Leases = leases
	}
	if host.Actions != nil {
		deps.Commands.Actions = host.Actions
	}
	// The field-gated converter the data-entry conversion endpoint uses.
	if em := appbuild.FieldGatedEntityManager(svc); em != nil {
		conv, err := entitymanager.NewConverter(em)
		if err != nil {
			return relamcp.Deps{}, err
		}
		deps.Commands.Conversions = conv
	}
	// The channel tools share the HTTP routes' service, so a post through
	// either wakes the other's watchers, and signals are kept. The service
	// reads the realtime config holder on every call, so a realtime.yaml
	// reload applies to the next tool call.
	if cs := appbuild.ChannelService(svc); cs != nil {
		deps.Channels = relamcp.ChannelDeps{Reader: cs, Sessions: cs, Writer: cs, Creator: cs, Waker: cs}
	}
	return deps, nil
}

// remoteChannelPoll is how often a remote watch replays its read_channel
// targets, for posts made by another rela-server process against the same
// database. A post in this process wakes the watch at once.
const remoteChannelPoll = 5 * time.Second

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
