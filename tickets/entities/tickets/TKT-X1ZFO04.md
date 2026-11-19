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

## Follow-up

The CISO review of #2449 found that the ACL guide did not list one residual: a faced entity that reuses the id of an entity deleted before the face migration can show that entity's records. Issue #2466 tracks it; #2468 adds it to the guide.
