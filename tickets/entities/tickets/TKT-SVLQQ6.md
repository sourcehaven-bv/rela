---
id: TKT-SVLQQ6
type: ticket
title: Kanban column colour, column summaries and currency display
kind: enhancement
priority: medium
effort: m
started: "2026-10-14"
completed: "2026-10-15"
status: done
---

## Description

A kanban board cannot show money the way a sales pipeline needs. Sourcehaven's
atlas CRM board (enum `columns:` on `stage`) should show a coloured dot in each
column header, lines such as "Deals: 2" and "Totaal: € 11.500" under it, and
card values as "€ 3.500".

Three gaps:

1. An enum-backed column gets no colour. Only `columns_from` columns take one
(from `style_from`). The `styles:` entry for the enum value is ignored.
2. A column cannot summarise its cards, such as a count or the sum of a
numeric property.
3. An integer property cannot render as a currency amount.

## Redesign (2026-10-15)

The first version (PR #1891, before merge) added a sum-only kanban key
`column_total: {property, label, currency}` and a per-field `currency:` on
kanban card fields. The user asked for a redesign before merge:

- Aggregates should be generic, not sum-only, and use one spec that list
footers, group_by headers and dashboard cards can adopt later.
- A currency is a fact about the property, not about one card field. Putting
it on the property formats the value the same way on every surface.

So `column_total` became `column_summary`, a list of `{function, property,
label}`, and the card field `currency:` became the schema option `display:
{currency: EUR}` on the property. The ticket went back to in-progress for the
rework; RR-ILJ049 records it.

## Scope

In scope:

- Enum column header colour from the `styles:` entry of the column value.
- New kanban key `column_summary:`, a list of `{function, property, label}`
with function count, sum, avg, min or max, computed client side over the
column's cards.
- A shared summary type (Go `dataentryconfig.Summary`, TS `Summary`).
- New schema property option `display: {currency: <ISO code>}`, applied on
kanban cards and summaries, list cells, view rows and entity detail.
- Config validation, docs, Go tests, Vitest and e2e tests.

Out of scope: summaries in list footers, group_by headers and dashboard cards
(the type is shaped for them, they are not wired), swimlane summaries,
server-side aggregation, display formats other than currency.

## Approach

- Go: `Kanban.ColumnSummary []Summary`; `validateSummaries` checks each
entry. count takes no property (refused, so a later meaning stays free); sum and
avg need a single-valued integer; min and max also accept date and datetime.
`column_summary` with `swimlane_property` is refused.
- Metamodel: `PropertyDef.Display *PropertyDisplay{Currency}` under the key
`display:` (`format:` already holds the date layout). Load checks the code shape
(three upper-case letters) and that the property is a single-valued integer.
Served on the schema API as `display`.
- rela-components: `Section.summary?: string[]`, one line each under the
column heading, joined with " · " on the collapsed rail.
- SPA: `formatSummary` (utils/summary.ts) computes a line; empty sets show 0
for count and sum and "–" for avg, min and max; a property in `_redacted` marks
the line "(partial)". `formatDisplayNumber` applies the currency in
`formatCellValue`, the number widget's display mode and view rows.

## Acceptance criteria

1. An enum column whose value has a `styles:` entry shows that colour in its
header; a value without a style shows none.
2. `column_summary` shows one "<label>: <value>" line per entry under every
column header, and on one line on a collapsed column. An empty column shows 0
for count and sum and "–" for avg, min and max.
3. A property with `display: {currency: EUR}` renders 3500 as "€ 3.500" in the
nl locale on cards, in summaries and on the entity page; inputs stay plain.
4. Config load rejects: an unknown function, a property on count, a missing
property on the other functions, a property of the wrong type or a list
property, `column_summary` with swimlanes, a malformed currency code, and
`display.currency` on a non-integer or list property.
