---
id: BUG-UI5TT6
type: bug
title: View table cells render every typed value as a badge
description: 'display: table cells wrap every typed value in Badge, so titles and dates show as gray capitalized pills.'
priority: medium
effort: s
why1: EntityDetail's table template rendered a cell as Badge whenever shouldUseBadge(value, propType) was true, and that returned true for any cell with a propType and a value.
why2: 'The server sets propType on every declared property cell (fillPropertyCell), not only on enums, so titles (string) and dates (date) took the Badge path. Badge falls back to gray and applies text-transform: capitalize.'
why3: The table section predates the widget registry. When list cells, kanban cards and nested sections moved to widget routing (densePropertyRoutingHint, nestedCellsFor), the table section kept its own per-type Badge branch.
why4: No test mounted a table section with a non-enum typed column, so the badge on a title was never asserted against.
why5: 'Systemic: each dense surface resolves cell rendering on its own, so migrating one surface to the widget registry leaves the others on stale heuristics with nothing to flag them.'
prevention: Render table cells through the server-resolved widget like nestedCellsFor and pin it with EntityDetail.table.test.ts. The remaining per-surface heuristic is viewFieldRoutingHint for cards/list fields, which still badges any typed field.
started: "2026-09-30"
completed: "2026-09-30"
status: done
---

## Report

A view section with `display: table` shows every typed cell as a gray,
capitalized pill. On an atlas project page the "Taken" table shows each task
title as a pill ("Bewijs Van Competentie …") and each due date as a pill.

## Expected

A title renders as text (a link when the column has `link: detail`), a date as a
formatted date, and only an enum as a badge. An empty cell stays blank.
