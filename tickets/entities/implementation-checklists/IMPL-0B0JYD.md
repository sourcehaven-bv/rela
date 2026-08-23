---
id: IMPL-0B0JYD
type: implementation-checklist
title: 'Implementation: Calendar views (month + week) for data-entry'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Backend: `internal/dataentryconfig` (Calendar/CalendarSource/CalendarEvent
structs, `validate_calendars.go`), sidebar arm in `views_handler.go`, wire type,
OpenAPI, Lua URL helper, schema analyze + cleanup arms, and the `compareValues`
datetime fix in `internal/dataentry/helpers.go`.

Frontend: `utils/calendarGrid.ts` (pure date math),
`composables/useCalendarEvents.ts`, `views/CalendarView.vue`,
`components/calendar/{CalendarGrid,CalendarEventChip}.vue`, plus the config
types, store slot, route and icon.

Error paths: a drag whose stored date cannot be parsed abandons the write and
raises a toast rather than sending a patch built from an unreadable value; a
server rejection rolls the optimistic update back and surfaces the message; a
source hitting the page cap renders a banner instead of looking merely quiet.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Go tests use table-driven subtests with a shared `calendarMetamodel()` fixture
and a `validCalendar()` builder, so each case introduces exactly one defect.
Frontend tests use `task()` / `meeting()` factories and a `setup()` harness
taking only the fields a case cares about.

Two tests were verified to FAIL against the pre-fix code rather than merely
passing against the new code:

- `TestCompareValues_Datetime` — 7 subtests fail when `compareValues` is
reverted to the date-only layout.
- `TestCleanup_CalendarBothArms` — fails when the `remove_calendar` apply arm
is removed, which is the silent half of the two-arm hazard.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Ran a real project (`/tmp/caltest`) against a freshly built `rela-server` with
two calendars — one single-source, one mixing `task` and `meeting` — and seven
seeded entities including a five-day span and a 23:30 event on the last day of
the month. Browser-driven verification in Chrome.

| AC | Evidence |
| --- | --- |
| 1 | `/calendar/schedule` and `/calendar/mixed` render; `/api/v1/_sidebar` returns both with `href: /calendar/<id>`, `icon: calendar` |
| 2 | Month grid renders Mon-first with correct spill days; week view shows 7 days labelled "17 aug – 23 aug" |
| 3 | The `mixed` calendar merges `task` (blue) and `meeting` (violet) chips onto one grid |
| 4 | All-day tasks render without a time; timed meetings render "11:00" and "01:30" |
| 5 | Chips are clickable and route to the entity / `edit_form` |
| 6 | **Dragged "Task 4" 24 Aug → 27 Aug: `due_date` written as `2026-08-27`.** Dragged "Long sprint" two days right: `due_date` 2026-08-10 → 2026-08-12 AND `due_end` 2026-08-14 → 2026-08-16 in one write — the four-day span preserved exactly |
| 7 | Covered by `CalendarView.drag.test.ts`: `_actions.update: false` renders `draggable="false"` and a forced drop issues no write |
| 8 | Verified against the live binary: a `string` date property, a bad `default_view`, and an unknown `end_date` each fail `rela validate` with a message naming the calendar and source index |
| 9 | Rows come from the generic collection endpoint, so the existing read gate applies; pinned by `TestSidebar_CalendarHiddenWithoutPermission` |
| 10 | A nav entry with an unheld `permission:` is absent from the sidebar |
| 12 | **The headline case.** `GET /api/v1/meetings?filter[starts_at][gte]=…&[lt]=…` returns both meetings, including "Late review" at `2026-08-31T23:30:00Z`. Demonstrated that the pre-fix comparison EXCLUDED both against a bare-date bound, and mis-ordered offset-bearing values as strings |
| 13 | "Late review" (23:30 on the window's last day) renders — the half-open bound case |
| 14 | Table-tested in `calendarGrid.test.ts` (+14/−11 zones) and in `CalendarView.test.ts` via mounted renders under two timezones |
| 15 | Navigating month → next rewrites `?date=` and refetches the new window |

Edge cases exercised live: a five-day event renders in all five cells; an event
crossing local midnight (23:30Z) correctly lands on the following day in
Amsterdam (+02:00) at 01:30; the empty state shows "No events in this period"
rather than a spinner; no console errors during any interaction.

One thing checked and found NOT to be a defect: week-view cells looked collapsed
in a screenshot, but reading the computed layout showed 320px cells in a 351px
grid — the browser was serving a stale bundle. The speculative CSS "fix" was
reverted rather than left in.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — the calendar validator deliberately
mirrors `validate_feeds.go` rather than extracting a shared helper, matching the
ticket decision to keep the two config surfaces independent; `KanbanCardField`
IS reused for chip fields because that type is already exactly right
- [x] No security issues introduced — no new endpoint, so the calendar inherits
the existing ACL read gate, field redaction and neighbour visibility; a request
may only name a configured calendar, and the drag write goes through the normal
entity write path
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

`just arch-lint` clean, `golangci-lint run ./internal/...` reports 0 issues, `go
test ./internal/...` all pass, frontend 1901 tests / 116 files pass, `vue-tsc`
clean, `eslint` 0 errors.
