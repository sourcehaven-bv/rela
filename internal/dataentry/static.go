package dataentry

import "embed"

// staticFiles embeds the Vue SPA and static assets (favicon).
//
//go:embed all:static/*
var staticFiles embed.FS
