package dataentry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/access"
	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/readtime"
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
//
// It fails in a headless build too, which embeds no SPA by design: a caller
// that serves the UI must not run headless. rela-server skips the check there.
func CheckEmbeddedSPA() error {
	spaFS, err := fs.Sub(staticFiles, "static/v2")
	// coverage-ignore-start: defensive: fs.Sub on the compile-time embed.FS with the literal "static/v2" path cannot
	// fail
	if err != nil {
		return fmt.Errorf("mount embedded SPA filesystem (static/v2): %w", err)
	}
	if _, err := fs.Stat(spaFS, spaIndexFile); err != nil {
		// coverage-ignore-end
		return fmt.Errorf("embedded SPA is missing index.html (run `just build-frontend`): %w", err)
	}
	return nil
}

// RouterOption configures [App.NewRouter].
type RouterOption func(*routerConfig)

type routerConfig struct {
	// accessLog, when non-nil, receives one `request` record per request at
	// Info (method, path, status, wall and database time). See requestStats
	// for why it never adds a header.
	accessLog *slog.Logger

	// queryCeiling is the per-request statement count past which a request
	// is logged as a warning; zero is off. See warnQueryCeiling.
	queryCeiling int64

	// spaFS, when non-nil, replaces the embedded SPA build. Tests only: the
	// CI Test job does not build the frontend, so without it `/` answers 404
	// and a client that reads the discovery Link header only on a 2xx (as
	// restish does) never sees it.
	spaFS fs.FS
}

// withSPAFS serves fsys as the SPA build. See routerConfig.spaFS.
func withSPAFS(fsys fs.FS) RouterOption {
	return func(c *routerConfig) { c.spaFS = fsys }
}

// WithAccessLog sends the per-request access log to l; nil turns it off.
func WithAccessLog(l *slog.Logger) RouterOption {
	return func(c *routerConfig) { c.accessLog = l }
}

// WithQueryCeiling logs a warning for every request that runs more than n
// store statements; zero or less turns the check off.
func WithQueryCeiling(n int64) RouterOption {
	return func(c *routerConfig) { c.queryCeiling = n }
}

// logGitSync logs that git sync is on. NewRouter calls it, after tenant
// mode had its chance to turn git sync off.
func (a *App) logGitSync() {
	if a.gitOps == nil {
		return
	}
	if g := a.Cfg().Git; g != nil {
		slog.Info("git sync enabled", "mode", g.Mode)
	}
}

