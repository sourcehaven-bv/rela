package dataentry

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/project"
)

func TestAppCSP_PathScopedNoEgress(t *testing.T) {
	// base MUST be an absolute scheme://host/path/ — a bare path is an invalid
	// CSP source that browsers ignore (falling back to default-src 'none', which
	// silently blocks the app's own scripts). This is the regression the
	// ab:// prefix guards against.
	const base = "http://127.0.0.1:8099/api/v1/_apps/dash/"
	csp := appCSP(base)

	// No egress: connect-src must be 'none' (the bridge is the only data path).
	if !strings.Contains(csp, "connect-src 'none'") {
		t.Errorf("app CSP must set connect-src 'none': %q", csp)
	}
	// default-src locked down.
	if !strings.Contains(csp, "default-src 'none'") {
		t.Errorf("app CSP must set default-src 'none': %q", csp)
	}
	// Resource directives must be PATH-SCOPED to the app, not 'self' (which
	// would include /api/). Assert script/style/img/font reference the base
	// path and never bare 'self'.
	for _, dir := range []string{"script-src", "style-src", "img-src", "font-src"} {
		idx := strings.Index(csp, dir+" ")
		if idx < 0 {
			t.Errorf("app CSP missing %s: %q", dir, csp)
			continue
		}
		seg := csp[idx:]
		if end := strings.Index(seg, ";"); end >= 0 {
			seg = seg[:end]
		}
		if !strings.Contains(seg, base) {
			t.Errorf("%s must be path-scoped to %q, got %q", dir, base, seg)
		}
		if strings.Contains(seg, "'self'") {
			t.Errorf("%s must not use 'self' (would include /api/): %q", dir, seg)
		}
		// Must be an absolute host source (scheme://...), not a bare path —
		// a path-only source is invalid CSP and silently blocks the app.
		if !strings.Contains(seg, "://") {
			t.Errorf("%s must use an absolute scheme://host source, got %q", dir, seg)
		}
	}
	// Exfil channels closed.
	for _, want := range []string{"form-action 'none'", "base-uri 'none'", "frame-ancestors 'self'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("app CSP missing %q: %q", want, csp)
		}
	}
}

func TestAppBaseURL(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "http://example.test:8080/api/v1/_apps/dash/", http.NoBody)
	req.Host = "example.test:8080"
	got, ok := appBaseURL(req, "dash")
	if !ok || got != "http://example.test:8080/api/v1/_apps/dash/" {
		t.Errorf("appBaseURL = (%q, %v)", got, ok)
	}
	// IPv6 host with brackets is allowed.
	req.Host = "[::1]:8080"
	if _, ok := appBaseURL(req, "dash"); !ok {
		t.Errorf("bracketed IPv6 host should be accepted")
	}
	// X-Forwarded-Proto from a trusted proxy upgrades the scheme.
	req.Host = "example.test:8080"
	req.Header.Set("X-Forwarded-Proto", "https")
	if got, ok := appBaseURL(req, "dash"); !ok || !strings.HasPrefix(got, "https://") {
		t.Errorf("expected https scheme via X-Forwarded-Proto, got (%q, %v)", got, ok)
	}
}

func TestAppBaseURL_RejectsCSPUnsafeHost(t *testing.T) {
	// Go's HTTP server accepts these chars in the Host header; they're
	// CSP-significant and must not reach the policy. A real browser never emits
	// them — this is defense-in-depth.
	req, _ := http.NewRequest(http.MethodGet, "http://x/api/v1/_apps/dash/", http.NoBody)
	for _, bad := range []string{"a,b", "a'b", "a*b", "a;b", "a b", ""} {
		req.Host = bad
		if _, ok := appBaseURL(req, "dash"); ok {
			t.Errorf("appBaseURL accepted unsafe host %q", bad)
		}
	}
}

