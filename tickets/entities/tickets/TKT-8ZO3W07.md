---
id: TKT-8ZO3W07
type: ticket
title: 'Lua: rela.now timestamp with time of day'
kind: enhancement
priority: medium
effort: s
started: "2026-11-06"
completed: "2026-11-06"
status: done
---

## Description

`rela.today` gives only the date. A script cannot stamp a moment such as a
submission time (OpenVWR gap list, correction 7).

Add `rela.now`: the current time as an RFC 3339 timestamp in UTC, with second
precision. Stamp it once per runtime from the same clock reading as `rela.today`
and `rela.now_unix`, so the three agree. Add a `WithClock` runtime option so
tests can pin the time. Automation templates already have `{{now}}`.

## Acceptance

- `rela.now` is set in every runtime, in UTC, and parses with `rela.date_add`.
- `rela.today`, `rela.now` and `rela.now_unix` come from one clock reading.
- Documented in the Lua guide "Context" table.

## Notes

The clock field took `lua.Runtime` past the archplan field-count ratchet, so the three document-mode fields are now one `documentMode` struct.
