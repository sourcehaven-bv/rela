---
id: IMPL-S6IND8
type: implementation-checklist
title: 'Implementation: Timed calendar-feed events from datetime sources (DTSTART with time, datetime start/end range)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (calfeed ical+json timed, feed_provider datetime, validate_feeds accept+mismatch)
- [x] Integration tests written (feed_handler ICS e2e for a datetime feed)
- [x] Happy path implemented (datetime source → timed DTSTART; date source → all-day VALUE=DATE)
- [x] Edge cases from planning handled (timed range DTEND verbatim; midnight-UTC stays timed by TYPE; same-kind mismatch rejected)
- [x] Error handling in place (mismatch → load error; unparseable value → event skipped, unchanged)

## Test Quality

- [x] Using fixture builders or factories for test data (dayTime helper; existing feedMetamodel/fakeSource extended)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (running rela-server + curl)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Live end-to-end against a running `rela-server` with a datetime feed source:
- **datetime source → timed**: `GET /_feeds/events.ics` returns `DTSTART:20260715T093000Z` and `DTSTART:20260713T090000Z` (time-of-day present, NO `VALUE=DATE`). ✓ (AC3, AC7)
- **JSON timed shape (RR-CT338H)**: `.json` returns `allDay=false`, `start` = RFC3339 instant, `date` **empty** — backward-compatible (existing date-only consumers unaffected). ✓ (AC5)
- **date source still all-day**: a `date`-typed `due_on` source → `DTSTART;VALUE=DATE:20260801` (byte-identical to before; ETag stable). ✓ (AC3, no regression)

Automated:
- AC1 datetime `date:` accepted — `validate_feeds_test.go` valid case. ✓
- AC2 start/end kind mismatch rejected (both directions) — `validate_feeds_test.go`. ✓
- AC3/AC4 timed DTSTART + timed-range DTEND (verbatim) — `ical_test.go` TestRenderEvent_Timed / _TimedRange; all-day test kept unchanged. ✓
- AC5 JSON timed round-trip — `json_test.go` TestRenderJSON_TimedEvent (+ all-day test asserts start/end empty). ✓
- AC6 provider datetime source → Timed=true, Start/End carry time-of-day — `feed_provider_test.go` TestDeclarativeFeed_DatetimeSourceIsTimed. ✓
- AC7 handler ICS e2e — `feed_handler_test.go` TestFeedHandler_ICS_TimedEvent. ✓

Gates: `go test ./...` all pass; `just arch-lint` clean; `just docs` regenerated
+ idempotent.

## Quality

- [x] Code follows project patterns (Timed bool mirrors the all-day/timed split; isFeedDateType helper matches the gate style; branch changes only format)
- [x] Checked for DRY opportunities (isFeedDateType helper used at both gates; reused formatDateTimeUTC; no premature abstraction)
- [x] No security issues introduced (a formatted timestamp can't carry CRLF/injection; Phase 1 guards untouched; no new I/O or trust boundary)
- [x] No silent failures (mismatch surfaced as a load error; existing skip-bad-data policy preserved)
- [x] No debug code left behind

**Design-review resolutions applied:** Timed bool (RR-NZ2I90, not AllDay bool —
zero-value all-day, no existing literal/ETag change); End verbatim/exclusive
uniform (RR-JC3XMY); both gates relaxed + mismatch names props+types
(RR-1KKN1N); tests added-from-zero, all-day test kept separate (RR-23YS91); JSON
start/end separate fields (RR-CT338H); dayTime helper (RR-W2JQKA).
