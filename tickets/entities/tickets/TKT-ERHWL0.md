---
id: TKT-ERHWL0
type: ticket
title: Memoize dashboard breakdown and table-row derivation
kind: refactor
priority: medium
effort: xs
status: backlog
---

## What

`getBreakdown` and `getTableRows` in `frontend/src/views/DashboardView.vue` are
plain functions, not `computed`. Each is called **twice** in the template, so
every re-render redoes the work twice over the full result set:

- `getBreakdown(card)` at `:196` (`v-for`) and `:209` (`v-if ... .length === 0`)
- `getTableRows(card)` at `:216` (`v-if ... .length > 0`) and `:225` (`v-for`)

`getBreakdown` is an O(N) group-by; `getTableRows` copies the array and runs an
O(N log N) `localeCompare` sort. At 4000 entities a breakdown card does two full
group-bys per render, and a table card two full sorts — to display at most
`card.limit` rows.

Note the config values directly above them (`title`, `description`, `cards`)
*are* `computed`; only the per-row data functions were left as methods.

## Why this one first

This is the only finding in FEAT-BSPT7O that touches no wire format and
duplicates no in-flight work:

- Pushing property filters into the store is **already built** in the
`rela-next-action` branch (`c5960f9c`, "perf(dataentry): push equality property
filters into the store").
- Bounding result sets overlaps `rela-query-paging` (TKT-JU3S5N, keyset paging
for `GraphQueryer`).
- Server-side aggregation reshapes the card payload, which is exactly what the
in-flight optional-body work also does — so it should land after that settles.

## How

Replace the two functions with a single `computed` map keyed by `cardKey(card)`,
returning the derived breakdown/rows per card. PR #1316 (TKT-53KICM) introduces
`cardKey()` for exactly this identity problem — reuse it rather than adding a
second keying scheme.

The template call sites stay as they are; they just read a memoized value.

## Sequencing

**Branch from `feat/dashboard-card-permissions` (PR #1316), not `develop`.**
#1316 rewrites `DashboardView.vue` (+58/−19), re-keying these getters from array
index to `cardKey`. It leaves them as plain double-called functions, so this
ticket survives that change intact — but rebasing onto develop would conflict in
exactly these lines.

## Acceptance

- `getBreakdown` / `getTableRows` each evaluate at most once per card per render.
- No change to rendered output for count, breakdown, or table cards.
- Existing `DashboardView.test.ts` (added by #1316) stays green.
