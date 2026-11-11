//go:build !headless

package dataentry

import "embed"

// Headless reports whether this binary was built with the `headless` tag, which
// leaves out the web UI: the Vue SPA and the custom-app editor bundle. The REST
// API, SSE events and remote MCP are served either way.
const Headless = false

// staticFiles embeds the Vue SPA and static assets (favicon).
//
//go:embed all:static/*
var staticFiles embed.FS

// appEditorDist embeds the <rela-editor> bundle; see appEditorAsset.
//
//go:embed all:app_editor_dist
var appEditorDist embed.FS
