package dataentry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/worldreader"
)

// MCPPath is the mount point for the remote MCP endpoint.
//
// It lives under `/api/` on purpose: [isAPIPath] matches it, so the endpoint
// inherits the full request chain — `stampAuditPrincipal` →
// `requireVerifiedJWT` → `attachACLRequest` — with no middleware change. A
// mount outside `/api/` would silently bypass both the identity gate and the
// ACL, which is exactly the failure RR-P2M7 guards against for the bare
// `/api`.
const MCPPath = "/api/v1/_mcp"

// toolForPath returns the audit Tool attribution for the surface a request
// arrived on. Everything is [principal.ToolDataEntry] except the remote MCP
// endpoint, which is [principal.ToolMCP].
//
// This exists because [principal.VerifiedFrom] is the only constructor that
// populates the unexported org/role/scope fields, so a caller cannot swap the
// Tool afterwards without dropping every asserted role — the Tool has to be
// decided before the projection. Deriving it from the path keeps that decision
// in one place rather than duplicating the projection per surface (RR-H8S10M).
//
// The MCP endpoint has no sub-paths today; the prefix form is deliberate so a
// future `/api/v1/_mcp/…` route cannot silently fall back to `data-entry`
// attribution.
func toolForPath(p string) string {
	if p == MCPPath || strings.HasPrefix(p, MCPPath+"/") {
		return principal.ToolMCP
	}
	return principal.ToolDataEntry
}

// MCPHandlerFactory builds the MCP HTTP handler.
//
// It returns a plain [http.Handler], NOT an SDK server type: `internal/mcp`
// is the only component permitted to import the MCP go-sdk (arch-lint's
// `mcpgo` vendor grant), so the SDK type must not appear in this package's
// API. The wiring site owns the SDK entirely — protocol version, stateless
// mode, transport — and hands back something this package can serve.
//
// It is called ONCE at router construction, not per request. Per-request
// state (the verified principal, the ACL Request) travels on the request
// ctx, which the middleware chain has already populated by the time the
// returned handler runs; the handler resolves it per call. A per-request
// factory would rebuild the whole tool registry on every message for no
// benefit.
//
// Returning an error refuses to build the router at all, so a broken MCP
// wiring is a startup failure rather than a per-request 500 discovered later.
type MCPHandlerFactory func(host MCPHost) (http.Handler, error)

// MCPHost is what the App lends the remote MCP server so its attachment tools
// apply the same upload policy as the web path (TKT-R6U15C). Passed to the
// factory rather than exposed as App methods, since App is over its method
// load line.
type MCPHost struct {
	// AttachmentPolicy returns the current metamodel and the effective
	// per-attachment byte limit, from ONE schema snapshot. It is a function,
	// not a value, because the App reloads its schema at runtime: a value
	// captured at boot would keep MCP uploads on the old MIME allowlist,
	// scan and `max_attachment_bytes` after the operator changed them.
	AttachmentPolicy func() (*metamodel.Metamodel, int64)

	// AttachmentRunner runs scan/transform commands. It is the web path's
	// runner, so both share its bounded pool. Nil when the runner could not
	// be built; a configured scan then rejects the upload.
	AttachmentRunner attachment.CommandRunner

	// WriteLock is the App's mutation mutex. MCP attachment writes hold it so
	// they serialize with every data-entry write.
	WriteLock sync.Locker

	// ReadWorld resolves the world an MCP read runs in: the operator's
	// browsing default (`app.default_world`), with the caller's world grant
	// checked, exactly as for a data-entry API read that names no world
	// (BUG-6XTX0G). Without it a faced entity is invisible to every MCP tool.
	ReadWorld worldreader.Source

	// SelectWorld binds the named world for every read on the returned ctx,
	// for an MCP tool call that names one. It applies the same lookup and
	// world grant as `?world=` on the data-entry API. Unlike that API it
	// refuses a denied world with an error rather than an empty result: world
	// names and their readability are already served by `list_worlds`, so
	// the error discloses nothing, and an agent needs to know why a world
	// shows nothing.
	SelectWorld func(ctx context.Context, name string) (context.Context, error)

	// WorldReadable reports whether the ctx principal may select name.
	WorldReadable func(ctx context.Context, name string) (bool, error)

	// DefaultWorld names the world a read that names none runs in:
	// `app.default_world`, or "default" when none is configured. Read per
	// call because the configuration hot-reloads.
	DefaultWorld func() string
}

