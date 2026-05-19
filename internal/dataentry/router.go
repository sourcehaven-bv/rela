package dataentry

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// envDataEntryUser is the local-dev escape hatch: if this env var is
// set, EnvPrincipalResolver returns its value as the principal user.
// Documented in docs/security.md alongside the --principal-header
// flag.
const envDataEntryUser = "RELA_DATAENTRY_USER"

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
	inner.HandleFunc("/api/toggle-checkbox", a.handleToggleCheckbox)
	inner.HandleFunc("/api/help/", a.handleEntityHelp)
	inner.HandleFunc("/api/command/", a.handleCommandExec)
	inner.HandleFunc("/api/command-cancel/", a.handleCommandCancel)
	inner.HandleFunc("/api/open-file", a.handleOpenFile)
	inner.HandleFunc("/api/git/status", a.handleGitStatus)
	inner.HandleFunc("/api/git/sync", a.handleGitSync)

	// REST API v1 - main API for Vue SPA
	a.registerAPIV1Routes(inner)

	// noCacheMiddleware sets no-cache headers on API responses so that
	// browsers always fetch fresh data after file changes trigger a reload.
	mux.Handle("/api/", a.noCacheMiddleware(inner))

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
	handler = stampAuditPrincipal(handler, resolver)
	return handler
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
// each request, stamping Tool=data-entry. An empty headerName
// disables the resolver — it returns a zero principal so a chained
// resolver can take over (the typical wiring is env → header →
// default, with empty results falling through).
//
// **Trust boundary.** The header value is only as trustworthy as
// the reverse proxy that sets it. Operators serving data-entry
// without a trusted proxy must not enable this resolver — anyone
// can spoof identity by setting the header on the wire. See
// docs/security.md for the deployment guidance.
//
// Sanitization: the header value is trimmed, length-capped at 256
// runes, and C0/DEL control characters are replaced with a regular
// space. Same policy audit.Filesystem applies to record fields.
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

// ChainResolvers returns a resolver that tries each supplied
// resolver in order and returns the first one whose User is
// non-empty. If no resolver yields a user, falls back to
// [defaultPrincipalResolver] (Tool=data-entry, User=unknown). Used
// by cmd/rela-server to layer env → header → default.
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

// sanitizeUser is the shared input filter for principal.User values
// derived from an HTTP header or env var. Trims surrounding
// whitespace, truncates to [principalUserMaxLen] runes (UTF-8
// safe), and replaces C0 (\x00-\x1f) and DEL (\x7f) with a regular
// space. Returns "" when the cleaned value is empty so chained
// resolvers can fall through.
func sanitizeUser(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) > principalUserMaxLen {
		s = truncateRunes(s, principalUserMaxLen)
	}
	if !hasControlRune(s) {
		return s
	}
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if isControlRune(r) {
			out = append(out, ' ')
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

func truncateRunes(s string, limit int) string {
	out := make([]rune, 0, limit)
	for i, r := range []rune(s) {
		if i >= limit {
			break
		}
		out = append(out, r)
	}
	return string(out)
}

func hasControlRune(s string) bool {
	for _, r := range s {
		if isControlRune(r) {
			return true
		}
	}
	return false
}

func isControlRune(r rune) bool {
	return (r >= 0 && r <= 0x1f) || r == 0x7f
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
