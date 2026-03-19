package dataentry

import "embed"

// staticFiles embeds the vendored JS/CSS dependencies.
// Run "just vendor-js" to download or update these files.
//
//go:embed all:static/*
var staticFiles embed.FS

// templateFiles embeds all HTML templates.
//
//go:embed templates/*.html
var templateFiles embed.FS
