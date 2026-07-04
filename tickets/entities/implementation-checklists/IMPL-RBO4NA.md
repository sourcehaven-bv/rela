---
id: IMPL-RBO4NA
type: implementation-checklist
title: 'Implementation: Phase 1: declarative calendar/feed export — internal/feed serializer, feeds: config (multi-source), ICS+JSON HTTP endpoint'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built the branch's `rela-server` and ran it against a throwaway copy of the real
`pim.rela` project (84 due-dated tasks). Subscribed the served feed in Apple
Calendar and confirmed:

- `GET /api/v1/_feeds/tasks.ics` → 200 `text/calendar`, valid VCALENDAR (CRLF,
VERSION/PRODID), 16 VEVENTs (open, due-dated tasks — the `status != done` filter
correctly excluded completed ones from the 84).
- Each event: `DTSTART;VALUE=DATE` all-day, real task title as SUMMARY, stable
`UID:task-<id>@rela`, `VALARM` with `TRIGGER:-PT9H`.
- **Absolute** deep link `URL:http://127.0.0.1:8199/entity/task/<id>` (fixed a
relative-URL bug found during this manual test).
- `rrule: "FREQ=DAILY"` → every event carries `RRULE:FREQ=DAILY` (overdue-visibility).
- `GET …/tasks.json` → 200 JSON with the same 16 events, `allDay: true`.
- The real `pim.rela` feed config validates against its actual metamodel.

Automated coverage backs each acceptance criterion: serializer correctness
(fold/escape/CRLF/UID/DTSTAMP/VALUE=DATE/VALARM, multibyte fold, ETag stability,
collection=Σ RenderEvent), config validation (metamodel checks, rrule
syntax-disambiguation, end_date), synthesizer mapping (filter, multi-source
merge, cursor, rrule literal+property), and full-router handler tests (ACL
scoping, CSRF exemption, both formats, 404s, empty feed).

## Quality

- [x] Code follows project patterns (mirrors `tracer` leaf package, `validateLists`,
`apps_handler` base-URL derivation, existing route/handler conventions)
- [x] Checked for DRY opportunities — shared `event_for`-style mapping, one
serializer for both ICS paths, syntax-disambiguation helper reused by validation
and synthesis
- [x] No security issues introduced (ACL read gate applied; CSRF exemption
conditioned on isCSRFExempt; no new secret store; feed writes denied)
- [x] No silent failures (config errors fail-fast at load; a single malformed
entity date skips that event by design, consistent with rela's
tolerate-invalid-data policy)
- [x] No debug code left behind