// NewRouter returns an http.Handler with all data entry routes registered.
// The Vue SPA serves as the primary UI at the root path.
//
// When adding a route, add a probe to the route table in
// router_walk_test.go so registration stays covered.
//
// Request order, outermost first: requestStats, stampAuditPrincipal, the JWT
// gate, then the ACL request, then world selection. The ACL reads the stamped
// (and JWT-corrected) principal, and the world grant check runs after the read
// gate is attached. SetJWTGate must run before NewRouter.
func (a *App) NewRouter(opts ...RouterOption) http.Handler {
	var cfg routerConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	mux := http.NewServeMux()
	a.logGitSync()

	staticFS, err := fs.Sub(staticFiles, "static")
	// coverage-ignore-start: panic-invariant: fs.Sub on the compile-time embed.FS with the literal "static" path cannot
	// fail
	if err != nil {
		panic("failed to mount embedded static filesystem (static): " + err.Error())
	}
	// coverage-ignore-end
	serveUI := !Headless || cfg.spaFS != nil
	if serveUI {
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	}

	spaFS, err := fs.Sub(staticFiles, "static/v2")
	// coverage-ignore-start: panic-invariant: fs.Sub on the compile-time embed.FS with the literal "static/v2" path
	// cannot fail
	if err != nil {
		panic("failed to mount embedded SPA filesystem (static/v2): " + err.Error())
	}
	// coverage-ignore-end
	if cfg.spaFS != nil {
		spaFS = cfg.spaFS
	}

	mux.HandleFunc("/api/events", a.sse.handleSSE)
	mux.HandleFunc("/api/v1/_events", a.sse.handleSSE)
	mux.HandleFunc("GET "+CollabPathPrefix+"{type}/{id}", func(w http.ResponseWriter, r *http.Request) {
		handleCollab(a.collab, w, r)
	})

	inner := http.NewServeMux()

	inner.HandleFunc("/api/help/", a.handleEntityHelp)
	a.commands.registerCommandRoutes(inner)
	inner.HandleFunc("/api/git/status", a.gitAPI.handleGitStatus)
	inner.HandleFunc("/api/git/sync", a.gitAPI.handleGitSync)

	a.registerAPIV1Routes(inner)
	inner.HandleFunc("/api/v1/_events/", a.live.handlePut)

	registerMCPRoute(inner, a.mcpHandler)

	mux.Handle("/api/", a.noCacheMiddleware(a.live.recordServed(inner)))

	a.registerWebhookRoutes(mux)

	registerDeclarativeWebhookRoutes(mux, &webhookRouter{
		state: a.State, write: a.write, rawStore: a.store,
		admit: make(chan struct{}, webhookMaxInFlight),
	})

	shell, shellErr := fs.ReadFile(spaFS, spaIndexFile)
	custom := newCustomAssets(a.assets, shell, func() bool {
		return shellErr == nil && !a.State().Cfg.App.DisableCustomInjection
	})
	if serveUI {
		mux.HandleFunc(customURLPrefix, custom.serveAsset)
	}

	root := headlessRoot()
	if serveUI {
		root = spaHandlerWithCustom(spaFS, custom, func() ShellBoot {
			cfg := a.State().Cfg
			return shellBootOf(cfg.App, cfg.Account)
		})
	}
	spa := withSpecLink(root)
	if a.caldavAliases != nil {
		if routes := newCalDAVRoutes(a); routes != nil {
			routes.registerWellKnown(mux, spa)
		}
	} else {
		mux.Handle("/", spa)
	}

	var handler http.Handler = mux
	if a.security != nil {
		handler = a.security.requireSameOrigin(handler)
		handler = a.security.requireLocalHost(handler)
	}
	resolver := a.principalResolver
	if resolver == nil {
		resolver = defaultPrincipalResolver
	}

	if d, ok := a.acl.(*acl.Declarative); ok && d != nil {
		warnUnmatchedRejectWithoutJWTGate(d.Policy(), a.jwtGate != nil)
	}
	handler = responseGates(handler, a)
	replay := liveReplayChain(a)
	a.live.replay.Store(&replay)
	handler = stashReplayPrincipal(handler)
	if a.jwtGate != nil {
		handler = requireVerifiedJWT(handler, *a.jwtGate)
		a.State().OpenAPIGen.SetAuthHeader(a.jwtGate.HeaderName)
	}
	handler = stampAuditPrincipal(handler, resolver)
	handler = requestStats(handler, cfg.accessLog, (&routeWordCache{schema: a.State}).words, cfg.queryCeiling)
	return handler
}