func TestAppEntryContentType(t *testing.T) {
	cases := map[string]string{
		"app.js":    "text/javascript; charset=utf-8",
		"a.MJS":     "text/javascript; charset=utf-8", // case-insensitive
		"s.css":     "text/css; charset=utf-8",
		"i.svg":     "image/svg+xml",
		"f.woff2":   "font/woff2",
		"d.json":    "application/json",
		"x.unknown": "application/octet-stream", // safe fallback
		"noext":     "application/octet-stream",
	}
	for entry, want := range cases {
		if got := appEntryContentType(entry); got != want {
			t.Errorf("appEntryContentType(%q) = %q, want %q", entry, got, want)
		}
	}
}

// TestAppTokensCSSInSyncWithFrontend pins the embedded token stylesheet
// (apps_tokens.css) byte-for-byte to the SPA's source of truth
// (frontend/src/styles/tokens.css). They must never drift: the SPA and the app
// stylesheet share these tokens. If this fails, re-copy the frontend file.
func TestAppTokensCSSInSyncWithFrontend(t *testing.T) {
	frontend, err := os.ReadFile(filepath.Join("..", "..", "frontend", "src", "styles", "tokens.css"))
	if err != nil {
		t.Fatalf("read frontend tokens.css: %v", err)
	}
	if string(frontend) != appTokensCSS {
		t.Errorf("apps_tokens.css has drifted from frontend/src/styles/tokens.css — re-copy it")
	}
}

func TestAppCSSSource(t *testing.T) {
	css := appCSSSource()
	for _, want := range []string{"--text-color", ":root.dark", ".btn", ".btn-primary", ".input", ".card"} {
		if !strings.Contains(css, want) {
			t.Errorf("appCSSSource missing %q", want)
		}
	}
	// Stays tokens + atomic controls — must NOT smuggle in component-shaped
	// classes (the documented line).
	for _, unwanted := range []string{".table", ".modal", ".select", ".dropdown"} {
		if strings.Contains(css, unwanted) {
			t.Errorf("appCSSSource should not include component-shaped %q", unwanted)
		}
	}
}

func TestParseAppMeta(t *testing.T) {
	t.Run("reads rela-app meta tags", func(t *testing.T) {
		html := `<html><head>
			<meta name="rela-app:bridge-version" content="1">
			<meta name="rela-app:title" content="My App">
			<meta name="rela-app:label" content="App">
			<meta name="rela-app:description" content="does things">
			<meta name="other" content="ignored">
		</head><body>x</body></html>`
		got := parseAppMeta([]byte(html))
		if got.Title != "My App" || got.Label != "App" || got.Description != "does things" || got.BridgeVersion != 1 {
			t.Errorf("parseAppMeta = %+v", got)
		}
	})
	t.Run("absent meta → empty fields, BridgeVersion 0", func(t *testing.T) {
		got := parseAppMeta([]byte("<html><head></head><body>x</body></html>"))
		if got.Title != "" || got.Label != "" || got.Description != "" || got.BridgeVersion != 0 {
			t.Errorf("expected zero, got %+v", got)
		}
	})
	t.Run("non-integer bridge-version → 0 (invalid)", func(t *testing.T) {
		got := parseAppMeta([]byte(`<html><head><meta name="rela-app:bridge-version" content="abc"></head></html>`))
		if got.BridgeVersion != 0 {
			t.Errorf("expected 0 for non-integer, got %d", got.BridgeVersion)
		}
	})
}

