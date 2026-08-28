---
id: IMPL-24MNKJ
type: implementation-checklist
title: 'Implementation: Docs describe weekday schedules as ISO-week-change; code fires on target-weekday-passed'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (`TestScheduleIsDue_weekday_notISOWeekBased` in `internal/scheduler/config_test.go` pins both divergence rows from the bug report)
- [x] ~~Integration tests written~~ (N/A: docs-only fix; no behavior change)
- [x] Happy path implemented (guide bullet corrected in `docs-project/entities/guides/GUIDE-scheduled-tasks.md`; `docs/scheduled-tasks.md` regenerated via `scripts/generate-docs.sh`, not hand-edited)
- [x] Edge cases from planning handled (both directions of the divergence pinned: due within same ISO week, not due across ISO week boundary)
- [x] ~~Error handling in place~~ (N/A: no production code changed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: two literal time.Date rows are the fixture — they ARE the documented cases)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (`go test ./internal/scheduler/` green; `grep "ISO week" docs/` returns nothing after regeneration)
- [x] Each acceptance criterion verified with test scenario from planning (doc bullet now reads "missed if the target weekday has occurred since the last run", matching `IsDue`'s weekdayKind branch)
- [x] Edge cases manually verified (case A and case B both asserted against the real `IsDue`)

**Verification Evidence:** `go test ./internal/scheduler/` → ok. Generated doc
and guide both now describe target-weekday-passed semantics; no ISO-week wording
remains in either file.

## Quality

- [x] Code follows project patterns (table-driven test with `t.Run` subtests per project test rules)
- [x] Checked for DRY opportunities (none: new test complements existing per-weekday tests without duplicating them — case A overlaps `TestScheduleIsDue_weekday_friday` deliberately, as the pinned pair must live together)
- [x] No security issues introduced
- [x] No silent failures
- [x] No debug code left behind