// warnUnmatchedRejectWithoutJWTGate warns when `unmatched_principal: reject` is
// configured but inert, and names why: no JWT gate is wired, or the
// principal_property lookup is disabled. It reads acl.Policy.RejectEffective,
// the same predicate the enforcement site uses, so the two cannot disagree.
//
// It warns rather than fails because acl.yaml is shared with the CLI and the
// scheduler, which never call NewRouter. It also makes a reordering of
// SetJWTGate after NewRouter visible at startup.
func warnUnmatchedRejectWithoutJWTGate(p *acl.Policy, verifiedIdentity bool) {
	if p == nil || p.EffectiveUnmatchedPrincipal() != acl.UnmatchedReject {
		return
	}
	if p.RejectEffective(verifiedIdentity) {
		return
	}
	switch {
	case !verifiedIdentity:
		slog.Warn("acl: unmatched_principal: reject is configured but no JWT gate is wired; " +
			"the setting has NO effect and unmatched principals are treated as anonymous. " +
			"Wire a JWT gate (SetJWTGate before NewRouter), or remove the key to make the " +
			"posture explicit.")
	default:
		slog.Warn("acl: unmatched_principal: reject is configured but the principal_property " +
			"lookup is not enabled; the setting has NO effect and unmatched principals are " +
			"treated as anonymous. Set user_entity_type and principal_property, or remove " +
			"the key to make the posture explicit.")
	}
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

// isUnnamedStamp reports whether ctx carries a principal [stampAuditPrincipal]
// stamped without a user: no resolver named one (no proxy header, no
// RELA_DATAENTRY_USER). That is a request without credentials, a client
// error. A proxy that sends the placeholder value "unknown" lands here too.
//
// A ctx the stamper never ran on does not match, and neither does a named
// user with an unknown tool. Both are wiring faults and keep their 500.
//
// This is not an access predicate: it says nothing about
// Identity.UnmatchedVerified or any grant. Its one use picks the status code
// of a refusal. That 401 carries no WWW-Authenticate header, like the JWT
// gate and writeNoIdentity: header mode has no scheme a client could answer.
func isUnnamedStamp(ctx context.Context) bool {
	p, ok := principal.Stamped(ctx)
	return ok && (p.User == "" || p.User == principal.Unknown)
}

// attachACLRequest opens the request's ACL state through [access.Opener.Open]
// for the principal stamped on ctx and attaches the read gate built from it,
// so one member-of walk serves the whole request. Wired only for a
// *acl.Declarative.
//
// Only `/api/` requests are gated, so the SPA shell stays reachable when the
// stamper is misconfigured. Inside `/api/` a failure to open refuses the
// request, so ACL never fails open: 401 for a request without a user
// ([isUnnamedStamp]), 500 otherwise. A request already attached upstream is
// reused after a principal check.
func attachACLRequest(next http.Handler, d *acl.Declarative, jwtVerified bool) http.Handler {
	opener, oerr := access.NewOpener(d, jwtVerified)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAPIPath(r.URL.Path) {
			next.ServeHTTP(w, r.WithContext(withACLEnforced(r.Context())))
			return
		}
		if oerr != nil {
			slog.Warn("acl: attachACLRequest: no opener", "err", oerr, "path", r.URL.Path)
			writeV1Error(w, r, http.StatusInternalServerError,
				"acl_internal", "ACL gate construction failed", "check server logs")
			return
		}
		log := slog.With("path", r.URL.Path, "method", r.Method)
		ctx, req, err := opener.Open(r.Context(), log)
		switch {
		case errors.Is(err, access.ErrPrincipalMismatch):
			writeV1Error(w, r, http.StatusInternalServerError,
				"acl_principal_mismatch",
				"Upstream ACL request principal does not match context principal",
				"check server logs")
			return
		case errors.Is(err, acl.ErrUnstampedPrincipal) && isUnnamedStamp(r.Context()):
			writeV1Error(w, r, http.StatusUnauthorized, "unauthenticated", "Authentication required",
				"no user identity: local $RELA_DATAENTRY_USER, or rela-server --jwt-* or --principal-header; "+
					"see docs/server-security.md")
			return
		case errors.Is(err, acl.ErrUnstampedPrincipal):
			writeV1Error(w, r, http.StatusInternalServerError,
				"acl_unstamped_principal",
				"Principal could not be resolved under ACL",
				"principal stamper produced an unstamped identity; check server logs")
			return
		case err != nil && requestCanceled(r, err):
			return
		case err != nil:
			writeV1Error(w, r, http.StatusInternalServerError,
				"acl_identity_unresolved",
				"Principal could not be resolved under ACL",
				"identity resolution failed; check server logs")
			return
		}
		gate, gerr := newACLReadGate(req)
		// coverage-ignore-start: defensive: Open returns a non-nil Request whenever its error is nil
		if gerr != nil {
			slog.Warn("acl: attachACLRequest: newACLReadGate failed",
				"err", gerr, "path", r.URL.Path)
			writeV1Error(w, r, http.StatusInternalServerError,
				"acl_internal", "ACL gate construction failed", "check server logs")
			return
		}
		// coverage-ignore-end
		next.ServeHTTP(w, r.WithContext(withReadGate(withACLEnforced(ctx), gate)))
	})
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
		User: principal.Unknown,
		Tool: principal.ToolDataEntry,
	}
}

