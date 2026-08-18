---
id: TKT-AIEGHU
type: ticket
title: Server-side aggregation for count and breakdown dashboard cards
kind: enhancement
priority: high
effort: m
status: backlog
---

## What

A `display: count` card renders one integer. A `display: breakdown` card renders
a group-by tally. Both currently download **every matching entity in full** and
compute the aggregate in the browser.

`handleV1Search` (`internal/dataentry/api_v1.go:1352`) serializes every match
with full properties and body, then sets `Meta.Total = len(data)` — the count is
derived *from* the full payload rather than instead of it. `DashboardView.vue`
calls `searchEntities(card.query)` per card and reduces client-side in
`getCardCount` / `getBreakdown`.

## Measured

Through `handleV1Search`, 7 cards (the dogfooded `tickets/data-entry.yaml` set,
all count/breakdown):

| entities | 7 cards, wall | JSON transferred |
|---|---|---|
| 500 | 3.5 ms | 256 KB |
| 1 000 | 5.6 ms | 513 KB |
| 2 000 | 11.1 ms | 1.0 MB |
| 4 000 | 28.4 ms | 2.1 MB |
| 8 000 | 53.8 ms | **4.1 MB** |

Linear, but with a brutal constant: one count card ships 379 KB at 4000 entities
to display a single number.

## How

Return an aggregate instead of rows when the card's display mode only needs one:
`{total}` for `count`, `{value: count}` map for `breakdown`. The card config
already carries `display` and `group_by`, so the server has everything it needs
to decide.

`store.GraphCount` already returns `(matched, total)` — prefer it over
materializing and counting. Mind RR-SSPCCI ("GraphCount total is a count oracle
over hidden rows"): the aggregate must be computed over the **ACL-scoped** set,
same as the current search path, or it becomes a disclosure channel for rows the
principal cannot read.

## Sequencing — check before starting

This reshapes the card payload, which is **also** what the in-flight
optional-body work does (making entity content optional on fetch). Land after
that, or coordinate — otherwise the two collide on the same wire shape.

Also depends on PR #1316 (TKT-53KICM), which adds `GET /api/v1/_dashboard` and
rewrites `DashboardView.vue`; that endpoint is the natural home for a card-aware
aggregate response.

Re-verify both before picking this up.

## Acceptance

- A `count` card transfers O(1) bytes regardless of graph size.
- A `breakdown` card transfers O(distinct group values), not O(matching entities).
- Aggregates respect ACL scoping — no count over rows the principal cannot read.
- `table` cards are out of scope here (see the limit/sort pushdown ticket).
