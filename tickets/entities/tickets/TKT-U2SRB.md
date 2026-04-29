---
id: TKT-U2SRB
type: ticket
title: Format dates with short month name in data-entry
kind: enhancement
priority: medium
effort: s
status: done
---

## Problem

Date values in the data-entry SPA render via `Date.toLocaleDateString()` with no
explicit options. The output is locale-dependent and uses purely numeric formats
(e.g. `1/15/2024` in en-US, `15/1/2024` in en-GB), which is ambiguous between US
and EU readers.

## Goal

Render dates with a **short abbreviated month name** (e.g. `15 Jan 2024`) so
day/month order is unambiguous while staying compact enough for table cells. Use
the month abbreviation, not the full month name — list cells must stay narrow.

## Scope

- `frontend/src/utils/format.ts` — `formatValue` (date branch) and
`formatCellValue` (date property branch) both call `date.toLocaleDateString()`.
Replace both with a single shared formatter that uses `{ year: 'numeric', month:
'short', day: 'numeric' }`.
- Update `frontend/src/utils/format.test.ts` to assert the new format.

## Out of scope

- Changing the date input widget format (still ISO `YYYY-MM-DD`).
- Localising the abbreviated month name to the user's preferred language
beyond what the browser's locale already provides.
- Touching `RruleBuilder.vue` or `DynamicForm.vue` validation paths — those
parse dates, they don't display them.