// writeApp creates apps/<id>/ with the given entries under root.
func writeApp(t *testing.T, root, id string, entries map[string]string) {
	t.Helper()
	dir := filepath.Join(root, appsDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range entries {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestScanApps(t *testing.T) {
	root := t.TempDir()
	writeApp(t, root, "dash", map[string]string{
		"index.html": `<html><head><meta name="rela-app:bridge-version" content="1"><meta name="rela-app:label" content="Dashboard"></head><body>x</body></html>`,
		"app.js":     `console.log('hi')`,
	})
	writeApp(t, root, "counter", map[string]string{
		"index.html": `<html><head><meta name="rela-app:bridge-version" content="1"></head><body>x</body></html>`,
	})
	// Not listed: index.html without a bridge-version meta (required).
	writeApp(t, root, "noversion", map[string]string{
		"index.html": `<html><head></head><body>x</body></html>`,
	})
	// Not listed: bridge version newer than the server supports.
	writeApp(t, root, "future", map[string]string{
		"index.html": `<html><head><meta name="rela-app:bridge-version" content="999"></head><body>x</body></html>`,
	})
	// Not an app: directory without index.html.
	if err := os.MkdirAll(filepath.Join(root, appsDir, "noindex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, appsDir, "noindex", "other.html"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Not an app: invalid id.
	writeApp(t, root, "Bad-ID-Upper", map[string]string{"index.html": "<html></html>"})
	// Not an app: a loose file (no longer supported).
	if err := os.WriteFile(filepath.Join(root, appsDir, "loose.html"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	apps, err := scanApps(root)
	if err != nil {
		t.Fatalf("scanApps: %v", err)
	}
	got := make(map[string]appInfo, len(apps))
	for _, a := range apps {
		got[a.ID] = a
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 live apps (dash, counter), got %d: %v", len(got), got)
	}
	if _, ok := got["noversion"]; ok {
		t.Errorf("app missing bridge-version must not be listed")
	}
	if _, ok := got["future"]; ok {
		t.Errorf("app with too-new bridge-version must not be listed")
	}
	if got["dash"].Label != "Dashboard" {
		t.Errorf("dash label = %q, want Dashboard", got["dash"].Label)
	}
	if _, ok := got["counter"]; !ok {
		t.Errorf("counter app missing")
	}
	if _, ok := got["noindex"]; ok {
		t.Errorf("dir without index.html must not be an app")
	}
	if _, ok := got["Bad-ID-Upper"]; ok {
		t.Errorf("invalid-id dir must be skipped")
	}
}

func TestScanApps_NoDir(t *testing.T) {
	apps, err := scanApps(t.TempDir())
	if err != nil {
		t.Fatalf("scanApps: %v", err)
	}
	if len(apps) != 0 {
		t.Errorf("expected no apps, got %d", len(apps))
	}
}

func TestOpenAppEntry_Traversal(t *testing.T) {
	root := t.TempDir()
	writeApp(t, root, "dash", map[string]string{
		"index.html":   "<html></html>",
		"sub/asset.js": "ok",
	})
	// A secret outside the app dir we must never reach.
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte("TOPSECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, appsDir, "appssecret.txt"), []byte("SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("valid entry loads", func(t *testing.T) {
		b, err := openAppEntry(root, "dash", "index.html")
		if err != nil || string(b) != "<html></html>" {
			t.Fatalf("got (%q, %v), want the file", b, err)
		}
	})
	t.Run("nested entry loads", func(t *testing.T) {
		b, err := openAppEntry(root, "dash", "sub/asset.js")
		if err != nil || string(b) != "ok" {
			t.Fatalf("got (%q, %v), want nested file", b, err)
		}
	})
	for _, bad := range []string{"../secret.txt", "../appssecret.txt", "../../etc/passwd", "/etc/passwd", "sub/../../secret.txt", ""} {
		t.Run("rejects "+bad, func(t *testing.T) {
			if _, err := openAppEntry(root, "dash", bad); err == nil {
				t.Errorf("openAppEntry(%q) = nil error, want rejection", bad)
			}
		})
	}
	t.Run("missing entry errors", func(t *testing.T) {
		if _, err := openAppEntry(root, "dash", "nope.js"); err == nil {
			t.Error("expected error for missing entry")
		}
	})
}

func TestHandleV1App(t *testing.T) {
	app := newHandlerTestApp(t)

	root := t.TempDir()
	writeApp(t, root, "demo", map[string]string{
		"index.html": `<!doctype html><html><head><title>Demo</title>` +
			`<meta name="rela-app:bridge-version" content="1">` +
			`<script src="_rela.js"></script></head><body>hi</body></html>`,
		"app.js":    `console.log('app')`,
		"style.css": `body{color:red}`,
	})
	app.paths = &project.Context{Root: root, CacheDir: filepath.Join(root, ".rela")}

	t.Run("serves index.html with path-scoped CSP header + nosniff", func(t *testing.T) {
		w := doRequest(t, app, http.MethodGet, "/api/v1/_apps/demo/")
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %.200s)", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "<title>Demo</title>") {
			t.Errorf("index not served verbatim: %.200s", w.Body.String())
		}
		csp := w.Header().Get("Content-Security-Policy")
		if !strings.Contains(csp, "/api/v1/_apps/demo/") || !strings.Contains(csp, "connect-src 'none'") {
			t.Errorf("CSP header missing path-scope / connect-src none: %q", csp)
		}
		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("missing nosniff")
		}
		// The CSP must NOT be a <meta> in the body (header-only now).
		if strings.Contains(w.Body.String(), "http-equiv") {
			t.Errorf("CSP should be a header, not a <meta> in the body")
		}
	})

	t.Run("serves a sibling asset with correct content-type", func(t *testing.T) {
		w := doRequest(t, app, http.MethodGet, "/api/v1/_apps/demo/app.js")
		if w.Code != http.StatusOK || w.Body.String() != `console.log('app')` {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
		if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
			t.Errorf("Content-Type = %q, want javascript", ct)
		}
	})

	t.Run("serves the reserved _rela.js SDK", func(t *testing.T) {
		w := doRequest(t, app, http.MethodGet, "/api/v1/_apps/demo/_rela.js")
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if !strings.Contains(w.Body.String(), "window.rela") {
			t.Errorf("SDK body missing window.rela: %.120s", w.Body.String())
		}
		if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
			t.Errorf("SDK Content-Type = %q", ct)
		}
	})

	t.Run("serves the reserved _rela.css (tokens + base controls)", func(t *testing.T) {
		w := doRequest(t, app, http.MethodGet, "/api/v1/_apps/demo/_rela.css")
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "--text-color") || !strings.Contains(body, ":root.dark") {
			t.Errorf("CSS missing theme tokens / dark block: %.120s", body)
		}
		if !strings.Contains(body, ".btn") || !strings.Contains(body, ".input") || !strings.Contains(body, ".card") {
			t.Errorf("CSS missing base controls (.btn/.input/.card)")
		}
		if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "css") {
			t.Errorf("CSS Content-Type = %q, want text/css", ct)
		}
		// The CSS is served from the app's own path, so the app's own
		// style-src directive already permits it (no extra CSP origin needed).
		csp := w.Header().Get("Content-Security-Policy")
		if !strings.Contains(csp, "style-src "+ /*absolute*/ "http") || !strings.Contains(csp, "/api/v1/_apps/demo/") {
			t.Errorf("style-src must path-scope the app (which covers _rela.css): %q", csp)
		}
	})

	t.Run("serves the reserved _rela-editor.js bundle", func(t *testing.T) {
		withTestEditorAssets(t)
		w := doRequest(t, app, http.MethodGet, "/api/v1/_apps/demo/_rela-editor.js")
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if w.Body.Len() == 0 {
			t.Error("editor bundle is empty")
		}
		// The bundle defines the <rela-editor> custom element.
		if !strings.Contains(w.Body.String(), "rela-editor") {
			t.Errorf("editor bundle missing the custom element tag")
		}
		if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
			t.Errorf("editor Content-Type = %q, want javascript", ct)
		}
		// Served from the app's own path → existing script-src <base> permits it.
		csp := w.Header().Get("Content-Security-Policy")
		if !strings.Contains(csp, "script-src ") || !strings.Contains(csp, "/api/v1/_apps/demo/") {
			t.Errorf("script-src must path-scope the app (covers _rela-editor.js): %q", csp)
		}
	})

	t.Run("serves the reserved _rela-editor.woff2 font", func(t *testing.T) {
		withTestEditorAssets(t)
		w := doRequest(t, app, http.MethodGet, "/api/v1/_apps/demo/_rela-editor.woff2")
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if w.Body.Len() == 0 {
			t.Error("editor font is empty")
		}
		if ct := w.Header().Get("Content-Type"); ct != "font/woff2" {
			t.Errorf("font Content-Type = %q, want font/woff2", ct)
		}
		// The app iframe is sandboxed (opaque/null origin), so the @font-face
		// request is cross-origin and the browser blocks it without CORS. The
		// font MUST send Access-Control-Allow-Origin or the toolbar renders as
		// tofu boxes (verified in-browser, TKT-5F9V56).
		if acao := w.Header().Get("Access-Control-Allow-Origin"); acao != "*" {
			t.Errorf("font Access-Control-Allow-Origin = %q, want * (sandboxed iframe is null-origin)", acao)
		}
		// font-src <base> already permits the same-path font (no CSP widening).
		csp := w.Header().Get("Content-Security-Policy")
		if !strings.Contains(csp, "font-src ") || !strings.Contains(csp, "/api/v1/_apps/demo/") {
			t.Errorf("font-src must path-scope the app (covers _rela-editor.woff2): %q", csp)
		}
	})

	t.Run("editor assets carry an ETag and 304 on If-None-Match", func(t *testing.T) {
		withTestEditorAssets(t)
		for _, entry := range []string{"_rela-editor.js", "_rela-editor.woff2"} {
			w := doRequest(t, app, http.MethodGet, "/api/v1/_apps/demo/"+entry)
			etag := w.Header().Get("ETag")
			if etag == "" {
				t.Fatalf("%s: missing ETag (caching not applied)", entry)
			}
			if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "must-revalidate") {
				t.Errorf("%s: Cache-Control = %q, want must-revalidate", entry, cc)
			}
			// Conditional request with the ETag → 304, empty body (not re-sent).
			req := httptest.NewRequest(http.MethodGet, "/api/v1/_apps/demo/"+entry, nil)
			req.Host = "localhost"
			req.Header.Set("If-None-Match", etag)
			rec := httptest.NewRecorder()
			app.handleV1App(rec, req)
			if rec.Code != http.StatusNotModified {
				t.Errorf("%s: If-None-Match should 304, got %d", entry, rec.Code)
			}
			if rec.Body.Len() != 0 {
				t.Errorf("%s: 304 body must be empty, got %d bytes", entry, rec.Body.Len())
			}
		}
	})

	t.Run("an app cannot shadow _rela-editor.js / .woff2 with its own files", func(t *testing.T) {
		withTestEditorAssets(t)
		writeApp(t, root, "shadowed", map[string]string{
			"index.html":         "<html></html>",
			"_rela-editor.js":    "EVIL",
			"_rela-editor.woff2": "EVIL",
		})
		js := doRequest(t, app, http.MethodGet, "/api/v1/_apps/shadowed/_rela-editor.js")
		if js.Code != http.StatusOK || strings.Contains(js.Body.String(), "EVIL") {
			t.Errorf("reserved _rela-editor.js must serve the real bundle: %.40s", js.Body.String())
		}
		font := doRequest(t, app, http.MethodGet, "/api/v1/_apps/shadowed/_rela-editor.woff2")
		if font.Code != http.StatusOK || font.Body.String() == "EVIL" {
			t.Errorf("reserved _rela-editor.woff2 must serve the real font, not the app file")
		}
	})

	t.Run("bare /_apps/<id> redirects to trailing slash", func(t *testing.T) {
		w := doRequest(t, app, http.MethodGet, "/api/v1/_apps/demo")
		if w.Code != http.StatusMovedPermanently {
			t.Errorf("status = %d, want 301", w.Code)
		}
		if loc := w.Header().Get("Location"); loc != "/api/v1/_apps/demo/" {
			t.Errorf("Location = %q", loc)
		}
	})

	t.Run("malformed id → 400", func(t *testing.T) {
		w := doRequest(t, app, http.MethodGet, "/api/v1/_apps/Bad.Id/")
		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("unknown app → 404", func(t *testing.T) {
		w := doRequest(t, app, http.MethodGet, "/api/v1/_apps/nope/")
		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", w.Code)
		}
	})

	t.Run("an app cannot shadow _rela.js with its own file", func(t *testing.T) {
		writeApp(t, root, "shadow", map[string]string{
			"index.html": "<html></html>",
			"_rela.js":   "EVIL",
		})
		w := doRequest(t, app, http.MethodGet, "/api/v1/_apps/shadow/_rela.js")
		if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "EVIL") {
			t.Errorf("reserved _rela.js must serve the real SDK, not the app file: %.80s", w.Body.String())
		}
	})

	t.Run("non-GET → 405", func(t *testing.T) {
		w := doRequest(t, app, http.MethodPost, "/api/v1/_apps/demo/")
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want 405", w.Code)
		}
	})

	t.Run("missing bridge-version → 422", func(t *testing.T) {
		writeApp(t, root, "noversion", map[string]string{
			"index.html": `<html><head><title>x</title></head><body>hi</body></html>`,
		})
		w := doRequest(t, app, http.MethodGet, "/api/v1/_apps/noversion/")
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("status = %d, want 422", w.Code)
		}
	})

	t.Run("too-new bridge-version → 422", func(t *testing.T) {
		writeApp(t, root, "future", map[string]string{
			"index.html": `<html><head><meta name="rela-app:bridge-version" content="999"></head><body>hi</body></html>`,
		})
		w := doRequest(t, app, http.MethodGet, "/api/v1/_apps/future/")
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("status = %d, want 422", w.Code)
		}
	})
}

