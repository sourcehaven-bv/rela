package dataentry

import (
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// appCSP returns the path-scoped Content-Security-Policy for an app, which is
// its whole security boundary. base must be an absolute scheme://host/path/
// prefix; a host-less source is ignored by browsers.
//
// Resource directives are scoped to the app's subpath, not 'self', which would
// include /api/. connect-src 'none' leaves the MessageChannel bridge as the
// only way to the API; form-action 'none' and the sandbox block form and
// navigation exfiltration. No 'unsafe-inline' (which also blocks style=""),
// as defense against an app's own injection bugs.
func appCSP(base string) string {
	return strings.Join([]string{
		"default-src 'none'",
		"script-src " + base,
		"style-src " + base,
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

// hostUnsafeForCSP matches any character not allowed in a hostname[:port] /
// bracketed IPv6 literal. Go's HTTP server accepts a few CSP-significant
// characters in the Host header (',  '  *  ;), and the host is spliced into the
// app's CSP source. A normal browser never emits those (they can't appear in a
// real DNS name), so this is defense-in-depth, not a known browser-exploitable
// hole — but it makes "the CSP is the boundary" airtight regardless of how the
// host string was produced.
var hostUnsafeForCSP = regexp.MustCompile(`[^a-zA-Z0-9.:\[\]-]`)

// appBaseURL builds the absolute app base prefix (scheme://host/api/v1/_apps/<id>/)
// from the request, for use in the path-scoped CSP. Scheme follows TLS / the
// X-Forwarded-Proto hint from a trusted proxy; host is the request Host. Returns
// ok=false if the Host contains characters that aren't valid in a host (and
// could otherwise inject CSP tokens) — the caller rejects the request.
func appBaseURL(r *http.Request, id string) (string, bool) {
	if r.Host == "" || hostUnsafeForCSP.MatchString(r.Host) {
		return "", false
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host + "/api/v1/_apps/" + id + "/", true
}

// handleV1App serves a custom app's files under GET /api/v1/_apps/<id>/: the
// index.html, sibling files, and the reserved bridge SDK `_rela.js`.
//
// An app is a folder apps/<id>/ with an index.html. It is same-origin with the
// API, so the path-scoped CSP header confines it. It reaches the API only
// through the host bridge, with the logged-in user's permissions.
//
//archplan:auth=required reason="serves app files; the app's API calls run as the user"
func (a *App) handleV1App(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/_apps/")
	id, entry, _ := strings.Cut(rest, "/")
	if !dataentryconfig.ValidAppID(id) {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_app_id", "Invalid app ID", "")
		return
	}

	if !strings.Contains(rest, "/") {
		http.Redirect(w, r, "/api/v1/_apps/"+id+"/", http.StatusMovedPermanently)
		return
	}

	if !appExists(r.Context(), a.assets, id) {
		writeV1Error(w, r, http.StatusNotFound, "app_not_found", "App not found", "")
		return
	}

	base, ok := appBaseURL(r, id)
	if !ok {
		writeV1Error(w, r, http.StatusBadRequest, "bad_host", "Invalid Host header", "")
		return
	}

	h := w.Header()
	h.Set("Content-Security-Policy", appCSP(base))
	h.Set("X-Content-Type-Options", "nosniff")

	if entry == appSDKEntry {
		h.Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = w.Write([]byte(appSDKSource()))
		return
	}
	if entry == appCSSEntry {
		h.Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write([]byte(appCSSSource(a.palette.Resolved())))
		return
	}
	if entry == appEditorEntry {
		serveCachedAsset(w, r, appContentTypes[".js"], appEditorJSETag(), appEditorSource())
		return
	}
	if entry == appEditorCSSEntry {
		serveCachedAsset(w, r, appContentTypes[".css"], appEditorCSSETag(), appEditorCSSSource())
		return
	}
	if entry == "" {
		entry = appIndexFile
	}
	if strings.HasPrefix(entry, "_") {
		writeV1Error(w, r, http.StatusNotFound, "app_entry_not_found", "Not found", "")
		return
	}

	body, err := openAppEntry(r.Context(), a.assets, id, entry)
	if err != nil {
		writeV1Error(w, r, http.StatusNotFound, "app_entry_not_found", "Not found", "")
		return
	}

	if entry == appIndexFile {
		if reason := validateBridgeVersion(parseAppMeta(body).BridgeVersion); reason != "" {
			slog.Warn("refusing app with invalid bridge version", "app", id, "reason", reason)
			writeV1Error(w, r, http.StatusUnprocessableEntity, "incompatible_app", reason, "")
			return
		}
	}

	h.Set("Content-Type", appEntryContentType(entry))
	_, _ = w.Write(body)
}

// appsToV1 projects the scanned apps to the client-facing view. Returns nil for
// an empty list so the JSON omits the "apps" key entirely.
func appsToV1(apps []appInfo) map[string]v1.App {
	if len(apps) == 0 {
		return nil
	}
	out := make(map[string]v1.App, len(apps))
	for _, app := range apps {
		out[app.ID] = v1.App{
			Title:       app.Title,
			Label:       app.Label,
			Description: app.Description,
		}
	}
	return out
}
