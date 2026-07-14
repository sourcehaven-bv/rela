---
id: DOCS-I0WHP
type: docs-checklist
title: 'Docs: timed calendar-feed events'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc updated for the timed path: `calfeed.Event.Timed`/`.Start`/`.End` (all-day-vs-timed contract, End rendered verbatim/exclusive), `RenderEvent` (VALUE=DATE vs timed UTC instant), `formatDateTimeUTC` (now also DTSTART/DTEND), `declarativeFeed` (all-day or timed per source type), `validateFeeds` (date- or datetime-typed), and the new `isFeedDateType`/`feedKindMismatch` helpers.

## Project Documentation

- [x] ~~CLAUDE.md~~ (N/A: no new package or cross-cutting convention — extends the existing calfeed/feed subsystem.)
- [x] ~~README.md~~ (N/A: generated; no project-level surface change.)

## User-facing Documentation

- [x] `GUIDE-data-entry.md` — the "Calendar feeds" source table now documents that `date:`/`end_date:` may be `date` or `datetime` (all-day vs timed), the same-kind rule, and UTC rendering; the events section reflects all-day vs timed. Rendered to `docs/data-entry.md` via `just docs`.
- [x] `GUIDE-metamodel.md` — the `datetime` "Datetime Properties" note now states that a datetime feed source emits a timed event (replacing the "planned follow-on" wording). Rendered to `docs/metamodel.md`.