// mcpHost builds the [MCPHost] for this App.
func mcpHost(a *App) MCPHost {
	return MCPHost{
		AttachmentPolicy: func() (*metamodel.Metamodel, int64) {
			s := a.schema.Current()
			return s.Meta, maxAttachmentBytes(s)
		},
		AttachmentRunner: a.attachmentRunner,
		WriteLock:        &a.writeMu,
		ReadWorld:        mcpReadWorld(a),
		SelectWorld: func(ctx context.Context, name string) (context.Context, error) {
			return mcpSelectWorld(ctx, a, name)
		},
		WorldReadable: func(ctx context.Context, name string) (bool, error) {
			return mcpWorldReadable(ctx, a, name)
		},
		DefaultWorld: func() string {
			if name := configuredDefaultWorld(a); name != "" {
				return name
			}
			return defaultWorldName
		},
	}
}

// mcpSelectedWorldKey carries the world an MCP tool call selected. Its value
// is a [store.WorldScope] that has passed the lookup and the grant check.
type mcpSelectedWorldKey struct{}

// mcpSelectWorld resolves name for the ctx principal and binds it on the
// returned ctx, where [mcpReadWorld] finds it.
func mcpSelectWorld(ctx context.Context, a *App, name string) (context.Context, error) {
	handle, err := resolveNamedWorld(ctx, a.worlds, name)
	switch {
	case errors.Is(err, errWorldUnknown):
		return ctx, fmt.Errorf("no such world %q; list_worlds names the worlds", name)
	case errors.Is(err, errWorldDenied):
		return ctx, fmt.Errorf("world %q is not readable by you", name)
	case err != nil:
		return ctx, fmt.Errorf("resolving world %q: %w", name, err)
	}
	return context.WithValue(ctx, mcpSelectedWorldKey{}, handle.scope), nil
}

// mcpWorldReadable reports whether the ctx principal may select name. An
// unknown world is not readable.
func mcpWorldReadable(ctx context.Context, a *App, name string) (bool, error) {
	_, err := resolveNamedWorld(ctx, a.worlds, name)
	switch {
	case errors.Is(err, errWorldUnknown), errors.Is(err, errWorldDenied):
		return false, nil
	case err != nil:
		return false, err
	}
	return true, nil
}

// mcpReadWorld returns the world source for the remote MCP endpoint.
//
// The configuration is read per call, never captured, because the watcher
// hot-reloads data-entry.yaml.
//
// A caller without a read grant on the configured world reads the default
// world. On the data-entry API such a caller gets an empty result and can
// still ask for `?world=default`, which needs no grant. Most MCP tools take
// no world, so an empty result would lock the caller out of them, including
// writes the ACL permits. Falling back discloses nothing: the
// default world is the one any caller may read, and the row and face gates
// still apply to every entity in it.
//
// A world the tool call selected through [MCPHost.SelectWorld] takes
// precedence over the configured default.
func mcpReadWorld(a *App) worldreader.Source {
	return func(ctx context.Context) (store.WorldScope, error) {
		if scope, ok := ctx.Value(mcpSelectedWorldKey{}).(store.WorldScope); ok {
			return scope, nil
		}
		if memo, ok := ctx.Value(mcpWorldKey{}).(*mcpWorldMemo); ok {
			memo.once.Do(func() { memo.scope, memo.err = resolveMCPWorld(ctx, a) })
			return memo.scope, memo.err
		}
		return resolveMCPWorld(ctx, a)
	}
}

