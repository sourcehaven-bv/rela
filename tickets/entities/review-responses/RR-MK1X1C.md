---
id: RR-MK1X1C
type: review-response
title: Retryable conflicts become 500
finding: errFacesChanged and the owned-entity race wrap store.ErrConflict, but dataentry delete sends them to writeInternalError; MCP and CLI show raw text.
severity: minor
status: open
---
