---
id: RR-Y0KEU
type: review-response
title: data-entry-ui description omits non-v1 /api/* surface and second SSE endpoint
finding: The rewritten description claims the API is 'REST under /api/v1/, plus SSE for live reload at /api/events'. router.go:57-63 shows seven non-v1 endpoints actively consumed by the SPA (/api/toggle-checkbox, /api/help/, /api/command/, /api/command-cancel/, /api/open-file, /api/git/status, /api/git/sync), and there are TWO SSE endpoints (/api/events and /api/v1/_events). Description should either acknowledge the legacy /api/* endpoints or be reworded to describe the route surface accurately.
severity: minor
resolution: Updated tickets/entities/concepts/data-entry-ui.md to acknowledge the legacy /api/* endpoints (toggle-checkbox, help, command/cancel, open-file, git status/sync) and the second SSE stream (/api/v1/_events).
status: addressed
---
