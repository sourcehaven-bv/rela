package dataentry

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// EnvDataEntryUserVar is the local-dev escape hatch: if this env var is
// set, EnvPrincipalResolver returns its value as the principal user.
// Documented in docs/server-security.md alongside the --principal-header
// flag.
//
// Exported so cmd/rela-server can reject it alongside --jwt-* without
// duplicating the literal: under verified-JWT identity an env var that
// overrides a cryptographically proven subject is the same downgrade the
// header fall-through was.
const EnvDataEntryUserVar = "RELA_DATAENTRY_USER"

// envDataEntryUser is the internal alias kept so existing call sites read
// naturally; it is the same variable name.
const envDataEntryUser = EnvDataEntryUserVar

// principalUserMaxLen caps the principal.User value at 256 UTF-8
// chars. Mirrors the cap audit.Filesystem applies to record fields —
// defense-in-depth against a misconfigured proxy sending huge values.
const principalUserMaxLen = 256

// CheckEmbeddedSPA verifies that the embedded Vue SPA bundle is present and
// usable. Production entry points (cmd/rela-server, cmd/rela-desktop) should
// call this at startup so a missing or empty build fails loudly with a clear
// message instead of silently serving a directory listing (the BUG-W144
// regression class). Tests that construct routers via NewRouter do not need
// to call this.
func CheckEmbeddedSPA() error {
	spaFS, err := fs.Sub(staticFiles, "static/v2")
	if err != nil {
		return fmt.Errorf("mount embedded SPA filesystem (static/v2): %w", err)
	}
	if _, err := fs.Stat(spaFS, "index.html"); err != nil {
		return fmt.Errorf("embedded SPA is missing index.html (run `just build-frontend`): %w", err)
	}
	return nil
}

