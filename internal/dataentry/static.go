package dataentry

import "embed"

// staticFiles embeds the vendored JS/CSS dependencies.
// Run "just vendor-js" to download or update these files.
//
//go:embed static/*
var staticFiles embed.FS
