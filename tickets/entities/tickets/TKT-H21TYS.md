---
id: TKT-H21TYS
type: ticket
title: 'Dashboard cards: configurable width'
kind: enhancement
priority: medium
effort: s
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

## Description

Every dashboard card in the data-entry SPA gets the same grid cell. A matrix, a
table with several columns, a ratio card with many groups or a long breakdown
needs more room. Operators should be able to give a card more width in
`data-entry.yaml`.

Scope:

1. A per-card width option on `dashboard.cards[]` and `dashboards.*.cards[]`.
The shape is decided in planning (`span: N` or `width: wide|full`).
2. Width only; whether height (`rowspan`) is in scope is decided in planning.
3. Responsive layout: on narrow viewports every card falls back to one
column. Reordering cards must not leave large gaps; reading and keyboard order
keep following the config order.
4. Load-time validation rejects invalid values with a clear error.
5. The new key is locked in the Configure allowlist (no editor).
6. Docs in the data-entry guide (`docs-project/` sources).
7. New UI strings, if any, use i18n keys in `en` and `nl`.