// editorBundleBuilt reports whether the editor bundle has been built+embedded.
// Like the SPA's static/v2, the editor dist is a gitignored build artifact, and
// the CI `go test ./...` job runs WITHOUT building the frontend — so tests that
// need the real bytes must skip when it's absent rather than fail. The
// production/release build always runs the frontend build first.
func editorBundleBuilt() bool { return len(appEditorSource()) > 0 }

// withTestEditorAssets swaps the editor bundle/font source vars with fixed test
// bytes for the duration of a test, restoring them after. This lets the serving
// path (content-type, CORS, caching, shadowing) be exercised in CI even when the
// frontend build hasn't run (the `go test ./...` job doesn't build it) — closing
// the coverage hole where the new reserved-entry branches would otherwise only
// run on a developer's machine. The fake JS deliberately contains the
// "rela-editor" marker and the reserved font path so the same assertions hold.
func withTestEditorAssets(t *testing.T) {
	t.Helper()
	const fakeJS = `(function(){customElements.define('rela-editor',class extends HTMLElement{});` +
		`/* @font-face url(` + appEditorFontEntry + `) */})();`
	fakeFont := []byte("wOF2-test-font-bytes")
	origJS, origFont := appEditorSource, appEditorFontSource
	appEditorSource = func() []byte { return []byte(fakeJS) }
	appEditorFontSource = func() []byte { return fakeFont }
	t.Cleanup(func() { appEditorSource, appEditorFontSource = origJS, origFont })
}

