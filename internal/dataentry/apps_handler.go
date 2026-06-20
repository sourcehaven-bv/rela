package dataentry

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// appCSP returns the path-scoped Content-Security-Policy for an app. It is the
// whole security boundary for an app: the app loads from /api/v1/_apps/<id>/
// (so it is same-origin with the API), and this header is what stops it from
// reaching anything but its own files plus the bridge.
//
// base MUST be an absolute scheme://host/path/ prefix (e.g.
// "http://127.0.0.1:8099/api/v1/_apps/dash/"), NOT a bare path: a CSP source
// expression without a host is invalid and browsers ignore it, falling back to
// default-src 'none' (which silently blocks the app's own scripts). We build it
// from the request scheme+host.
//
//   - Resource directives are scoped to the app's own absolute subpath, NOT
//     'self': 'self' would include /api/, letting `<img src="/api/v1/tickets/x">`
//     pull data. Scoping to the app's path confines resource loads to the app.
//   - connect-src 'none' blocks the app's own fetch/XHR/WebSocket — so the only
//     way to the API is the host MessageChannel bridge (a message post, not a
//     network connection, so it is unaffected).
//   - form-action 'none' + the iframe sandbox (no allow-top-navigation) block
//     form-POST and navigation exfiltration.
func appCSP(base string) string {
	return strings.Join([]string{
		"default-src 'none'",
		"script-src " + base + " 'unsafe-inline'",
		"style-src " + base + " 'unsafe-inline'",
		"img-src " + base + " data: blob:",
		"font-src " + base,
		"connect-src 'none'",
		"form-action 'none'",
		"base-uri 'none'",
		"frame-src 'none'",
		"child-src 'none'",
		"frame-ancestors 'self'",
	}, "; ")
}

// appBaseURL builds the absolute app base prefix (scheme://host/api/v1/_apps/<id>/)
// from the request, for use in the path-scoped CSP. Scheme follows TLS / the
// X-Forwarded-Proto hint from a trusted proxy; host is the request Host.
func appBaseURL(r *http.Request, id string) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host + "/api/v1/_apps/" + id + "/"
}

// handleV1App serves a custom app's files for embedding in a sandboxed iframe.
// Endpoints under GET /api/v1/_apps/<id>/...:
//
//   - /api/v1/_apps/<id>/            → the app's index.html
//   - /api/v1/_apps/<id>/<path>      → a sibling file from the app directory
//   - /api/v1/_apps/<id>/_rela.js    → the in-iframe bridge SDK (reserved)
//
// Apps are folder-discovered: an app is apps/<id>/ containing index.html (no
// config registry). The app loads from this real URL (so its sub-resources
// resolve) and is same-origin with the API; the path-scoped CSP header is what
// keeps it confined to its own files + the bridge. The app inherits the
// logged-in user's permissions — it talks to the API only through the host
// MessageChannel bridge, so it can do nothing the user couldn't already do.
func (a *App) handleV1App(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	// Path shape: /api/v1/_apps/<id>[/<entry...>]
	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/_apps/")
	id, entry, _ := strings.Cut(rest, "/")
	if !dataentryconfig.ValidAppID(id) {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_app_id", "Invalid app ID", "")
		return
	}

	// A bare /_apps/<id> (no trailing slash) would make the app's relative
	// asset URLs resolve against /_apps/ instead of /_apps/<id>/. Redirect to
	// the canonical trailing-slash form.
	if !strings.Contains(rest, "/") {
		http.Redirect(w, r, "/api/v1/_apps/"+id+"/", http.StatusMovedPermanently)
		return
	}

	if !appExists(a.paths.Root, id) {
		writeV1Error(w, r, http.StatusNotFound, "app_not_found", "App not found", "")
		return
	}

	h := w.Header()
	h.Set("Content-Security-Policy", appCSP(appBaseURL(r, id)))
	h.Set("X-Content-Type-Options", "nosniff")

	// Reserved endpoints — served from the binary, not the app's files.
	if entry == appSDKEntry {
		h.Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = w.Write([]byte(appSDKSource()))
		return
	}
	if entry == appCSSEntry {
		h.Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write([]byte(appCSSSource()))
		return
	}

	if entry == "" {
		entry = appIndexFile
	}
	// Disallow serving reserved (underscore-prefixed) entry names from the
	// app's own files, so an app can't shadow _rela.js.
	if strings.HasPrefix(entry, "_") {
		writeV1Error(w, r, http.StatusNotFound, "app_entry_not_found", "Not found", "")
		return
	}

	body, err := openAppEntry(a.paths.Root, id, entry)
	if err != nil {
		writeV1Error(w, r, http.StatusNotFound, "app_entry_not_found", "Not found", "")
		return
	}

	h.Set("Content-Type", appEntryContentType(entry))
	_, _ = w.Write(body)
}

// appsToV1 projects the scanned apps to the client-facing view. Returns nil for
// an empty list so the JSON omits the "apps" key entirely.
func appsToV1(apps []appInfo) map[string]V1App {
	if len(apps) == 0 {
		return nil
	}
	out := make(map[string]V1App, len(apps))
	for _, app := range apps {
		out[app.ID] = V1App{
			Title:       app.Title,
			Label:       app.Label,
			Description: app.Description,
		}
	}
	return out
}

// scanAppsOrLog scans the project's apps/ directory, logging (not failing) on
// error so a transient scan problem degrades to "no apps" rather than breaking
// the whole config response.
func (a *App) scanAppsOrLog() []appInfo {
	apps, err := scanApps(a.paths.Root)
	if err != nil {
		slog.Warn("scanning apps directory failed", "error", err)
		return nil
	}
	return apps
}
