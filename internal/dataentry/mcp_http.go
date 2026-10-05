package dataentry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
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

	// Attachments is the manager's attachment surface the App uses, so MCP
	// attachment writes, web uploads, copies and face deletes on one
	// property exclude each other.
	Attachments entitymanager.Attachments

	// AttachmentUploads is the App's upload bound, so MCP and web uploads
	// share one budget.
	AttachmentUploads *attachment.Limiter

	// SelectWorld returns the world name selects, for an MCP tool call that
	// names one. It applies the same lookup and world grant as `?world=` on
	// the data-entry API. Unlike that API it refuses a denied world with an
	// error rather than an empty result: world names and their readability
	// are already served by `list_worlds`, so the error discloses nothing,
	// and an agent needs to know why a world shows nothing.
	SelectWorld func(ctx context.Context, name string) (store.WorldScope, error)

	// WorldReadable reports whether the ctx principal may select name.
	WorldReadable func(ctx context.Context, name string) (bool, error)

	// DefaultWorld names the world a read that names none runs in. Read
	// per call because the schema hot-reloads.
	DefaultWorld func() string
}

// MCPWorlds is the host's world functions as methods, the shape the MCP
// server's world selector takes.
type MCPWorlds struct{ host MCPHost }

// Worlds returns h's world functions as an [MCPWorlds].
func (h MCPHost) Worlds() MCPWorlds { return MCPWorlds{host: h} }

// SelectWorld calls [MCPHost.SelectWorld].
func (w MCPWorlds) SelectWorld(ctx context.Context, name string) (store.WorldScope, error) {
	return w.host.SelectWorld(ctx, name)
}

// WorldReadable calls [MCPHost.WorldReadable].
func (w MCPWorlds) WorldReadable(ctx context.Context, name string) (bool, error) {
	return w.host.WorldReadable(ctx, name)
}

// DefaultWorld calls [MCPHost.DefaultWorld].
func (w MCPWorlds) DefaultWorld() string { return w.host.DefaultWorld() }

// mcpHost builds the [MCPHost] for this App.
func mcpHost(a *App) MCPHost {
	return MCPHost{
		AttachmentPolicy: func() (*metamodel.Metamodel, int64) {
			s := a.schema.Current()
			return s.Meta, maxAttachmentBytes(s)
		},
		AttachmentRunner:  a.attachmentRunner,
		Attachments:       a.attachmentOwner,
		AttachmentUploads: a.attachmentUploads,
		SelectWorld: func(ctx context.Context, name string) (store.WorldScope, error) {
			return mcpSelectWorld(ctx, a, name)
		},
		WorldReadable: func(ctx context.Context, name string) (bool, error) {
			return mcpWorldReadable(ctx, a, name)
		},
		DefaultWorld: func() string { return effectiveDefaultWorld(a) },
	}
}

// mcpSelectWorld resolves name for the ctx principal.
func mcpSelectWorld(ctx context.Context, a *App, name string) (store.WorldScope, error) {
	handle, err := resolveNamedWorld(ctx, a.worlds, name, effectiveDefaultWorld(a))
	switch {
	case errors.Is(err, errWorldUnknown):
		return store.WorldScope{}, fmt.Errorf("no such world %q; list_worlds names the worlds", name)
	case errors.Is(err, errWorldDenied):
		return store.WorldScope{}, fmt.Errorf("world %q is not readable by you", name)
	case err != nil:
		return store.WorldScope{}, fmt.Errorf("resolving world %q: %w", name, err)
	}
	return handle.scope, nil
}

// mcpWorldReadable reports whether the ctx principal may select name. An
// unknown world is not readable.
func mcpWorldReadable(ctx context.Context, a *App, name string) (bool, error) {
	_, err := resolveNamedWorld(ctx, a.worlds, name, effectiveDefaultWorld(a))
	switch {
	case errors.Is(err, errWorldUnknown), errors.Is(err, errWorldDenied):
		return false, nil
	case err != nil:
		return false, err
	}
	return true, nil
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
	a.mcpHandler = h
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
