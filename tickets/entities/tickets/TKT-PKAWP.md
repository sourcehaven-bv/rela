---
id: TKT-PKAWP
type: ticket
title: Fix rrule property display in lists to match detail page
kind: enhancement
priority: medium
effort: xs
status: done
---

RRULE property values render as raw iCal strings (e.g. `FREQ=DAILY;INTERVAL=2`)
in EntityList table cells, while the entity detail page formats them as
human-readable text (e.g. `every 2 days`) via PropertyDisplay → formatValue.

Root cause: `formatCellValue` in `frontend/src/utils/format.ts` (used by
EntityList) handles `date` and `boolean` types but has no `rrule` branch;
`formatValue` (used by PropertyDisplay) does. The two paths need parity.

## Acceptance criteria

- An rrule-typed property in a list column renders the same human-readable text as on the entity detail page (uses `RRule.fromString(...).toText()`).
- Empty / null rrule values render as the existing list empty-cell style (no regression).
- Malformed rrule strings fall back to the raw string (matches `formatValue` behaviour).
- Unit tests in `frontend/src/utils/format.test.ts` cover `formatCellValue` with `rrule` type, including the `RRULE:` prefix, `DTSTART:... RRULE:...`, and a malformed value.
