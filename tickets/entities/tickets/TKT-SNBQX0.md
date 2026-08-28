---
id: TKT-SNBQX0
type: ticket
title: 'CalDAV prep: VTODO renderer + completion fields in internal/calfeed'
kind: enhancement
priority: medium
effort: s
status: done
---

## Description

Extend `internal/calfeed` to serialize `VTODO` alongside the existing `VEVENT`,
as the first (and dependency-free) step of CalDAV Phase 2. Design in
**RES-1Y2EB5**.

`RenderEvent` (ical.go:66) hardcodes `BEGIN:VEVENT`, so this is a sibling
renderer plus model fields — not a rewrite. `ETag` / `CollectionTag` already
exist and must keep working unchanged for both component types.

### Deliverables

1. **Model fields on `calfeed.Event`** (or a sibling `Todo` type — decide during
planning; a shared type keeps `ETag`/`CollectionTag` generic, a separate type
keeps VEVENT honest):
   - `Status` — `NEEDS-ACTION` / `COMPLETED` / `CANCELLED`
   - `Completed time.Time` — the `COMPLETED` UTC timestamp
   - `PercentComplete int`
   - `Due time.Time` — VTODO uses `DUE`, not `DTSTART`
   - `Priority int` (optional; verified to survive an Apple round-trip)
2. **`RenderTodo`** emitting a `VTODO` block, reusing the existing
`writeProp` / `foldLine` / `escapeText` / CRLF machinery verbatim.
3. **`RenderCollection` accepts VTODO**, emitting
`supported-calendar-component-set`-consistent output. A collection is
VEVENT-only or VTODO-only — never mixed (see AC4).
4. **`ETag` / `CollectionTag` cover the new fields** — the tags must change when
completion state changes, and must remain DTSTAMP-independent (the existing
zero-clock trick).

### Ground truth (verified against Apple Reminders, 2026-08-09)

Captured `.ics` files from the live test are in `.ignored/` — use them as test
fixtures. Key facts:

- **Completion is three properties written together**: `STATUS:COMPLETED`,
`COMPLETED:<UTC>`, `PERCENT-COMPLETE:100`. Emit all three consistently; RFC 4791
§7.8.9's canonical "pending todos" filter keys on **`COMPLETED` being absent**,
so a completed todo must carry it.
- Reminders **adds `DTSTART;VALUE=DATE` mirroring `DUE`** unprompted, plus
`X-APPLE-SORT-ORDER`, `CREATED`, `LAST-MODIFIED`.
- It **rewrites the whole VCALENDAR** with its own `PRODID` and re-sorts
properties — so nothing downstream may diff on raw bytes.

### Acceptance criteria

1. `RenderTodo` emits a spec-valid `VTODO` (CRLF, 75-octet folding, escaping) —
table-driven tests mirroring the existing `ical_test.go` style.
2. An all-day `DUE` emits `DUE;VALUE=DATE`; a datetime due emits a UTC instant,
matching the existing `Timed` branch semantics.
3. A completed todo emits all three completion properties; a pending one emits
`STATUS:NEEDS-ACTION` and **no** `COMPLETED`.
4. `RenderCollection` rejects (or is structurally incapable of) mixing VEVENT and
VTODO in one collection — Apple binds Reminders to a VTODO-only collection and
Calendar.app to a separate VEVENT one (confirmed twice in the live test).
5. `ETag` changes when completion state changes and is unchanged across
re-renders of identical content.
6. Round-trip fixture test: the captured Apple `.ics` files in `.ignored/` parse
to the model and re-render equivalently **at the semantic level** (not
byte-equal — Apple re-sorts and re-stamps).
7. No new dependency; `calfeed` stays a leaf package importing nothing new.

### Out of scope

- The CalDAV protocol surface (separate ticket — this is pure serialization).
- The declarative mapping config.
- `emersion/go-webdav` — this ticket adds no vendor. (Note the eventual adapter
will convert between `calfeed` types and `*ical.Calendar`; keeping `calfeed`
vendor-free is what makes that boundary clean.)
