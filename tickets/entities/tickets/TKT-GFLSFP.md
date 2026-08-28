---
id: TKT-GFLSFP
type: ticket
title: 'CalDAV: collection colour is configured but never served'
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Symptom

`meta.color` is accepted in the `caldav:` config and never reaches the wire.
Apple Reminders then tries to set its own colour and gets a hard `501`.

**Severity corrected (2026-08-12).** This was escalated on the claim that the
501 *blocked* Reminders' sync — an 8-request discovery cycle with zero REPORTs
was read as the client abandoning the collection. That diagnosis was wrong on
both counts:

- The absent REPORTs were the **ctag working correctly**: an unchanged
  collection tag tells the client to skip enumeration. Editing an entity moved
  the tag and Reminders synced immediately.
- The 501 itself did not come from rela or from Pratique, but from a **local
  Python debug tap** missing a `do_PROPPATCH` handler; CPython's
  `BaseHTTPRequestHandler` answers 501 for any unregistered method.

So this is a genuine bug of ordinary severity: `meta.color` is accepted in the
`caldav:` config, validated, and then never serialized onto the wire. Nothing
is blocked; the collection simply renders in the client's default colour.

## Cause

1. **rela / go-webdav** — `backend.PropPatch` returned a flat
`501 PropPatch not implemented` (caldav/server.go:664). **FIXED** on this
branch: `withPropPatch` answers a RFC 4918 §9.2 `207 Multi-Status` with a
per-property `403 Forbidden`, which tells the client "understood, refused"
instead of "server broken". 403 rather than 200 deliberately — claiming success
for a value we discard makes the client show a colour that reverts on the next
poll.
2. ~~**Pratique** — answers 501 for `PROPPATCH`, `MKCALENDAR` and `MOVE`~~
**RETRACTED.** The 501 came from a local debug tap standing in front of rela,
not from Pratique, which has no method allow-list and forwards WebDAV verbs
unconditionally. The issue file raised against Pratique was withdrawn.

**Remaining work:** serve `calendar-color` on the collection PROPFIND so a
configured `meta.color` actually reaches the client. go-webdav's
`propFindCalendar` builds a closed property map, so this needs the same wrapper
seam `withCTag` uses.

## Still outstanding: the colour itself

Separately from the 501, `meta.color` is validated at config load and then
dropped: `caldavBackend.calendarFor` maps `Meta.Name` and `Meta.Description`
onto `caldav.Calendar`, which has no colour field, so `calendar-color` is never
emitted.

Options:

- **Serve the configured colour** (read-only): splice `calendar-color` into the
collection PROPFIND exactly as `withCTag` splices `getctag`. Cheap, and makes
`meta.color` mean something. A client's own colour choice would still revert.
- **Accept the value per user**: TKT-LD2D33.
- **Reject `meta.color` at config load** until one of the above lands, so the
config does not promise what it cannot deliver.