// TestAppEditorBundleEmbedded verifies the editor bundle + font, WHEN BUILT,
// define the custom element and reference the same-path font. It skips on a
// clean checkout where the frontend build hasn't run (see editorBundleBuilt).
func TestAppEditorBundleEmbedded(t *testing.T) {
	if !editorBundleBuilt() {
		t.Skip("editor bundle not built (run: cd frontend && npx vite build --config vite.editor.config.ts)")
	}
	js := appEditorSource()
	if !strings.Contains(string(js), "rela-editor") {
		t.Error("editor bundle does not define the rela-editor custom element")
	}
	if !strings.Contains(string(js), appEditorFontEntry) {
		t.Errorf("editor bundle must reference the reserved font path %q (its @font-face)", appEditorFontEntry)
	}
	if len(appEditorFontSource()) == 0 {
		t.Fatal("editor woff2 font not embedded")
	}
}

func TestValidateBridgeVersion(t *testing.T) {
	if reason := validateBridgeVersion(currentBridgeVersion); reason != "" {
		t.Errorf("current version should be valid, got %q", reason)
	}
	if validateBridgeVersion(0) == "" {
		t.Error("missing/zero version must be rejected")
	}
	if validateBridgeVersion(currentBridgeVersion+1) == "" {
		t.Error("too-new version must be rejected")
	}
	// Older-than-current is allowed (forward compatibility) once we're past v1.
	if currentBridgeVersion > 1 && validateBridgeVersion(1) != "" {
		t.Error("older (but supported) version should be allowed")
	}
}