// mcpWorldKey carries an [mcpWorldMemo] on an MCP request's ctx.
type mcpWorldKey struct{}

// mcpWorldMemo holds the world resolved for one MCP request. Every read in a
// tool call then runs in the same world, even if the configuration reloads
// part-way through, and the grant check runs once rather than once per read.
type mcpWorldMemo struct {
	once  sync.Once
	scope store.WorldScope
	err   error
}

// withMCPWorldMemo gives each request its own [mcpWorldMemo]. The stateless
// transport serves one JSON-RPC exchange per request.
func withMCPWorldMemo(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), mcpWorldKey{}, &mcpWorldMemo{})
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

func resolveMCPWorld(ctx context.Context, a *App) (store.WorldScope, error) {
	name := configuredDefaultWorld(a)
	handle, err := resolveNamedWorld(ctx, a.worlds, name)
	switch {
	case errors.Is(err, errWorldDenied):
		return store.WorldScope{}, nil
	case err != nil:
		return store.WorldScope{}, fmt.Errorf("resolving app.default_world %q: %w", name, err)
	}
	return handle.scope, nil
}

// SetRemoteMCP enables the remote MCP endpoint, which is OFF by default.
//
// It refuses a configuration that cannot be served safely, at startup, rather
// than at first request:
//
//   - a nil factory has nothing to serve;
//   - a factory that errors means the MCP wiring is broken;
//   - **no JWT gate is refused outright.** The endpoint needs a CSRF exemption
//     (a non-browser MCP client sends no Origin), and that exemption is only
//     sound while rela itself verifies a bearer token and requires it. In
//     header-identity mode `requireVerifiedJWT` is never wrapped and the
//     terminal resolver yields `User: "unknown"` — combining that with the
//     exemption would publish an unauthenticated remote write surface. A
//     declarative-ACL deployment would still fail closed
//     (`acl.ErrUnstampedPrincipal` rejects `unknown`), but a NopACL deployment
//     would not, and this must not depend on a second, unrelated setting.
//     The go-sdk's DNS-rebinding guard is disabled on the same grounds (see
//     mcp.Server.HTTPHandler); relaxing this refusal must re-enable it.
//
// The same reasoning as `validateIdentityFlags` in cmd/rela-server: an
// auth downgrade happens per request, long after anyone reads a startup
// warning, so it is refused rather than warned about.
//
// Must be called before [App.NewRouter].
func (a *App) SetRemoteMCP(factory MCPHandlerFactory) error {
	if factory == nil {
		return errors.New("dataentry: remote MCP requires a non-nil handler factory")
	}
	if a.jwtGate == nil {
		return errors.New("dataentry: remote MCP requires verified JWT identity " +
			"(-jwt-issuer/-jwt-audience/-jwt-jwks-url): the endpoint is CSRF-exempt " +
			"because MCP clients send no Origin, and that exemption is only sound " +
			"while rela verifies a bearer token itself. Header identity fails open " +
			"to \"unknown\". See docs/server-security.md")
	}
	h, err := factory(mcpHost(a))
	if err != nil {
		return fmt.Errorf("dataentry: building the MCP handler: %w", err)
	}
	if h == nil {
		return errors.New("dataentry: the MCP handler factory returned a nil handler")
	}
	a.mcpHandler = withMCPWorldMemo(h)
	return nil
}

// registerMCPRoute mounts h at [MCPPath], or registers nothing when h is nil
// — so an upgraded server that did not opt in has no such route at all
// (absent, not 403).
//
// A plain function rather than a method on App: it needs one handler, not the
// aggregate, and App is already over its god-object load line
// (TKT-N0IKN9). Taking the dependency as a parameter keeps it off the count.
func registerMCPRoute(mux *http.ServeMux, h http.Handler) {
	if h == nil {
		return
	}
	mux.Handle(MCPPath, h)
}
