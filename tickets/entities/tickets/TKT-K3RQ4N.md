---
id: TKT-K3RQ4N
type: ticket
title: Application-log error lines log the raw request path
kind: enhancement
priority: low
effort: s
status: backlog
description: Follow-up to TKT-F7EROJ (#1782). Error and auth-failure lines in internal/dataentry still log r.URL.Path with entity ids and attachment file names.
---

## Description

TKT-F7EROJ moved the access log (and the attachment and blocked-request
warnings) to the route shape. These application-log lines still log the raw
`r.URL.Path`:

- `api_v1.go` gate and list errors (`writeGateError` and the list-load
branches)
- `router.go` error lines
- `nextaction_handler.go`
- `jwtgate.go` auth failures (also `remote_addr`)
- the `writeInternalError` helper from BUG-Y34ZSZ

They fire on errors only, and the id helps diagnose them, so this is a trade-off
rather than a defect. Decide per site: route shape via `shapedPath`, or keep the
id and document why.
