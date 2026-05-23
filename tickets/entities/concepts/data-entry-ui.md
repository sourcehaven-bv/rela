---
id: data-entry-ui
type: concept
title: Data Entry Web UI
summary: Vue 3 SPA for entity/relation management, served by a Go backend
description: |
  Config-driven web application for data entry operations. Built with:
  - Vue 3 + Pinia SPA in `frontend/` (Vite build, embedded into the Go binary
    at `internal/dataentry/static/v2/` via `go:embed`); component tree under
    `frontend/src/components/{ui,forms,lists,entity,common}`
  - Go HTTP API in `internal/dataentry`: REST under `/api/v1/` plus a small
    set of legacy `/api/*` endpoints (help, command/cancel, open-file, git
    status/sync) still consumed directly by the SPA, plus two SSE streams
    (`/api/events`, `/api/v1/_events`) for live reload
  - Client-side routing with Vue Router; the Go server falls through to
    `index.html` for unknown paths so deep links work
layer: server
package: internal/dataentry
status: stable
---
