package dataentry

import (
	"embed"
	"io/fs"
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

// appEditorSource returns the <rela-editor> Custom Element IIFE bundle served at
// /api/v1/_apps/<id>/_rela-editor.js.
func appEditorSource() []byte { return appEditorAsset("rela-editor.js") }

// appEditorFontSource returns the toolbar glyph webfont served at
// /api/v1/_apps/<id>/_rela-editor.woff2.
func appEditorFontSource() []byte { return appEditorAsset("rela-editor.woff2") }
