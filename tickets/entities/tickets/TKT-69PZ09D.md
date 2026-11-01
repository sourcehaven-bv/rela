---
id: TKT-69PZ09D
type: ticket
title: Dashboard cards render values like the rest of the app
kind: enhancement
priority: medium
effort: m
started: "2026-11-01"
completed: "2026-11-01"
status: done
---

## Description

Dashboard cards do not render values the way the rest of the data-entry app
does. Observed on Sourcehaven's atlas CRM dashboard (`dashboards: crm`), which
uses `query:` cards:

1. A `display: breakdown` card with `group_by:` on an enum shows raw enum keys
(`eerste_contact`, `ziekenhuis_umc`) instead of the authored labels, with no
`styles:` colour, ordered by count rather than by the declared enum order.
2. A `display: table` card shows raw enum keys and plain numbers. List cells
show enum badges with labels and styles, integer properties with `display:
{currency}` as amounts (TKT-SVLQQ6) and formatted dates.
3. A table card is as narrow as a count card.
4. The dashboard docs example writes `sort:` as a map, but the loader needs a
list (`sort: [{property, direction}]`).

## Scope

In scope:

- Breakdown cards (query and list source): enum values show as a badge with
the authored label and style colour; the bar takes the style colour. Groups
follow the declared enum order, then values outside the enum by count, with
unset values last. This is the order the server already uses for list cards
(`groupCounts`), so query and list cards agree.
- Table cards (query and list source): cells render through the dense cell
widgets the list view uses (`densePropertyRoutingHint`,
`defaultRegistry.resolveDense`, `formatCellValue`), so enums show badges,
currency integers show amounts and dates are formatted. The detail link and the
hidden-value marker stay as they are.
- Docs: fix the map-form `sort:` examples in GUIDE-data-entry (dashboard card,
list example, full example); document breakdown order and table cell rendering.

Out of scope / already done:

- Card width: done in TKT-H21TYS (`width: normal|wide|full`, PR #2007). No new
key.
- Loader accepting the single-map `sort:` form: no other `sort:` accepts it;
`rela migrate` (`sort-config-to-list`) converts it everywhere. Not added.
- Relation columns in dashboard tables (unchanged).
