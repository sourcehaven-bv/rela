package dataentry

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"net/http"
)

// The <rela-editor> Custom Element bundle and its glyph webfont, built by the
// standalone editor build (frontend/vite.editor.config.ts) into
// frontend → ../internal/dataentry/app_editor_dist and embedded here. Served at
// the reserved per-app paths _rela-editor.js / _rela-editor.woff2 (see apps.go
// and apps_handler.go). TKT-5F9V56.
//
// Like the SPA bundle (static/v2), these are BUILD ARTIFACTS — gitignored and
// produced by `npm run build` (frontend/package.json runs the editor build too).
// The embed uses a glob with a committed .gitkeep placeholder so the package
// still COMPILES on a clean checkout where the build hasn't run (the same reason
// static.go embeds `static/*`, matched by a committed favicon, rather than a
// specific file). On such a checkout appEditorSource() returns nil and
// TestAppEditorBundleEmbedded fails loudly — the production build always runs
// the frontend build first, so the real bytes are present in shipped binaries.
//
// Rebuild with:
//
//	cd frontend && npx vite build --config vite.editor.config.ts

//go:embed all:app_editor_dist
var appEditorDist embed.FS

func appEditorAsset(name string) []byte {
	b, err := fs.ReadFile(appEditorDist, "app_editor_dist/"+name)
	if err != nil {
		return nil // not built yet (clean checkout); guarded by a test
	}
	return b
}

// appEditorSource / appEditorFontSource are package-level VARS (not funcs) so
// tests can inject a fake bundle and exercise the serving path even when the
// frontend build hasn't run (the CI `go test ./...` job doesn't build the
// frontend — see withTestEditorAssets in apps_test.go). Production code never
// reassigns them.
var (
	// appEditorSource returns the <rela-editor> IIFE bundle served at
	// /api/v1/_apps/<id>/_rela-editor.js.
	appEditorSource = func() []byte { return appEditorAsset("rela-editor.js") }
	// appEditorFontSource returns the toolbar glyph webfont served at
	// /api/v1/_apps/<id>/_rela-editor.woff2.
	appEditorFontSource = func() []byte { return appEditorAsset("rela-editor.woff2") }
)

// ETags for the editor assets, computed lazily from the current bytes so a
// test that swaps the source vars gets a matching ETag. The assets are
// immutable for the process lifetime (embedded at build time), so a content
// hash is a stable strong validator: a new build → new bytes → new ETag, which
// can't serve a stale asset across deploys (unlike `immutable` max-age on an
// unversioned URL). Empty when the asset isn't built.
func appEditorJSETag() string   { return weakETagFor(appEditorSource()) }
func appEditorFontETag() string { return weakETagFor(appEditorFontSource()) }

func weakETagFor(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	sum := sha256.Sum256(b)
	return `"` + hex.EncodeToString(sum[:16]) + `"`
}

// serveCachedAsset writes body with an ETag + must-revalidate caching so an app
// iframe (which reloads on navigation / host remounts) revalidates with a cheap
// 304 instead of re-transferring the full bundle every time, while still picking
// up a new build immediately (the ETag changes with the bytes). Returns true if
// it handled a 304 (caller should stop).
func serveCachedAsset(w http.ResponseWriter, r *http.Request, contentType, etag string, body []byte) {
	h := w.Header()
	h.Set("Content-Type", contentType)
	if etag != "" {
		h.Set("ETag", etag)
		h.Set("Cache-Control", "public, max-age=0, must-revalidate")
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	_, _ = w.Write(body)
}
