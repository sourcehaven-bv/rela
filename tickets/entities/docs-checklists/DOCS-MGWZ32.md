---
id: DOCS-MGWZ32
type: docs-checklist
title: 'Docs: Calendar views (month + week) for data-entry'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on new exported types and functions
- [x] Non-obvious decisions explained at the point they would be questioned

`Calendar`, `CalendarSource` and `CalendarEvent` carry the reasoning for the
choices a reader would otherwise re-litigate: why the structs are independent of
`Feed` (so either can change incompatibly without dragging the other), why
`CalendarEvent` has no `Title` (the source's `summary` already names it), and
why chip fields resolve per-source best-effort rather than strictly.

`calendarGrid.ts` documents the rule the whole feature rests on — an all-day
value is a calendar date and never passes through a timezone; a timed value's
day is a function of the display timezone.

## Project Documentation

- [x] `docs/data-entry.md` — new "Calendars" section
- [x] N/A `docs/metamodel.md` — no metamodel change
- [x] N/A `docs/cli-reference.md` — no CLI change
- [x] N/A `CLAUDE.md` — no new cross-cutting pattern

The docs section covers the config shape, both field tables, event chips, drag
semantics, timezone behaviour, the feed/calendar anchor-sharing pattern, the
validation rules, and an explicit "Limits" list (no day/year view, no recurrence
expansion, no resize, no sidebar count).

**Verified, not just written:** the documented example was run through `rela
validate` against a real project. That caught a genuine defect — the anchor
pattern the docs describe did not work, because strict top-level key checking
rejected the anchor holder. Fixed by allowing underscore-prefixed keys that do
not shadow a real section name.

## External Documentation

- [x] N/A — no README or release-note change; the feature is documented in the
data-entry guide alongside the other view types.
