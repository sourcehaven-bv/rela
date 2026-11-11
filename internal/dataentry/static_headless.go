//go:build headless

package dataentry

import "embed"

// Headless reports whether this binary was built with the `headless` tag. See
// the non-headless declaration in static.go.
const Headless = true

// staticFiles and appEditorDist are empty in a headless build: no SPA, no
// static assets and no custom-app editor bundle are compiled in.
var (
	staticFiles   embed.FS
	appEditorDist embed.FS
)