// HeaderPrincipalResolver reads Principal.User from headerName on each
// request, stamping Tool=data-entry. The resolver is never nil; an empty
// headerName yields one that always returns a zero Principal.
//
// Trust boundary: the header is only as trustworthy as the reverse proxy that
// sets it. Without a trusted proxy anyone can spoof identity; see
// docs/server-security.md. The value is sanitized by [sanitizeUser], so a
// control-only value falls through.
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

	// OrgName is the verified `org_name` claim, the org's display name. Shown
	// by the account menu (GET /api/v1/_me); nothing in the ACL evaluates it.
	OrgName string

	// PrincipalType and Scopes drive client attenuation (TKT-IAC8TX): the
	// former selects a ceiling in acl.yaml, the latter re-opens pieces of it.
	// Both are absent for a proxy that doesn't model them, which means "no
	// ceiling applies" — the principal keeps its acting user's grants.
	PrincipalType string
	Scopes        []string

	// Email is the verified `email` claim, when the proxy supplies one. Absent
	// for a proxy that doesn't model it, which is not an error. Threaded onto the
	// Principal so lazy provisioning (TKT-ANUJDS) can stamp it on a stub user
	// entity; nothing in the ACL evaluates it.
	Email string

	// ExpiresAt is when the verifier stops accepting the assertion; see
	// withCredentialExpiry. Zero means no expiry is known.
	ExpiresAt time.Time
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
// which fails closed. This resolver falls through on a verification failure,
// which under a header chain is an auth downgrade.
//
// JWTPrincipalResolver verifies a signed assertion from headerName (ES256
// against the proxy's JWKS, via v) and stamps the verified subject, org and
// role claims via [principal.Verified]. It is the only resolver that may carry
// roles: a role from a spoofable header would bypass authorization. A missing
// header, a failed verification, a nil v or an empty headerName yields a zero
// Principal. A "Bearer " prefix is stripped case-insensitively.
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
		p, reason := verifiedPrincipal(id, principal.ToolDataEntry)
		if !reason.ok() {
			return principal.Principal{}
		}
		return p
	}
}

// verifiedPrincipal projects a verified [AssertedIdentity] into a stamped
// Principal, or reports a reject reason when the subject is unusable. It is
// the single place assertion claims become a Principal, shared by
// [JWTPrincipalResolver] and [requireVerifiedJWT] so the two cannot drift.
//
// Call it ONLY on the output of a completed signature verification: a role
// from an unverified source would bypass authorization. tool is the audit
// attribution of the surface; it is a parameter because
// [principal.VerifiedFrom] is the only constructor that sets the role fields.
func verifiedPrincipal(id AssertedIdentity, tool string) (principal.Principal, rejectReason) {
	if id.Subject == "" {
		return principal.Principal{}, rejectUnusable
	}
	user := sanitizeUser(id.Subject)
	if user == "" {
		return principal.Principal{}, rejectUnusable
	}
	if principal.IsReserved(user) {
		return principal.Principal{}, rejectReserved
	}
	roles := sanitizeClaimList(id.Roles)
	scopes := sanitizeClaimList(id.Scopes)
	return principal.VerifiedFrom(user, tool, principal.Claims{
		OrgID:         sanitizeUser(id.OrgID),
		OrgSlug:       sanitizeUser(id.OrgSlug),
		OrgName:       sanitizeUser(id.OrgName),
		Roles:         roles,
		PrincipalType: sanitizeUser(id.PrincipalType),
		Scopes:        scopes,
		Email:         sanitizeUser(id.Email),
	}), rejectNone
}

// rejectReason says WHY [verifiedPrincipal] refused a subject, or that it did
// not. It replaced a bare `ok bool` so the caller's log classification is driven
// by the same evaluation that made the decision (RR-OJRCNY).
//
// The bug it prevents: the gate used to re-derive "was this reserved?" from the
// RAW `id.Subject`, while verifiedPrincipal decided on the SANITIZED value. Those
// disagree exactly when sanitizing CREATES a reserved name — "\x01system:scheduler"
// was correctly denied but logged as a benign unusable-subject INFO rather than
// the security WARN, so the one log line operators are told to grep for missed
// the variant an attacker would actually send. Returning the reason makes the
// two impossible to drift, which is the same argument that made verifiedPrincipal
// a single shared function in the first place.
type rejectReason int

