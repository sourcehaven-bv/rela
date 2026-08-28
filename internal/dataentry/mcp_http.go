package dataentry

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/principal"
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
type MCPHandlerFactory func() (http.Handler, error)

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
	h, err := factory()
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