// NewRouter returns an http.Handler with all data entry routes registered.
// The Vue SPA serves as the primary UI at the root path.
//
// When adding a route, add a probe to the route table in
// router_walk_test.go so registration stays covered.
func (a *App) NewRouter() http.Handler {
	mux := http.NewServeMux()

	// Legacy /static/ mount. The Vue bundle is also reachable here as
	// /static/v2/*, but the SPA's built index.html references assets as
	// /assets/*, served via the catch-all below.
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic("failed to mount embedded static filesystem (static): " + err.Error())
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Vue SPA served at root. The build output dir is kept as `static/v2` to
	// avoid churn in frontend/vite.config.ts; see TKT-MNOO. Presence of
	// index.html is verified at startup by CheckEmbeddedSPA.
	spaFS, err := fs.Sub(staticFiles, "static/v2")
	if err != nil {
		panic("failed to mount embedded SPA filesystem (static/v2): " + err.Error())
	}

	// SSE endpoints — excluded from reload-lock (long-lived connection)
	mux.HandleFunc("/api/events", a.handleSSE)
	mux.HandleFunc("/api/v1/_events", a.handleSSE)

	// All other routes are wrapped with the reload-lock middleware
	inner := http.NewServeMux()

	// APIs used by Vue SPA
	inner.HandleFunc("/api/help/", a.handleEntityHelp)
	a.commands.registerCommandRoutes(inner)
	inner.HandleFunc("/api/git/status", a.handleGitStatus)
	inner.HandleFunc("/api/git/sync", a.handleGitSync)

	// REST API v1 - main API for Vue SPA
	a.registerAPIV1Routes(inner)

	// Sync API (FEAT-NJ9FEN) - machine-to-machine fs↔pg sync, under /api/sync/.
	a.sync.registerSyncRoutes(inner)

	// noCacheMiddleware sets no-cache headers on API responses so that
	// browsers always fetch fresh data after file changes trigger a reload.
	mux.Handle("/api/", a.noCacheMiddleware(inner))

	// Inbound-IdP webhook (POST /webhooks/idp) — mounted only when a receiver is
	// configured (SetWebhookReceiver). Registered on the OUTER mux, NOT on
	// `inner`: `inner` is only reachable under the `/api/` prefix, and
	// `/webhooks/idp` does not carry it, so registering there left the route
	// unreachable — every request fell through to the SPA catch-all below and
	// returned 200 HTML, so the handler never ran (BUG-F3ADZO). It lives outside
	// `/api/` on purpose: it authenticates itself by verifying a signed JWT body,
	// not a proxy header or cookie, so it is CSRF-immune by construction and
	// needs neither the same-origin gate nor the JWT identity gate.
	a.registerWebhookRoutes(mux)

	// Serve Vue SPA at root (catch-all for client-side routing)
	mux.Handle("/", spaHandler(spaFS))

	// Apply security middlewares as the outermost wrapper so they protect
	// every route, including the SSE handlers and static assets. The
	// requireSameOrigin middleware internally exempts non-sensitive paths
	// (e.g. static assets, SPA shell) so the SPA still loads cross-origin.
	var handler http.Handler = mux
	if a.security != nil {
		handler = a.security.requireSameOrigin(handler)
		handler = a.security.requireLocalHost(handler)
	}
	resolver := a.principalResolver
	if resolver == nil {
		resolver = defaultPrincipalResolver
	}
	// Wrap order (CRIT-1): attachACLRequest reads the principal from
	// ctx, so stampAuditPrincipal MUST run first. In Go middleware the
	// LAST wrap is the OUTERMOST — request flow descends from
	// outermost-wrap to innermost-wrap. We therefore:
	//   1) wrap attachACLRequest first (innermost of these two), then
	//   2) wrap stampAuditPrincipal (outermost) so it runs first and
	//      stamps before ACL reads.
	// Reversed order silently fails every /api/ request with 500
	// `acl_unstamped_principal` when ACL is configured, because the
	// principal is still the unstamped default at the time
	// ForPrincipal is called.
	if d, ok := a.acl.(*acl.Declarative); ok && d != nil {
		// jwtVerified tells attachACLRequest that identity is a verified-JWT
		// assertion (the gate is installed), so an unmatched principal can be
		// distinguished from a spoofable-header one — the fact the
		// `unmatched_principal: reject` gate keys on. Wiring state, not a
		// per-principal marker (a JWT and a header principal are identical on
		// the Principal).
		//
		// This snapshots a.jwtGate at NewRouter time. SetJWTGate MUST run before
		// NewRouter (it does in production wiring and in every test) — otherwise
		// this captures nil and `unmatched_principal: reject` silently never
		// fires. If a future refactor reorders these, that invariant breaks
		// quietly; keep SetJWTGate ahead of NewRouter.
		handler = attachACLRequest(handler, d, a.jwtGate != nil)
	}
	// The JWT gate wraps BETWEEN attachACLRequest and stampAuditPrincipal, so at
	// request time it runs after the stamper and before ACL. This ordering is
	// load-bearing and the obvious alternative is wrong:
	//
	//   - Outermost (after stampAuditPrincipal) would let the stamper run LAST
	//     and OVERWRITE the verified subject with the resolver's `unknown`,
	//     silently discarding the identity we just proved. Same failure class as
	//     CRIT-1 above, in the opposite direction.
	//   - Innermost (inside attachACLRequest) would let ACL open a Request for
	//     the unverified principal before the gate got a chance to deny.
	//
	// In the order below, stampAuditPrincipal stamps every request (so `/` and
	// `/static/` still get a principal), the gate then replaces it with the
	// verified subject on API paths or denies 401, and ACL reads the corrected
	// principal.
	if a.jwtGate != nil {
		handler = requireVerifiedJWT(handler, *a.jwtGate)
	}
	handler = stampAuditPrincipal(handler, resolver)
	return handler
}

// isAPIPath reports whether p addresses the data API — the surface that carries
// entity data and therefore the one both [attachACLRequest] and
// [requireVerifiedJWT] gate. Everything else (the SPA shell at `/`, static
// assets, the self-authenticating IdP webhook) is deliberately outside.
//
// RR-P2M7: the bare `/api` is included explicitly. Go's ServeMux would not match
// it under the `/api/` pattern, so an endpoint mounted there would silently
// bypass the gates. Both callers share this predicate so they cannot drift.
func isAPIPath(p string) bool {
	return strings.HasPrefix(p, "/api/") || p == "/api"
}

