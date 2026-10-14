---
id: TKT-SVLQQ6
type: ticket
title: Kanban column colour and column totals
kind: enhancement
priority: medium
effort: m
started: "2026-10-14"
completed: "2026-10-14"
status: done
---

## Description

A kanban board cannot show money the way a sales pipeline needs. Sourcehaven's
atlas CRM board (enum `columns:` on `stage`) should show a coloured dot in each
column header, a line "Totaal: € 11.500" under it, and card values as "€ 3.500".

Three gaps:

1. An enum-backed column gets no colour. Only `columns_from` columns take one
(from `style_from`). The `styles:` entry for the enum value is ignored.
2. A column cannot show the sum of a numeric property over its cards.
3. A card field cannot render an integer as a currency amount.

## Scope

In scope:

- Enum column header colour from the `styles:` entry of the column value.
- New kanban key `column_total: {property, label, currency}`, summed client
side over the column's cards.
- New card field option `currency: <ISO code>`.
- Config validation, docs, Vitest and e2e tests.

Out of scope: totals in list views, swimlane totals, server-side aggregation,
currency on calendar chips and gantt tooltips.

## Approach

- Go: `Kanban.ColumnTotal *KanbanColumnTotal` and `KanbanCardField.Currency`.
Validation in `validateKanbans`: the property must be an `integer` property of
the board's entity type; the currency must be three upper-case letters (ISO 4217
shape); `column_total` on a board with `swimlane_property` is refused.
`currency` on a calendar chip or gantt tooltip field is refused.
- rela-components: `Section.summary?: string`, drawn under the column heading
and on the collapsed rail.
- KanbanView: enum columns take `styleStatusColor(stylesForProperty(...))`;
the column total is summed over the column's filtered cards and formatted with
`Intl.NumberFormat` (currency style, no decimals for whole numbers).

## Acceptance criteria

1. An enum column whose value has a `styles:` entry shows that colour in its
header; a value without a style shows none.
2. `column_total` shows "<label>: <amount>" under every column header,
including on a collapsed column and on an empty column (0).
3. A card field with `currency: EUR` renders 3500 as "€ 3.500" in the nl
locale (app locale conventions).
4. Config load rejects: unknown or non-integer `column_total.property`,
a malformed currency code, `column_total` with swimlanes, `currency` on a
non-integer card field or on a relation/comments field, and `currency` on
calendar and gantt fields.