const (
	// rejectNone means the subject was accepted; the Principal is usable.
	rejectNone rejectReason = iota
	// rejectUnusable means the subject was empty, or sanitized away entirely —
	// an IdP/claims problem for the operator to chase.
	rejectUnusable
	// rejectReserved means the subject named a reserved `system:` identity —
	// a misconfigured IdP, or someone probing for TKT-9PCL7D.
	rejectReserved
)

// ok reports whether the subject was accepted.
func (r rejectReason) ok() bool { return r == rejectNone }

// sanitizeClaimList applies sanitizeUser to every element, dropping any that
// sanitizes to empty. Shared by the roles and scopes projections: both are
// already bounded by the verifier, and both are policy-map keys where an empty
// string can never match anything.
func sanitizeClaimList(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if s := sanitizeUser(v); s != "" {
			out = append(out, s)
		}
	}
	return out
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
			return strings.TrimSpace(trimmed)
		}
	}
	return v
}

// ChainResolvers returns a resolver that tries each resolver in order and
// returns the first one with a non-empty User, falling back to
// [defaultPrincipalResolver]. Used by cmd/rela-server to layer
// env → header → default.
//
// A resolver signals fall-through with a zero [principal.Principal]. The chain
// ignores Tool; a resolver that needs another Tool must also return a User.
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

// sanitizeUser filters a principal.User value from a header or env var: it
// replaces C0 and DEL with spaces, truncates to [principalUserMaxLen] runes and
// then trims whitespace, so a control-only value becomes "" and falls through.
// The replacement must run before the trim.
//
// It does not reject reserved `system:` principals: it can only signal
// fall-through, a silent demotion. [stampAuditPrincipal] and
// [verifiedPrincipal] reject them loudly. See [principal.IsReserved].
func sanitizeUser(raw string) string {
	if raw == "" {
		return ""
	}
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

// stampAuditPrincipal stamps the Principal resolve returns on every request
// ctx.
//
// A reserved principal ([principal.IsReserved]) comes from request-controlled
// input and would be a privilege escalation, so it is refused: 403 on `/api/`,
// and elsewhere the request is served as the anonymous default so the SPA
// stays reachable. The JWT gate repeats the check on its verified subject via
// [verifiedPrincipal].
func stampAuditPrincipal(next http.Handler, resolve PrincipalResolver) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := resolve(r)
		if principal.IsReserved(p.User) {
			slog.WarnContext(r.Context(), "rejected reserved principal asserted by request",
				"user", p.User,
				"path", r.URL.Path,
				"method", r.Method,
				"remote_addr", r.RemoteAddr)
			if isAPIPath(r.URL.Path) {
				writeV1Error(w, r, http.StatusForbidden, "reserved_principal",
					"Reserved principal",
					"A '"+principal.ReservedPrefix+"' or '"+principal.IntegrationPrefix+
						"' prefixed identity is internal to rela and cannot be asserted by a request.")
				return
			}
			p = defaultPrincipalResolver(r)
		}
		ctx := principal.With(r.Context(), p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// openAPISpecPath is where the generated OpenAPI spec is served.
const openAPISpecPath = "/api/v1/_openapi.json"

// withSpecLink announces the OpenAPI spec on a GET or HEAD of the site root
// with an RFC 8631 `Link: rel="service-desc"` header. A generic client given
// only the origin, such as restish, follows it to the spec. The spec itself
// stays an /api/ path behind the same gate as every other API call.
func withSpecLink(next http.Handler) http.Handler {
	link := "<" + openAPISpecPath + `>; rel="service-desc"`
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
			w.Header().Add("Link", link)
		}
		next.ServeHTTP(w, r)
	})
}

// headlessRootBody is what a headless build answers at `/`. It names only the
// API paths, which every build serves, so it discloses nothing.
const headlessRootBody = "rela-server (headless): REST API at /api/v1, OpenAPI at " + openAPISpecPath + "\n"