// attachACLRequest opens an acl.Request for the principal stamped on
// ctx (by stampAuditPrincipal above) and attaches it via both
// [acl.WithRequest] and [withReadGate], so downstream affordance +
// read-gate consumers reuse the one member-of walk per HTTP request
// (RR-JJYW). Wired only when the ACL is a *acl.Declarative — NopACL /
// ReadOnlyACL paths don't open Requests.
//
// **Scope (RR-T15E).** Only `/api/` requests pay the cost — and only
// `/api/` requests fail loud on misconfiguration. The SPA shell at
// `/` and static assets at `/static/` MUST stay reachable even when
// ACL is configured and the principal stamper has a bug; otherwise a
// misconfigured stamper renders the UI as a raw JSON 500 and locks
// operators out of the very surface they need to recover from it.
//
// **Fail-loud (RR-875A).** Inside `/api/` an
// [acl.ErrUnstampedPrincipal] returns 500 with a structured error
// rather than silently proceeding with no Request attached. The
// earlier silent fall-through turned ACL into fail-open under a
// stamper misconfig.
//
// RR-8ZGO: respect a Request already attached by an upstream handler
// (chiefly tests that wrap the handler with their own
// acl.WithRequest). The middleware is at the outer edge of the
// production chain so this guard rarely fires in production, but it
// makes the test composition story safer.
func attachACLRequest(next http.Handler, d *acl.Declarative, jwtVerified bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAPIPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		ctx := r.Context()
		// RR-E703: if an upstream layer already stamped a Request, also
		// attach the matching readGate before forwarding. Without this,
		// every read becomes AllowAll for the upstream-only caller —
		// silent fail-open.
		if existing := acl.FromContext(ctx); existing != nil {
			// SIG-2: verify the existing Request's principal matches
			// the ctx principal. A mismatch is a wiring bug (an
			// upstream layer attached a Request from a different
			// identity); under that condition the gate would run
			// against the wrong policy with no loud signal.
			//
			// Note: principal_property resolution (resolvePrincipalEntity
			// below) has NOT run on this branch — it only applies when we
			// open a fresh Request. So both sides here are the raw,
			// pre-resolution principal and match in production (nothing
			// upstream resolves). If a future upstream layer ever attaches
			// a Request built from a *resolved* principal while ctx still
			// holds the raw one, this correctly 500s as a genuine mismatch.
			ctxPrin := principal.From(ctx)
			if !existing.Principal().Equal(ctxPrin) {
				slog.Warn("acl: attachACLRequest: existing Request principal mismatch",
					"path", r.URL.Path,
					"method", r.Method,
					"ctx_user", ctxPrin.User,
					"ctx_tool", ctxPrin.Tool,
					"req_user", existing.Principal().User,
					"req_tool", existing.Principal().Tool)
				writeV1Error(w, r, http.StatusInternalServerError,
					"acl_principal_mismatch",
					"Upstream ACL request principal does not match context principal",
					"check server logs")
				return
			}
			gate, gerr := newACLReadGate(existing)
			if gerr != nil {
				slog.Warn("acl: attachACLRequest: newACLReadGate failed (existing)",
					"err", gerr, "path", r.URL.Path)
				writeV1Error(w, r, http.StatusInternalServerError,
					"acl_internal", "ACL gate construction failed", "check server logs")
				return
			}
			ctx = withReadGate(ctx, gate)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		// Resolve the raw principal to a user entity via the policy's
		// `principal_property` lookup (a no-op unless both policy keys are
		// set). On a single match we re-stamp ctx with the resolved ID so
		// BOTH the ACL walk below and the audit writer (which re-reads
		// principal.From(ctx)) attribute the write to the real entity while
		// preserving the raw header value in RawUser. Any non-match /
		// ambiguous / errored lookup keeps the raw principal (fail-open to
		// pre-feature behavior) — see acl.Declarative.ResolvePrincipal.
		ctx = resolvePrincipalEntity(ctx, d, r, jwtVerified)

		req, err := d.ForPrincipal(principal.From(ctx))
		if err != nil {
			// RR-372L: log the raw error server-side; never emit it
			// in the response body. ForPrincipal's contract leaves
			// room for future enhancements (LDAP errors, internal
			// identifiers) that would otherwise become attacker-
			// readable here.
			p := principal.From(ctx)
			slog.Warn("acl: attachACLRequest: ForPrincipal failed",
				"err", err,
				"path", r.URL.Path,
				"method", r.Method,
				"user", p.User,
				"tool", p.Tool)
			writeV1Error(w, r, http.StatusInternalServerError,
				"acl_unstamped_principal",
				"Principal could not be resolved under ACL",
				"principal stamper produced an unstamped identity; check server logs")
			return
		}
		gate, gerr := newACLReadGate(req)
		if gerr != nil {
			// Unreachable: ForPrincipal returned non-nil err above on
			// any failure mode; req here is non-nil. Defense in depth
			// — if a future change to ForPrincipal lets a nil through,
			// fail-loud rather than silently fall-open.
			slog.Warn("acl: attachACLRequest: newACLReadGate failed",
				"err", gerr, "path", r.URL.Path)
			writeV1Error(w, r, http.StatusInternalServerError,
				"acl_internal", "ACL gate construction failed", "check server logs")
			return
		}
		ctx = acl.WithRequest(ctx, req)
		ctx = withReadGate(ctx, gate)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// resolvePrincipalEntity applies the ACL policy's `principal_property`
// lookup to the ctx principal. When the policy has the lookup enabled and
// the raw principal resolves to exactly one user entity, it returns a ctx
// carrying a principal whose User is the entity ID and whose RawUser is
// the original identifier (so the audit log records both). In every other
// case — lookup disabled, no match, ambiguous match, or backend error —
// it returns ctx unchanged so the write falls back to the raw principal
// (pre-feature behavior); ambiguity and errors are logged, a plain
// no-match is not (a principal absent from the graph is expected, e.g. a
// break-glass identity assigned by raw UPN).
// jwtVerified reports whether identity came from a verified-JWT assertion (the
// gate is installed). Combined with a genuine no-match while the lookup is
// enabled, it lets the caller mark the request unmatched-verified so
// `unmatched_principal: reject` can deny its writes — see [acl.WithUnmatchedVerified].
func resolvePrincipalEntity(
	ctx context.Context, d *acl.Declarative, r *http.Request, jwtVerified bool,
) context.Context {
	p := principal.From(ctx)
	id, err := d.ResolvePrincipal(ctx, p.User)
	if err != nil {
		slog.Warn("acl: principal_property lookup failed; using raw principal",
			"path", r.URL.Path, "method", r.Method, "err", err)
		return ctx
	}
	if id == "" || id == p.User {
		// No entity resolved. If identity is a verified JWT and the lookup was
		// actually enabled (id=="" from a disabled lookup is not a "no-match",
		// it's "not attempted"), mark the request so the write path can enforce
		// `unmatched_principal: reject`. The mark is inert unless the policy
		// opts in.
		if jwtVerified && id == "" && d.Policy().PrincipalPropertyLookupEnabled() {
			ctx = acl.WithUnmatchedVerified(ctx)
		}
		return ctx
	}
	// Rebuild via Verified so the assertion claims survive the substitution.
	// A plain composite literal here would silently drop org and roles — the
	// resolved principal would keep its identity but lose every asserted grant.
	out := principal.Verified(id, p.Tool, p.OrgID(), p.OrgSlug(), p.Roles())
	out.RawUser = p.User
	return principal.With(ctx, out)
}

// PrincipalResolver maps an incoming HTTP request to the audit
// Principal that should be stamped on its context. Compose multiple
// resolvers via [ChainResolvers] to layer (e.g.) an env-var override
// over a header reader over the default.
type PrincipalResolver func(*http.Request) principal.Principal

// defaultPrincipalResolver stamps Principal{User: "unknown", Tool:
// "data-entry"} on every request. Used when neither the
// `--principal-header` flag nor `$RELA_DATAENTRY_USER` yields a
// user. The "unknown" placeholder is intentional — recording the
// server process owner for every edit by every human web user would
// be actively misleading.
func defaultPrincipalResolver(_ *http.Request) principal.Principal {
	return principal.Principal{
		User: "unknown",
		Tool: principal.ToolDataEntry,
	}
}

// HeaderPrincipalResolver reads Principal.User from headerName on
// each request, stamping Tool=data-entry.
//
// The returned resolver is never nil. An empty headerName yields a
// resolver that always returns a zero Principal — the [ChainResolvers]
// composition relies on this shape, so callers don't need to special-
// case the disabled state. Production wiring in cmd/rela-server
// passes the raw flag value; the empty-default flag stays inert.
//
// **Trust boundary.** The header value is only as trustworthy as
// the reverse proxy that sets it. Operators serving data-entry
// without a trusted proxy must not enable this resolver — anyone
// can spoof identity by setting the header on the wire. See
// docs/server-security.md for the deployment guidance.
//
// Sanitization: control characters (C0 + DEL) in the header value
// are replaced with regular spaces, the value is truncated to 256
// runes (UTF-8 safe), and surrounding whitespace is trimmed.
// Control-only values therefore sanitize to "" and fall through.
func HeaderPrincipalResolver(headerName string) PrincipalResolver {
	if headerName == "" {
		return func(*http.Request) principal.Principal {
			return principal.Principal{}
		}
	}
	return func(r *http.Request) principal.Principal {
		user := sanitizeUser(r.Header.Get(headerName))
		if user == "" {
			return principal.Principal{}
		}
		return principal.Principal{User: user, Tool: principal.ToolDataEntry}
	}
}

// EnvPrincipalResolver reads Principal.User from
// $RELA_DATAENTRY_USER. Returns a zero principal when the env is
// unset or whitespace-only — chain it (typically first) so it acts
// as a local-dev escape hatch that overrides any incoming header.
//
// The env var is read on *every* request rather than cached at
// construction so test fixtures using `t.Setenv` work without
// rebuilding the resolver. The cost is one map lookup per request
// (Go runtime takes a RLock); negligible relative to the per-request
// work of the audit middleware that follows.
//
// Sanitization mirrors [HeaderPrincipalResolver].
func EnvPrincipalResolver() PrincipalResolver {
	return func(*http.Request) principal.Principal {
		user := sanitizeUser(os.Getenv(envDataEntryUser))
		if user == "" {
			return principal.Principal{}
		}
		return principal.Principal{User: user, Tool: principal.ToolDataEntry}
	}
}

// AssertedIdentity is the verified-assertion payload this package consumes. It
// is dataentry's OWN type, not the verifier's: the wiring site adapts whatever
// the concrete verifier returns into this shape (see the adapter in
// cmd/rela-server), so dataentry never imports the verifier package and the
// verifier stays an arch-lint leaf.
//
// Subject is the only field callers may treat as required. The rest are absent
// for any proxy that doesn't model orgs or roles, which is not an error.
type AssertedIdentity struct {
	Subject string
	OrgID   string
	OrgSlug string
	Roles   []string
}

// assertionVerifier verifies a signed assertion and projects the org/role
// claims this package stamps onto a Principal. It is the seam for BOTH the
// production gate ([requireVerifiedJWT], via [App.SetJWTGate]) and the
// deprecated [JWTPrincipalResolver]; both must carry the full claims, or an
// asserted_role_assignments policy grants nothing (TKT-OJL2GN). Satisfied by an
// adapter over *jwtauth.Verifier — kept local (and unexported, per the other
// consumer interfaces here) so both are testable with a stub.
type assertionVerifier interface {
	VerifyAssertion(ctx context.Context, raw string) (AssertedIdentity, error)
}

// Deprecated: production wiring uses [requireVerifiedJWT] via [App.SetJWTGate],
// which fails CLOSED. This resolver returns a zero Principal on a verification
// failure so a chain falls through to the next source — under a header chain
// that is an auth downgrade, which is why cmd/rela-server no longer wires it.
// Retained for callers embedding dataentry with their own chain semantics.
//
// JWTPrincipalResolver reads a signed identity assertion from headerName,
// verifies it (ES256 against the proxy's JWKS, via v), and stamps the verified
// STABLE subject as Principal.User. This is provider-agnostic — any OIDC proxy
// that injects a signed JWT works by configuring the issuer/audience/JWKS/header.
//
// Unlike [HeaderPrincipalResolver] (which trusts the proxy set the header), this
// resolver CRYPTOGRAPHICALLY verifies the assertion, so it is safe even when the
// header could reach the server from the network — a spoofed header without a
// valid signature simply fails verification and falls through.
//
// Tool is [principal.ToolDataEntry]: the assertion changes WHO authenticated, not
// the entry point (a verified user still arrives via the data-entry HTTP surface).
//
// It also carries the assertion's org and role claims onto the Principal via
// [principal.Verified]. This is the ONLY resolver that may do so — the header
// and env resolvers have no verified source for them, and a role reaching the
// ACL from a spoofable header would be a complete authorization bypass. The
// unexported fields on Principal make that structural rather than a convention.
//
// A missing header, an "Authorization: Bearer <jwt>" wrapper (the scheme is
// stripped case-insensitively per RFC 6750), or any verification failure yields a
// zero Principal so the chain falls through — a nil v also yields an inert
// resolver. Empty headerName ⇒ inert (matches the disabled-flag shape of the
// other resolvers).
func JWTPrincipalResolver(v assertionVerifier, headerName string) PrincipalResolver {
	if v == nil || headerName == "" {
		return func(*http.Request) principal.Principal { return principal.Principal{} }
	}
	return func(r *http.Request) principal.Principal {
		raw := stripBearer(r.Header.Get(headerName))
		if raw == "" {
			return principal.Principal{}
		}
		id, err := v.VerifyAssertion(r.Context(), raw)
		if err != nil {
			return principal.Principal{}
		}
		p, ok := verifiedPrincipal(id)
		if !ok {
			return principal.Principal{}
		}
		return p
	}
}

// verifiedPrincipal projects a verified [AssertedIdentity] into a stamped
// Principal, or reports ok=false when the subject is unusable (empty, or all
// control characters that sanitize away). It is the SINGLE place the assertion
// claims become a Principal, shared by [JWTPrincipalResolver] (the deprecated
// chain path) and [requireVerifiedJWT] (the production gate) so the two cannot
// drift on how a subject is sanitized or how roles are filtered — a drift that
// once silently dropped roles on the gate path (TKT-OJL2GN).
//
// It carries the org and role claims via [principal.Verified], the only
// constructor that populates them from request-path input (the audit
// wire-format [principal.Principal.UnmarshalJSON] reads them back too, but only
// from a record this process already wrote and verified). A role reaching the
// ACL from an unverified source would be a complete authorization bypass, so
// this must be called ONLY on the output of a completed signature verification.
func verifiedPrincipal(id AssertedIdentity) (principal.Principal, bool) {
	if id.Subject == "" {
		return principal.Principal{}, false
	}
	// The subject is a controlled id (opaque, from the IdP), but sanitize it the
	// same way as header/env users for defense in depth (length cap, control-char
	// strip). A control-only value sanitizes to "" and is rejected.
	user := sanitizeUser(id.Subject)
	if user == "" {
		return principal.Principal{}, false
	}
	// Org and roles get the same treatment. Roles are already bounded by the
	// verifier's projection; sanitizing here covers control chars and any future
	// verifier that forgets to. A role that sanitizes away is dropped rather than
	// kept as "" — an empty role name can never match a policy mapping, so
	// keeping it would only pad the attribution set.
	roles := make([]string, 0, len(id.Roles))
	for _, role := range id.Roles {
		if s := sanitizeUser(role); s != "" {
			roles = append(roles, s)
		}
	}
	return principal.Verified(
		user, principal.ToolDataEntry,
		sanitizeUser(id.OrgID), sanitizeUser(id.OrgSlug), roles), true
}

// stripBearer trims the header value and removes an optional "Bearer" auth
// scheme (case-insensitive, per RFC 6750, tolerating any run of whitespace after
// it) so the header may carry either a raw JWT or an "Authorization: Bearer <jwt>"
// value. Returns the bare token, or "" if the value is empty.
func stripBearer(v string) string {
	v = strings.TrimSpace(v)
	const scheme = "bearer"
	if len(v) > len(scheme) && strings.EqualFold(v[:len(scheme)], scheme) {
		rest := v[len(scheme):]
		if trimmed := strings.TrimLeft(rest, " \t"); trimmed != rest {
			// Only strip when the scheme was actually followed by whitespace, so a
			// token that merely starts with "bearer" isn't mangled.
			return strings.TrimSpace(trimmed)
		}
	}
	return v
}

// ChainResolvers returns a resolver that tries each supplied
// resolver in order and returns the first one whose User is
// non-empty. If no resolver yields a user, falls back to
// [defaultPrincipalResolver] (Tool=data-entry, User=unknown). Used
// by cmd/rela-server to layer env → header → default.
//
// **Chain contract for resolver authors.** Return a zero
// [principal.Principal] (User=="") to signal fall-through. The
// chain advances on `p.User == ""` and *ignores* Tool — every
// data-entry resolver hard-codes Tool=ToolDataEntry today, so
// distinguishing on Tool would be cosmetic. If a future resolver
// needs to return a different Tool, give it a non-empty User too
// and the chain will honor both.
func ChainResolvers(resolvers ...PrincipalResolver) PrincipalResolver {
	return func(r *http.Request) principal.Principal {
		for _, resolve := range resolvers {
			p := resolve(r)
			if p.User != "" {
				return p
			}
		}
		return defaultPrincipalResolver(r)
	}
}

// sanitizeUser is the input filter for principal.User values derived
// from an HTTP header or env var. Replaces C0 (\x00-\x1f) and DEL
// (\x7f) with regular spaces in a single pass, truncates to
// [principalUserMaxLen] runes (UTF-8 safe), and trims surrounding
// whitespace. Returns "" when the cleaned value is empty so chained
// resolvers can fall through.
//
// **Important — order matters.** Control-char replacement runs
// *before* the final whitespace trim. A value of `"\x00\x00"`
// would survive `strings.TrimSpace` (NULs are not whitespace),
// then become `"  "` after substitution; the final trim catches it
// and returns "". Trimming first would let such payloads through
// as literal-space user attribution in the audit log.
func sanitizeUser(raw string) string {
	if raw == "" {
		return ""
	}
	// Single pass: replace control chars + length-cap.
	out := make([]rune, 0, principalUserMaxLen)
	var n int
	for _, r := range raw {
		if n >= principalUserMaxLen {
			break
		}
		if isControlRune(r) {
			out = append(out, ' ')
		} else {
			out = append(out, r)
		}
		n++
	}
	return strings.TrimSpace(string(out))
}

func isControlRune(r rune) bool {
	return r <= 0x1f || r == 0x7f
}

// stampAuditPrincipal stamps a Principal (resolved by resolve) on
// every request ctx. See plan AC4 for the test that pins this
// behavior.
func stampAuditPrincipal(next http.Handler, resolve PrincipalResolver) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := principal.With(r.Context(), resolve(r))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// spaHandler wraps a filesystem and serves index.html for any path that doesn't
// match an existing file. This enables client-side routing in SPAs.
func spaHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "" || path == "/" {
			path = "index.html"
		}

		// Check if the file exists
		if _, err := fs.Stat(fsys, strings.TrimPrefix(path, "/")); err != nil {
			// File doesn't exist, serve index.html for SPA routing
			r.URL.Path = "/"
		}

		fileServer.ServeHTTP(w, r)
	})
}
