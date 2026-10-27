---
id: TKT-2SGPJZ
type: ticket
title: 'Dashboard cards: list source, server-side values, matrix and ratio'
kind: enhancement
priority: medium
effort: xl
started: "2026-10-27"
completed: "2026-10-27"
status: done
---

## Description

Dashboard cards today carry config only. The SPA runs each card's `query:`
through `/_search` with no limit and folds counts in the browser. Cards cannot
reuse a list's filters, condition, query scope or world, and there is no matrix
(risk heat map) or ratio (coverage) card.

Scope (approved brief G3, ticket 2):

1. `list: <name>` as a card source. The card inherits the list's entity type,
filters, `condition:`, `query_scope`, ACL and world. Exactly one of `list:` and
`query:`. The card links to the list.
2. A server endpoint computes card values: count, breakdown, table, matrix,
ratio. Rows are row-gated and redacted before any fold. Header reads, batched,
no per-row lookups, pinned by a budget test. Values hidden by `visible:` are
left out and the card is marked `partial`. Nothing is cached across principals.
`permission:` card filtering stays.
3. `display: matrix` with `rows`, `columns` (enum or integer property),
optional `tone_by` (enum) and `colors` (tone or palette colour per `tone_by`
value). Cell colour is the most severe `tone_by` value present (declared enum
order, last = most severe). Empty cells are neutral. Without `tone_by`, cells
shade by count. A cell opens the list filtered on its two values. Rows whose
`tone_by` value is hidden count but do not colour.
4. `display: ratio` with `match:` (a query scope on the list's type, the
numerator) and optional `group_by`. Shows `matched / total (pct)` per group.
5. Frontend tiles, keyboard accessible, click-through.
6. Docs in the data-entry guide.

Out of scope (backlog): trend cards (TKT-40TEOZ), related-entity axes
(TKT-G3H9OB), schema lookups (TKT-B4U6EK), date triggers (TKT-1LO3KU).
