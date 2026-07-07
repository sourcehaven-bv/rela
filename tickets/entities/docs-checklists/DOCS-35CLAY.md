---
id: DOCS-35CLAY
type: docs-checklist
title: 'Docs: calendar/feed export'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on the new `internal/calfeed` package + exported types/functions
- [x] Godoc on the synthesizer, provider interface, and handler (design decisions
captured: syntax-disambiguated rrule, `--` UID separator rationale, cursor
tombstone caveat, CRLF-strip invariant)

## Project Documentation

- [x] `CLAUDE.md` — added `internal/calfeed` to the package table.
- [x] ~~README.md~~ (N/A: generated; no project-level surface change beyond the guide.)

## User-facing Documentation

- [x] `GUIDE-data-entry.md` — new "Calendar feeds" section: the `feeds:` schema
(meta + multi-source), all source fields, filters, recurrence (`rrule` syntax
rules + the overdue-visibility recipe), the event model (UID, deep links, JSON),
and serving/ACL/access. Rendered to `docs/data-entry.md` via `just docs`.
