---
id: TKT-X1ZFO04
type: ticket
title: Audit read API and Changes tab in data-entry
kind: enhancement
priority: high
effort: l
tags: security
started: "2026-11-18"
completed: "2026-11-18"
status: done
---

## Description

PR 2 of FEAT-0BH46JQ. Depends on TKT-USQBMG6.

- Per entity: a Changes tab, gated like history reads; `visible:`-hidden values redacted through `internal/visibility`.
- Global: an audit view behind `audit:read`, covering deleted entities and `denied-write` records.
- Paged in the store, with a budget test.
- Offered only when the configured audit implementation can read.
