---
id: DOCS-VRL26
type: docs-checklist
title: 'Docs: datetime property type + widget'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc / comments on the new `datetime` path: `PropertyTypeDatetime`, `DefaultDatetimeFormat`, the validation arm (accepts string + time.Time), the filter/sort arms (shared date rank, strict-instant `=`), and the frontend tz helpers (`browserTimeZone`, `utcISOToLocalInput`, `localInputToUtcISO`, `formatDatetime`) + `DatetimeWidget.vue` (non-destructive, tz indicator).

## Project Documentation

- [x] ~~CLAUDE.md~~ (N/A: no new cross-cutting convention.)
- [x] ~~README.md~~ (N/A: generated; no project-level surface change.)

## User-facing Documentation

- [x] `GUIDE-metamodel.md` — new `datetime` builtin in the Property Types table + reserved names, a "Datetime Properties" section (UTC RFC3339 storage, bare-date = midnight-UTC, strict-instant `=`, chronological sort), and the operator/sort tables. Rendered to `docs/metamodel.md`.
- [x] `GUIDE-data-entry.md` — the datetime widget in the Widget Types table + a "Datetime fields and time zones" section (UTC storage, Settings display-timezone picker, non-destructive editing, the midnight-UTC display quirk). Rendered to `docs/data-entry.md`.
