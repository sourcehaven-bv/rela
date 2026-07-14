---
id: TKT-VG3NUA
type: ticket
title: Timed calendar-feed events from datetime sources (DTSTART with time, datetime start/end range)
kind: enhancement
priority: medium
effort: m
status: review
---

## Description

Now that the **`datetime`** property type exists (TKT-ZYOBSN), make the
calendar-feed export emit **timed** events for datetime-typed sources — the
follow-on that Phase 1 (TKT-RDM9M5) explicitly deferred because there was no
clean way to author a time-bearing value.

Phase 1 ships **all-day only** (`DTSTART;VALUE=DATE`). This ticket makes the
feed **type-driven**:

- `date` source → all-day event (`VALUE=DATE`) — unchanged.
- `datetime` source → **timed** event (`DTSTART:<UTC>` with time-of-day).
- `datetime` **start + optional `datetime` end** → a timed-range VEVENT
(`DTSTART`/`DTEND` both timed).

### Deliverables

1. **`internal/dataentryconfig/validate_feeds.go`** — allow a feed `date:` /
`end_date:` source property to be **`date` OR `datetime`** (currently gated to
`date` only at :89 and :127). Reject a nonsensical mix where it matters (decide
during planning — e.g. all-day start + timed end).
2. **`internal/calfeed`** — extend the `Event` model + iCal serializer to emit a
**timed** `DTSTART`/`DTEND` (`YYYYMMDDTHHMMSSZ`, UTC) when the event is timed,
keeping `VALUE=DATE` for all-day. A per-event `AllDay bool` (or equivalent)
drives the branch. `formatDateTimeUTC` already exists for `DTSTAMP`.
3. **`internal/dataentry/feed_provider.go`** — when a source's date prop is
`datetime`, produce a timed `Event` (carry the parsed `time.Time` with its
time-of-day) instead of an all-day one; likewise for `end_date`.
4. Tests: calfeed timed-event rendering (incl. iCal correctness — CRLF, DTSTART
line format), validate_feeds accepting datetime, feed_provider mapping a
datetime source to a timed event, and the mixed date/datetime decision.
5. Docs: update the feed docs (`docs/data-entry.md` + the metamodel datetime
note that currently says "timed events are a planned follow-on") and the feed
config reference.

### Design notes / prior art

- iCal correctness pins from Phase 1's review (RR-0E20T7, RR-2V0019) apply —
reuse the CRLF/fold/DTSTAMP handling; timed `DTSTART` must be UTC `Z`.
- The datetime type stores UTC RFC3339, so `ParseDateValue` already yields a
full `time.Time`; the provider just stops truncating to a day.
- Type-driven, no feed-config redesign — the Phase 1 design anticipated this
("picked up type-driven once `datetime` lands").

### Out of scope

- Timezone (`TZID`/`VTIMEZONE`) in the feed — Phase 1 + this ticket emit UTC
`Z`. A per-feed display tz is a later enhancement.
- CalDAV, RRULE, VTODO — later phases (unchanged from Phase 1 scope).