// headlessRoot is the root catch-all of a headless build: GET or HEAD `/`
// answers a short text pointer to the API, and every other path answers 404.
// [withSpecLink] adds the OpenAPI Link header, so a client that discovers the
// API from `/` keeps working.
func headlessRoot() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
			http.NotFound(w, r)
			return
		}
		h := w.Header()
		h.Set("Content-Type", "text/plain; charset=utf-8")
		h.Set("Content-Length", strconv.Itoa(len(headlessRootBody)))
		if r.Method == http.MethodHead {
			return
		}
		_, _ = io.WriteString(w, headlessRootBody)
	})
}

// spaHandlerWithCustom wraps spaHandler so a request resolving to the SPA
// shell gets a variant referencing the operator's custom.css/custom.js. An
// unreadable shell degrades to the plain SPA handler; real files pass through.
//
// The four shells are precomputed, and which one to serve is chosen per
// request by an existence check, so adding custom.css needs no restart. boot
// names the app and its language and is rewritten per request, because
// data-entry.yaml is reloadable.
func spaHandlerWithCustom(fsys fs.FS, custom *customAssets, boot func() ShellBoot) http.Handler {
	return http.HandlerFunc(spaShell{fsys: fsys, custom: custom, boot: boot, fallback: spaHandler(fsys)}.handleShell)
}

// spaShell serves the SPA for [spaHandlerWithCustom]. A named handler, so a
// trace of a request to the SPA names it.
type spaShell struct {
	fsys     fs.FS
	custom   *customAssets
	boot     func() ShellBoot
	fallback http.Handler
}

//archplan:auth=none reason="the SPA shell and its static files load before login"
func (s spaShell) handleShell(w http.ResponseWriter, r *http.Request) {
	body := s.custom.shell(r.Context())
	if body == nil || !servesSPAShell(s.fsys, r.URL.Path) {
		s.fallback.ServeHTTP(w, r)
		return
	}
	body = bootShell(body, s.boot())

	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Length", strconv.Itoa(len(body)))
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(body)
}

// servesSPAShell reports whether a request path falls through to the SPA shell
// rather than resolving to a real embedded file. Mirrors spaHandler's rule.
func servesSPAShell(fsys fs.FS, urlPath string) bool {
	if urlPath == "" || urlPath == "/" {
		return true
	}
	_, err := fs.Stat(fsys, strings.TrimPrefix(urlPath, "/"))
	return err != nil
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

		if _, err := fs.Stat(fsys, strings.TrimPrefix(path, "/")); err != nil {
			r.URL.Path = "/"
		}

		fileServer.ServeHTTP(w, r)
	})
}

// responseGates wraps h in the middleware that decides what a GET returns:
// attachACLRequest outermost, then attachWorld, then attachAsOf, so at
// request time the ACL request exists before the world check reads it, and
// the read instant is bound last, after identity is resolved on today's
// state. NewRouter and
// liveReplayChain both call it, so a live-view replay passes the same gates
// as the GET it stands for.
func responseGates(h http.Handler, a *App) http.Handler {
	h = attachAsOf(h, a)
	h = attachWorld(h, a)
	if d, ok := a.acl.(*acl.Declarative); ok && d != nil {
		h = attachACLRequest(h, d, a.jwtGate != nil)
	}
	return attachReadTime(h)
}

// attachReadTime stamps the request's read time on its context (see
// internal/readtime), outermost, so every conditional grant (TKT-TKSCSBT)
// the request evaluates is judged at one instant. It replaces a time already on ctx: each request, and each live-view replay,
// is its own operation.
func attachReadTime(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(readtime.With(r.Context(), nowFunc())))
	})
}

// liveReplayChain is the handler a live-view replay runs through
// (TKT-HJ5Z9Y): the v1 API routes on their own mux, so a replay cannot reach
// the event stream, webhooks or the SPA, wrapped in responseGates. The
// transport checks (host, origin, CSRF, request stats) are left out: they
// judge the browser request, which the stream already passed, and do not
// change a response body.
func liveReplayChain(a *App) http.Handler {
	api := http.NewServeMux()
	a.registerAPIV1Routes(api)
	return responseGates(api, a)
}
