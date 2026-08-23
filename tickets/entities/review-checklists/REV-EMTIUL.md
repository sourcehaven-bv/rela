---
id: REV-EMTIUL
type: review-checklist
title: 'Review: Calendar views (month + week) for data-entry'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — `go test ./internal/...` clean; frontend
1907 tests across 116 files
- [x] Lint clean (`just lint`) — `golangci-lint run ./internal/...` reports
0 issues; `eslint` 0 errors; `just arch-lint` OK
- [x] Comment lint gate clean (`just comment-lint`) — no findings across 10239
comments
- [x] Coverage maintained (`just coverage-check`)

**Comment findings.** `just comment-report` flagged one finding this diff
introduced: `validateCalendars`' doc restated what the two per-check functions
already said (38% shared across three sites). Fixed by pointing at
`[validateCalendarShell]` / `[validateCalendarSource]` and keeping only the fact
neither states — why errors surface at load rather than on first drag. Zero
findings in the new files now; the backlog did not grow.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-5411LZ (critical), RR-ZC4KC1 (critical), RR-SFYA4S
(significant), RR-2S0XIP (significant), RR-PIAQY2 (significant), RR-VHCGRL
(minor). All `addressed`.

The two criticals are worth naming, because both were silent:

- **RR-5411LZ** — `where:` was validated at config load and then never applied.
A calendar declaring `where: ["status != done"]` showed done tasks, with
validation confirming the clause was understood. Now pushed into the same
request as the date window so the SERVER evaluates it against the raw entity,
which also preserves the ordering that keeps membership principal-independent.
- **RR-ZC4KC1** — `eventsByDay` walked from an event's start to its end with no
upper bound, defended by a comment asserting a safety property that did not
hold. One far-future end date meant millions of iterations on the main thread.

Three findings were self-found rather than reported: the `UnixNano` overflow
(introduced by this change), the refetch race, and the `eventsByDay` bound
(found while reading the loop the reviewer had also queued).

Two review passes ran; the first stalled after confirming its findings, and the
second was scoped to what remained.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** all PASS. Full evidence table in IMPL-0B0JYD; the
load-bearing ones re-verified against a live server after the review fixes:

| AC | Status | Evidence |
| --- | --- | --- |
| 1-5 | PASS | Both calendars render and are reachable; month and week grids correct; sources merge; all-day and timed both render |
| 6 | PASS | Dragged a task 24 Aug → 27 Aug; dragged a five-day event two days right and BOTH `due_date` and `due_end` moved, span intact |
| 7 | PASS | `draggable="false"` plus a defence-in-depth refusal in the drop handler |
| 8 | PASS | Bad type, bad `default_view`, unknown `end_date` each fail `rela validate` naming calendar and source index |
| 9-10 | PASS | Rows come from the ACL-gated collection endpoint; nav `permission:` hides the entry |
| 11 | PASS | `where:` now evaluated server-side against the raw entity |
| 12 | PASS | Datetime source returns both meetings including 23:30 on the window's last day; demonstrated the pre-fix code excluded them |
| 13-15 | PASS | Half-open bound, day-assignment by display timezone, period navigation |

**Post-review re-verification:** marked a task `done` and confirmed it
disappeared from the `where`-filtered calendar while remaining on the unfiltered
one — the behaviour that was silently broken.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-MGWZ32

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- pending -->
